// bench-replay-e2e.mjs — end-to-end decision bench carrier for the
// benchmark plan row `bp.decision-latency`. The shipped replay subcommand
// judges a candidate policy over audit corpora through the full
// ingest -> validate -> evaluate -> verdict path; this harness times that
// path across repeated runs and prints one machine-readable row.
//
// Honesty rules of this carrier:
//   - every number is a wall-clock measurement of the real decision path
//     INCLUDING process startup and runtime init (startup-inclusive);
//   - state is `recorded`: estimation from a development container is not
//     verified fact and must be re-measured by the CI benchmark battery
//     before being cited outside engineering documents;
//   - the contract target stays the pre-pinned goal in the plan; this
//     carrier never promotes its own reading into a promise;
//   - corpora, policy and binary paths are passed in or discovered from
//     fixed repo-relative locations; nothing is invented if a required
//     input is missing (fail closed, no fake zero, no fake number).
//
// usage:
//   node scripts/bench-replay-e2e.mjs --binary <collector> --policy <candidate.json>
//       [--corpus <jsonl> ...] [--reps <n>] [--selftest]
import { spawnSync } from "node:child_process";
import fs from "node:fs";
import os from "node:os";
import path from "node:path";

const root0 = path.resolve(import.meta.dirname, "..");
const DEFAULT_CORPORA = [
  path.join(root0, "testdata", "golden", "normal.jsonl"),
  path.join(root0, "testdata", "golden", "danger.jsonl"),
];

function parseArgs(argv) {
  const a = { reps: 15, corpora: [], selftest: false };
  for (let i = 0; i < argv.length; i++) {
    const t = argv[i];
    if (t === "--binary") a.binary = argv[++i];
    else if (t === "--policy") a.policy = argv[++i];
    else if (t === "--corpus") a.corpora.push(argv[++i]);
    else if (t === "--reps") a.reps = Number(argv[++i]);
    else if (t === "--selftest") a.selftest = true;
    else throw new Error("bench-replay-e2e: unknown argument " + t);
  }
  return a;
}

function countEvents(files) {
  let n = 0;
  for (const f of files) {
    const lines = fs.readFileSync(f, "utf8").split("\n");
    for (const l of lines) if (l.trim() !== "") n++;
  }
  return n;
}

function median(xs) {
  const s = [...xs].sort((p, q) => p - q);
  const m = Math.floor(s.length / 2);
  return s.length % 2 ? s[m] : (s[m - 1] + s[m]) / 2;
}

function runOnce(binary, policy, corpora) {
  const t0 = process.hrtime.bigint();
  const r = spawnSync(binary, ["replay", "--policy", policy, ...corpora], {
    encoding: "utf8",
    maxBuffer: 64 * 1024 * 1024,
  });
  const t1 = process.hrtime.bigint();
  if (r.error) throw new Error("bench-replay-e2e: binary failed to run: " + r.error.message);
  if (r.status !== 0) {
    throw new Error("bench-replay-e2e: replay exited " + r.status + ": " + String(r.stderr).trim().slice(0, 400));
  }
  return Number(t1 - t0) / 1e6; // ms, startup-inclusive
}

function bench(a) {
  if (!a.binary || !fs.existsSync(a.binary)) throw new Error("bench-replay-e2e: --binary required and must exist");
  if (!a.policy || !fs.existsSync(a.policy)) throw new Error("bench-replay-e2e: --policy required and must exist (candidate derived from built-in rules; see reproduction note at bottom)");
  const corpora = a.corpora.length ? a.corpora : DEFAULT_CORPORA;
  for (const c of corpora) if (!fs.existsSync(c)) throw new Error("bench-replay-e2e: missing corpus " + c);
  if (!Number.isInteger(a.reps) || a.reps < 3) throw new Error("bench-replay-e2e: --reps must be an integer >= 3");
  const events = countEvents(corpora);
  if (events <= 0) throw new Error("bench-replay-e2e: corpora hold zero events (refusing a fake reading)");
  const runs = [];
  for (let i = 0; i < a.reps; i++) runs.push(runOnce(a.binary, a.policy, corpora));
  const perEventUs = runs.map((ms) => (ms * 1000) / events);
  const line =
    "BENCH-REPLAY-E2E binary=" + path.basename(a.binary) +
    " events=" + events + " reps=" + a.reps +
    " wall_ms_min=" + Math.min(...runs).toFixed(2) +
    " wall_ms_median=" + median(runs).toFixed(2) +
    " us_per_event_median=" + median(perEventUs).toFixed(1) +
    " state=recorded (startup-inclusive, development container, estimation is not verified fact;" +
    " re-measure with the CI benchmark battery before citing outside engineering documents)";
  console.log(line);
  return line;
}

function selftest() {
  const dir = fs.mkdtempSync(path.join(os.tmpdir(), "bench-selftest-"));
  // Stub binary: accepts the replay shape, exits 0 after ~5ms of work.
  const stub = path.join(dir, "stub.sh");
  fs.writeFileSync(stub, "#!/bin/sh\n[ \"$1\" = replay ] || exit 3\ni=0\nwhile [ $i -lt 500000 ]; do i=$((i+1)); done\nexit 0\n", { mode: 0o755 });
  const pol = path.join(dir, "cand.json");
  fs.writeFileSync(pol, "{}");
  const corpus = path.join(dir, "c.jsonl");
  fs.writeFileSync(corpus, "x\n".repeat(5));
  const ko = (name, fn) => { try { fn(); console.log("selftest FAIL " + name + ": expected a closed failure, got success"); process.exitCode = 1; } catch (e) { console.log("selftest PASS " + name + " (rejected: " + e.message.slice(0, 70) + ")"); } };
  const ok = (name, fn) => { try { fn(); console.log("selftest PASS " + name); } catch (e) { console.log("selftest FAIL " + name + ": " + e.message); process.exitCode = 1; } };
  ok("positive row shape", () => {
    const line = bench({ binary: stub, policy: pol, corpora: [corpus], reps: 3 });
    for (const tok of ["BENCH-REPLAY-E2E", "events=5", "reps=3", "us_per_event_median", "state=recorded"]) {
      if (!line.includes(tok)) throw new Error("row missing token " + tok);
    }
  });
  ko("missing binary fails closed", () => { bench({ binary: path.join(dir, "nope"), policy: pol, corpora: [corpus], reps: 3 }); });
  ko("missing policy fails closed", () => { bench({ binary: stub, policy: null, corpora: [corpus], reps: 3 }); });
  ko("empty corpus refuses fake reading", () => {
    const empty = path.join(dir, "empty.jsonl");
    fs.writeFileSync(empty, "\n\n");
    bench({ binary: stub, policy: pol, corpora: [empty], reps: 3 });
  });
  ko("nonzero replay exit propagates", () => {
    const bad = path.join(dir, "bad.sh");
    fs.writeFileSync(bad, "#!/bin/sh\nexit 7\n", { mode: 0o755 });
    bench({ binary: bad, policy: pol, corpora: [corpus], reps: 3 });
  });
  fs.rmSync(dir, { recursive: true, force: true });
}

try {
  const a = parseArgs(process.argv.slice(2));
  if (a.selftest) selftest();
  else bench(a);
} catch (e) {
  console.error("bench-replay-e2e: " + e.message);
  process.exit(1);
}
// Reproduction note: the candidate policy is the built-in rule set
// serialized as JSON (schema.Policy shape); dump it once from a scratch
// helper inside the module that calls rules.Builtin() and encodes the
// result, then pass it via --policy. This carrier itself stays Go-free.
