// coverage-truth-check.mjs — independent dual-source re-count of the
// coverage truthfulness matrix. The matrix (machine JSON + human docs
// table) is checked against SHIPPED SOURCE CODE — platform scan files,
// collector wiring, capability vocabulary, schema contract — never
// against itself. Hollow cells (unresolvable evidence pointers), ghost
// tiers, docs/json drift, marketing-equality phrasing, a second plane
// token dialect, a counter census that moves without upgrading the
// matrix, and any reach of the matrix into the decision plane all fail
// the gate.
//
//   usage: node scripts/coverage-truth-check.mjs [--selftest]
import fs from "node:fs";
import path from "node:path";

const TIERS = ["FULL", "LIMITED", "MONITOR ONLY", "UNAVAILABLE"];
const PLATFORMS = ["linux", "darwin", "windows"];
const SPEC_CLASSES = ["agent", "os", "tool", "mcp", "capability"];
const PLANE_TOKEN = "none-in-observation-phase";
const FORBIDDEN_PHRASES = [
  "all platforms are equally protected",
  "identical protection",
  "uniform protection",
  "equal protection across platforms",
  "same protection on all platforms",
  "protected the same way on every platform",
  "所有平台完全一样",
  "完全一样的保护",
];
const DECISION_DIRS = [
  "internal/policy", "internal/rules", "internal/bus", "internal/auditlog",
  "cmd/agent-collector", "cmd/hello-collector",
];

const fail = (m) => { console.error("TRUTH RED: " + m); throw new Error(m); };

function readIf(root, rel) {
  try { return fs.readFileSync(path.join(root, rel), "utf8"); } catch { return null; }
}

function listGo(root, relDir) {
  const d = path.join(root, relDir);
  if (!fs.existsSync(d)) return [];
  return fs.readdirSync(d).filter((f) => f.endsWith(".go"))
    .map((f) => path.posix.join(relDir, f));
}

function load(root) {
  const jrel = "testdata/golden/coverage-truthfulness.json";
  const drel = "docs/coverage-truthfulness.md";
  const json = JSON.parse(readIf(root, jrel) ?? failMissing(jrel));
  const docs = readIf(root, drel) ?? failMissing(drel);
  const schemaV2 = readIf(root, "docs/schema-v2.md") ?? failMissing("docs/schema-v2.md");
  const files = new Map();
  const wantEvidence = (o) => {
    if (!o || typeof o !== "object") return;
    if (Array.isArray(o)) return o.forEach(wantEvidence);
    if (typeof o.file === "string") files.set(o.file, readIf(root, o.file));
    for (const k of Object.keys(o)) wantEvidence(o[k]);
  };
  wantEvidence(json.rows); wantEvidence(json.unsupported_goos);
  files.set("internal/discovery/discovery.go", readIf(root, "internal/discovery/discovery.go"));
  files.set("cmd/agent-collector/run.go", readIf(root, "cmd/agent-collector/run.go"));
  const decision = [];
  for (const d of DECISION_DIRS) for (const f of listGo(root, d)) decision.push([f, readIf(root, f)]);
  // non-test shipped go files for the counter census
  const prod = [];
  for (const d of ["internal", "cmd"]) {
    const walk = (dir) => {
      for (const e of fs.readdirSync(path.join(root, dir), { withFileTypes: true })) {
        const p = dir + "/" + e.name;
        if (e.isDirectory()) walk(p);
        else if (e.name.endsWith(".go") && !e.name.endsWith("_test.go")) prod.push([p, readIf(root, p)]);
      }
    };
    if (fs.existsSync(path.join(root, d))) walk(d);
  }
  return { json, docs, schemaV2, files, decision, prod };
}
function failMissing(p) { throw new Error("missing tracked file: " + p); }

function parseMatrixTable(docs) {
  const rows = [];
  const lines = docs.split("\n");
  let inMatrix = false;
  for (const l of lines) {
    if (/^## The matrix\s*$/.test(l)) { inMatrix = true; continue; }
    if (inMatrix && /^##\s/.test(l)) break;
    if (!inMatrix || !l.startsWith("|")) continue;
    const c = l.split("|").map((s) => s.trim());
    if (c.length < 8 || c[1] === "surface" || /^[-]+$/.test(c[2])) continue;
    rows.push({ id: c[1].replace(/`/g, ""), tiers: [c[3], c[4], c[5]] });
  }
  return rows;
}

function check(b) {
  const { json, docs } = b;
  // closed vocabularies
  if (json.tiers.join("|") !== TIERS.join("|")) fail("tier vocabulary drifted from the closed four-tier set");
  if (json.platforms.join("|") !== PLATFORMS.join("|")) fail("platform list drifted");
  if (json.enforcement_plane_token !== PLANE_TOKEN) fail("matrix must carry the one shipped plane token spelling");
  // rows: shape, classes, ghost classes
  const seenClass = new Set();
  const ids = [];
  for (const r of json.rows) {
    ids.push(r.id);
    if (!SPEC_CLASSES.includes(r.class)) fail("row " + r.id + " names a ghost class " + r.class);
    seenClass.add(r.class);
    const keys = Object.keys(r.cells);
    if (keys.join("|") !== PLATFORMS.join("|")) fail("row " + r.id + " cell set != platforms");
    for (const [pf, cell] of Object.entries(r.cells)) {
      if (!TIERS.includes(cell.tier)) fail("row " + r.id + "/" + pf + " tier not in closed set: " + cell.tier);
      if (!Array.isArray(cell.evidence) || cell.evidence.length === 0)
        fail("hollow cell without evidence: " + r.id + "/" + pf);
    }
  }
  for (const c of SPEC_CLASSES) if (!seenClass.has(c)) fail("spec class " + c + " has no row — the matrix may not silently drop a covered class");
  // evidence pointers must resolve in the shipped tree (no hollow anchors)
  const resolve = (ev, where) => {
    for (const e of ev) {
      const t = b.files.get(e.file) ?? (() => { throw new Error("evidence file not loaded: " + e.file + " (" + where + ")"); })();
      if (t === null) fail("evidence file missing from tree: " + e.file + " (" + where + ")");
      if (!t.includes(e.token)) fail("evidence pointer does not resolve in " + e.file + ": " + JSON.stringify(e.token) + " (" + where + ")");
    }
  };
  if (!json.unsupported_goos.evidence?.length) fail("unsupported-GOOS statement has no evidence");
  resolve(json.unsupported_goos.evidence, "unsupported_goos");
  for (const r of json.rows) {
    if (r.shared_evidence?.length) resolve(r.shared_evidence, r.id + "/shared");
    for (const [pf, cell] of Object.entries(r.cells)) resolve(cell.evidence, r.id + "/" + pf);
  }
  // counters wired in the collector, census for the declared-unset seed
  const run = b.files.get("cmd/agent-collector/run.go");
  const disc = b.files.get("internal/discovery/discovery.go");
  for (const [name, c] of Object.entries(json.counters)) {
    if (c.populated) {
      if (!c.event_attr || !run.includes('"' + c.event_attr + '"')) fail("counter " + name + " claims populated but attr key is not wired in the collector");
      const field = c.source_field.split(".").pop();
      if (!disc.includes(field)) fail("counter " + name + " source field not found in shipped stats: " + c.source_field);
    } else {
      if (c.event_attr !== null) fail("counter " + name + " is declared-unset but names an event attribute");
      const field = c.source_field.split(".").pop();
      const occ = b.prod.reduce((n, [f, t]) => n + ((t || "").split(field).length - 1), 0);
      if (occ !== c.production_occurrence_expectation)
        fail("counter census moved for " + name + ": " + occ + " non-test occurrences, matrix expects " + c.production_occurrence_expectation + " — upgrade the matrix in the same PR, never silently");
    }
  }
  // docs/json lockstep: same rows, same order, same tiers
  const table = parseMatrixTable(docs);
  if (table.map((r) => r.id).join("|") !== ids.join("|")) fail("docs table rows differ from the machine form (set, order, or ghost rows)");
  table.forEach((tr, i) => {
    const r = json.rows[i];
    PLATFORMS.forEach((pf, k) => {
      if (tr.tiers[k] !== r.cells[pf].tier) fail("lockstep drift at " + tr.id + "/" + pf + ": docs says " + tr.tiers[k] + ", machine form says " + r.cells[pf].tier);
    });
  });
  // marketing-equality ban over both human and machine carriers
  const carriers = docs + "\n" + JSON.stringify(json);
  const low = carriers.toLowerCase();
  for (const p of FORBIDDEN_PHRASES) if (low.includes(p)) fail("forbidden uniform-claim phrasing present: " + p);
  // one plane token dialect only (E-free spelling check across all carriers)
  const s36 = b.schemaV2.slice(b.schemaV2.search(/^## 36\./m));
  for (const m of carriers.matchAll(/\bnone-in-[a-z0-9-]+\b/g)) if (m[0] !== PLANE_TOKEN) fail("second plane-token dialect: " + m[0]);
  for (const m of s36.matchAll(/\bnone-in-[a-z0-9-]+\b/g)) if (m[0] !== PLANE_TOKEN) fail("second plane-token dialect in the schema section: " + m[0]);
  if (!docs.includes(PLANE_TOKEN)) fail("human table must carry the enforcement-plane token");
  // schema contract section present and substantive
  if (!/^## 36\. Coverage truthfulness matrix \(slice W11\.2\)/m.test(b.schemaV2)) fail("schema contract section 36 heading missing");
  for (const t of TIERS) if (!s36.includes(t)) fail("section 36 does not name the tier " + t);
  if (!s36.includes("declared-unset") || !s36.includes("lockstep")) fail("section 36 must carry the counter-census and lockstep obligations");
  // zero reach into the decision plane (Phase 0 red line)
  for (const [f, t] of b.decision) if ((t || "").includes("coverage-truthfulness"))
    fail("matrix name reached a decision-plane or collector file: " + f);
}

function clone(o) { return JSON.parse(JSON.stringify(o)); }

function expectRed(label, mutate) {
  const b = cloneObject(load(process.cwd()));
  mutate(b);
  let red = false;
  try { check(b); } catch { red = true; }
  if (!red) fail("selftest control did not fire: " + label);
  return true;
}
function cloneObject(b) {
  return {
    json: clone(b.json), docs: b.docs, schemaV2: b.schemaV2,
    files: new Map(b.files), decision: b.decision.map((x) => [...x]),
    prod: b.prod.map((x) => [...x]),
  };
}

function selftest() {
  const controls = [
    ["ghost tier", (b) => { b.json.rows[0].cells.linux.tier = "SOLID"; }],
    ["hollow cell", (b) => { b.json.rows[1].cells.darwin.evidence = []; }],
    ["pointer does not resolve", (b) => { b.json.rows[2].cells.windows.evidence = [{ file: "internal/discovery/scan_windows.go", token: "this token exists nowhere in the tree" }]; }],
    ["docs tier drift", (b) => { b.docs = b.docs.replace("| `os.process-snapshot` | os | FULL", "| `os.process-snapshot` | os | LIMITED"); }],
    ["docs ghost row", (b) => { b.docs = b.docs.replace("| `os.cmdline-visibility`", "| `os.ghost-surface`"); }],
    ["counter silently populated", (b) => { b.prod.forEach((p) => { p[1] = (p[1] || "") + "\n// NoCmdlineAt NoCmdlineAt NoCmdlineAt"; }); }],
    ["declared-unset names an attr", (b) => { b.json.counters.no_cmdline_at.event_attr = "no_cmdline_at"; }],
    ["populated counter unwired", (b) => { b.files.set("cmd/agent-collector/run.go", b.files.get("cmd/agent-collector/run.go").replace('"no_cmdline"', '"not_the_attr"')); }],
    ["marketing-equality phrasing", (b) => { b.docs += "\nAll platforms are equally protected, always.\n"; }],
    ["second plane dialect", (b) => { b.docs += "\nsee none-in-observation-phase-v2\n"; }],
    ["class silently dropped", (b) => { b.json.rows = b.json.rows.filter((r) => r.class !== "capability"); }],
    ["decision-plane reach", (b) => { b.decision.push(["internal/policy/evil.go", "// coverage-truthfulness leaked into the evaluator"]); }],
    ["schema section gutted", (b) => { b.schemaV2 = b.schemaV2.replace(/^## 36\..*$/m, "## 36. placeholder"); }],
  ];
  let n = 0;
  for (const [label, m] of controls) { expectRed(label, m); n++; }
  const clean = load(process.cwd());
  check(clean); // negative control: the real tree must stay green
  console.log("SELFTEST GREEN (" + n + " positive controls fired, real tree green)");
}

function main() {
  const root = path.join(import.meta.dirname, "..");
  if (process.argv.includes("--selftest")) {
    process.chdir(root);
    selftest();
    return;
  }
  check(load(root));
  console.log("TRUTH GREEN — coverage matrix lockstep with shipped source");
}
main();
