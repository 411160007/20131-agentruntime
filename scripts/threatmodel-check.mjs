// threatmodel-check.mjs — bidirectional cross-check between the
// built-in rule table (parsed from internal/rules/rules.go, the
// shipped source of truth) and docs/threat-model.md.
//
// Checks:
//   1. every built-in rule id appears in the threat model (≥1 threat);
//   2. every rule id referenced in the threat model exists in the
//      built-in table (no stale/ghost references);
//   3. every threat row (TM-xx) names at least one rule id OR carries
//      an explicit known_gap marker — silence is not an option;
//   4. threat row count ≥ 10 (v0 floor) and rule count == 12;
//   5. external copy discipline: no border-device marketing word
//      (any casing) in docs/threat-model.md or README.
//
// --selftest re-runs checks 1–3 against a deliberately broken temp
// copy of the doc and FAILS unless every injected defect is caught
// (positive control), and against the real doc (must pass).
import { readFileSync, writeFileSync, mkdtempSync, rmSync } from 'node:fs';
import { join } from 'node:path';
import { tmpdir } from 'node:os';

const RULE_RX = /"caps": \[[^\]]*\]/g;

function builtinRuleIds(srcGo) {
  // rule objects: {"id": "x.y", ... } inside the embedded JSON
  const ids = [];
  for (const m of srcGo.matchAll(/\{"id":\s*"([a-z]+\.[a-z0-9]+)"/g)) ids.push(m[1]);
  return ids;
}

function check(docText, ruleIds, label) {
  const errs = [];
  const set = new Set(ruleIds);
  // references in doc: backticked rule ids
  const docRefs = new Set();
  for (const m of docText.matchAll(/`([a-z]+\.[a-z0-9]+)`/g)) {
    if (set.has(m[1]) || /^(cred|destroy|exec|net|mcp|agent|path|audit)\./.test(m[1])) docRefs.add(m[1]);
  }
  for (const id of set) {
    if (!docRefs.has(id)) errs.push(`${label}: rule ${id} never referenced in threat model`);
  }
  for (const ref of docRefs) {
    if (!set.has(ref)) errs.push(`${label}: doc references unknown rule ${ref}`);
  }
  // threat rows: table lines starting with | TM-
  const rows = docText.split('\n').filter((l) => /^\|\s*TM-\d+\s*\|/.test(l));
  if (rows.length < 10) errs.push(`${label}: only ${rows.length} threat rows (floor 10)`);
  let gapRows = 0;
  for (const r of rows) {
    const namesRule = [...set].some((id) => r.includes('`' + id + '`'));
    const markedGap = /known_gap/.test(r);
    if (!namesRule && !markedGap) {
      errs.push(`${label}: threat row ${r.slice(1, 8).trim()} names no rule and no known_gap`);
    }
    if (markedGap) gapRows++;
  }
  if (gapRows === 0) errs.push(`${label}: zero known_gap rows — an honest gap list is mandatory`);
  return errs;
}

function marketingWords(path) {
  const t = readFileSync(path, 'utf8');
  return /firewall/i.test(t) ? [`${path}: border-device marketing word present`] : [];
}

const go = readFileSync('internal/rules/rules.go', 'utf8');
const ids = builtinRuleIds(go);
const doc = readFileSync('docs/threat-model.md', 'utf8');
if (ids.length !== 12) { console.error(`RULE COUNT DRIFT: built-in table carries ${ids.length}, want 12`); process.exit(1); }

let errs = [...check(doc, ids, 'live'), ...marketingWords('docs/threat-model.md'), ...marketingWords('README.md')];

if (process.argv.includes('--selftest')) {
  const tmp = mkdtempSync(join(tmpdir(), 'tmc'));
  try {
    // defect 1: drop the only reference to a rule (orphan rule)
    const orphan = doc.replace(/`agent\.masquerade`/g, '`something.else`');
    // defect 2: a threat row stripped of rule names AND known_gap marker
    const silent = doc.replace(/\| TM-11 \|(.*?)\|\s*$/m, (m) => m.replace(/known_gap/g, 'watched'));
    // defect 3: a ghost rule reference
    const ghost = doc + '\nExtra: `audit.secretweapon` answers everything.\n';
    let caught = 0;
    if (check(orphan, ids, 'orphan').length) caught++;
    if (check(silent, ids, 'silent').length) caught++;
    if (check(ghost, ids, 'ghost').length) caught++;
    if (caught !== 3) {
      console.error(`SELFTEST FAIL: only ${caught}/3 injected defects caught`);
      console.error(JSON.stringify({ orphan: check(orphan, ids, 'o').length, silent: check(silent, ids, 's').length, ghost: check(ghost, ids, 'g').length }));
      process.exit(1);
    }
    if (errs.length) { console.error('SELFTEST FAIL: live doc does not pass:', errs); process.exit(1); }
    console.log('THREATMODEL SELFTEST OK: 3/3 injected defects caught; live doc clean (12 rules, bidirectional pairing)');
    process.exit(0);
  } finally {
    rmSync(tmp, { recursive: true, force: true });
  }
}

if (errs.length) { console.error('THREATMODEL CHECK FAIL:\n' + errs.join('\n')); process.exit(1); }
console.log(`THREATMODEL OK: 12 rules ↔ threat rows bidirectional, ${doc.split('\n').filter((l) => /^\|\s*TM-\d+\s*\|/.test(l)).length} threats, gaps explicit, marketing-word scan clean`);
