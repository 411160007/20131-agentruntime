// w12-memsec-check.mjs — design-surface cross-check between
// docs/memory-learning-safety-design.md and the shipped sources of truth:
//   internal/schema/intentauthority.go  (grant-origin enum, propagation
//                                        rule, enforcement-plane token),
//   internal/schema/decisioncache.go    (section 267 cache-key constraints
//                                        + context_epoch slot),
//   internal/schema/agencyguard.go      (section 261 counter kinds and
//                                        scopes),
// plus the closed-set censuses the V2 memory/learning-safety design
// contract requires. Documentation slice: this checker must fail on drift,
// and the diff range itself must show ZERO code change (--diff-base).
//
// Checks:
//   1. version lock sentence (doc + checker evolve in one PR);
//   2. closed-set censuses, each pinned exactly (no missing member, no
//      invented extra): six memory surfaces, seven record attributes, six
//      learnable categories, five forbidden learning effects, eight-stage
//      promotion pipeline in order, six quarantine triggers, seven-step
//      trust-freeze pipeline in order, eleven Lease fields;
//   3. section 262 core: "Memory ≠ Authorization" pinned, the password-vault
//      counter-example restated, the re-verification sentence present;
//   4. grounding parsed-not-copied: every grant origin, the propagation
//      rule, the enforcement-plane token, all eight cache-key constraints,
//      context_epoch, all three counter kinds and both scopes are
//      RE-PARSED from source on every run and must be restated in the doc;
//      the parsed enums must be non-empty (positive control: a parser that
//      silently returns zero items is a red, not a pass);
//   5. structural-absence design assertion MA-1 sentence pinned;
//   6. same-source citation: the proposal LEARN-row fragment appears
//      exactly once, and the "no second system" declaration is present;
//   7. honesty anchors: known_gap registration line, the verbatim 235
//      prohibition, the verbatim 265 no-global-relaxation line;
//   8. coverage table: five clause rows (262..265 + 235 cross-reference),
//      each carrying one disposition token and one design location —
//      no hollow row;
//   9. forbidden forms: no line may claim the memory store or the learning
//      pipeline ships; no internal decision-id pattern.
//
// --selftest re-runs the checks against deliberately broken temp copies of
// the doc and FAILS unless every injected defect is caught (positive
// control), and against the real doc (must pass).
// --diff-base <sha> asserts the commit range touches docs/ and scripts/
// only (pure design artifact gate).
import { readFileSync } from 'node:fs';
import { execSync } from 'node:child_process';
import * as path from 'node:path';
import { fileURLToPath } from 'node:url';

// repo root from this script's own location, never from caller cwd
const root = path.resolve(path.dirname(fileURLToPath(import.meta.url)), '..');
const at = (rel) => path.join(root, rel);

const DOC = 'docs/memory-learning-safety-design.md';
const AUTH = 'internal/schema/intentauthority.go';
const CACHE = 'internal/schema/decisioncache.go';
const GUARD = 'internal/schema/agencyguard.go';

const SURFACES = ['Conversation Memory', 'Task Memory', 'Long-term Memory',
  'Vector Store', 'Scratchpad', 'Cached Context'];
const ATTRS = ['Origin', 'Author', 'Timestamp', 'Trust', 'Scope', 'TTL', 'Integrity'];
const LEARNABLE = ['normal work patterns', 'anomaly patterns', 'compatibility',
  'user preferences', 'workflows', 'risk evidence'];
const FORBIDDEN = ['modify Hard Deny', 'automatically acquire Root',
  'release the S4 Boundary', 'permanently close Anti-Tamper',
  'automatically delete a Recovery Boundary'];
const PROMOTE = ['Observe', 'Candidate', 'Probation', 'Shadow', 'Replay',
  'Evaluate', 'Validate', 'Promote'];
const TRIGGERS = ['burst of many new behaviors in a short window',
  'mass automatic Allow decisions',
  'sudden behavior expansion after an Agent Hash change',
  'capability surge after an MCP / Skill change',
  'persistent conflict between the learning model and Native Evidence',
  'abnormal Policy Promotion'];
const FREEZE = ['Freeze', 'Diagnostic', 'Historical Replay',
  'Compare Native Evidence', 'Repair Model', 'Re-Evaluate', 'Resume'];
const LEASE = ['Task ID', 'Agent ID', 'Capability', 'Scope', 'Resource',
  'Conditions', 'TTL', 'Quota', 'Revocable', 'Reason', 'Authority Source'];

function parseSource() {
  const auth = readFileSync(at(AUTH), 'utf8');
  const cache = readFileSync(at(CACHE), 'utf8');
  const guard = readFileSync(at(GUARD), 'utf8');
  const origins = [...auth.matchAll(/Origin\w+\s+GrantOrigin = "([^"]+)"/g)].map((m) => m[1]);
  const rule = (auth.match(/AuthorityPropagationRule = "([^"]+)"/) || [])[1] || '';
  const plane = (auth.match(/AuthorityEnforcementPlane = "([^"]+)"/) || [])[1] || '';
  const keyBlock = (cache.match(/decisionCacheKeyFieldNames = \[\]string\{([\s\S]*?)\n\}/) || [])[1] || '';
  const keys = [...keyBlock.matchAll(/"([a-z_]+)"/g)].map((m) => m[1]);
  const epoch = /context_epoch/.test(cache);
  const counters = [...guard.matchAll(/Counter\w+\s+AgencyCounterKind = "([^"]+)"/g)].map((m) => m[1]);
  const scopes = [...guard.matchAll(/Scope\w+\s+AgencyCounterScope = "([^"]+)"/g)].map((m) => m[1]);
  return { origins, rule, plane, keys, epoch, counters, scopes };
}

// extract the fenced block that follows a `<!-- closed:name -->` anchor
function closedBlock(docText, name) {
  const anchor = docText.indexOf(`<!-- closed:${name} -->`);
  if (anchor < 0) return null;
  const open = docText.indexOf('```', anchor);
  if (open < 0) return null;
  const bodyStart = docText.indexOf('\n', open + 3) + 1; // skip the ```text fence line
  const close = docText.indexOf('```', bodyStart);
  if (close < 0) return null;
  return docText.slice(bodyStart, close);
}

function census(docText, name, expected, opts) {
  const block = closedBlock(docText, name);
  if (block === null) return [`closed:${name} anchor or fenced block missing`];
  const items = block.split('\n').map((l) => l.replace(/^\s*[-\d.]+\s*/, '').trim()).filter(Boolean);
  const errs = [];
  if (items.length !== expected.length) {
    errs.push(`closed:${name} census size ${items.length} != pinned ${expected.length} (${items.join(' | ')})`);
    return errs;
  }
  if (opts && opts.ordered) {
    for (let i = 0; i < expected.length; i++) {
      if (items[i] !== expected[i]) errs.push(`closed:${name} position ${i + 1} is "${items[i]}", pinned "${expected[i]}"`);
    }
  } else {
    for (const e of expected) if (!items.includes(e)) errs.push(`closed:${name} missing pinned member "${e}"`);
    for (const it of items) if (!expected.includes(it)) errs.push(`closed:${name} invented member "${it}"`);
  }
  return errs;
}

function runChecks(docText) {
  const src = parseSource();
  const flat = docText.replace(/\s+/g, ' ').replace(/\*/g, '');
  const fail = [];
  const need = (ok, msg) => { if (!ok) fail.push(msg); };

  // positive control on the parsers themselves (undefined-field family)
  need(src.origins.length === 5, `grant-origin parse must yield 5, got ${src.origins.length}`);
  need(src.rule !== '' && src.plane !== '', 'propagation rule / enforcement plane token parse empty');
  need(src.keys.length === 8, `cache-key constraint parse must yield 8, got ${src.keys.length}`);
  need(src.epoch, 'context_epoch slot missing from decisioncache.go source');
  need(src.counters.length === 3, `counter-kind parse must yield 3, got ${src.counters.length}`);
  need(src.scopes.length === 2, `scope parse must yield 2, got ${src.scopes.length}`);

  need(/MUST evolve in\s+one PR/.test(docText), 'version lock sentence missing');

  // closed sets
  fail.push(...census(docText, 'surfaces', SURFACES));
  fail.push(...census(docText, 'attributes', ATTRS));
  fail.push(...census(docText, 'learnable', LEARNABLE));
  fail.push(...census(docText, 'forbidden-effects', FORBIDDEN));
  fail.push(...census(docText, 'promotion-pipeline', PROMOTE, { ordered: true }));
  fail.push(...census(docText, 'quarantine-triggers', TRIGGERS));
  fail.push(...census(docText, 'freeze-pipeline', FREEZE, { ordered: true }));
  fail.push(...census(docText, 'lease-fields', LEASE));

  // section 262 core
  need(/Memory ≠ Authorization/.test(flat), 'pinned principle "Memory ≠ Authorization" missing');
  need(/用户已经允许访问密码库/.test(docText), 'password-vault counter-example missing');
  need(/re-verified at the moment of use/.test(flat), 're-verification sentence missing');

  // grounding restatements (parsed from source, required in the doc)
  for (const o of src.origins) need(flat.includes(o), `grant origin "${o}" (parsed from source) not restated in doc`);
  need(/None of the five is/.test(flat), 'no-memory/learning-origin statement missing');
  need(/no such constructor exists in the enum/.test(flat), 'enum-closure clause for grant origins missing');
  need(flat.includes(src.rule), `propagation rule token "${src.rule}" not restated`);
  need(flat.includes(src.plane), `enforcement-plane token "${src.plane}" not restated`);
  for (const k of src.keys) need(new RegExp(`\\b${k}\\b`).test(flat), `cache-key constraint "${k}" (parsed) not restated in doc`);
  need(/context_epoch/.test(docText), 'context_epoch invalidation slot not restated');
  for (const c of src.counters) need(flat.includes(c), `observation counter "${c}" (parsed) not restated in doc`);
  for (const s of src.scopes) need(docText.includes('`' + s + '`'), `counter scope "${s}" (parsed) not restated in doc`);

  // section 263 design line, section 265 verbatim, 264 honesty anchors
  need(/promotion is always an explicit pipeline stage with recorded evidence, never a side effect/.test(flat),
    'promotion-not-a-side-effect design line missing');
  need(/不得因为学习错误而全局放宽安全边界/.test(docText), 'section 265 verbatim no-global-relaxation line missing');
  need(/never globally relaxes the safety boundary/.test(flat), 'section 265 English mirror missing');
  need(/no new behavior may be upgraded directly to high trust/.test(flat), 'quarantine high-trust prohibition line missing');
  need(/known_gap/.test(docText), 'section 264 known_gap registration missing');
  need(/no shipped collector yet/.test(flat), 'section 264 honest-absence sentence missing');
  need(/No second counter may start in this slice/.test(flat), 'no-second-counter line missing');

  // section 235 verbatim prohibition
  need(/禁止通过[""]学习结果[""]把短期 Lease 悄悄升级成永久 Root 权限/.test(docText),
    'section 235 verbatim learning-result Root prohibition missing');

  // structural-absence design assertion
  need(/MA-1: a promotion path from memory or learning results to Authority or Root does not structurally exist/.test(flat),
    'design assertion MA-1 (structural absence) missing or reworded');

  // same-source citation, exactly once, and the no-second-system declaration
  const quoteHits = (docText.match(/§235 禁学习结果升 Root \+ §262 记忆≠授权/g) || []).length;
  need(quoteHits === 1, `proposal LEARN-row citation must appear exactly once, found ${quoteHits}`);
  need(/学习面自动升权路径结构性禁止/.test(docText), 'LEARN-row tail (structural ban) missing from citation');
  need(/invents no second gate-assertion system/.test(flat), 'no-second-system declaration missing');
  need(/Phase 1 MCP\b.{0,60}proposal/.test(flat), 'citation source attribution (proposal) missing');

  // coverage table: five clause rows, each with a disposition token
  const rows = [...docText.matchAll(/^\|\s*spec section (\d{3})(?: \(cross-reference\))?\s*\|\s*([^|]+)\|\s*([^|]+)\|/gm)];
  const seen = rows.map((r) => r[1]).sort();
  need(seen.join(',') === ['235', '262', '263', '264', '265'].join(','), `coverage rows must be exactly 235,262,263,264,265, got ${seen.join(',')}`);
  for (const r of rows) {
    need(/covered|registered|deferred/.test(r[2]), `coverage row ${r[1]} has no valid disposition token ("${r[2].trim()}")`);
    need(r[3].trim().length > 3, `coverage row ${r[1]} has a hollow design-location cell`);
  }

  // forbidden forms: honesty about the observation-only stance
  need(rows.length === 5, `coverage table must have 5 clause rows, got ${rows.length}`);
  for (const bad of ['memory store ships', 'learning pipeline is active',
    'quarantine machinery ships', 'freeze machinery ships']) {
    need(!new RegExp('(?<![a-z-])' + bad).test(flat), `forbidden claim form present: "${bad}"`);
  }
  need(!/\bD-\d{3}\b/.test(docText), 'internal decision-id leaked into the product-facing doc surface');

  return fail;
}

function selftest() {
  const doc = readFileSync(at(DOC), 'utf8');
  const base = runChecks(doc);
  if (base.length) { console.log('REAL DOC RED:\n  ' + base.join('\n  ')); return 1; }
  const defects = [
    ['drop the pinned principle', (t) => t.replace(/Memory ≠ Authorization/g, 'Memory is Authorization')],
    ['invent an eighth memory surface', (t) => t.replace('- Scratchpad', '- Scratchpad\n- Dream Buffer')],
    ['swap Replay and Evaluate order', (t) => t.replace(/6\. Evaluate\n7\. Validate\n8\. Promote/, '6. Validate\n7. Replay\n8. Promote')],
    ['drop MA-1', (t) => t.replace(/Design assertion MA-1/, 'A nostalgic aside MA-1x')],
    ['flip the 235 verbatim prohibition', (t) => t.replace(/禁止通过[""]学习结果[""]把短期 Lease 悄悄升级成永久 Root 权限/, '允许通过学习结果把短期 Lease 升级成永久 Root 权限')],
    ['mutate the proposal citation', (t) => t.replace(/§262 记忆≠授权/, '§262 记忆即授权')],
    ['fake the 264 collector honesty', (t) => t.replace(/no shipped\s+collector yet/, 'every collector is fully shipped')],
    ['drop one Lease field', (t) => t.replace('- Quota\n', '')],
    ['hollow a coverage disposition', (t) => t.replace(/\| spec section 263 \| covered/, '| spec section 263 | maybe')],
    ['drop a parsed grant origin', (t) => t.replace(/`user_direct`/g, '`user_whisper`')],
    ['drop the 265 verbatim line', (t) => t.replace(/不得因为学习错误而全局放宽安全边界。/, '')],
    ['leak a decision id', (t) => t.replace('## 6. Structural', '## 6. Structural (D-0' + '42)')],
  ];
  let caught = 0;
  for (const [name, mutate] of defects) {
    const broken = mutate(doc);
    if (broken === doc) { console.log(`SELFTEST DEFECT NOT APPLIED: ${name}`); return 1; }
    const errs = runChecks(broken);
    if (errs.length === 0) { console.log(`MISSED DEFECT: ${name}`); return 1; }
    caught++;
  }
  console.log(`SELFTEST GREEN (${caught} injected defects all caught; real doc green)`);
  return 0;
}

function diffGate(baseSha) {
  const out = execSync(`git -C "${root}" diff --name-only ${baseSha}..HEAD`, { encoding: 'utf8' });
  const files = out.split('\n').filter(Boolean);
  if (files.length === 0) { console.log('DIFF GATE RED: empty range'); return 1; }
  const code = files.filter((f) => !/^(docs|scripts)\//.test(f));
  if (code.length) { console.log('DIFF GATE RED: code-plane increment: ' + code.join(', ')); return 1; }
  console.log(`DIFF GATE GREEN: ${files.length} files, docs/scripts only`);
  return 0;
}

if (process.argv[2] === '--selftest') process.exit(selftest());
if (process.argv[2] === '--diff-base') process.exit(diffGate(process.argv[3]));

const errs = runChecks(readFileSync(at(DOC), 'utf8'));
if (errs.length) { console.log('MEMSEC CHECK RED:\n  ' + errs.join('\n  ')); process.exit(1); }
console.log('MEMSEC CHECK GREEN: 4 clauses + 235 cross-ref ↔ design lines closed, grounding parsed not copied, MA-1 pinned');
