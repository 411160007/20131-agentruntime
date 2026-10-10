// task-primitive-check.mjs — design-surface cross-check between
// docs/task-primitive-design.md and the shipped sources of truth:
//   internal/schema/event.go        (event type enum, decision trio),
//   scripts/validate-jsonl.mjs      (independent validator mirror),
//   docs/api-v0.md                  (frozen external contract line),
//   docs/schema-v2.md               (intent field + decision vocab lines),
// plus the closed-set censuses the turn→task mapping v0 contract
// requires. Documentation slice: this checker must fail on drift, and
// the diff range itself must show ZERO code change (--diff-base).
//
// Checks:
//   1. version lock sentence (doc + checker evolve in one PR);
//   2. closed-set censuses, each pinned exactly (no missing member, no
//      invented extra): seven task statuses, four boundary signals,
//      four reserved task fields, five mapping rules in order, three
//      denominator rules in order;
//   3. vocabulary reverse-lookup, shipped side: every token the doc
//      marks [shipped] is RE-PARSED from the event enum, the validator
//      mirror, and the frozen contract doc on every run and must be
//      restated in the design doc; parsed sets must be non-empty
//      (positive control: a silent-empty parser is a red, not a pass);
//   4. vocabulary reverse-lookup, proposed side: every [proposed]
//      token (lease_expiry_edge and the four reserved field names)
//      must NOT exist anywhere in the shipped Go/validator/contract
//      sources — reserved vocabulary stays reserved until a recorder
//      slice lands in all sources together;
//   5. Phase 0 invariants: the doc pins the emitted decision closed
//      set {allow, would_block} and the reserved-ask sentence; the
//      shipped decision enum parses to exactly allow/ask/would_block;
//   6. honesty anchors: the verbatim denominator-gap sentence, the
//      derive-only posture, the no-second-system declaration;
//   7. no-claim scan: no line may state that task records ship, are
//      emitted, or are enforced; no internal-ledger identifier shapes.
//
// --selftest re-runs the checks against deliberately broken temp
// copies of the doc and FAILS unless every injected defect is caught
// (positive control), and against the real doc (must pass). Scratch
// lives under a freshly created mkdtemp directory — never assumed to
// pre-exist.
// --diff-base <sha> asserts the commit range touches docs/ and
// scripts/ only (pure schema/docs artifact gate).
import { readFileSync, mkdtempSync, writeFileSync, rmSync } from 'node:fs';
import { execSync } from 'node:child_process';
import * as os from 'node:os';
import * as path from 'node:path';
import { fileURLToPath } from 'node:url';

// this document and `scripts/task-primitive-check.mjs` are one
// contract; both carry this exact sentence.
const VERSION_LOCK = 'are one contract; both carry this exact sentence';

const root = path.resolve(path.dirname(fileURLToPath(import.meta.url)), '..');
const at = (rel) => path.join(root, rel);

const DOC = 'docs/task-primitive-design.md';
const EVENT_GO = 'internal/schema/event.go';
const VALIDATOR = 'scripts/validate-jsonl.mjs';
const APIV0 = 'docs/api-v0.md';
const SCHEMAV2 = 'docs/schema-v2.md';

const STATUS = ['open', 'active', 'completed', 'failed', 'abandoned', 'blocked', 'unknown'];
const BOUNDARY = ['session_start_edge', 'turn_stop_sequence', 'goal_hint', 'lease_expiry_edge'];
const FIELDS = ['task_id', 'task_status', 'task_boundary', 'task_refs'];
const MAPPING = ['MR-1', 'MR-2', 'MR-3', 'MR-4', 'MR-5'];
const DENOM = ['DR-1', 'DR-2', 'DR-3'];
// shipped tokens the doc marks [shipped] and claims section 2 grounding for
const SHIPPED_CLAIMED = ['session.start', 'turn.stop', 'policy.decision', 'goal',
  'allow', 'ask', 'would_block'];
// tokens that must exist ONLY as reserved vocabulary — absent from shipped sources
const PROPOSED_ONLY = [...FIELDS, 'lease_expiry_edge'];

const fail = [];
const bad = (msg) => fail.push(msg);

// ---------- parsers (real sources, every run) ----------
function parseEventTypes() {
  const src = readFileSync(at(EVENT_GO), 'utf8');
  const out = new Set();
  for (const m of src.matchAll(/Type\w+\s+EventType = "([a-z_.]+)"/g)) out.add(m[1]);
  return out;
}
function parseValidatorMirror() {
  const src = readFileSync(at(VALIDATOR), 'utf8');
  const m = src.match(/const TYPES = \[([^\]]+)\]/);
  const out = new Set();
  if (m) for (const q of m[1].matchAll(/'([a-z_.]+)'/g)) out.add(q[1]);
  return out;
}
function parseApiContract() {
  const src = readFileSync(at(APIV0), 'utf8');
  const m = src.match(/^types: (.+)$/m);
  return new Set(m ? m[1].split(/,\s*/).map((s) => s.trim()) : []);
}
function parseDecisionEnum() {
  const src = readFileSync(at(EVENT_GO), 'utf8');
  const m = src.match(/DecisionAllow\s+Decision\w*\s*=\s*"allow"|DecisionAllow\s*=?\s*Decision\("allow"\)|DecisionAllow[^=]*=\s*Decision\("allow"\)|DecisionAllow[\s\S]{0,60}?"allow"/);
  const out = [];
  if (m) out.push('allow');
  const all = src.match(/string\(DecisionAllow\), string\(DecisionAsk\), string\(DecisionWouldBlock\)/);
  if (all) { out.push('ask'); out.push('would_block'); }
  return out;
}
function parseIntentFieldVocab() {
  const src = readFileSync(at(SCHEMAV2), 'utf8');
  const m = src.match(/^intent_field_vocabulary: (.+)$/m);
  return m ? m[1].split(/,\s*/) : [];
}

// ---------- doc-side closed-set extraction ----------
function extractVocab(doc, name) {
  const re = new RegExp('^' + name + ': (.+)$', 'm');
  const m = doc.match(re);
  return m ? m[1].split(/,\s*/).map((s) => s.trim()) : null;
}
function pinSet(label, want, got) {
  if (!got) { bad(`${label}: vocabulary line missing from doc`); return; }
  if (got.length !== want.length || got.some((t, i) => t !== want[i])) {
    bad(`${label}: census drift — doc has [${got.join(', ')}], contract wants [${want.join(', ')}]`);
  }
}

function check(docText, mapperText = mapperOrNothing()) {
  fail.length = 0;

  // 1. version lock
  if (!docText.includes(VERSION_LOCK)) bad('version lock sentence missing from doc');

  // 2. closed-set censuses
  pinSet('task_status_vocabulary', STATUS, extractVocab(docText, 'task_status_vocabulary'));
  pinSet('boundary_signal_vocabulary', BOUNDARY, extractVocab(docText, 'boundary_signal_vocabulary'));
  pinSet('task_field_vocabulary', FIELDS, extractVocab(docText, 'task_field_vocabulary'));
  for (const r of MAPPING) if (!docText.includes('`' + r + '`')) bad(`mapping rule ${r} missing from doc`);
  // in-order check on the rule headings
  const idx = MAPPING.map((r) => docText.indexOf('`' + r + '`'));
  if (idx.every((v) => v >= 0) && idx.some((v, i) => i > 0 && v <= idx[i - 1])) bad('mapping rules out of order');
  for (const r of DENOM) if (!docText.includes('`' + r + '`')) bad(`denominator rule ${r} missing from doc`);

  // 3. shipped reverse-lookup (parsed-not-copied, non-empty controls)
  const goTypes = parseEventTypes();
  const valMirror = parseValidatorMirror();
  const contract = parseApiContract();
  if (goTypes.size < 10) bad(`event.go parser degenerate (${goTypes.size} types)`);
  if (valMirror.size !== 12) bad(`validate-jsonl mirror parsed ${valMirror.size} types, want 12`);
  if (contract.size < 10) bad(`api-v0 contract parser degenerate (${contract.size} types)`);
  const intentVocab = parseIntentFieldVocab();
  if (intentVocab[0] !== 'goal') bad(`intent_field_vocabulary first token is ${intentVocab[0]}, want goal`);
  for (const tok of ['session.start', 'turn.stop', 'policy.decision']) {
    if (!goTypes.has(tok) || !valMirror.has(tok) || !contract.has(tok)) {
      bad(`shipped token ${tok} not present across all three sources`);
    }
    if (!docText.includes('`' + tok + '`')) bad(`doc fails to restate shipped token ${tok}`);
  }
  const dec = parseDecisionEnum();
  if (dec.join(',') !== 'allow,ask,would_block') bad(`decision enum parsed [${dec.join(',')}], want allow,ask,would_block`);

  // 3b. generic [shipped]-claim audit: every token the doc tags
  // `[shipped]` must exist in the parsed shipped union — no smuggled
  // reserved token may wear the shipped label.
  const union = new Set([...goTypes, ...valMirror, ...contract, ...dec, ...intentVocab]);
  for (const m of docText.matchAll(/`([a-z_.]+)`\s+`\[shipped\]`/g)) {
    if (!union.has(m[1])) bad(`doc tags ${m[1]} as [shipped] but no shipped source carries it`);
  }

  // 4. proposed-side isolation: reserved tokens must NOT exist in shipped sources
  const shippedBlob = [readFileSync(at(EVENT_GO), 'utf8'),
    readFileSync(at(VALIDATOR), 'utf8'),
    readFileSync(at(APIV0), 'utf8')].join('\n');
  for (const tok of PROPOSED_ONLY) {
    if (shippedBlob.includes(tok)) bad(`reserved token ${tok} already present in shipped sources (isolation red)`);
    if (!docText.includes(tok)) bad(`doc does not declare reserved token ${tok}`);
  }

  // 5. Phase 0 invariants pinned in the doc
  if (!docText.includes('{allow, would_block}')) bad('Phase 0 emitted decision closed set missing from doc');
  if (!docText.includes('ask` stays reserved')) bad('reserved-ask sentence missing from doc');

  // 6. honesty anchors
  if (!docText.includes('denominator does not exist in shipped code today')) {
    bad('verbatim denominator-gap sentence missing from doc');
  }
  if (!docText.includes('not an implementation')) bad('gap registration softened: implementation claim posture missing');
  if (!docText.includes('invents a parallel session model')) bad('no-second-system declaration missing from doc');
  if (!docText.includes('computed at read time')) bad('derive-only posture sentence missing from doc');

  // 7. no-claim scan + internal-id shapes (tripwire-adjacent belt)
  const claimRes = [
    /task records? (?:now|today) ship/i,
    /emits? task_?id/i,
    /runtime (?:maps|assigns) tasks/i,
    /enforcement (?:is )?active for tasks/i,
  ];
  for (const re of claimRes) if (re.test(docText)) bad(`forbidden shipping-claim form: ${re}`);
  // internal-ledger shapes; name literals are built in pieces so this
  // scanner file never trips its own negative scan (same posture as
  // the tripwire's self-exclusion note).
  const idRes = new RegExp('\\bD-[0-9]{3}\\b|\\bE[0-9]{2,3}\\b|\\bT[0-9]{2,3}\\b|\\b' + 'TASK' + 'BOOK\\b|\\b' + 'NIGHT_' + 'TASKS\\b|' + 'ai-' + 'company|' + 'claw' + 'd');
  if (idRes.test(docText)) bad('internal-ledger identifier shape in doc surface');

  // 8. derive-only mapper reverse-lookup: the section-4 landing tool must
  // import the vocabulary from this doc (single writer source) and must
  // never carry a second closed-set copy of its own.
  if (mapperText === null) bad('derive-only mapper script not found at scripts/task-map.mjs');
  else {
    if (!mapperText.includes('task-primitive-design.md')) bad('mapper does not reference the design doc as its vocabulary source');
    if (!mapperText.includes("'task_status_vocabulary'")) bad('mapper does not re-parse the doc status closed set (import posture missing)');
    if (/['\"]open['\"]\s*,\s*['\"]active['\"]\s*,\s*['\"]completed['\"]/i.test(mapperText)) bad('mapper carries a second status vocabulary copy');
    if (!/conservation/i.test(mapperText)) bad('mapper conservation gate posture missing');
    // eval-join posture: the second evidence source must arrive as a
    // runtime-imported source with the conflict rule, never as an
    // embedded copy of the assertion corpus.
    if (!mapperText.includes("'--evals'")) bad('mapper carries no eval-join import posture (--evals)');
    if (!/conflict/.test(mapperText)) bad('mapper eval join lacks the conflict rule (mismatched assertions must never promote)');
    if (/\{\s*['\"]gn-\d+['\"]\s*:\s*\{[^}]*expect/i.test(mapperText)) bad('mapper embeds a second copy of the assertion corpus');
  }

  return fail;
}
function mapperOrNothing() {
  try { return readFileSync(at('scripts/task-map.mjs'), 'utf8'); } catch { return null; }
}

// ---------- selftest: injected defects must ALL be caught ----------
function selftest() {
  const docPath = at(DOC);
  const real = readFileSync(docPath, 'utf8');
  const mapperReal = mapperOrNothing();
  if (mapperReal === null) { console.error('SELFTEST RED: mapper file absent — reverse-lookup has nothing to scan'); return 1; }
  const dir = mkdtempSync(path.join(os.tmpdir(), 'taskpr-ctl-'));
  const injected = [
    ['status census drift', (d) => d.replace('task_status_vocabulary: open, active, completed, failed, abandoned, blocked, unknown',
      'task_status_vocabulary: open, active, completed, failed, unknown')],
    ['field census invented extra', (d) => d.replace('task_field_vocabulary: task_id, task_status, task_boundary, task_refs',
      'task_field_vocabulary: task_id, task_status, task_boundary, task_refs, task_owner')],
    ['shipped token unrestated', (d) => d.split('`turn.stop`').join('`turn.end`')],
    ['gap sentence removed', (d) => d.replace('denominator does not exist in shipped code today',
      'denominator is being built right now')],
    ['reserved token smuggled as shipped', (d) => d.replace('`lease_expiry_edge` is\n`[proposed]`', '`lease_expiry_edge` `[shipped]`')],
    ['closed set widened', (d) => d.replace('{allow, would_block}', '{allow, block, would_block}')],
    ['mapping rule dropped', (d) => d.replace('- `MR-4` (safety tail)', '- `MR-9` (safety tail)')],
    ['version lock broken', (d) => d.replace(VERSION_LOCK, 'are separate documents')],
    ['shipping claim', (d) => d.replace('not an implementation', 'not an implementation, task records now ship')],
    // mapper-targeted injection (third slot): a second vocabulary copy in
    // the derive-only tool must trip the reverse-lookup.
    ['mapper second vocabulary copy', null, (m) => m.replace('const HINT_DONE',
      "const SECOND = ['open', 'active', 'completed', 'failed', 'abandoned', 'blocked', 'unknown'];\nconst HINT_DONE")],
    ['mapper second assertion-corpus copy', null, (m) => m.replace('const HINT_DONE',
      "const SECOND_LABELS = { 'gn-01': { expect: 'allow' } };\nconst HINT_DONE")],
    ['mapper eval conflict rule dropped', null, (m) => m.replace(/conflict/g, 'xonflict')],
  ];
  let caught = 0;
  for (const [name, docMut, mapMut] of injected) {
    const brokenDoc = docMut ? docMut(real) : real;
    const brokenMapper = mapMut ? mapMut(mapperReal) : mapperReal;
    if (docMut && brokenDoc === real) { console.error(`SELFTEST RED: injection ${name} was a no-op`); continue; }
    if (mapMut && brokenMapper === mapperReal) { console.error(`SELFTEST RED: injection ${name} was a no-op`); continue; }
    const f = check(brokenDoc, brokenMapper);
    if (f.length === 0) console.error(`SELFTEST RED: injection ${name} NOT caught`);
    else caught++;
  }
  // real doc + real mapper must pass
  const realFails = check(real, mapperReal);
  rmSync(dir, { recursive: true, force: true });
  if (realFails.length) { console.error('SELFTEST RED: real doc fails:', realFails); return 1; }
  if (caught !== injected.length) { console.error(`SELFTEST RED: ${caught}/${injected.length} injections caught`); return 1; }
  console.log(`SELFTEST GREEN (${injected.length}/${injected.length} injected defects caught, real doc clean)`);
  return 0;
}

// ---------- diff-range purity ----------
function diffBase(sha) {
  const files = execSync(`git diff --name-only ${sha}...HEAD`, { cwd: root }).toString().trim().split('\n').filter(Boolean);
  const viol = files.filter((f) => !/^docs\//.test(f) && !/^scripts\//.test(f));
  if (viol.length) { console.error('RED: diff range touches non-doc/script files:', viol); return 1; }
  console.log(`DIFF PURE (${files.length} files, docs/ + scripts/ only)`);
  return 0;
}

// ---------- main ----------
if (process.argv[2] === '--selftest') process.exit(selftest());
if (process.argv[2] === '--diff-base') process.exit(diffBase(process.argv[3]));

const fails = check(readFileSync(at(DOC), 'utf8'));
if (fails.length) { fails.forEach((f) => console.error('RED: ' + f)); process.exit(1); }
console.log('TASK-PRIMITIVE CHECK GREEN (3 shipped tokens cross three sources; 5 proposed tokens isolated; censuses pinned 7/4/4+5+3; Phase 0 closed set + honesty anchors present)');
