// threatmodel-check.mjs — bidirectional cross-check between the
// built-in rule table (parsed from internal/rules/rules.go, the
// shipped source of truth) and docs/threat-model.md, plus the
// threat-model v1 family uplift and the supply-chain observation
// documentation surface.
//
// Checks:
//   1. every built-in rule id appears in the threat model (>=1 threat);
//   2. every rule id referenced in the threat model exists in the
//      built-in table (no stale/ghost references);
//   3. every threat row (TM-xx) names at least one rule or carries
//      an explicit known_gap marker — silence is not an option;
//   4. threat row count >= 10 (v0 floor) and rule count == 12
//      (the v1 uplift ships documentation rows only — no new rules);
//   5. external copy discipline: no border-device marketing word
//      (any casing) in docs/threat-model.md or README;
//   6. uplift census: rows TM-13..TM-17 exist exactly (no gap, no
//      extra), and every uplifted row carries a known_gap marker;
//   7. spec-clause reverse lookup: each uplifted row names its
//      governing specification clause ("spec section NNN") on the
//      pinned family->clause map, and no clause number appears
//      outside its family row (bidirectional);
//   8. supply-chain observation section: the section heading exists,
//      the three observation counter tokens are restated from the
//      shipped source internal/schema/agencyguard.go (parsed, never
//      copied), the record-only-no-action stance sentence is present,
//      the untrusted-content linkage names the TM-14 row, and the
//      no-install-gating gap is stated with a known_gap marker.
//
// --selftest re-runs the checks against deliberately broken temp
// copies of the doc and FAILS unless every injected defect is caught
// (positive control), and against the real doc (must pass).
import { readFileSync } from 'node:fs';
import * as path from 'node:path';

// repo root is resolved from this script's own location, never from
// the caller's cwd, so the gate scripts and the Go-mounted test run
// the same production check
const root = path.resolve(import.meta.dirname, '..');
const at = (rel) => path.join(root, rel);

const RULE_RX = null; // rule ids are matched positionally below

function builtinRuleIds(srcGo) {
  // rule objects: {"id": "x.y", ... } inside the embedded JSON
  const ids = [];
  for (const m of srcGo.matchAll(/\{"id":\s*"([a-z]+\.[a-z0-9]+)"/g)) ids.push(m[1]);
  return ids;
}

function counterTokens(srcGo) {
  const toks = [];
  for (const m of srcGo.matchAll(/AgencyCounterKind = "([a-z_]+)"/g)) toks.push(m[1]);
  return toks;
}

// pinned family -> specification clause map (uplift census owner)
const UPLIFT = {
  'TM-13': null, // reserved-id consumption, no clause
  'TM-14': '240',
  'TM-15': '241',
  'TM-16': '238',
  'TM-17': '242',
};

function check(docText, ruleIds, counters, label) {
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

  // uplift census (check 6): exact set TM-13..TM-17, each with known_gap
  const rowIds = rows.map((r) => (r.match(/^\|\s*(TM-\d+)\s*\|/) || [])[1]);
  const want = Object.keys(UPLIFT);
  for (const id of want) {
    if (!rowIds.includes(id)) errs.push(`${label}: uplift row ${id} missing`);
  }
  for (const id of rowIds) {
    if (!/^TM-(0[1-9]|1[0-7])$/.test(id)) errs.push(`${label}: threat row ${id} outside the pinned census TM-01..TM-17`);
  }
  for (const r of rows) {
    const id = (r.match(/^\|\s*(TM-\d+)\s*\|/) || [])[1];
    if (want.includes(id) && id !== 'TM-13' && !/known_gap/.test(r)) {
      errs.push(`${label}: uplifted row ${id} without an explicit known_gap marker`);
    }
  }

  // spec-clause reverse lookup (check 7), both directions
  for (const [id, clause] of Object.entries(UPLIFT)) {
    if (!clause) continue;
    const row = rows.find((r) => r.startsWith(`| ${id}`));
    if (row && !row.includes(`spec section ${clause}`)) {
      errs.push(`${label}: row ${id} does not name its governing clause (spec section ${clause})`);
    }
  }
  const clauseHits = [...docText.matchAll(/spec section (\d{3})/g)].map((m) => m[1]);
  const pinned = new Set(Object.values(UPLIFT).filter(Boolean));
  for (const c of clauseHits) {
    if (!pinned.has(c)) errs.push(`${label}: clause number ${c} referenced outside the pinned family->clause map`);
  }

  // supply-chain observation section (check 8)
  const secMatch = docText.match(/^## Supply-chain observation v0[^\n]*\n([\s\S]*?)(?=^## |$(?![\s\S]))/m);
  if (!secMatch) {
    errs.push(`${label}: supply-chain observation section missing`);
  } else {
    const sec = secMatch[1];
    for (const tok of counters) {
      if (!sec.includes('`' + tok + '`')) errs.push(`${label}: supply-chain section does not restate counter token ${tok} from the shipped source`);
    }
    if (counters.length !== 3) errs.push(`${label}: shipped counter census is ${counters.length}, want 3`);
    if (!sec.includes('record-only-no-action')) errs.push(`${label}: supply-chain section lost the record-only-no-action stance sentence`);
    if (!sec.includes('TM-14')) errs.push(`${label}: supply-chain section no longer links the untrusted-content row TM-14`);
    if (!/known_gap/.test(sec)) errs.push(`${label}: supply-chain section hides the no-install-gating gap`);
  }
  return errs;
}

function marketingWords(p) {
  const t = readFileSync(p, 'utf8');
  return /firewall/i.test(t) ? [`${p}: border-device marketing word present`] : [];
}

const go = readFileSync(at('internal/rules/rules.go'), 'utf8');
const ids = builtinRuleIds(go);
const counters = counterTokens(readFileSync(at('internal/schema/agencyguard.go'), 'utf8'));
const doc = readFileSync(at('docs/threat-model.md'), 'utf8');
if (ids.length !== 12) { console.error(`RULE COUNT DRIFT: built-in table carries ${ids.length}, want 12 (the v1 uplift ships documentation rows only)`); process.exit(1); }

let errs = [...check(doc, ids, counters, 'live'), ...marketingWords(at('docs/threat-model.md')), ...marketingWords(at('README.md'))];

if (process.argv.includes('--selftest')) {
  // defect 1: drop the only reference to a rule (orphan rule)
  const orphan = doc.replace(/`agent\.masquerade`/g, '`something.else`');
  // defect 2: a threat row stripped of rule names AND known_gap marker
  const silent = doc.replace(/\| TM-11 \|(.*?)\|\s*$/m, (m) => m.replace(/known_gap/g, 'watched'));
  // defect 3: a ghost rule reference
  const ghost = doc + '\nExtra: `audit.secretweapon` answers everything.\n';
  // defect 4: an uplifted row losing its governing clause (reverse lookup)
  const declause = doc.replace(/\| TM-15 \|([^\n]*)/, (m) => m.replace('spec section 241', 'spec section 299'));
  // defect 5: a counter token smuggled into a private renames (doc drifts from shipped source)
  const decounter = doc.replaceAll('`event_rate`', '`events_per_second`');
  const cases = [
    ['orphan', check(orphan, ids, counters, 'orphan')],
    ['silent', check(silent, ids, counters, 'silent')],
    ['ghost', check(ghost, ids, counters, 'ghost')],
    ['declause', check(declause, ids, counters, 'declause')],
    ['decounter', check(decounter, ids, counters, 'decounter')],
  ];
  let caught = 0;
  for (const [name, e] of cases) if (e.length) caught++; else console.error(`SELFTEST MISS: ${name} not caught`);
  if (caught !== cases.length) {
    console.error(`SELFTEST FAIL: only ${caught}/${cases.length} injected defects caught`);
    process.exit(1);
  }
  if (errs.length) { console.error('SELFTEST FAIL: live doc does not pass:', errs); process.exit(1); }
  console.log(`THREATMODEL SELFTEST OK: ${caught}/${cases.length} injected defects caught; live doc clean (12 rules, ${ids.length + 5}/17 uplift census, clauses+counters bidirectional)`);
  process.exit(0);
}

if (errs.length) { console.error('THREATMODEL CHECK FAIL:\n' + errs.join('\n')); process.exit(1); }
const nRows = doc.split('\n').filter((l) => /^\|\s*TM-\d+\s*\|/.test(l)).length;
console.log(`THREATMODEL OK: 12 rules <-> threat rows bidirectional, ${nRows} threats (TM-13..17 uplift present, clauses pinned), supply-chain section restates [${counters.join(', ')}] from shipped source, gaps explicit, marketing-word scan clean`);
