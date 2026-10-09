// deathline-check.mjs — machine gate for the death-line monitor slice.
//
// Two jobs, both offline and deterministic:
//   1. doc<->code sync: the criteria table in docs/death-line-monitor.md and
//      the DEADLINE constants used by the readings collector must agree
//      token for token (dates, every metric threshold, every continuation
//      rule). Neither side may drift silently.
//   2. ledger consistency: every row of docs/death-line-ledger.csv must be
//      machine-re-derivable — header exact, dates strictly ascending and
//      unique, uv/uv_status honest pairing, hits_a/verdict_a recomputed from
//      the constants matching the stored values. A hand-typed number fails.
//
// usage: node scripts/deathline-check.mjs [--doc <path>] [--ledger <path>]
//        node scripts/deathline-check.mjs --selftest
import fs from 'node:fs';
import os from 'node:os';
import path from 'node:path';
import { fileURLToPath } from 'node:url';

import {
  DEADLINE,
  CSV_HEADER,
  deriveWindowA,
  buildRow,
  applyRow,
} from './deathline-readings.mjs';

const HERE = path.dirname(fileURLToPath(import.meta.url));
const DEFAULT_DOC = path.join(HERE, '..', 'docs', 'death-line-monitor.md');
const DEFAULT_LEDGER = path.join(HERE, '..', 'docs', 'death-line-ledger.csv');

const problems = [];
const bad = (m) => problems.push(m);

function parseWindowRow(line, wantName) {
  if (!new RegExp(`\\|\\s*window ${wantName}\\s*\\|`).test(line)) return null;
  const dm = line.match(/deadline (\d{4}-\d{2}-\d{2})/);
  if (!dm) { bad(`window ${wantName}: no deadline date parsed`); return null; }
  const tokens = {};
  for (const m of line.matchAll(/([a-z_]+) >= (\d+)/g)) tokens[m[1]] = Number(m[2]);
  const minHits = tokens.hits;
  delete tokens.hits;
  return { deadline: dm[1], metrics: tokens, minHits };
}

function checkDocSync(docText) {
  const t0 = docText.match(/T0 = (\d{4}-\d{2}-\d{2})/);
  if (!t0) bad('doc: no machine line "T0 = YYYY-MM-DD" found');
  else if (t0[1] !== DEADLINE.t0) bad(`doc T0 ${t0[1]} != code ${DEADLINE.t0}`);

  for (const [name, codeWin] of [['A', DEADLINE.windowA], ['B', DEADLINE.windowB]]) {
    const line = docText.split('\n').find((l) => new RegExp(`\\|\\s*window ${name}\\s*\\|`).test(l));
    if (!line) { bad(`doc: window ${name} row missing`); continue; }
    const docWin = parseWindowRow(line, name);
    if (!docWin) continue;
    if (docWin.deadline !== codeWin.deadline) bad(`doc window ${name} deadline ${docWin.deadline} != code ${codeWin.deadline}`);
    if (docWin.minHits !== codeWin.minHits) bad(`doc window ${name} hits ${docWin.minHits} != code minHits ${codeWin.minHits}`);
    const keysDoc = Object.keys(docWin.metrics).sort().join(',');
    const keysCode = Object.keys(codeWin.metrics).sort().join(',');
    if (keysDoc !== keysCode) bad(`doc window ${name} metric set [${keysDoc}] != code [${keysCode}]`);
    for (const k of keysCode.split(',')) {
      if (docWin.metrics[k] !== undefined && codeWin.metrics[k] !== undefined && docWin.metrics[k] !== codeWin.metrics[k]) {
        bad(`doc window ${name} ${k}: doc ${docWin.metrics[k]} != code ${codeWin.metrics[k]}`);
      }
    }
  }
}

function checkLedger(ledgerText) {
  const lines = ledgerText.split(/\r?\n/).filter((l) => l.length);
  if (!lines.length) { bad('ledger: empty'); return; }
  if (lines[0] !== CSV_HEADER) { bad(`ledger: header drift [${lines[0]}]`); return; }
  const rows = lines.slice(1);
  if (!rows.length) bad('ledger: no reading rows yet (collect one)');
  let prevDate = '';
  for (let i = 0; i < rows.length; i++) {
    const c = rows[i].split(',');
    const at = `ledger row ${i + 1}`;
    if (c.length !== 7) { bad(`${at}: ${c.length} cells, want 7`); continue; }
    const [date, downloads, stars, uv, uvStatus, hitsA, verdictA] = c;
    if (!/^\d{4}-\d{2}-\d{2}$/.test(date)) bad(`${at}: bad date ${date}`);
    if (prevDate && date <= prevDate) bad(`${at}: date ${date} not strictly after ${prevDate}`);
    prevDate = date;
    if (!/^\d+$/.test(downloads)) bad(`${at}: downloads not an integer: ${downloads}`);
    if (!/^\d+$/.test(stars)) bad(`${at}: stars not an integer: ${stars}`);
    if (uvStatus === 'UNAVAILABLE' && uv !== 'NA') bad(`${at}: UNAVAILABLE must carry NA, got ${uv}`);
    if (uvStatus === 'MEASURED' && !/^\d+$/.test(uv)) bad(`${at}: MEASURED must carry an integer, got ${uv}`);
    if (uvStatus !== 'MEASURED' && uvStatus !== 'UNAVAILABLE') bad(`${at}: unknown uv_status ${uvStatus}`);
    const d = deriveWindowA({ downloads, stars, uv, uv_status: uvStatus });
    if (d.hits_a !== hitsA) bad(`${at}: stored hits_a ${hitsA} != re-derived ${d.hits_a}`);
    if (d.verdict_a !== verdictA) bad(`${at}: stored verdict_a ${verdictA} != re-derived ${d.verdict_a}`);
  }
}

export function checkAll(docText, ledgerText) {
  problems.length = 0;
  checkDocSync(docText);
  checkLedger(ledgerText);
  return problems.slice();
}

function runCli(argv) {
  const flag = (n) => argv.includes('--' + n);
  const opt = (n, d) => { const i = argv.indexOf('--' + n); return i >= 0 && argv[i + 1] !== undefined ? argv[i + 1] : d; };
  const probs = checkAll(fs.readFileSync(opt('doc', DEFAULT_DOC), 'utf8'), fs.readFileSync(opt('ledger', DEFAULT_LEDGER), 'utf8'));
  if (probs.length) { for (const p of probs) console.error('DEATHLINE CHECK RED: ' + p); return 1; }
  console.log('DEATHLINE CHECK GREEN (doc<->code sync + ledger re-derivation)');
  return 0;
}

function selftest() {
  const fails = [];
  let controls = 0;
  const wantRed = (name, fn) => {
    controls++;
    let caught = false;
    try { caught = fn(); } catch { caught = 'throw'; }
    if (!caught) fails.push(name + ' (injection NOT caught)');
  };
  const dir = fs.mkdtempSync(path.join(os.tmpdir(), 'deathline-check-selftest-'));
  const doc0 = fs.readFileSync(DEFAULT_DOC, 'utf8');
  const row1 = buildRow({ date: '2026-10-09', downloads: 53, stars: 1, uv: 'NA', uv_status: 'UNAVAILABLE' });
  const row2 = buildRow({ date: '2026-10-16', downloads: 1001, stars: 1, uv: '6000', uv_status: 'MEASURED' });
  const ledger0 = applyRow(applyRow(CSV_HEADER + '\n', row1, false).text, row2, false).text;

  const isRed = (doc, ledger) => checkAll(doc, ledger).length > 0;

  // pristine pair must be green (positive control for the harness itself)
  controls++;
  if (isRed(doc0, ledger0)) { fails.push('pristine pair unexpectedly RED: ' + checkAll(doc0, ledger0).join(' | ')); }

  // doc-side injections
  wantRed('doc threshold drift', () => isRed(doc0.replace('downloads >= 1000', 'downloads >= 1001'), ledger0));
  wantRed('doc deadline drift', () => isRed(doc0.replace('deadline 2026-10-28', 'deadline 2026-10-27'), ledger0));
  wantRed('doc metric token removed', () => isRed(doc0.replace('intent >= 50', 'intent off'), ledger0));
  wantRed('doc continuation rule drift', () => isRed(doc0.replace('hits >= 2', 'hits >= 3'), ledger0));
  wantRed('doc T0 drift', () => isRed(doc0.replace('T0 = 2026-09-28', 'T0 = 2026-09-29'), ledger0));

  // ledger-side injections (hand-edited rows must never re-derive silently)
  wantRed('ledger hits tampered', () => isRed(doc0, ledger0.replace(',6000,MEASURED,2', ',6000,MEASURED,3')));
  const liar = row1.replace(',NA,UNAVAILABLE,', ',0,UNAVAILABLE,');
  wantRed('ledger fabricated zero under UNAVAILABLE', () => isRed(doc0, CSV_HEADER + '\n' + liar + '\n'));
  wantRed('ledger duplicate date', () => isRed(doc0, ledger0 + row2 + '\n'));
  wantRed('ledger header drift', () => isRed(doc0, ledger0.replace(CSV_HEADER, 'date,downloads,stars')));
  wantRed('ledger empty body', () => isRed(doc0, CSV_HEADER + '\n'));

  if (fails.length) {
    for (const f of fails) console.error('SELFTEST RED: ' + f);
    return 1;
  }
  console.log(`DEATHLINE CHECK SELFTEST OK (${controls} controls, scratch=${dir})`);
  return 0;
}

const isMain = process.argv[1] && path.resolve(process.argv[1]) === fileURLToPath(import.meta.url);
if (isMain) {
  const argv = process.argv.slice(2);
  process.exit(argv.includes('--selftest') ? selftest() : runCli(argv));
}
