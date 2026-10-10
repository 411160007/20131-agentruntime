// northstar-readings.mjs — collector that appends machine-derived North Star
// readings (Safe Agent Task Success Rate v0) to docs/north-star-ledger.csv.
//
// Design contract (mirrors docs/task-primitive-design.md section 6):
//   * every number comes from one live run of the derive-only mapper
//     (scripts/task-map.mjs) over the shipped rules-corpus streams and the
//     shipped assertion corpus; nothing is hand-typed into the ledger;
//   * DR-3 posture: the printed ratio always ships with its honesty
//     envelope (total, live, unknown, blocked). A ledger row whose ratio
//     cannot be earned is written as NA + UNAVAILABLE — never as a
//     fabricated zero, and a ratio without an envelope is an unverifiable
//     number;
//   * DR-1 posture: the denominator counts terminal intervals only; live
//     intervals are reported beside the ratio (separate column), never
//     merged into it;
//   * DR-2 posture: the numerator counts earned completions only; the
//     evidence-source split (later_turn, eval_assertion) is recorded so a
//     numerator with no sources on disk reads as a regression;
//   * the collector re-derives the ratio string independently of the
//     mapper; any drift between the two derivations refuses the row;
//   * the ledger is append-only with strictly ascending unique dates;
//     re-reading the same day is refused unless --force replaces that row;
//   * zero emission: this tool never writes into any stream and never
//     touches the mapper or the corpora (single write surface: the ledger).
//
// usage:
//   node scripts/northstar-readings.mjs [--date YYYY-MM-DD]
//       [--ledger <path>] [--dry-run] [--json] [--force]
//   node scripts/northstar-readings.mjs --selftest
import fs from 'node:fs';
import os from 'node:os';
import path from 'node:path';
import { execFileSync } from 'node:child_process';
import { fileURLToPath } from 'node:url';

const HERE = path.dirname(fileURLToPath(import.meta.url));
const ROOT = path.resolve(HERE, '..');

// Read surface = the shipped rules-corpus streams joined against the
// shipped assertion corpus (paths, not numbers: nothing here copies a
// count; every quantity is re-derived from disk on each run).
const RULES_STREAMS = [
  path.join(ROOT, 'testdata', 'golden', 'normal.jsonl'),
  path.join(ROOT, 'testdata', 'golden', 'danger.jsonl'),
];
const EVALS_CORPUS = path.join(ROOT, 'testdata', 'golden', 'labels.json');
const MAPPER = path.join(HERE, 'task-map.mjs');

export const CSV_HEADER =
  'date,numerator,denominator,printed_ratio,total_intervals,live,unknown,blocked,evidence_later_turn,evidence_eval_assertion,ratio_status';
export const N_COLS = CSV_HEADER.split(',').length;

const DATE_RE = /^\d{4}-\d{2}-\d{2}$/;
const INT_RE = /^\d+$/;

// ---- live derivation (offline: reads shipped files through the mapper) ----

export function runMapper(streams, evals) {
  const out = execFileSync(process.execPath, [MAPPER, ...streams, '--evals', evals], {
    encoding: 'utf8',
    maxBuffer: 64 * 1024 * 1024,
  });
  const parsed = JSON.parse(out);
  if (!parsed || parsed.derive_only !== true || parsed.conservation !== 'GREEN' || !parsed.readings) {
    throw new Error('mapper output shape unexpected (derive_only/conservation/readings)');
  }
  return parsed.readings;
}

// readingsToCells maps the mapper readings onto ledger columns and
// independently re-derives the ratio string (double derivation guard).
export function readingsToCells(rd) {
  const denom = rd.dr1.denominator;
  const num = rd.dr2.numerator;
  const src = rd.dr2.evidence_sources || {};
  if (denom === 0) {
    // empty denominator: honest absence, never a fabricated zero ratio
    return {
      numerator: 'NA', denominator: 'NA', printed_ratio: 'NA',
      total_intervals: String(rd.total_intervals),
      live: String(rd.dr1.live_excluded),
      unknown: 'NA', blocked: 'NA',
      evidence_later_turn: 'NA', evidence_eval_assertion: 'NA',
      ratio_status: 'UNAVAILABLE',
    };
  }
  const ratio = `${num}/${denom}`;
  if (rd.dr3_envelope.printed_ratio !== ratio) {
    throw new Error(`ratio derivation drift: mapper printed ${rd.dr3_envelope.printed_ratio}, collector re-derived ${ratio}`);
  }
  if (rd.dr3_envelope.total !== rd.dr1.live_excluded + rd.dr1.denominator) {
    throw new Error(`envelope identity violated: total ${rd.dr3_envelope.total} != live ${rd.dr1.live_excluded} + denominator ${rd.dr1.denominator}`);
  }
  if (num > denom) throw new Error(`numerator ${num} exceeds denominator ${denom}`);
  return {
    numerator: String(num), denominator: String(denom), printed_ratio: ratio,
    total_intervals: String(rd.dr3_envelope.total),
    live: String(rd.dr3_envelope.live),
    unknown: String(rd.dr3_envelope.unknown),
    blocked: String(rd.dr3_envelope.blocked),
    evidence_later_turn: String(src.later_turn ?? 0),
    evidence_eval_assertion: String(src.eval_assertion ?? 0),
    ratio_status: 'DERIVED',
  };
}

export function buildRow({ date, c }) {
  if (!DATE_RE.test(date)) throw new Error(`bad date: ${date}`);
  const cells = [
    date, c.numerator, c.denominator, c.printed_ratio, c.total_intervals,
    c.live, c.unknown, c.blocked, c.evidence_later_turn, c.evidence_eval_assertion, c.ratio_status,
  ];
  if (c.ratio_status === 'UNAVAILABLE') {
    for (const k of ['numerator', 'denominator', 'printed_ratio', 'unknown', 'blocked', 'evidence_later_turn', 'evidence_eval_assertion']) {
      if (c[k] !== 'NA') throw new Error(`UNAVAILABLE must carry NA in ${k}`);
    }
    if (!INT_RE.test(String(c.total_intervals)) || !INT_RE.test(String(c.live))) {
      throw new Error('UNAVAILABLE rows still carry machine-derived total/live integers');
    }
  } else if (c.ratio_status === 'DERIVED') {
    for (const v of [c.numerator, c.denominator, c.total_intervals, c.live, c.unknown, c.blocked, c.evidence_later_turn, c.evidence_eval_assertion]) {
      if (!INT_RE.test(String(v))) throw new Error(`DERIVED rows must carry integers, got: ${v}`);
    }
    if (Number(c.numerator) > Number(c.denominator)) throw new Error('numerator exceeds denominator');
    if (Number(c.total_intervals) !== Number(c.live) + Number(c.denominator)) throw new Error('envelope identity violated in row');
    if (c.printed_ratio !== `${c.numerator}/${c.denominator}`) throw new Error('ratio string does not match its own cells');
  } else {
    throw new Error(`unknown ratio_status: ${c.ratio_status}`);
  }
  return cells.join(',');
}

export function readLedger(text) {
  const lines = text.split(/\r?\n/).filter((l) => l.length);
  if (!lines.length) throw new Error('ledger is empty (header missing)');
  if (lines[0] !== CSV_HEADER) throw new Error(`ledger header drift: ${lines[0]}`);
  return lines.slice(1);
}

// validateAppend enforces append-only shape: strictly ascending unique dates.
export function validateAppend(ledgerText, row) {
  let rows;
  try {
    rows = readLedger(ledgerText);
  } catch (e) {
    return { ok: false, reason: e.message };
  }
  const date = row.split(',')[0];
  let sameDateIndex = -1;
  let lastDate = '';
  for (let i = 0; i < rows.length; i++) {
    const cells = rows[i].split(',');
    if (cells.length !== N_COLS) return { ok: false, reason: `row ${i + 1} has ${cells.length} cells, want ${N_COLS}` };
    if (!DATE_RE.test(cells[0])) return { ok: false, reason: `row ${i + 1} bad date ${cells[0]}` };
    if (cells[0] === date) sameDateIndex = i;
    lastDate = cells[0];
    if (i > 0) {
      const prev = rows[i - 1].split(',')[0];
      if (cells[0] <= prev) return { ok: false, reason: `date order violated: ${cells[0]} after ${prev}` };
    }
  }
  if (sameDateIndex >= 0) return { ok: false, reason: `date ${date} already present (use --force to re-read it)`, sameDateIndex };
  if (rows.length && date < lastDate) return { ok: false, reason: `back-dating refused: ${date} is older than last row ${lastDate}` };
  return { ok: true, sameDateIndex: -1 };
}

export function applyRow(ledgerText, row, force) {
  const v = validateAppend(ledgerText, row);
  if (v.ok) return { text: ledgerText.replace(/\n*$/, '') + '\n' + row + '\n', action: 'append' };
  if (v.sameDateIndex !== undefined && v.sameDateIndex >= 0 && force) {
    const lines = ledgerText.split(/\r?\n/).filter((l) => l.length);
    lines[1 + v.sameDateIndex] = row;
    return { text: lines.join('\n') + '\n', action: 'replace' };
  }
  throw new Error('REFUSED: ' + v.reason);
}

// ---- selftest: offline, deterministic, positive controls on every guard ----

function selftest() {
  const fails = [];
  let controls = 0; // run-time derived, never a literal count
  const want = (name, cond) => { controls++; if (!cond) fails.push(name); };
  const wantThrow = (name, fn) => { controls++; try { fn(); fails.push(name + ' (no throw)'); } catch { /* expected */ } };

  const derivedCells = (n, d, extra = {}) => ({
    numerator: String(n), denominator: String(d), printed_ratio: `${n}/${d}`,
    total_intervals: String(extra.total ?? d + (extra.live ?? 0)),
    live: String(extra.live ?? 0),
    unknown: String(extra.unknown ?? 0), blocked: String(extra.blocked ?? 0),
    evidence_later_turn: String(extra.lt ?? 1), evidence_eval_assertion: String(extra.ea ?? 0),
    ratio_status: 'DERIVED',
  });

  // row-shape guards
  wantThrow('bad date rejected', () => buildRow({ date: '2026-13-99x', c: derivedCells(1, 2) }));
  wantThrow('ratio string drift rejected', () => buildRow({ date: '2026-10-11', c: { ...derivedCells(1, 2), printed_ratio: '3/2' } }));
  wantThrow('numerator>denominator rejected', () => buildRow({ date: '2026-10-11', c: derivedCells(3, 2) }));
  wantThrow('envelope identity violated rejected', () => buildRow({ date: '2026-10-11', c: derivedCells(1, 2, { total: 99 }) }));
  wantThrow('non-integer cell rejected', () => buildRow({ date: '2026-10-11', c: { ...derivedCells(1, 2), unknown: '1.5' } }));
  wantThrow('junk status rejected', () => buildRow({ date: '2026-10-11', c: { ...derivedCells(1, 2), ratio_status: 'GUESS' } }));

  // UNAVAILABLE honest-absence shape
  const naCells = {
    numerator: 'NA', denominator: 'NA', printed_ratio: 'NA',
    total_intervals: '4', live: '4', unknown: 'NA', blocked: 'NA',
    evidence_later_turn: 'NA', evidence_eval_assertion: 'NA', ratio_status: 'UNAVAILABLE',
  };
  const naRow = buildRow({ date: '2026-10-11', c: naCells });
  want('UNAVAILABLE row carries NA ratio not zero', naRow.split(',')[3] === 'NA' && naRow.split(',')[10] === 'UNAVAILABLE');
  wantThrow('UNAVAILABLE must keep NA everywhere it claims NA', () => buildRow({ date: '2026-10-11', c: { ...naCells, numerator: '0' } }));

  // readingsToCells guards (synthetic mapper readings, no literals into the ledger)
  const rd0 = readingsToCellsSafe({ total_intervals: 3, dr1: { denominator: 0, live_excluded: 3 }, dr2: { numerator: 0, evidence_sources: {} }, dr3_envelope: { total: 3, live: 3, unknown: 0, blocked: 0, printed_ratio: 'n/a (empty denominator)' } });
  want('empty denominator becomes UNAVAILABLE', rd0.ratio_status === 'UNAVAILABLE');
  wantThrow('ratio derivation drift refused', () => readingsToCells({
    total_intervals: 5, dr1: { denominator: 4, live_excluded: 1 }, dr2: { numerator: 2, evidence_sources: { later_turn: 2, eval_assertion: 0 } },
    dr3_envelope: { total: 5, live: 1, unknown: 1, blocked: 0, printed_ratio: '9/4' },
  }));
  wantThrow('envelope identity violation refused at readings layer', () => readingsToCells({
    total_intervals: 9, dr1: { denominator: 4, live_excluded: 1 }, dr2: { numerator: 2, evidence_sources: {} },
    dr3_envelope: { total: 9, live: 1, unknown: 1, blocked: 0, printed_ratio: '2/4' },
  }));

  // append/replace ledger controls
  const rowA = buildRow({ date: '2026-10-09', c: derivedCells(1, 2, { live: 3, total: 5 }) });
  const rowB = buildRow({ date: '2026-10-16', c: derivedCells(2, 4, { live: 1, total: 5 }) });
  const empty = CSV_HEADER + '\n';
  const s1 = applyRow(empty, rowA, false);
  want('first append works', s1.action === 'append' && s1.text.split('\n').filter(Boolean).length === 2);
  const s2 = applyRow(s1.text, rowB, false);
  want('second append works', s2.action === 'append');
  wantThrow('duplicate date refused', () => applyRow(s2.text, rowB, false));
  const s3 = applyRow(s2.text, rowB, true);
  want('force replaces', s3.action === 'replace' && s3.text.includes(rowB));
  wantThrow('back-date refused', () => applyRow(s2.text, buildRow({ date: '2026-10-01', c: derivedCells(1, 2) }), false));
  want('broken header refused', validateAppend('date,numerator\n', rowA).ok === false);
  want('short row refused', validateAppend(CSV_HEADER + '\n2026-10-09,1,2\n', rowB).ok === false);

  // positive control on the real read surface: the shipped corpora through
  // the shipped mapper must earn a DERIVED row end-to-end (floor, not
  // census — exact values change when the corpora grow and are never
  // hand-copied anywhere).
  let liveRow = null;
  try {
    const rd = runMapper(RULES_STREAMS, EVALS_CORPUS);
    liveRow = buildRow({ date: '2026-10-11', c: readingsToCells(rd) });
  } catch (e) {
    fails.push('live mapper derivation failed: ' + e.message);
  }
  controls++;
  if (liveRow) {
    const cells = liveRow.split(',');
    want('live row is DERIVED', cells[10] === 'DERIVED');
    want('live ratio matches its cells', cells[3] === `${cells[1]}/${cells[2]}`);
    want('live envelope identity holds', Number(cells[4]) === Number(cells[5]) + Number(cells[2]));
  }

  // scratch hygiene: never assume a temp parent exists
  const dir = fs.mkdtempSync(path.join(os.tmpdir(), 'northstar-selftest-'));
  want('scratch dir usable', fs.existsSync(dir));
  fs.rmSync(dir, { recursive: true, force: true });

  if (fails.length) {
    for (const f of fails) console.error('SELFTEST RED: ' + f);
    process.exit(1);
  }
  console.log(`NORTHSTAR READINGS SELFTEST OK (${controls} controls, live row ${liveRow ? 'earned' : 'MISSING'}, scratch offline)`);
}

function readingsToCellsSafe(rd) { try { return readingsToCells(rd); } catch { return null; } }

// ---- cli ----
const isMain = process.argv[1] && path.resolve(process.argv[1]) === fileURLToPath(import.meta.url);
if (isMain) {
  const argv = process.argv.slice(2);
  const flag = (name) => argv.includes('--' + name);
  const opt = (name, dflt) => {
    const i = argv.indexOf('--' + name);
    return i >= 0 && argv[i + 1] !== undefined ? argv[i + 1] : dflt;
  };
  if (flag('selftest')) {
    selftest();
    process.exit(0);
  }
  const date = opt('date', new Date().toISOString().slice(0, 10));
  const ledgerPath = opt('ledger', path.join(ROOT, 'docs', 'north-star-ledger.csv'));
  let rd;
  try {
    rd = runMapper(RULES_STREAMS, EVALS_CORPUS);
  } catch (e) {
    console.error('RED: live mapper derivation failed — no row is written, no zero is fabricated: ' + e.message);
    process.exit(1);
  }
  let row;
  try { row = buildRow({ date, c: readingsToCells(rd) }); } catch (e) { console.error('RED: ' + e.message); process.exit(1); }
  if (flag('dry-run')) { console.log(row); process.exit(0); }
  const cur = fs.existsSync(ledgerPath) ? fs.readFileSync(ledgerPath, 'utf8') : CSV_HEADER + '\n';
  let out;
  try { out = applyRow(cur, row, flag('force')); } catch (e) { console.error('RED: ' + e.message); process.exit(1); }
  fs.writeFileSync(ledgerPath, out.text);
  console.log(`OK ${out.action} ${row} -> ${ledgerPath}`);
  if (flag('json')) console.log(JSON.stringify({ date, row: row.split(','), readings: rd }, null, 2));
}
