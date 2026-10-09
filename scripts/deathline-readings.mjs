// deathline-readings.mjs — collector that appends machine-derived survival
// readings (downloads, stars, optional uv) to docs/death-line-ledger.csv.
//
// Design contract (mirrors docs/death-line-monitor.md):
//   * every number comes from the live read-only GitHub API or from an
//     explicit operator argument; nothing is hand-typed into the ledger;
//   * a metric with no machine source is written as NA + UNAVAILABLE and
//     never counts as a hit and never as a fabricated zero;
//   * the ledger is append-only with strictly ascending unique dates;
//     re-reading the same day is refused unless --force replaces that row;
//   * hits/verdict columns are derived here (single derivation point) and
//     re-derived independently by deathline-check.mjs on every gate run.
//
// usage:
//   node scripts/deathline-readings.mjs --repo <owner>/<name> [--uv <int>|na]
//       [--date YYYY-MM-DD] [--ledger <path>] [--doc <path>] [--dry-run]
//       [--json] [--force]
//   node scripts/deathline-readings.mjs --selftest
import fs from 'node:fs';
import os from 'node:os';
import path from 'node:path';
import { fileURLToPath } from 'node:url';

export const DEADLINE = {
  t0: '2026-09-28',
  windowA: {
    deadline: '2026-10-28',
    metrics: { downloads: 1000, uv: 5000, stars: 100 },
    minHits: 2,
  },
  windowB: {
    deadline: '2026-11-27',
    metrics: { stars: 300, engagement: 15, intent: 50 },
    minHits: 1,
  },
};

export const CSV_HEADER = 'date,downloads,stars,uv,uv_status,hits_a,verdict_a';

const DATE_RE = /^\d{4}-\d{2}-\d{2}$/;
const INT_RE = /^\d+$/;

export function parseUv(raw) {
  if (raw === undefined || raw === null || raw === 'na') {
    return { uv: 'NA', uv_status: 'UNAVAILABLE' };
  }
  const s = String(raw).trim();
  if (!INT_RE.test(s)) throw new Error(`bad --uv value: ${s}`);
  return { uv: s, uv_status: 'MEASURED' };
}

// deriveWindowA is the single source of derived columns. uv only counts when
// MEASURED; UNAVAILABLE is an honest absence, not a zero.
export function deriveWindowA({ downloads, stars, uv, uv_status }) {
  const a = DEADLINE.windowA.metrics;
  let hits = 0;
  if (Number(downloads) >= a.downloads) hits++;
  if (Number(stars) >= a.stars) hits++;
  if (uv_status === 'MEASURED' && Number(uv) >= a.uv) hits++;
  return { hits_a: String(hits), verdict_a: hits >= DEADLINE.windowA.minHits ? 'PASS' : 'FAIL' };
}

export function buildRow({ date, downloads, stars, uv, uv_status }) {
  if (!DATE_RE.test(date)) throw new Error(`bad date: ${date}`);
  if (!INT_RE.test(String(downloads))) throw new Error(`downloads must be an integer: ${downloads}`);
  if (!INT_RE.test(String(stars))) throw new Error(`stars must be an integer: ${stars}`);
  if (uv_status === 'UNAVAILABLE' && uv !== 'NA') throw new Error('UNAVAILABLE must carry NA');
  if (uv_status === 'MEASURED' && !INT_RE.test(String(uv))) throw new Error('MEASURED must carry an integer');
  const d = deriveWindowA({ downloads, stars, uv, uv_status });
  return [date, String(downloads), String(stars), String(uv), uv_status, d.hits_a, d.verdict_a].join(',');
}

export function readLedger(text) {
  const lines = text.split(/\r?\n/).filter((l) => l.length);
  if (!lines.length) throw new Error('ledger is empty (header missing)');
  if (lines[0] !== CSV_HEADER) throw new Error(`ledger header drift: ${lines[0]}`);
  return lines.slice(1);
}

// validateAppend enforces append-only shape: strictly ascending unique dates.
// Returns {ok, reason, sameDateIndex}.
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
    if (cells.length !== 7) return { ok: false, reason: `row ${i + 1} has ${cells.length} cells, want 7` };
    if (!DATE_RE.test(cells[0])) return { ok: false, reason: `row ${i + 1} bad date ${cells[0]}` };
    if (cells[0] === date) sameDateIndex = i;
    lastDate = cells[0];
    if (i > 0) {
      const prev = rows[i - 1].split(',')[0];
      if (cells[0] <= prev) return { ok: false, reason: `date order violated: ${cells[0]} after ${prev}` };
    }
  }
  if (sameDateIndex >= 0) {
    return { ok: false, reason: `date ${date} already present (use --force to re-read it)`, sameDateIndex };
  }
  if (rows.length && date < lastDate) {
    return { ok: false, reason: `back-dating refused: ${date} is older than last row ${lastDate}` };
  }
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

// ---- live collection (read-only public API, no credentials required) ----

async function ghJson(pth) {
  const r = await fetch('https://api.github.com' + pth, {
    headers: { accept: 'application/vnd.github+json', 'user-agent': 'deathline-readings' },
  });
  if (!r.ok) throw new Error(`github api ${pth} -> HTTP ${r.status}`);
  return r.json();
}

export async function collectLive(repo) {
  if (!/^[^/\s]+\/[^/\s]+$/.test(repo)) throw new Error(`bad --repo: ${repo}`);
  const meta = await ghJson(`/repos/${repo}`);
  const releases = await ghJson(`/repos/${repo}/releases?per_page=100`);
  if (!Array.isArray(releases)) throw new Error('unexpected releases shape');
  let downloads = 0;
  for (const rel of releases) for (const a of rel.assets || []) downloads += Number(a.download_count || 0);
  return { downloads, stars: Number(meta.stargazers_count) };
}

// ---- selftest: offline, deterministic, positive controls on every guard ----

function selftest() {
  const fails = [];
  let controls = 0; // run-time derived, never a literal count
  const want = (name, cond) => { controls++; if (!cond) fails.push(name); };
  const wantThrow = (name, fn) => { controls++; try { fn(); fails.push(name + ' (no throw)'); } catch { /* expected */ } };

  // derivation controls
  const d1 = deriveWindowA({ downloads: 1000, stars: 100, uv: 'NA', uv_status: 'UNAVAILABLE' });
  want('two live hits pass', d1.hits_a === '2' && d1.verdict_a === 'PASS');
  const d2 = deriveWindowA({ downloads: 999, stars: 99, uv: 'NA', uv_status: 'UNAVAILABLE' });
  want('below thresholds fail', d2.hits_a === '0' && d2.verdict_a === 'FAIL');
  const d3 = deriveWindowA({ downloads: 0, stars: 0, uv: '5000', uv_status: 'MEASURED' });
  want('measured uv counts', d3.hits_a === '1');
  const d4 = deriveWindowA({ downloads: 0, stars: 0, uv: 'NA', uv_status: 'UNAVAILABLE' });
  want('unavailable uv never a hit', d4.hits_a === '0');

  // row shape guards
  wantThrow('bad date rejected', () => buildRow({ date: '2026-13-99x', downloads: 1, stars: 1, uv: 'NA', uv_status: 'UNAVAILABLE' }));
  wantThrow('non-integer downloads rejected', () => buildRow({ date: '2026-10-09', downloads: '53.5', stars: 1, uv: 'NA', uv_status: 'UNAVAILABLE' }));
  wantThrow('MEASURED must carry integer', () => buildRow({ date: '2026-10-09', downloads: 1, stars: 1, uv: 'NA', uv_status: 'MEASURED' }));
  wantThrow('UNAVAILABLE must carry NA', () => buildRow({ date: '2026-10-09', downloads: 1, stars: 1, uv: '0', uv_status: 'UNAVAILABLE' }));
  wantThrow('parseUv rejects junk', () => parseUv('five'));

  // append/replace ledger controls
  const rowA = buildRow({ date: '2026-10-09', downloads: 53, stars: 1, uv: 'NA', uv_status: 'UNAVAILABLE' });
  const rowB = buildRow({ date: '2026-10-16', downloads: 60, stars: 2, uv: '120', uv_status: 'MEASURED' });
  const empty = CSV_HEADER + '\n';
  const s1 = applyRow(empty, rowA, false);
  want('first append works', s1.action === 'append' && s1.text.split('\n').filter(Boolean).length === 2);
  const s2 = applyRow(s1.text, rowB, false);
  want('second append works', s2.action === 'append');
  wantThrow('duplicate date refused', () => applyRow(s2.text, rowB, false));
  const s3 = applyRow(s2.text, rowB, true);
  want('force replaces', s3.action === 'replace' && s3.text.includes('60,2,120'));
  wantThrow('back-date refused', () => applyRow(s2.text, buildRow({ date: '2026-10-01', downloads: 1, stars: 1, uv: 'NA', uv_status: 'UNAVAILABLE' }), false));
  want('broken header refused', validateAppend('date,downloads\n', rowA).ok === false);
  const badShape = CSV_HEADER + '\n2026-10-09,53,1,NA\n';
  want('short row refused', validateAppend(badShape, rowB).ok === false);

  // scratch hygiene: never assume a temp parent exists
  const dir = fs.mkdtempSync(path.join(os.tmpdir(), 'deathline-selftest-'));
  want('scratch dir usable', fs.existsSync(dir));

  if (fails.length) {
    for (const f of fails) console.error('SELFTEST RED: ' + f);
    process.exit(1);
  }
  console.log(`DEATHLINE READINGS SELFTEST OK (${controls} controls, offline, scratch=${dir})`);
}

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
  const repo = opt('repo');
  const date = opt('date', new Date().toISOString().slice(0, 10));
  const ledgerPath = opt('ledger', path.join(path.dirname(fileURLToPath(import.meta.url)), '..', 'docs', 'death-line-ledger.csv'));
  let uv;
  try { uv = parseUv(opt('uv', 'na')); } catch (e) { console.error('RED: ' + e.message); process.exit(1); }
  let live;
  try { live = await collectLive(repo || ''); } catch (e) {
    console.error('RED: live collection failed — no row is written, no zero is fabricated: ' + e.message);
    process.exit(1);
  }
  let row;
  try { row = buildRow({ date, downloads: live.downloads, stars: live.stars, ...uv }); } catch (e) { console.error('RED: ' + e.message); process.exit(1); }
  if (flag('dry-run')) { console.log(row); process.exit(0); }
  const cur = fs.existsSync(ledgerPath) ? fs.readFileSync(ledgerPath, 'utf8') : CSV_HEADER + '\n';
  let out;
  try { out = applyRow(cur, row, flag('force')); } catch (e) { console.error('RED: ' + e.message); process.exit(1); }
  fs.writeFileSync(ledgerPath, out.text);
  console.log(`OK ${out.action} ${row} -> ${ledgerPath}`);
  if (flag('json')) console.log(JSON.stringify({ date, ...live, ...uv, hits_a: row.split(',')[5], verdict_a: row.split(',')[6] }));
}
