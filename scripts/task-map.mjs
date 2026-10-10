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
//   node scripts/task-map.mjs <file.jsonl ...> --evals <labels.json>
//   node scripts/task-map.mjs --selftest
//
// --evals joins an assertion corpus (event id -> {expect, class, dim}) as
// the second outcome-evidence source of MR-3, read-only and derive-only:
// an asserted turn whose observed decision equals the assertion is a
// verified outcome for that turn; a mismatch (conflict) never promotes.
// The join never writes into any stream and never fabricates completion.
// Without --evals the mapper behaves exactly as its single-source form.
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

// ---------- eval assertion corpus import (runtime parse, never copied) ----------
function importEvals(spec, rootDir) {
  if (!spec) return null;
  const p = path.isAbsolute(spec) ? spec : path.resolve(rootDir, spec);
  let raw;
  try { raw = readFileSync(p, 'utf8'); } catch { console.error('EVAL SOURCE RED: cannot read ' + p); process.exit(2); }
  let obj;
  try { obj = JSON.parse(raw); } catch { console.error('EVAL SOURCE RED: ' + p + ' is not valid JSON'); process.exit(2); }
  if (!obj || typeof obj !== 'object' || Array.isArray(obj)) {
    console.error('EVAL SOURCE RED: ' + p + ' is not an assertion object keyed by event id'); process.exit(2);
  }
  const out = new Map();
  for (const [id, v] of Object.entries(obj)) {
    if (!v || typeof v.expect !== 'string') { console.error(`EVAL SOURCE RED: assertion ${id} carries no expect string`); process.exit(2); }
    out.set(id, v.expect);
  }
  return out;
}

// ---------- mapping MR-1..MR-5 ----------
function mapStream(file, vocab, labels) {
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
        state: vocab.statuses[0], turns: [], turnObs: [], decisions: [], hint: null, hintPos: -1,
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
      const turnId = String(e.id || `${iv.task_id}#${iv.turns.length}`);
      iv.turns.push(turnId);
      iv.turnObs.push({ id: turnId, observed: String(e.decision || '') });
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

  for (const iv of intervals) { joinEvals(iv, labels); resolveInterval(iv, vocab); } // MR-3..MR-5
  const counters = { lines: lines.length, badLines, intentLines, totalTurnStops, scopedTurnStops, orphanTurns, orphanEvents, scopedEvents };
  return { intervals, counters };
}

// eval join: per-interval coverage over in-scope turns only. Ids asserted
// in the corpus but absent from this stream are corpus-side facts, never
// counted here (no fabrication path).
function joinEvals(iv, labels) {
  const cov = { matched: 0, conflict: 0, unasserted: 0 };
  if (labels) {
    for (const t of iv.turnObs) {
      if (!labels.has(t.id)) cov.unasserted++;
      else if (labels.get(t.id) === t.observed) cov.matched++;
      else cov.conflict++;
    }
  } else for (const t of iv.turnObs) cov.unasserted++;
  iv.eval = cov;
}

function resolveInterval(iv, vocab) {
  const [open, active, completed, failed, abandoned, blocked, unknown] = vocab.statuses;
  const done = iv.turns.length > 0;
  const lastDecision = iv.decisions.length ? iv.decisions[iv.decisions.length - 1] : null;
  const hintIsDone = iv.hint && HINT_DONE.test(iv.hint) && !HINT_FAIL.test(iv.hint);
  const hintIsFail = iv.hint && HINT_FAIL.test(iv.hint);
  // MR-3 evidence, two sources, both earned-not-assumed:
  //   later_turn    — a turn.stop strictly after the hint position;
  //   eval_assertion — asserted in-scope turns with zero conflicts (a
  //                   mismatched assertion never promotes).
  const laterTurnEvidence = Boolean(iv.hintHadLaterTurn);
  const evalEvidence = Boolean(iv.eval && iv.eval.matched >= 1 && iv.eval.conflict === 0);
  const hintEvidence = laterTurnEvidence || evalEvidence;
  iv.evidence = laterTurnEvidence ? 'later_turn' : evalEvidence ? 'eval_assertion' : 'none';
  // MR-4 safety tail: last in-scope decision would_block wins unless a
  // terminal candidate with outcome evidence already closed the interval.
  if (hintIsDone && hintEvidence && lastDecision !== 'would_block') { iv.state = completed; return; }
  if (hintIsFail && hintEvidence) { iv.state = failed; return; }
  if (lastDecision === 'would_block') { iv.evidence = 'none'; iv.state = blocked; return; }
  if (iv.hint) { iv.state = unknown; return; } // candidate without sufficient evidence (conflict or unverifiable wording) stays unknown
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
    // eval join conservation: every in-scope turn is exactly one of
    // matched/conflict/unasserted, and no terminal promotion rides on
    // zero evidence.
    let covSum = 0;
    for (const iv of intervals) {
      covSum += iv.eval.matched + iv.eval.conflict + iv.eval.unasserted;
      if (iv.eval.matched + iv.eval.conflict + iv.eval.unasserted !== iv.turns.length) {
        v.push(`${file}: eval coverage on ${iv.task_id} ${iv.eval.matched}+${iv.eval.conflict}+${iv.eval.unasserted} != ${iv.turns.length} turns`);
      }
      const terminalEarned = iv.state === vocab.statuses[2] || iv.state === vocab.statuses[3];
      if (terminalEarned && iv.evidence === 'none') {
        v.push(`${file}: ${iv.task_id} reached ${iv.state} with evidence=none (promotion requires a source)`);
      }
      if (terminalEarned && iv.turns.length === 0) {
        v.push(`${file}: ${iv.task_id} reached ${iv.state} with zero in-scope turns`);
      }
    }
    if (covSum !== counters.scopedTurnStops) {
      v.push(`${file}: eval coverage census ${covSum} != scoped turn.stops ${counters.scopedTurnStops}`);
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
  const earned = all.filter((iv) => iv.state === completed || iv.state === failed);
  const evidence_sources = { later_turn: 0, eval_assertion: 0 };
  for (const iv of earned) if (iv.evidence in evidence_sources) evidence_sources[iv.evidence]++;
  const cov = { matched: 0, conflict: 0, unasserted: 0 };
  for (const iv of all) { cov.matched += iv.eval.matched; cov.conflict += iv.eval.conflict; cov.unasserted += iv.eval.unasserted; }
  const turnsTotal = cov.matched + cov.conflict + cov.unasserted;
  const fmt = (x) => (denominator === 0 ? 'n/a (empty denominator)' : `${x}/${denominator}`);
  return {
    total_intervals: all.length,
    per_state: c,
    dr1: { denominator, live_excluded: c[open] + c[active], posture: 'terminal intervals (incl. terminal unknown) count once; live intervals are reported beside the ratio, never merged into it' },
    dr2: { numerator, ceiling_note: `ratio ${fmt(numerator)} — ceiling equals the unknown share (${fmt(c[unknown])}) until outcome-evidence coverage grows`, evidence_sources },
    dr3_envelope: { total: all.length, live: c[open] + c[active], unknown: c[unknown], blocked: c[blocked], printed_ratio: fmt(numerator), rule: 'a ratio printed without this envelope is an unverifiable number' },
    eval_coverage: { ...cov, turns_total: turnsTotal, rule: 'matched counts assertions whose expect equals the observed decision; conflicts never promote; coverage is corroboration visibility, not causation' },
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
  const INJECTIONS = 6;
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
    // 5b. eval join, second evidence source: hint candidate with an
    // asserted matched turn and NO later turn promotes as eval_assertion.
    {
      const jf = path.join(dir, 'eval-join.jsonl');
      writeFileSync(jf, [ev('session.start', 'e1', 'E'), ev('turn.stop', 'e2', 'E'),
        JSON.stringify({ goal: 'the release slice is complete' })].join('\n') + '\n');
      const lf = path.join(dir, 'eval-join-labels.json');
      writeFileSync(lf, JSON.stringify({ e2: { expect: 'allow', class: 'behavior', dim: 'agent_behavior' } }));
      const t6 = mapFile(jf, vocab, importEvals(lf, dir));
      const iv6 = t6.intervals[0];
      expectTrue('eval-asserted candidate promotes without later turn', iv6.state === vocab.statuses[2]);
      expectTrue('promotion tagged eval_assertion', iv6.evidence === 'eval_assertion');
      expectTrue('coverage counts the matched turn', iv6.eval.matched === 1 && iv6.eval.conflict === 0);
      // conflict never promotes: same stream, mismatched assertion
      const lf2 = path.join(dir, 'eval-join-conflict.json');
      writeFileSync(lf2, JSON.stringify({ e2: { expect: 'would_block', class: 'behavior', dim: 'agent_behavior' } }));
      const t7 = mapFile(jf, vocab, importEvals(lf2, dir));
      expectTrue('conflicting assertion keeps candidate unknown', t7.intervals[0].state === vocab.statuses[6]);
      expectTrue('conflict is accounted in coverage', t7.intervals[0].eval.conflict === 1);
      // fabricated ids asserted but not present: zero join, no crash
      const lf3 = path.join(dir, 'eval-join-phantom.json');
      writeFileSync(lf3, JSON.stringify({ e2: { expect: 'allow' }, 'zz-phantom-9': { expect: 'allow' } }));
      const t8 = mapFile(jf, vocab, importEvals(lf3, dir));
      expectTrue('phantom assertions stay outside interval coverage', t8.intervals[0].eval.matched === 1 && t8.intervals[0].eval.unasserted === 0);
    }
    // 5c. negative control: promotion stamped without any evidence source
    expectRed('earned terminal without evidence source', () => {
      const t = mapFile(gf, vocab);
      t.intervals[1].state = vocab.statuses[2]; // B:1 (no turns) forged completed
      t.intervals[1].evidence = 'none';
      gateOrThrow(new Map([[gf, t]]), vocab);
    });
    // 5d. negative control: eval coverage census tampering trips the gate
    expectRed('eval coverage census drift', () => {
      const t = mapFile(gf, vocab, new Map([['a2', 'allow']]));
      t.intervals[0].eval.matched += 1; // inflate without a matching turn
      gateOrThrow(new Map([[gf, t]]), vocab);
    });
    // 6. real corpora must pass the conservation gate untouched (single source)
    for (const f of ['testdata/golden/normal.jsonl', 'testdata/golden/danger.jsonl']) {
      const p = path.join(root, f);
      const t = mapFile(p, vocab);
      gateOrThrow(new Map([[p, t]]), vocab);
    }
    // 6b. real corpora joined against the shipped assertion corpus must
    // stay conservation-green and demonstrate live join teeth (at least
    // one in-scope turn is asserted and matched — a floor, not a census).
    {
      const labels = importEvals('testdata/golden/labels.json', root);
      const m = new Map();
      for (const f of ['testdata/golden/normal.jsonl', 'testdata/golden/danger.jsonl']) {
        const p = path.join(root, f);
        m.set(p, mapFile(p, vocab, labels));
      }
      gateOrThrow(m, vocab);
      const rd = readings(m, vocab);
      expectTrue('real-corpus join has teeth (matched >= 1)', rd.eval_coverage.matched >= 1);
      expectTrue('real-corpus join stays conflict-free on shipped labels', rd.eval_coverage.conflict === 0);
    }
  } finally {
    rmSync(dir, { recursive: true, force: true });
  }
  const ok = caught === INJECTIONS;
  console.log(`SELFTEST ${ok ? 'GREEN' : 'RED'} (${caught}/${INJECTIONS} injections caught: ${caughtNames.join('|')} | missed: ${missedNames.join('|') || 'none'}; ${assertions} assertions run)`);
  return ok ? 0 : 1;
}

// helpers used by both main and selftest
function mapFile(f, vocab, labels) {
  return mapStream(f, vocab, labels);
}
function gateOrThrow(results, vocab) {
  const g = checkConservation(results, vocab);
  if (g.violations.length) throw new Error('conservation gate RED:\n  ' + g.violations.join('\n  '));
}

// ---------- main ----------
const argv = process.argv.slice(2);
if (argv[0] === '--selftest') process.exit(selftest(importVocab()));
let labels = null;
const files = [];
for (let i = 0; i < argv.length; i++) {
  if (argv[i] === '--evals') { labels = importEvals(argv[++i], root); continue; }
  files.push(argv[i]);
}
if (!files.length) { console.error('usage: task-map.mjs <file.jsonl ...> [--evals <labels.json>] | --selftest'); process.exit(2); }
const vocab = importVocab();
const results = new Map();
for (const f of files) results.set(f, mapFile(path.resolve(root, f), vocab, labels));
const g = checkConservation(results, vocab);
if (g.violations.length) { console.error('CONSERVATION GATE RED:\n  ' + g.violations.join('\n  ')); process.exit(1); }
const rd = readings(results, vocab);
const table = [...results.values()].flatMap((r) => r.intervals).map((iv) => ({
  task_id: iv.task_id, state: iv.state, turns: iv.turns.length, boundary: iv.boundary,
  hint: iv.hint || null, decisions: iv.decisions, evidence: iv.evidence, eval: iv.eval,
}));
console.log(JSON.stringify({ derive_only: true, emitted: false, vocabulary_source: 'docs/task-primitive-design.md', eval_join: labels ? 'active' : 'single-source', tasks: table, readings: rd, conservation: 'GREEN' }, null, 2));
