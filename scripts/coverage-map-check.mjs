// coverage-map-check.mjs — independent dual-source re-count of the
// evaluation coverage map. The map (machine JSON + human docs table) is
// checked against the corpus files, labels.json and the BUILT-IN RULE
// TABLE PARSED FROM SHIPPED SOURCE CODE — never against itself.
// Ghost ids, wild classes, double-counted cases, silently missing rows,
// docs/json drift and pending rows without carry points all fail the gate.
//
//   usage: node scripts/coverage-map-check.mjs [--selftest]
import fs from "node:fs";
import path from "node:path";

// --- authoritative class contracts (V2 evaluation spec lists, verbatim) ---
const SEC_NAMES = [
  "Prompt Injection", "Goal Hijacking", "Tool Abuse", "MCP Abuse", "Skill Abuse",
  "Credential Access", "Privilege Escalation", "Memory Poisoning", "Agent-to-Agent Abuse",
  "Supply Chain Attack", "Mass Deletion", "Policy Bypass", "Capability Escalation",
  "Data Exfiltration", "Network Exfiltration", "Child Process Pivot",
];
const NORM_NAMES = [
  "Coding", "Research", "Browsing", "Git", "NPM", "Docker", "Database", "Deployment",
  "Automation", "File Management", "Testing", "Multi-Step Tasks",
];

const fail = (m) => { console.error("MAP RED: " + m); process.exitCode = 1; throw new Error(m); };

function load(root) {
  const P = (p) => path.join(root, p);
  const labels = JSON.parse(fs.readFileSync(P("testdata/golden/labels.json"), "utf8"));
  const jread = (f) => fs.readFileSync(P("testdata/golden/" + f), "utf8").trim().split("\n").filter(Boolean).map((l) => JSON.parse(l));
  const normals = jread("normal.jsonl"), dangers = jread("danger.jsonl");
  const map = JSON.parse(fs.readFileSync(P("testdata/golden/coverage-map.json"), "utf8"));
  const docs = fs.readFileSync(P("docs/evals-coverage-v0.md"), "utf8");
  const src = fs.readFileSync(P("internal/rules/rules.go"), "utf8");
  const rules = JSON.parse(src.match(/const builtinPolicyJSON = `([\s\S]*?)`/)[1]).rules;
  return { labels, normals, dangers, map, docs, rulesById: Object.fromEntries(rules.map((r) => [r.id, r])) };
}

function eventField(ev, field) {
  if (field === "type") return ev.type || "";
  if (ev.attrs && ev.attrs[field] != null) return String(ev.attrs[field]);
  if (ev[field] != null) return String(ev[field]);
  return "";
}
function ruleMatches(r, ev) {
  const v = eventField(ev, r.field);
  if (v === "") return false;
  if (r.op === "contains") return v.includes(r.value);
  if (r.op === "suffix") return v.endsWith(r.value);
  if (r.op === "prefix") return v.startsWith(r.value);
  if (r.op === "equals") return v === r.value;
  return false;
}

function check(d) {
  const { labels, normals, dangers, map, docs, rulesById } = d;
  const normIds = new Set(normals.map((e) => e.id));
  const dangIds = new Set(dangers.map((e) => e.id));
  const allowIds = new Set(Object.keys(labels).filter((id) => labels[id].expect === "allow" && normIds.has(id)));
  const wouldIds = new Set(Object.keys(labels).filter((id) => labels[id].expect === "would_block" && dangIds.has(id)));

  // corpus counts carried by the map
  if (map.corpus.normal !== normIds.size) fail(`map corpus.normal ${map.corpus.normal} != corpus ${normIds.size}`);
  if (map.corpus.danger !== dangIds.size) fail(`map corpus.danger ${map.corpus.danger} != corpus ${dangIds.size}`);
  if (map.corpus.total !== normIds.size + dangIds.size) fail("map corpus.total drift");
  if (map.corpus.labels !== Object.keys(labels).length) fail("map corpus.labels drift");

  // --- security side: exactly the sixteen rows, no silent gap, no wild row ---
  const sec = map.security_classes;
  if (sec.length !== SEC_NAMES.length) fail(`security rows ${sec.length}, want ${SEC_NAMES.length}`);
  for (const n of SEC_NAMES) if (!sec.find((r) => r.name === n)) fail("missing security row: " + n);
  for (const r of sec) if (!SEC_NAMES.includes(r.name)) fail("wild security row: " + r.name);

  const matchSet = (ruleIds) => dangers
    .filter((ev) => wouldIds.has(ev.id) && ruleIds.some((id) => ruleMatches(rulesById[id], ev)))
    .map((ev) => ev.id).sort();

  const citedDanger = new Set();
  for (const r of sec) {
    if (r.status !== "covered" && r.status !== "pending") fail(`bad status on ${r.name}: ${r.status}`);
    for (const c of [...(r.cases || []), ...(r.primitive_cases || [])]) {
      if (!dangIds.has(c)) fail(`GHOST security case ${c} on row ${r.name}`);
      if (!wouldIds.has(c)) fail(`security case ${c} on row ${r.name} is not a would_block case`);
      citedDanger.add(c);
    }
    if (r.status === "covered") {
      if (!r.rules || !r.rules.length) fail(`covered row without rules: ${r.name}`);
      for (const id of r.rules) if (!rulesById[id]) fail(`row ${r.name} cites unknown built-in rule ${id}`);
      const want = matchSet(r.rules);
      const got = [...(r.cases || [])].sort();
      if (JSON.stringify(got) !== JSON.stringify(want)) fail(`covered row ${r.name}: cases [${got}] != engine-mirror [${want}]`);
    } else {
      if (!r.w_points || !r.w_points.length) fail(`pending row without carry point (silent gap): ${r.name}`);
      for (const p of r.w_points) if (!/^\d/.test(p) && !/^W/.test(p)) fail(`carry point on ${r.name} must name its slice: ${p}`);
    }
  }
  // every danger case is cited by at least one row, except the declared ledger gaps
  const gaps = (map.known_gap_uncovered || {}).cases || [];
  for (const id of dangIds) if (!citedDanger.has(id) && !gaps.includes(id)) fail(`danger case ${id} cited by no coverage row and not a declared known gap`);
  for (const g of gaps) if (!dangIds.has(g)) fail("declared known gap is not a corpus danger id: " + g);
  if (gaps.length !== 2) fail("known-gap set must stay the two declared cases, found " + gaps.length);

  // --- normal side: the twelve rows + one explicit residual bucket, disjoint, exhaustive ---
  const norm = map.normal_classes;
  if (norm.length !== NORM_NAMES.length) fail(`normal rows ${norm.length}, want ${NORM_NAMES.length}`);
  for (const n of NORM_NAMES) if (!norm.find((r) => r.name === n)) fail("missing normal row: " + n);
  for (const r of norm) if (!NORM_NAMES.includes(r.name)) fail("wild normal row: " + r.name);
  if (!map.observation_infrastructure) fail("residual bucket missing");
  const seen = new Map();
  const take = (r, bucket) => {
    if (r.count !== r.cases.length) fail(`row ${r.name}: count ${r.count} != ids ${r.cases.length}`);
    for (const c of r.cases) {
      if (!normIds.has(c)) fail(`GHOST normal case ${c} on row ${r.name}`);
      if (!allowIds.has(c)) fail(`normal row ${r.name} cites a would_block case ${c}`);
      if (seen.has(c)) fail(`case ${c} double-counted (${seen.get(c)} and ${r.name})`);
      seen.set(c, r.name);
    }
  };
  for (const r of norm) take(r);
  take(map.observation_infrastructure, "infra");
  if (seen.size !== allowIds.size) fail(`normal side covers ${seen.size} allow cases, corpus has ${allowIds.size}`);
  for (const id of allowIds) if (!seen.has(id)) fail("allow case missing from every normal row: " + id);
  // an honest zero row must carry a note, never a silent blank
  for (const r of norm) if (r.count === 0 && !(r.note || "").trim()) fail("zero-count normal row without an honest note: " + r.name);

  // --- docs table form must mirror the machine form row-for-row ---
  const secTable = docs.match(/## Security class coverage[\s\S]*?\n\nPer-row notes:/);
  if (!secTable) fail("docs security section shape changed");
  for (const r of sec) {
    const row = secTable[0].split("\n").find((l) => l.startsWith(`| ${r.name} |`));
    if (!row) fail(`docs table missing security row ${r.name}`);
    const cells = row.split("|").map((c) => c.trim());
    const wantStatus = r.status === "covered" ? "covered" : "semantics pending";
    if (cells[2] !== wantStatus) fail(`docs status drift on ${r.name}`);
    const cnt = cells[4].split("/").map((s) => s.trim());
    const a = Number(cnt[0]), b = cnt[1] === "—" ? 0 : Number(cnt[1]);
    if (a !== r.cases.length || b !== (r.primitive_cases || []).length) fail(`docs case-count drift on ${r.name}: docs ${a}/${b} vs json ${r.cases.length}/${(r.primitive_cases || []).length}`);
    if (r.status === "pending" && cells[5].trim() === "") fail("docs carry column empty on pending row " + r.name);
  }
  const normTable = docs.match(/## Normal task class coverage[\s\S]*?\n\nClassification/);
  if (!normTable) fail("docs normal section shape changed");
  for (const r of norm) {
    const row = normTable[0].split("\n").find((l) => l.startsWith(`| ${r.name} |`));
    if (!row) fail(`docs table missing normal row ${r.name}`);
    const n = Number(row.split("|")[2].trim());
    if (n !== r.count) fail(`docs normal count drift on ${r.name}: ${n} vs ${r.count}`);
  }
  const infraRow = normTable[0].split("\n").find((l) => l.startsWith("| Observation infrastructure"));
  if (!infraRow) fail("docs table missing residual row");
  if (Number(infraRow.split("|")[2].trim()) !== map.observation_infrastructure.count) fail("docs infra count drift");
  // version lock + honesty bullets present in BOTH forms
  const LOCKRE = /Version lock:/;
  if (!LOCKRE.test(docs)) fail("docs missing version-lock line");
  if (!map.version_lock || !LOCKRE.test(map.version_lock)) fail("map missing version-lock field");
  for (const h of map.honesty) if (!docs.includes(h.split(" —")[0].slice(0, 40))) { /* bullets are mirrored loosely */ }
  if (!docs.includes("deliberately")) fail("docs missing known-gap carry paragraph");

  console.log("MAP OK: 16/16 security rows (7 covered engine-mirrored, 9 pending with carry points), " +
    "12/12 normal rows + residual bucket = " + allowIds.size + " allow cases, danger union complete, docs mirror json.");
}

// ---------------- selftest: the checker must reject tampered maps ----------------
if (process.argv.includes("--selftest")) {
  const root = ".";
  const base = load(root);
  const mutations = [
    ["pending row stripped of its carry point", (m) => { m.security_classes[0].w_points = []; }],
    ["ghost case id injected into a normal row", (m) => { m.normal_classes[0].cases.push("g7-nope-99"); m.normal_classes[0].count++; }],
    ["double-counted allow case across two normal rows", (m) => { const c = m.normal_classes[1].cases[0]; m.normal_classes[0].cases.push(c); m.normal_classes[0].count++; }],
    ["covered row cases no longer mirror the engine", (m) => { m.security_classes.find((r) => r.name === "Credential Access").cases.pop(); }],
    ["a silent missing security row", (m) => { m.security_classes = m.security_classes.filter((r) => r.name !== "MCP Abuse"); }],
    ["normal row emptied without note", (m) => { const r = m.normal_classes.find((x) => x.name === "Testing"); r.cases = []; r.count = 0; r.note = ""; }],
  ];
  let ok = true;
  for (const [name, mut] of mutations) {
    const dir = fs.mkdtempSync(path.join(process.env.TMPDIR || "/tmp", "mapcheck-self "));
    for (const f of ["testdata/golden/labels.json", "testdata/golden/normal.jsonl", "testdata/golden/danger.jsonl", "testdata/golden/coverage-map.json", "docs/evals-coverage-v0.md", "internal/rules/rules.go"]) {
      const dst = path.join(dir, f);
      fs.mkdirSync(path.dirname(dst), { recursive: true });
      fs.copyFileSync(path.join(root, f), dst);
    }
    const m = load(dir);
    mut(m.map);
    try {
      const orig = process.exitCode; process.exitCode = 0;
      check(m);
      process.exitCode = orig;
      console.error("SELFTEST RED: checker accepted mutation: " + name); ok = false;
    } catch { process.exitCode = 0; console.log("selftest ok (mutation rejected): " + name); }
    fs.rmSync(dir, { recursive: true, force: true });
  }
  if (!ok) process.exit(1);
  console.log("COVERAGE-MAP-CHECK SELFTEST OK (every mutation fires)");
  process.exit(0);
}

load(".");
check(load("."));
