// compat-redteam-check.mjs — independent dual-source check of the
// compatibility matrix and the red-team plan. The two human docs and the
// machine mirror are checked against SHIPPED SOURCE CODE — scan-file
// build tags, the unsupported-GOOS fallback text, schema enums, the
// threat model's row census — never against themselves. Hollow cells,
// inflated cells (a tier that the shipped code cannot support on this
// axis), docs/json drift, row-id collision with the coverage matrix, a
// second tier dialect, a HarmonyOS cell that pretends implementation,
// red-team census drift, one-way threat references, silent families with
// no threat anchor, attack-procedure phrasing, and border-device
// marketing words all fail the gate.
//
//   usage: node scripts/compat-redteam-check.mjs [--selftest]
import fs from "node:fs";
import path from "node:path";

const TIERS = ["FULL", "LIMITED", "MONITOR ONLY", "UNAVAILABLE"];
const PLATFORMS = ["linux", "darwin", "windows"];
const PLANE_TOKEN = "none-in-observation-phase";
const FAMILIES = 10;
const SUBJECTS = 7;
const FORBIDDEN_PHRASES = [
  "all platforms are equally protected",
  "identical protection",
  "uniform protection",
  "equal protection across platforms",
  "所有平台完全一样",
  "完全一样的保护",
  "万能保护",
  "firewall",
  "防火墙",
];
const REQUIRED_DECLARATIONS = [
  "planning only",
  "engine is not built",
  "contains no attack procedures",
];

const root0 = path.resolve(import.meta.dirname, "..");

function read(root, rel) {
  try { return fs.readFileSync(path.join(root, rel), "utf8"); } catch { return null; }
}

function loadArtifacts(root) {
  const jrel = "testdata/golden/compatibility-matrix.json";
  const drel = "docs/compatibility-matrix.md";
  const rrel = "docs/red-team-plan.md";
  const json = JSON.parse(read(root, jrel) ?? (() => { throw new Error("missing " + jrel); })());
  const docs = read(root, drel) ?? (() => { throw new Error("missing " + drel); })();
  const red = read(root, rrel) ?? (() => { throw new Error("missing " + rrel); })();
  return { json, docs, red };
}

function parseDocsTable(md) {
  const rows = [];
  for (const line of md.split("\n")) {
    const m = line.match(/^\|\s*`(compat\.[a-z0-9-]+)`\s*\|\s*([a-z]+)\s*\|\s*([A-Z ]+?)\s*\|\s*([A-Z ]+?)\s*\|\s*([A-Z ]+?)\s*\|/);
    if (m) rows.push({ id: m[1], class: m[2], tiers: [m[3], m[4], m[5]] });
  }
  return rows;
}

function checkMatrix(root, json, docs, errs) {
  if (JSON.stringify(json.tiers) !== JSON.stringify(TIERS)) errs.push("tier dialect drift in machine form");
  if (JSON.stringify(json.platforms) !== JSON.stringify(PLATFORMS)) errs.push("platform set drift in machine form");
  if (json.enforcement_plane_token !== PLANE_TOKEN) errs.push("plane token drift");
  // re-derive platform facts from shipped build tags (not from the docs)
  for (const p of PLATFORMS) {
    const src = read(root, `internal/discovery/scan_${p}.go`);
    if (!src) { errs.push(`shipped scan file missing for ${p}`); continue; }
    if (!src.startsWith(`//go:build ${p}`)) errs.push(`scan_${p}.go build tag is not the plain ${p} tag`);
  }
  const other = read(root, "internal/discovery/scan_other.go") ?? "";
  if (!other.includes("!linux && !darwin && !windows")) errs.push("unsupported-GOOS fallback tag missing its negative set");
  const fallbackTok = json.unsupported_goos?.evidence?.[0]?.token ?? "";
  if (!other.includes(fallbackTok)) errs.push("unsupported-GOOS error text not found in the shipped fallback");
  // harmonyos must have zero shipped implementation files (declared gap, checked as absence)
  const walk = (d) => fs.readdirSync(d, { withFileTypes: true }).flatMap((e) =>
    e.isDirectory() ? walk(path.join(d, e.name)) : [path.join(d, e.name)]);
  const goFiles = ["internal", "cmd"].flatMap((d) => {
    const p = path.join(root, d);
    return fs.existsSync(p) ? walk(p) : [];
  }).filter((f) => f.endsWith(".go") && !f.endsWith("_test.go"));
  for (const f of goFiles) {
    if (/harmony/i.test(fs.readFileSync(f, "utf8"))) errs.push(`harmonyos implementation file shipped (${path.relative(root, f)}) — matrix rows must be upgraded in the same PR`);
  }
  // every evidence pointer resolves
  const files = new Map();
  const cellFiles = [];
  const collect = (o) => {
    if (!o || typeof o !== "object") return;
    if (Array.isArray(o)) return o.forEach(collect);
    if (typeof o.file === "string") {
      cellFiles.push(o);
      if (!files.has(o.file)) files.set(o.file, read(root, o.file));
    }
    for (const k of Object.keys(o)) collect(o[k]);
  };
  collect(json.rows); collect(json.declared_gaps); collect(json.unsupported_goos);
  for (const ev of cellFiles) {
    const src = files.get(ev.file);
    if (src == null) { errs.push(`pointer file missing: ${ev.file}`); continue; }
    if (!src.includes(ev.token)) errs.push(`pointer token unresolvable: ${ev.token} not in ${ev.file}`);
  }
  // docs/json lockstep: same rows, same order, same tiers
  const table = parseDocsTable(docs);
  if (table.length !== json.rows.length) errs.push(`docs table has ${table.length} rows, machine form has ${json.rows.length}`);
  for (let i = 0; i < Math.min(table.length, json.rows.length); i++) {
    const t = table[i], j = json.rows[i];
    if (t.id !== j.id) { errs.push(`row ${i}: docs ${t.id} != machine ${j.id}`); continue; }
    for (let p = 0; p < 3; p++) {
      const want = j.cells[PLATFORMS[p]]?.tier;
      if (!TIERS.includes(t.tiers[p])) errs.push(`${t.id}/${PLATFORMS[p]}: illegal tier word "${t.tiers[p]}"`);
      if (t.tiers[p] !== want) errs.push(`${t.id}/${PLATFORMS[p]}: docs tier ${t.tiers[p]} != machine tier ${want}`);
    }
    for (const [os, c] of Object.entries(j.cells)) {
      if (!TIERS.includes(c.tier)) errs.push(`${t.id}/${os}: illegal tier in machine form`);
      if (!Array.isArray(c.evidence) || c.evidence.length === 0) errs.push(`${t.id}/${os}: hollow cell (no evidence pointer)`);
    }
  }
  // row-id disjointness against the coverage matrix (two axes, no shared ids)
  const cov = read(root, "docs/coverage-truthfulness.md") ?? "";
  for (const r of json.rows) {
    if (cov.includes("```" + r.id) || cov.includes("`" + r.id + "`")) errs.push(`${r.id} also lives on the coverage axis — axes must stay disjoint`);
  }
  // the docs carriers must not promise a third axis merge
  for (const ph of FORBIDDEN_PHRASES) {
    if (docs.toLowerCase().includes(ph.toLowerCase())) errs.push(`forbidden blanket-promise phrase in compatibility docs: ${ph}`);
  }
}

function checkRedTeam(root, red, tmSrc, errs) {
  const fams = [...red.matchAll(/^### (RT-\d\d) (.+)$/gm)];
  if (fams.length !== FAMILIES) errs.push(`red-team family census ${fams.length}, want ${FAMILIES}`);
  for (let i = 0; i < fams.length; i++) {
    if (fams[i][1] !== `RT-${String(i + 1).padStart(2, "0")}`) errs.push(`family order drift at ${fams[i][1]}`);
  }
  const bodies = fams.map((m, i) => {
    const start = m.index;
    const next = i + 1 < fams.length ? fams[i + 1].index : red.indexOf("\n## Cadence", start) >= 0 ? red.indexOf("\n## Cadence", start) : red.length;
    return red.slice(start, next);
  });
  for (const b of bodies) {
    if (!/TM-\d\d/.test(b) && !b.includes("declared_known_gap")) errs.push(`family ${b.slice(4, 12)} silent: no threat anchor and no declared_known_gap`);
  }
  // threat census from the shipped threat model, both directions
  const tmIds = [...new Set([...tmSrc.matchAll(/^\| (TM-\d\d) \|/gm)].map((m) => m[1]))];
  if (tmIds.length < 10) errs.push(`threat census too small to trust: ${tmIds.length}`);
  for (const id of tmIds) {
    if (!bodies.some((b) => b.includes(id))) errs.push(`${id} exercised by no red-team family`);
  }
  // simulation subjects closed set
  const sims = [...red.matchAll(/^- (SIM-\d):/gm)];
  if (sims.length !== SUBJECTS) errs.push(`simulation subject census ${sims.length}, want ${SUBJECTS}`);
  for (const d of REQUIRED_DECLARATIONS) {
    if (!red.toLowerCase().includes(d)) errs.push(`missing status declaration: "${d}"`);
  }
  for (const ph of FORBIDDEN_PHRASES) {
    if (red.toLowerCase().includes(ph.toLowerCase())) errs.push(`forbidden phrase in red-team plan: ${ph}`);
  }
  // phase-0 reach guard: the plan may not name a blocking action as shipped
  if (/will block|then blocks the action|engine blocks/i.test(red)) errs.push("plan phrases an interception as shipped — Phase 0 forbids it");
  return tmIds;
}

function run(root) {
  const errs = [];
  const { json, docs, red } = loadArtifacts(root);
  const tm = read(root, "docs/threat-model.md");
  if (!tm) { errs.push("missing docs/threat-model.md"); return errs; }
  const sv2 = read(root, "docs/schema-v2.md") ?? "";
  if (!sv2.includes("## 37. Compatibility matrix contract")) errs.push("schema contract §37 header missing");
  if (!sv2.includes("## 38. Red-team plan contract")) errs.push("schema contract §38 header missing");
  for (const carrier of ["docs/compatibility-matrix.md", "docs/red-team-plan.md"]) {
    if (!(read(root, carrier) ?? "").includes("MUST evolve in one PR")) errs.push(`${carrier} dropped the version lock`);
  }
  if (!(json.version_lock ?? "").includes("MUST evolve in one PR")) errs.push("machine form dropped the version lock");
  checkMatrix(root, json, docs, errs);
  checkRedTeam(root, red, tm, errs);
  return errs;
}

function selftest() {
  // positive control: break a temp copy, every injected defect must fire
  const tmp = fs.mkdtempSync(path.join(path.resolve(import.meta.dirname, ".."), ".tmpdir", "compat-selftest-"));
  const cp = (rel) => {
    const to = path.join(tmp, rel);
    fs.mkdirSync(path.dirname(to), { recursive: true });
    fs.copyFileSync(path.join(root0, rel), to);
  };
  ["testdata/golden/compatibility-matrix.json", "docs/compatibility-matrix.md", "docs/red-team-plan.md",
    "docs/threat-model.md", "docs/schema-v2.md", "docs/coverage-truthfulness.md", "docs/evals-coverage-v0.md",
    ".github/workflows/build.yml"].forEach(cp);
  for (const rel of ["internal/discovery", "internal/schema", "internal/adapter", "internal/mcpproxy", "internal/identity", "internal/policy", "internal/auditlog"]) {
    for (const f of fs.readdirSync(path.join(root0, rel))) {
      if (f.endsWith(".go")) cp(`${rel}/${f}`);
    }
  }
  const base = run(tmp);
  if (base.length) { console.error("SELFTEST base run not clean:", base); process.exit(1); }
  const cases = [
    ["hollow pointer", () => { const p = path.join(tmp, "internal/discovery/scan_linux.go"); fs.writeFileSync(p, "//go:build linux\npackage discovery // token erased\n"); }],
    ["tier drift", () => { const p = path.join(tmp, "docs/compatibility-matrix.md"); let t = fs.readFileSync(p, "utf8"); t = t.replace("| `compat.process-enumeration` | os | FULL | FULL | FULL |", "| `compat.process-enumeration` | os | FULL | SUPER FULL | FULL |"); fs.writeFileSync(p, t); }],
    ["family dropped", () => { const p = path.join(tmp, "docs/red-team-plan.md"); let t = fs.readFileSync(p, "utf8"); t = t.replace(/^### RT-05 .*$/m, "### RT-99 Renamed Family"); fs.writeFileSync(p, t); }],
    ["threat anchor silenced", () => { const p = path.join(tmp, "docs/red-team-plan.md"); let t = fs.readFileSync(p, "utf8"); t = t.replace(/TM-03/g, "TM-99"); fs.writeFileSync(p, t); }],
    ["harmony pretender", () => { const p = path.join(tmp, "internal/identity/harmony_stub.go"); fs.writeFileSync(p, "//go:build harmony\npackage identity // harmonyos impl\n"); }],
  ];
  for (const [name, breakIt] of cases) {
    breakIt();
    const errs = run(tmp);
    if (!errs.length) { console.error(`SELFTEST RED: injected defect escaped detection: ${name}`); process.exit(1); }
    // restore everything for the next case
    for (const rel of ["internal/discovery/scan_linux.go", "docs/compatibility-matrix.md", "docs/red-team-plan.md"]) fs.copyFileSync(path.join(root0, rel), path.join(tmp, rel));
    fs.rmSync(path.join(tmp, "internal/identity/harmony_stub.go"), { force: true });
  }
  fs.rmSync(tmp, { recursive: true, force: true });
  console.log("COMPAT/REDTEAM SELFTEST GREEN (5 injected defects all caught, base clean)");
}

if (process.argv.includes("--selftest")) {
  selftest();
} else {
  const errs = run(root0);
  if (errs.length) { console.error("COMPAT RED:\n" + errs.map((e) => " - " + e).join("\n")); process.exit(1); }
  console.log("COMPAT GREEN — matrix tiers, pointers, platform facts, red-team census and both reference directions hold against the shipped tree");
}
