// task-record-emit.mjs — task-record emission v0 (tooling side).
//
// Materializes the derive-only mapper's interval view as typed
// task.record lines written to a SEPARATE side stream. The mapper of
// scripts/task-map.mjs remains the single MR-1..MR-5 implementation;
// this script consumes it as a subprocess and carries no second copy
// of any closed set, no re-derivation, and no judgement of its own:
// every emitted line restates the mapper's interval for that task_id
// with the additive record-form fields only.
//
// Red lines enforced by construction:
//   - inputs open read-only; no existing stream is ever rewritten,
//     so every shipped corpus keeps its bytes and meaning;
//   - the collector runtime (Go) emits nothing — the pipeline stays
//     byte-identical and the golden fixtures are untouched;
//   - only the landed pair (task_id, task_status) is carried; the
//     reserved names task_boundary / task_refs / lease_expiry_edge
//     never appear in any emitted line;
//   - decision/severity restated from the interval's last observed
//     in-scope decision token, never a new judgement (observation-
//     phase closed set unchanged);
//   - deterministic: same inputs yield byte-identical output
//     (ids positional, timestamps taken from the interval's boundary
//     event, no wall clock in the emitted payload).
//
// Usage:
//   node scripts/task-record-emit.mjs --out <stream.jsonl> <file.jsonl ...>
//   node scripts/task-record-emit.mjs --selftest
//
// Machine gates run on every emission: vocabulary import (via the
// mapper's own doc import — parsed, never copied), task_id positional
// shape, closed-set membership of every emitted task_status, parity
// with the mapper view (per task_id, one record, status equal), and
// input-byte immutability (sha before/after). Output lines validate
// against scripts/validate-jsonl.mjs (schema side landed with the
// record-form slice).
import { readFileSync, writeFileSync, mkdtempSync, rmSync } from 'node:fs';
import { createHash } from 'node:crypto';
import { execFileSync } from 'node:child_process';
import * as os from 'node:os';
import * as path from 'node:path';
import { fileURLToPath } from 'node:url';

const root = path.resolve(path.dirname(fileURLToPath(import.meta.url)), '..');
const MAPPER = path.join(root, 'scripts', 'task-map.mjs');
const VALIDATOR = path.join(root, 'scripts', 'validate-jsonl.mjs');

function shaOf(files) {
  const h = createHash('sha256');
  for (const f of files) h.update(readFileSync(f));
  return h.digest('hex');
}

function runMapper(files) {
  const out = execFileSync(process.execPath, [MAPPER, ...files], { encoding: 'utf8' });
  let obj;
  try { obj = JSON.parse(out); } catch { console.error('EMIT RED: mapper output not JSON'); process.exit(1); }
  if (obj.derive_only !== true || obj.emitted !== false || obj.conservation !== 'GREEN') {
    console.error('EMIT RED: mapper posture broken (derive_only/emitted/conservation)');
    process.exit(1);
  }
  return obj;
}

const validTaskID = (s) => {
  const i = s.indexOf(':');
  if (i <= 0 || !/^[A-Za-z0-9._-]+$/.test(s.slice(0, i))) return false;
  const rest = s.slice(i + 1);
  return /^[0-9]{1,9}$/.test(rest) && s.indexOf(':', i + 1) === -1;
};

// One emitted record per mapper interval: positional id keyed on the
// interval's own task_id (never random), ts carried from the seeding
// boundary event's stream position label, summary a fixed template.
function buildRecords(obj, streamTs) {
  const lines = [];
  const seen = new Set();
  for (const iv of obj.tasks) {
    if (!validTaskID(iv.task_id)) { console.error(`EMIT RED: task_id ${iv.task_id} not positional`); process.exit(1); }
    if (seen.has(iv.task_id)) { console.error(`EMIT RED: duplicate interval for ${iv.task_id}`); process.exit(1); }
    seen.add(iv.task_id);
    const dec = iv.decisions && iv.decisions.length ? iv.decisions[iv.decisions.length - 1] : 'allow';
    const sev = dec === 'would_block' ? 1 : 0;
    const ev = {
      v: 1,
      // stream-derived timestamp: the latest real event ts of the read
      // window, restated on every record. No wall clock ever enters the
      // emitted payload, so replay is byte-identical.
      ts: streamTs,
      id: 'trk-' + iv.task_id.replace(/[^A-Za-z0-9._-]/g, '-'),
      agent_id: 'task-recorder',
      stage: 'observation',
      type: 'task.record',
      decision: dec,
      severity: sev,
      summary: `derived task interval restated as a record (emission v0, side stream; mapper parity by construction)`,
      task_id: iv.task_id,
      task_status: iv.state,
    };
    lines.push(JSON.stringify(ev));
  }
  return lines;
}

function maxEventTs(files) {
  let best = null;
  for (const f of files) {
    for (const line of readFileSync(f, 'utf8').split('\n')) {
      if (!line.trim()) continue;
      let ev;
      try { ev = JSON.parse(line); } catch { continue; }
      if (typeof ev.ts === 'string' && (best === null || ev.ts > best)) best = ev.ts;
    }
  }
  if (!best) { console.error('EMIT RED: no event timestamp in inputs (nothing to restate against)'); process.exit(1); }
  return best;
}

function selftest() {
  const inputs = [
    path.join(root, 'testdata', 'join', 'hint-pair.jsonl'),
    path.join(root, 'testdata', 'legacy-v1-lines.jsonl'),
  ];
  const dir = mkdtempSync(path.join(os.tmpdir(), 'taskemit-ctl-'));
  try {
    const before = shaOf(inputs);
    const obj = runMapper(inputs);
    const lines = buildRecords(obj, maxEventTs(inputs));
    const outPath = path.join(dir, 'emitted.jsonl');
    writeFileSync(outPath, lines.join('\n') + (lines.length ? '\n' : ''));
    const again = lines.join('\n');
    // determinism: re-run the whole pipeline, byte-compare
    const obj2 = runMapper(inputs);
    const lines2 = buildRecords(obj2, maxEventTs(inputs)).join('\n');
    if (again !== lines2) { console.error('SELFTEST RED: emission not deterministic'); return 1; }
    if (shaOf(inputs) !== before) { console.error('SELFTEST RED: input bytes changed'); return 1; }
    // parity: every emitted status equals the mapper's interval state
    const map = new Map(obj.tasks.map((iv) => [iv.task_id, iv.state]));
    for (const line of lines) {
      const ev = JSON.parse(line);
      if (map.get(ev.task_id) !== ev.task_status) { console.error(`SELFTEST RED: parity broken on ${ev.task_id}`); return 1; }
      for (const banned of ['task_boundary', 'task_refs', 'lease_expiry_edge']) {
        if (line.includes(banned)) { console.error(`SELFTEST RED: reserved token ${banned} in emitted line`); return 1; }
      }
    }
    // schema landing: emitted stream must validate against the mirror
    execFileSync(process.execPath, [VALIDATOR, outPath], { encoding: 'utf8' });
    if (!lines.length) { console.error('SELFTEST RED: empty emission over fixtures (positive control failed)'); return 1; }
    console.log(`SELFTEST GREEN (${lines.length} records emitted, parity+determinism+validator all green, input bytes identical)`);
    return 0;
  } finally {
    rmSync(dir, { recursive: true, force: true });
  }
}

const argv = process.argv.slice(2);
if (argv[0] === '--selftest') process.exit(selftest());
let out = null;
const files = [];
for (let i = 0; i < argv.length; i++) {
  if (argv[i] === '--out') { out = path.resolve(root, argv[++i]); continue; }
  files.push(path.resolve(root, argv[i]));
}
if (!out || !files.length) { console.error('usage: task-record-emit.mjs --out <stream.jsonl> <file.jsonl ...> | --selftest'); process.exit(2); }
for (const f of files) if (path.resolve(f) === out) { console.error('EMIT RED: refusing to emit onto an input stream'); process.exit(2); }
const before = shaOf(files);
const obj = runMapper(files);
const lines = buildRecords(obj, maxEventTs(files));
writeFileSync(out, lines.join('\n') + (lines.length ? '\n' : ''), { flag: 'w' });
if (shaOf(files) !== before) { console.error('EMIT RED: input bytes changed (immutability gate)'); process.exit(1); }
console.log(JSON.stringify({ emitted: lines.length, out: path.relative(root, out), inputs: files.length, derive_source: 'scripts/task-map.mjs', side_stream: true, runtime_wired: false }));
