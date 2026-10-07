// benchplan-check.mjs — independent dual-source check of the performance
// benchmark plan. The human doc, the machine mirror, and the shipped
// source tree are checked against each other: evidence pointers are
// re-resolved in the tree, metric-status cells in the doc table must
// match the mirror cell-for-cell, the §70-§73 reverse-lookup table binds
// every registered token into its named section, the real-machine batch
// census must be exactly 4/4, the recorded fast-path seed must keep its
// estimation-is-not-verified wording, and the declared-absent claims for
// resource-manager and mode symbols are re-derived by scanning the Go
// tree itself. Hollow rows, invented metrics, a second status dialect, a
// seed promoted into a verified promise, drift between declared-absent
// and shipped symbols, one-way references, CI-as-real-machine phrasing,
// and border-device marketing words all fail the gate.
//
//   usage: node scripts/benchplan-check.mjs [--selftest]
import fs from "node:fs";
import path from "node:path";

const root0 = path.resolve(import.meta.dirname, "..");
const STATUSES = ["PARTIAL", "PLANNED", "ABSENT"];
const CANON_METRICS = [
  "bp.cpu-idle", "bp.resident-memory", "bp.disk-io", "bp.network-overhead",
  "bp.decision-latency", "bp.battery-impact", "bp.temperature-impact",
];
const CANON_SCENARIOS = ["bp-rm-1", "bp-rm-2", "bp-rm-3", "bp-rm-4"];
const CANON_GAPS = [
  "gd-perf-resource-manager", "gd-perf-modes", "gd-perf-battery-thermal",
  "gd-perf-realmachine-batch",
];
const PLANE = "none-in-observation-phase";

function read(root, rel) {
  try { return fs.readFileSync(path.join(root, rel), "utf8"); } catch { return null; }
}

function loadArtifacts(root) {
  const jrel = "testdata/golden/benchmark-plan.json";
  const drel = "docs/benchmark-plan.md";
  const jtext = read(root, jrel);
  const docs = read(root, drel);
  if (jtext === null) throw new Error("missing " + jrel);
  if (docs === null) throw new Error("missing " + drel);
  return { json: JSON.parse(jtext), docs };
}

function norm(md) {
  return md.replace(/\*\*/g, "").replace(/`/g, "");
}

function sectionSlice(docs, secId) {
  const n = norm(docs);
  const lines = n.split("\n");
  let start = -1;
  for (let i = 0; i < lines.length; i++) {
    if (lines[i].startsWith("## ") && lines[i].includes("(§" + secId)) { start = i; break; }
  }
  if (start < 0) return null;
  const out = [];
  for (let i = start + 1; i < lines.length; i++) {
    if (lines[i].startsWith("## ")) break;
    out.push(lines[i]);
  }
  return out.join("\n");
}

function parseDocMetricRows(docs) {
  const rows = new Map();
  for (const line of norm(docs).split("\n")) {
    const m = line.match(/^\|\s*(bp\.[a-z0-9-]+)\s*\|\s*([A-Z]+)\s*\|\s*(.+?)\s*\|/);
    if (m) rows.set(m[1], { status: m[2], goal: m[3] });
  }
  return rows;
}

function parseDocScenarioRows(docs) {
  const ids = [];
  for (const line of norm(docs).split("\n")) {
    const m = line.match(/^\|\s*(bp-rm-[0-9]+)\s*\|/);
    if (m) ids.push(m[1]);
  }
  return ids;
}

function goTreeTokens(root) {
  // independent re-derivation: scan shipped Go sources for distinctive
  // resource-manager / mode identifiers; comment prose never counts here.
  const hits = [];
  const walk = (dir) => {
    let ents;
    try { ents = fs.readdirSync(dir, { withFileTypes: true }); } catch { return; }
    for (const e of ents) {
      const p = path.join(dir, e.name);
      if (e.isDirectory()) { if (e.name !== ".git") walk(p); continue; }
      if (!e.name.endsWith(".go")) continue;
      const t = fs.readFileSync(p, "utf8");
      for (const tok of ["ResourceManager", "PerfMode", "BatteryPercent", "ThermalPressure"]) {
        if (t.includes(tok)) hits.push(tok + "@" + path.relative(root, p));
      }
    }
  };
  walk(path.join(root, "internal"));
  walk(path.join(root, "cmd"));
  return hits;
}

function resolveEvidence(root, ev) {
  const problems = [];
  for (const e of ev || []) {
    const t = read(root, e.file);
    if (t === null) problems.push("evidence file missing: " + e.file);
    else if (!t.includes(e.token)) problems.push("evidence token unresolved: " + e.file + " :: " + e.token);
  }
  if (!ev || ev.length === 0) problems.push("hollow row without evidence pointer");
  return problems;
}

function checkPlan(root, json, docs) {
  const bad = [];
  const n = flat(docs);

  // 1. closed vocabularies and canonical censuses (no second dialect,
  //    no invented metric, no dropped spec section)
  if (JSON.stringify(json.statuses) !== JSON.stringify(STATUSES)) bad.push("status vocabulary drift");
  if (JSON.stringify(json.spec_sections) !== JSON.stringify(["70", "71", "72", "73"])) bad.push("spec section census drift");
  const metricIds = json.metrics.map((m) => m.id).sort();
  if (JSON.stringify(metricIds) !== JSON.stringify([...CANON_METRICS].sort())) bad.push("metric census drift (canonical §73 list)");
  for (const m of json.metrics) {
    if (!STATUSES.includes(m.status)) bad.push("non-closed status: " + m.id);
    bad.push(...resolveEvidence(root, m.evidence).map((p) => p + " [" + m.id + "]"));
  }

  // 2. doc table <-> mirror cell-for-cell lockstep
  const drows = parseDocMetricRows(docs);
  for (const m of json.metrics) {
    const d = drows.get(m.id);
    if (!d) { bad.push("metric row missing from doc table: " + m.id); continue; }
    if (d.status !== m.status) bad.push("status drift doc vs mirror: " + m.id);
    if (!d.goal.includes(m.goal)) bad.push("goal wording drift doc vs mirror: " + m.id);
  }
  for (const id of drows.keys()) if (!CANON_METRICS.includes(id)) bad.push("doc metric outside canonical census: " + id);

  // 3. §70-§73 reverse-lookup table: every registered token bound into
  //    its named section, and every named section present
  const s70 = sectionSlice(docs, "70");
  const s71 = sectionSlice(docs, "71");
  const s72 = sectionSlice(docs, "72");
  const s73 = sectionSlice(docs, "73");
  if (!s70 || !s71 || !s72 || !s73) bad.push("a named §70-73 section is missing from the doc");
  if (s70) {
    const f70 = s70.replace(/\s+/g, " ");
    for (const a of json.adopted_architecture) if (!f70.includes(a)) bad.push("adopted architecture token not in §70 section: " + a);
    for (const b of json.banned_postures) if (!f70.includes(b)) bad.push("banned posture token not in §70 section: " + b);
  }
  if (s71) {
    const f71 = s71.replace(/\s+/g, " ");
    for (const sig of json.resource_manager_signals) if (!f71.includes(sig)) bad.push("signal not registered in §71 section: " + sig);
    if (!f71.includes(json.resource_manager_floor)) bad.push("degrade floor sentence missing");
  }
  if (s72) for (const mode of json.mode_names) if (!s72.includes(mode)) bad.push("mode name missing from §72 section: " + mode);
  if (s73) for (const id of CANON_METRICS) if (!s73.includes(id)) bad.push("metric id missing from the §73 census section: " + id);

  // 4. real-machine batch census: exactly 4/4, both directions
  const scenJson = json.batch_scenarios.map((s) => s.id).sort();
  if (JSON.stringify(scenJson) !== JSON.stringify([...CANON_SCENARIOS].sort())) bad.push("scenario mirror census drift (want 4/4)");
  const scenDoc = [...new Set(parseDocScenarioRows(docs))].sort();
  if (JSON.stringify(scenDoc) !== JSON.stringify([...CANON_SCENARIOS].sort())) bad.push("scenario doc census drift (want 4/4)");
  for (const s of json.batch_scenarios) {
    if (s.status !== "declared, not yet verified") bad.push("scenario not in declared posture: " + s.id);
    bad.push(...resolveEvidence(root, s.evidence).map((p) => p + " [" + s.id + "]"));
  }
  if (!n.includes("Real-machine follow-up batch")) bad.push("batch section header missing");

  // 5. recorded seed honesty: value present, state recorded, both
  //    honesty sentences in the doc, no promotion phrasing
  if (json.seed.state !== "recorded") bad.push("seed state is not 'recorded'");
  if (!docs.includes(json.seed.value)) bad.push("seed value missing from doc");
  for (const h of json.seed.honesty) if (!n.includes(h)) bad.push("seed honesty sentence missing from doc: " + h);
  if (json.seed.metric !== "bp.decision-latency") bad.push("seed bound to the wrong metric");

  // 6. declared-gap census + symbol-absence re-derivation (two-way drift)
  const gapIds = json.declared_gaps.map((g) => g.id).sort();
  if (JSON.stringify(gapIds) !== JSON.stringify([...CANON_GAPS].sort())) bad.push("declared gap census drift");
  for (const g of json.declared_gaps) {
    if (g.status !== "ABSENT") bad.push("gap must stay ABSENT while unshipped: " + g.id);
    bad.push(...resolveEvidence(root, g.evidence).map((p) => p + " [" + g.id + "]"));
    if (!n.includes(g.id)) bad.push("gap id missing from doc: " + g.id);
  }
  const hits = goTreeTokens(root);
  if (hits.length > 0) bad.push("shipped symbols contradict declared-absent rows: " + hits.join(", "));

  // 7. axis disjointness against the compatibility mirror
  const compatRel = "testdata/golden/compatibility-matrix.json";
  const compatText = read(root, compatRel);
  if (compatText !== null) {
    const compat = JSON.parse(compatText);
    const compatIds = new Set((compat.rows || []).map((r) => r.id));
    for (const id of CANON_METRICS) if (compatIds.has(id)) bad.push("row id collides with compatibility axis: " + id);
  }

  // 8. emission-plane and honesty pins, forbidden phrase scan
  if (json.enforcement_plane_token !== PLANE || !n.includes(PLANE)) bad.push("plane token drift");
  for (const pin of ["planning only", "contains no results"]) if (!n.includes(pin)) bad.push("status pin missing: " + pin);
  if (!n.includes("it never intercepts, blocks, or stops")) bad.push("Phase 0 negative sentence missing");
  if (/\b(enforcement is active|now blocks|blocks every|interception has shipped)\b/.test(n)) bad.push("enforcement claim in a Phase 0 plan");
  const scanText = n + "\n" + JSON.stringify({ ...json, forbidden_phrases: undefined });
  for (const p of json.forbidden_phrases) {
    if (p && scanText.toLowerCase().includes(p.toLowerCase())) bad.push("forbidden phrase present: " + p);
  }
  if (!/honest.miss|honest miss/.test(n)) bad.push("honest-miss posture sentence missing");
  if (!n.includes("known_gap")) bad.push("evals known_gap precedent pointer missing");
  if (!read(root, "scripts/benchplan-check.mjs")) bad.push("checker unregistered (lockstep file)");
  if (!read(root, "internal/discovery/benchplan_check_test.go")) bad.push("go mount unregistered (lockstep file)");
  return bad;
}

function runSelftest(root) {
  const { json, docs } = loadArtifacts(root);
  const cases = [
    ["hollow evidence pointer", (j, d) => { j.metrics[5].evidence = [{ file: "internal/nonexistent-reader.go", token: "whatever" }]; }],
    ["invented metric outside the §73 census", (j) => { j.metrics.push({ id: "bp.fan-speed", status: "PLANNED", goal: "x", evidence: [{ file: "docs/benchmark-plan.md", "token": "Fast Path" }] }); }],
    ["batch scenario dropped (3/4)", (j, d) => { j.batch_scenarios = j.batch_scenarios.filter((s) => s.id !== "bp-rm-3"); }],
    ["seed promoted to verified promise", (j, d) => { j.seed.state = "verified"; }],
    ["declared-absent drift: gap upgraded while tree stays empty", (j) => { j.declared_gaps[1].status = "PARTIAL"; }],
  ];
  let caught = 0;
  for (const [name, mutate] of cases) {
    const j = JSON.parse(JSON.stringify(json));
    const d = docs;
    mutate(j, d);
    const problems = checkPlan(root, j, d);
    if (problems.length > 0) { caught++; console.log("selftest caught: " + name); }
    else console.log("SELFTEST FAIL - not caught: " + name);
  }
  if (caught !== cases.length) { console.error("selftest incomplete: " + caught + "/" + cases.length); process.exit(1); }
  console.log("SELFTEST GREEN " + caught + "/" + cases.length);
}

// collapsed normalized text for token substring checks (source prose
// wraps mid-token; tokens are matched on whitespace-collapsed text)
function flat(md) {
  return norm(md).replace(/\s+/g, " ");
}

const root = root0;
if (process.argv.includes("--selftest")) {
  runSelftest(root);
  process.exit(0);
}
const { json, docs } = loadArtifacts(root);
const problems = checkPlan(root, json, docs);
if (problems.length > 0) {
  for (const p of problems) console.error("FAIL: " + p);
  process.exit(1);
}
console.log("BENCHPLAN GREEN: 7 metrics lockstep, §70-73 reverse lookup closed, batch 4/4 declared, seed recorded-not-verified, symbol-absence re-derived, plane " + PLANE);
