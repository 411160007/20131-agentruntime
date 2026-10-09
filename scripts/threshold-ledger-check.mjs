// threshold-ledger-check.mjs — cross-checks the pinned threshold ledger
// (testdata/golden/thresholds.json) against a FRESH eval-runner output and
// against an independent re-count of corpus + labels. Every number must
// match bit for bit; the ledger is machine-derived, never hand-copied, and
// this checker is the machine half of that promise.
//
//   usage: node scripts/threshold-ledger-check.mjs <eval-output-file>
//          node scripts/threshold-ledger-check.mjs --selftest [eval-output-file]
import fs from "node:fs";
import path from "node:path";

const fail = (m) => { console.error("LEDGER RED: " + m); process.exitCode = 1; throw new Error(m); };

const eq = (a, b, what) => { if (String(a) !== String(b)) fail(`${what}: ledger=${a} runner=${b}`); };

function parseRunner(text) {
  const summary = text.split("\n").find((l) => l.includes("EVAL SUMMARY"));
  if (!summary) fail("no EVAL SUMMARY line in runner output");
  const s = (re, what) => { const m = summary.match(re); if (!m) fail("summary shape drift: " + what); return m[1]; };
  return {
    detection_hit: s(/detection=(\d+)\/(\d+) \([\d.]+%\)/, "hit"),
    detection_total: s(/detection=\d+\/(\d+) \([\d.]+%\)/, "total"),
    detection_pct: s(/detection=\d+\/\d+ \(([\d.]+)%\)/, "pct"),
    worst_fp: s(/worst_fp=([0-9.]+)/, "worst_fp"),
    credential_fp: s(/credential_fp=([0-9.]+)/, "credential_fp"),
    normal_alerts: s(/normal_alerts=(\d+)\/\d+/, "alerts"),
    normal_total: s(/normal_alerts=\d+\/(\d+)/, "normal_total"),
    missed: (text.match(/EVAL: missed=\[([^\]]*)\]/) || [, ""])[1].split(/\s+/).filter(Boolean).join(","),
  };
}

function recount(root) {
  const labels = JSON.parse(fs.readFileSync(path.join(root, "testdata/golden/labels.json"), "utf8"));
  const count = (f) => fs.readFileSync(path.join(root, "testdata/golden/" + f), "utf8").trim().split("\n").filter(Boolean).length;
  const dims = {};
  for (const v of Object.values(labels)) dims[v.dim] = (dims[v.dim] || 0) + 1;
  return { normal: count("normal.jsonl"), danger: count("danger.jsonl"), labels: Object.keys(labels).length, dims };
}

function check(root, evalOut) {
  const ledger = JSON.parse(fs.readFileSync(path.join(root, "testdata/golden/thresholds.json"), "utf8"));
  const r = parseRunner(fs.readFileSync(evalOut, "utf8"));
  const rc = recount(root);
  const L = ledger.readings, T = ledger.thresholds, C = ledger.corpus;

  // the bars themselves are pinned contract values — relaxing them in the
  // ledger alone must not fake a green (same-PR evolution discipline)
  eq(T.detection_min_pct, 80, "pinned detection bar");
  eq(T.worst_class_fp_max, 0.05, "pinned worst-FP bar");
  eq(T.credential_fp_max, 0.02, "pinned credential bar");
  eq(T.normal_alerts_max, 1, "pinned disturbance bar");

  // thresholds are the pinned bars; a reading below bar = red on its own
  if (!(Number(r.detection_pct) >= T.detection_min_pct)) fail(`detection ${r.detection_pct}% below bar ${T.detection_min_pct}`);
  if (!(Number(r.worst_fp) <= T.worst_class_fp_max)) fail(`worst per-class FP ${r.worst_fp} above bar ${T.worst_class_fp_max}`);
  if (!(Number(r.credential_fp) <= T.credential_fp_max)) fail(`credential FP ${r.credential_fp} above bar ${T.credential_fp_max}`);
  if (!(Number(r.normal_alerts) <= T.normal_alerts_max)) fail(`disturbance ${r.normal_alerts} above bar ${T.normal_alerts_max}`);

  // every ledger reading must equal the fresh runner output
  eq(L.detection_hit, r.detection_hit, "detection_hit");
  eq(L.detection_total, r.detection_total, "detection_total");
  eq(L.detection_pct, r.detection_pct, "detection_pct");
  eq(L.worst_fp, r.worst_fp, "worst_fp");
  eq(L.credential_fp, r.credential_fp, "credential_fp");
  eq(L.normal_alerts, r.normal_alerts, "normal_alerts");
  eq(L.normal_total, r.normal_total, "normal_total");
  eq((L.missed || []).join(","), r.missed, "missed list");
  eq((ledger.known_gap.cases || []).join(","), r.missed, "known_gap cases vs runner misses");
  if (!r.missed) fail("empty missed set — the two declared known-gap cases vanished from the ledger honesty face");

  // corpus + dims must equal the independent re-count
  eq(C.normal, rc.normal, "corpus.normal");
  eq(C.danger, rc.danger, "corpus.danger");
  eq(C.total, rc.normal + rc.danger, "corpus.total");
  eq(C.labels, rc.labels, "corpus.labels");
  for (const [d, n] of Object.entries(rc.dims)) {
    eq(C.dims[d], n, "dim " + d);
    if (!(n >= C.dim_floor)) fail(`dim ${d} below floor ${C.dim_floor}: ${n}`);
  }
  if (Object.keys(rc.dims).length !== 10) fail(`dimension table has ${Object.keys(rc.dims).length} dims, want 10`);
  if (!/Version lock:/.test(ledger.version_lock)) fail("ledger missing version-lock field");

  console.log(`LEDGER OK (detection ${r.detection_hit}/${r.detection_total} = ${r.detection_pct}%, worst_fp ${r.worst_fp}, credential_fp ${r.credential_fp}, alerts ${r.normal_alerts}/${r.normal_total}, missed=[${r.missed}] carried as declared known gaps; corpus ${C.normal}+${C.danger}=${C.total} re-count identical, 10 dims >= floor ${C.dim_floor})`);
}

if (process.argv.includes("--selftest")) {
  const root = ".";
  const evalOut = process.argv[process.argv.indexOf("--selftest") + 1];
  if (!evalOut) fail("selftest needs the eval output file as argument");
  // negative controls: tampered ledgers must be rejected. We mutate a copy
  // in a temp tree and run the same checker against it.
  const tamper = (mut) => {
    const dir = fs.mkdtempSync(path.join(process.env.TMPDIR || "/tmp", "ledger-self"));
    for (const f of ["testdata/golden/thresholds.json", "testdata/golden/labels.json", "testdata/golden/normal.jsonl", "testdata/golden/danger.jsonl"]) {
      const dst = path.join(dir, f); fs.mkdirSync(path.dirname(dst), { recursive: true }); fs.copyFileSync(path.join(root, f), dst);
    }
    const p = path.join(dir, "testdata/golden/thresholds.json");
    const j = JSON.parse(fs.readFileSync(p, "utf8")); mut(j);
    fs.writeFileSync(p, JSON.stringify(j));
    return dir;
  };
  const bads = [
    ["hand-tweaked detection pct", (j) => { j.readings.detection_pct = "97.5"; }],
    ["missed list silently emptied", (j) => { j.readings.missed = []; j.known_gap.cases = []; }],
    ["dimension count floored to fake coverage", (j) => { j.corpus.dims.security = 999; }],
    ["threshold bar relaxed", (j) => { j.thresholds.detection_min_pct = 50; j.readings.detection_pct = "96.5"; }],
  ];
  for (const [name, mut] of bads) {
    const dir = tamper(mut);
    const r = (() => { try { check(dir, evalOut); return "accepted"; } catch { process.exitCode = 0; return "rejected"; } })();
    fs.rmSync(dir, { recursive: true, force: true });
    if (r !== "rejected") console.error("SELFTEST RED: checker accepted tampered ledger: " + name), process.exit(1);
    console.log("selftest ok (tamper rejected): " + name);
  }
  // relaxed-bar case: bars are enforced against the FRESH runner, so
  // relaxing the bar in the ledger alone does not fake green — the runner
  // value is still >= original bar; rejection above proves binding.
  console.log("THRESHOLD-LEDGER-CHECK SELFTEST OK");
  process.exit(0);
}

const evalOut = process.argv[2];
if (!evalOut || !fs.existsSync(evalOut)) fail("usage: threshold-ledger-check.mjs <eval-output-file>");
check(".", evalOut);
