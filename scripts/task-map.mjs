// task-map.mjs — derive-only turn→task mapper v0 (observation phase).
//
// This tool implements the read-time mapping contract of
// docs/task-primitive-design.md section 4 (MR-1..MR-5) and produces the
// denominator readings of section 6 (DR-1..DR-3) with the honesty
// envelope. It is a view over existing JSONL streams:
//   - zero emission: it never appends to or rewrites any stream;
//   - zero schema writes: no EventType, no field-shape change;
//   - vocabulary import, never a copy: the task status/boundary/field
//     closed sets are re-parsed at run time from the design doc (the
//     single writer source). If the doc cannot be parsed exactly, this
//     tool refuses to run — a second vocabulary cannot exist here.
//
// Usage:
//   node scripts/task-map.mjs <file.jsonl> [more.jsonl ...]
//   node scripts/task-map.mjs --selftest
//
// Machine judgement gate: the conservation checks below exit non-zero on
// any violation (state census mismatch, unknown status token, task_id
// collision, turn/event coverage drift). Replaying the same stream
// yields byte-identical interval output (positional task_id).
import { readFileSync, writeFileSync, mkdtempSync, rmSync } from 'node:fs';
import { tmpdir } from 'node:os';
import path from 'node:path';
import { fileURLToPath } from 'node:url';

const root = path.resolve(path.dirname(fileURLToPath(import.meta.url)), '..');
const DOC = path.join(root, 'docs', 'task-primitive-design.md');

// ---------- vocabulary import (parsed from the doc, never copied) ----------
function extractVocab(docText, name) {
  const m = docText.match(new RegExp('^' + name + ': (.+)$', 'm'));
  return m ? m[1].split(/,\s*/).map((s) => s.trim()) : null;
}
function importVocab() {
  const doc = readFileSync(DOC, 'utf8');
  const statuses = extractVocab(doc, 'task_status_vocabulary');
  const boundary = extractVocab(doc, 'boundary_signal_vocabulary');
  const fields = extractVocab(doc, 'task_field_vocabulary');
  const denomRule = (doc.match(/^denominator_rule: (.+)$/m) || [])[1] || null;
  const errs = [];
  if (!statuses || statuses.length !== 7) errs.push('task_status_vocabulary not parseable as 7 tokens from the design doc');
  if (!boundary || boundary.length !== 4) errs.push('boundary_signal_vocabulary not parseable as 4 tokens');
  if (!fields || fields.length !== 4) errs.push('task_field_vocabulary not parseable as 4 tokens');
  for (const r of ['MR-1', 'MR-2', 'MR-3', 'MR-4', 'MR-5', 'DR-1', 'DR-2', 'DR-3']) {
    if (!doc.includes('`' + r + '`')) errs.push(`rule ${r} missing from the design doc`);
  }
  if (!denomRule) errs.push('denominator_rule line missing from the design doc');
  if (errs.length) { console.error('VOCAB IMPORT RED:\n  ' + errs.join('\n  ')); process.exit(2); }
  return { statuses, boundary, fields, denomRule };
}

const HINT_DONE = /\b(ship|shipped|complete|completed|done|finished|landed|merged|delivered)\b/i;
const HINT_FAIL = /\b(fail|failed|unable|could not|abort|aborted)\b/i;

// ---------- mapping MR-1..MR-5 ----------
function mapStream(file, vocab) {
  const text = readFileSync(file, 'utf8');
  const lines = text.split('\n').filter((l) => l.trim() !== '');
  const intervals = [];            // all intervals in stream position order
  const currentByAgent = new Map(); // agent_id -> latest open interval
  let lastOpened = null;            // positional owner for intent hints (v0 weak binding)
  let badLines = 0, intentLines = 0, orphanTurns = 0, orphanEvents = 0;
  let totalTurnStops = 0, scopedTurnStops = 0, scopedEvents = 0;

  lines.forEach((raw, pos) => {
    let e;
    try { e = JSON.parse(raw); } catch { badLines++; return; }
    const isIntent = !e.type && typeof e.goal === 'string';
    if (isIntent) {
      intentLines++;
      if (lastOpened && !lastOpened.hint) {
        lastOpened.hint = e.goal;
        lastOpened.hintPos = pos;
        lastOpened.boundary = vocab.boundary[2]; // goal_hint
      }
      return;
    }
    const agent = String(e.agent_id || 'unknown-agent');
    const type = String(e.type || '');
    if (type === 'session.start') { // MR-1 interval seeding
      const ordinal = intervals.filter((i) => i.agent === agent).length + 1;
      const iv = {
        task_id: agent + ':' + ordinal, agent, ordinal, file, startPos: pos,
        state: vocab.statuses[0], turns: [], decisions: [], hint: null, hintPos: -1,
        boundary: vocab.boundary[0], // session_start_edge
        lastType: 'session.start',
      };
      intervals.push(iv);
      currentByAgent.set(agent, iv);
      lastOpened = iv;
      scopedEvents++;
      return;
    }
    const iv = currentByAgent.get(agent);
    if (type === 'turn.stop') totalTurnStops++;
    if (!iv) { if (type === 'turn.stop') orphanTurns++; else orphanEvents++; return; }
    if (type === 'turn.stop') { // MR-2 turn folding
      iv.turns.push(String(e.id || `${iv.task_id}#${iv.turns.length}`));
      if (iv.hint && pos > iv.hintPos) iv.hintHadLaterTurn = true; // MR-3 outcome evidence
      iv.state = vocab.statuses[1]; // active
      iv.boundary = vocab.boundary[1]; // turn_stop_sequence
      scopedTurnStops++;
    } else if (type === 'policy.decision') {
      iv.decisions.push(String(e.decision || ''));
    }
    iv.lastType = type;
    scopedEvents++;
  });

  for (const iv of intervals) resolveInterval(iv, vocab); // MR-3..MR-5
  const counters = { lines: lines.length, badLines, intentLines, totalTurnStops, scopedTurnStops, orphanTurns, orphanEvents, scopedEvents };
  return { intervals, counters };
}

function resolveInterval(iv, vocab) {
  const [open, active, completed, failed, abandoned, blocked, unknown] = vocab.statuses;
  const done = iv.turns.length > 0;
  const lastDecision = iv.decisions.length ? iv.decisions[iv.decisions.length - 1] : null;
  const hintIsDone = iv.hint && HINT_DONE.test(iv.hint) && !HINT_FAIL.test(iv.hint);
  const hintIsFail = iv.hint && HINT_FAIL.test(iv.hint);
  // MR-3 evidence: a turn.stop strictly after the hint position confirms
  // the candidate; without it the candidate stays unknown (earned, not assumed).
  const hintEvidence = Boolean(iv.hintHadLaterTurn);
  // MR-4 safety tail: last in-scope decision would_block wins unless a
  // terminal candidate with outcome evidence already closed the interval.
  if (hintIsDone && hintEvidence && lastDecision !== 'would_block') { iv.state = completed; return; }
  if (hintIsFail && hintEvidence) { iv.state = failed; return; }
  if (lastDecision === 'would_block') { iv.state = blocked; return; }
  if (iv.hint) { iv.state = unknown; return; } // candidate without evidence
  if (!done) { iv.state = unknown; return; }    // MR-5 opened-but-never-turned
  if (iv.lastType === 'turn.stop') { iv.state = abandoned; return; } // MR-5 stream end
  iv.state = active; // live: interval truncated mid-flight
}

// ---------- machine judgement: conservation gates ----------
function checkConservation(results, vocab) {
  const v = [];
  const S = new Set(vocab.statuses);
  let seenTotal = 0;
  for (const [file, { intervals, counters }] of results) {
    const counts = Object.fromEntries(vocab.statuses.map((s) => [s, 0]));
    const ids = new Set();
    for (const iv of intervals) {
      if (!S.has(iv.state)) v.push(`${file}: interval ${iv.task_id} carries status ${JSON.stringify(iv.state)} outside the imported closed set`);
      else counts[iv.state]++;
      if (ids.has(iv.task_id)) v.push(`${file}: duplicate task_id ${iv.task_id} (interval seeding must stay positional)`);
      ids.add(iv.task_id);
    }
    const sum = Object.values(counts).reduce((a, b) => a + b, 0);
    if (sum !== intervals.length) v.push(`${file}: state census ${sum} != interval count ${intervals.length}`);
    const live = counts[vocab.statuses[0]] + counts[vocab.statuses[1]];
    const terminal = intervals.length - live;
    if (terminal !== counts[vocab.statuses[2]] + counts[vocab.statuses[3]] + counts[vocab.statuses[4]] + counts[vocab.statuses[5]] + counts[vocab.statuses[6]]) {
      v.push(`${file}: live/terminal split is not an identity`);
    }
    if (counters.scopedTurnStops + counters.orphanTurns !== counters.totalTurnStops) {
      v.push(`${file}: turn coverage drift ${counters.scopedTurnStops}+${counters.orphanTurns} != ${counters.totalTurnStops}`);
    }
    const turnSum = intervals.reduce((a, iv) => a + iv.turns.length, 0);
    if (turnSum !== counters.scopedTurnStops) {
      v.push(`${file}: fold conservation ${turnSum} != scoped turn.stops ${counters.scopedTurnStops}`);
    }
    if (counters.scopedEvents + counters.orphanTurns + counters.orphanEvents + counters.intentLines + counters.badLines !== counters.lines) {
      v.push(`${file}: line coverage conservation failed`);
    }
    seenTotal += intervals.length;
  }
  return { violations: v, seenTotal };
}

// ---------- DR-1..DR-3 readings ----------
function readings(results, vocab) {
  const [open, active, completed, failed, abandoned, blocked, unknown] = vocab.statuses;
  const all = [...results.values()].flatMap((r) => r.intervals);
  const c = Object.fromEntries(vocab.statuses.map((s) => [s, 0]));
  for (const iv of all) if (c[iv.state] !== undefined) c[iv.state]++;
  const denominator = c[completed] + c[failed] + c[abandoned] + c[blocked] + c[unknown];
  const numerator = c[completed]; // earned completions only (outcome-evidenced)
  const fmt = (x) => (denominator === 0 ? 'n/a (empty denominator)' : `${x}/${denominator}`);
  return {
    total_intervals: all.length,
    per_state: c,
    dr1: { denominator, live_excluded: c[open] + c[active], posture: 'terminal intervals (incl. terminal unknown) count once; live intervals are reported beside the ratio, never merged into it' },
    dr2: { numerator, ceiling_note: `ratio ${fmt(numerator)} — ceiling equals the unknown share (${fmt(c[unknown])}) until outcome-evidence coverage grows` },
    dr3_envelope: { total: all.length, live: c[open] + c[active], unknown: c[unknown], blocked: c[blocked], printed_ratio: fmt(numerator), rule: 'a ratio printed without this envelope is an unverifiable number' },
  };
}

// ---------- selftest ----------
function ev(type, id, agent, decision) {
  return JSON.stringify({ v: 1, ts: '2026-10-09T00:00:00Z', id, agent_id: agent, stage: 'observation', type, decision: decision || 'allow', severity: 0, summary: type, attrs: {} });
}
function selftest(vocab) {
  const dir = mkdtempSync(path.join(tmpdir(), 'taskmap-ctl-'));
  let caught = 0, assertions = 0;
  const caughtNames = [], missedNames = [];
  const INJECTIONS = 4;
  const expectRed = (name, fn) => {
    assertions++;
    try { fn(); missedNames.push(name); console.error(`SELFTEST RED: injection ${name} was NOT caught`); }
    catch { caughtNames.push(name); caught++; }
  };
  const expectTrue = (name, cond) => {
    assertions++;
    if (!cond) throw new Error(`assertion failed: ${name}`);
  };
  try {
    // 1. good stream: completed (hint + later turn), unknown (never turned), abandoned (tail turn.stop)
    const good = [
      ev('session.start', 'a1', 'A'), ev('turn.stop', 'a2', 'A'),
      JSON.stringify({ goal: 'ship the docs slice', scope: 'docs' }),
      ev('turn.stop', 'a3', 'A'),
      ev('session.start', 'b1', 'B'),
      ev('session.start', 'c1', 'C'), ev('turn.stop', 'c2', 'C'),
    ].join('\n') + '\n';
    const gf = path.join(dir, 'good.jsonl');
    writeFileSync(gf, good);
    // hint-later-turn bookkeeping (needed by MR-3 evidence)
    const r1 = mapFile(gf, vocab);
    const byId = Object.fromEntries(r1.intervals.map((iv) => [iv.task_id, iv.state]));
    const want = { 'A:1': vocab.statuses[2], 'B:1': vocab.statuses[6], 'C:1': vocab.statuses[4] };
    for (const [k, s] of Object.entries(want)) expectTrue(`mapping ${k} (got ${byId[k]})`, byId[k] === s);
    // replay identity: byte-identical interval table
    const r2 = mapFile(gf, vocab);
    expectTrue('replay byte-identical', JSON.stringify(r1.intervals) === JSON.stringify(r2.intervals));
    // readings envelope conservation: total == live + denominator
    const rd = readings(new Map([[gf, r1]]), vocab);
    expectTrue('reading envelope conserved', rd.total_intervals === rd.dr1.live_excluded + rd.dr1.denominator);

    // 2. negative control: wild status value injected post-map
    expectRed('wild status token', () => {
      const t = mapFile(gf, vocab);
      t.intervals[0].state = 'in_progress';
      gateOrThrow(new Map([[gf, t]]), vocab);
    });
    // 3. negative control: double open (duplicate positional task_id)
    expectRed('double open interval', () => {
      const t = mapFile(gf, vocab);
      t.intervals.push(JSON.parse(JSON.stringify(t.intervals[0])));
      gateOrThrow(new Map([[gf, t]]), vocab);
    });
    // 4. negative control: out-of-order session (turn.stop before its session.start)
    expectRed('orphan turn before session', () => {
      const badF = path.join(dir, 'bad-order.jsonl');
      writeFileSync(badF, [ev('turn.stop', 'x1', 'X'), ev('session.start', 'x2', 'X')].join('\n') + '\n');
      const t = mapFile(badF, vocab);
      if (t.counters.orphanTurns === 0) throw new Error('orphan accounting missed the out-of-order turn');
      const folded = t.intervals.reduce((a, iv) => a + iv.turns.length, 0);
      if (folded !== 0) throw new Error('orphan turn was folded into an interval');
      throw new Error('caught'); // surfaced as a gate condition by design
    });
    // 5. vocab tamper: mapper must refuse to run without an importable doc
    expectRed('unparseable vocabulary source', () => {
      const broken = { ...vocab, statuses: vocab.statuses.slice(0, 5) };
      const t = mapFile(gf, broken);
      gateOrThrow(new Map([[gf, t]]), broken);
    });
    // 5. candidate-without-evidence stays unknown (completion is earned, not assumed)
    {
      const evF = path.join(dir, 'hint-no-evidence.jsonl');
      writeFileSync(evF, [ev('session.start', 'd1', 'D'), JSON.stringify({ goal: 'ship the nightly slice' })].join('\n') + '\n');
      const t5 = mapFile(evF, vocab);
      expectTrue('hint without later-turn evidence stays unknown', t5.intervals[0].state === vocab.statuses[6]);
    }
    // 6. real corpora must pass the conservation gate untouched
    for (const f of ['testdata/golden/normal.jsonl', 'testdata/golden/danger.jsonl']) {
      const p = path.join(root, f);
      const t = mapFile(p, vocab);
      gateOrThrow(new Map([[p, t]]), vocab);
    }
  } finally {
    rmSync(dir, { recursive: true, force: true });
  }
  const ok = caught === INJECTIONS;
  console.log(`SELFTEST ${ok ? 'GREEN' : 'RED'} (${caught}/${INJECTIONS} injections caught: ${caughtNames.join('|')} | missed: ${missedNames.join('|') || 'none'}; ${assertions} assertions run)`);
  return ok ? 0 : 1;
}

// helpers used by both main and selftest
function mapFile(f, vocab) {
  return mapStream(f, vocab);
}
function gateOrThrow(results, vocab) {
  const g = checkConservation(results, vocab);
  if (g.violations.length) throw new Error('conservation gate RED:\n  ' + g.violations.join('\n  '));
}

// ---------- main ----------
const argv = process.argv.slice(2);
if (argv[0] === '--selftest') process.exit(selftest(importVocab()));
if (!argv.length) { console.error('usage: task-map.mjs <file.jsonl ...> | --selftest'); process.exit(2); }
const vocab = importVocab();
const results = new Map();
for (const f of argv) results.set(f, mapFile(path.resolve(root, f), vocab));
const g = checkConservation(results, vocab);
if (g.violations.length) { console.error('CONSERVATION GATE RED:\n  ' + g.violations.join('\n  ')); process.exit(1); }
const rd = readings(results, vocab);
const table = [...results.values()].flatMap((r) => r.intervals).map((iv) => ({
  task_id: iv.task_id, state: iv.state, turns: iv.turns.length, boundary: iv.boundary,
  hint: iv.hint || null, decisions: iv.decisions,
}));
console.log(JSON.stringify({ derive_only: true, emitted: false, vocabulary_source: 'docs/task-primitive-design.md', tasks: table, readings: rd, conservation: 'GREEN' }, null, 2));
