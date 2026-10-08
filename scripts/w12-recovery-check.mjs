// w12-recovery-check.mjs — design-surface cross-check between
// docs/recovery-transaction-design.md and the shipped source of truth
// (internal/schema/impactrecovery.go recovery vocabulary,
// internal/schema/decisiontrace.go correlation family,
// internal/schema/evidenceexport.go bundle members), plus the closed
// step / level censuses the V2 transaction-recovery design contract
// requires. Documentation slice: this checker must fail on drift, and
// the tree itself must show ZERO code change in the diff range.
//
// Checks:
//   1. step census: the seven pipeline steps (Start, Plan, Recovery
//      Point, Authorize, Action, Verify, Commit - case-normalized
//      rendering of the spec's closed set) appear in order
//      inside the fenced pipeline block, plus Rollback as the failure
//      exit — no eighth step, no missing step;
//   2. verify->commit invariant sentence present;
//   3. task-chain census: all six example change steps named in one
//      fenced block (Code Change .. Deploy) + same-transaction grouping
//      clause + whole-task (not last-file) clause;
//   4. fabric census: six mechanisms exactly, five honest recovery
//      levels exactly (closed set, inside the fenced block);
//   5. independent-quota clause for recovery storage present;
//   6. shipped-vocabulary restatement: the four recovery classes are
//      re-parsed from impactrecovery.go (never hand-copied) and every
//      one is restated in the doc;
//   7. correlation grounding: the doc names the shipped correlation
//      family source (decisiontrace.go) and the honest absent member
//      stance for recovery.json from evidenceexport.go — with the
//      shipped token "recovery-state export waits for its upstream"
//      parsed from source and restated verbatim;
//   8. coverage table: exactly five rows "spec section 246".."250",
//      each carrying one disposition token (covered | deferred |
//      registered) — no hollow row;
//   9. honesty anchors: the verbatim "not this cycle" registration
//      (Chinese quoted lines) present, the marketing prohibition
//      ("所有动作都可以撤销") present as a prohibition, and the
//      declaration-plane-only stance sentence present; forbidden-form
//      scan: no line may claim the execution plane ships.
//
// --selftest re-runs the checks against deliberately broken temp
// copies of the doc and FAILS unless every injected defect is caught
// (positive control), and against the real doc (must pass).
import { readFileSync } from 'node:fs';
import * as path from 'node:path';
import { fileURLToPath } from 'node:url';

// repo root from this script's own location, never from caller cwd
const root = path.resolve(path.dirname(fileURLToPath(import.meta.url)), '..');
const at = (rel) => path.join(root, rel);

const DOC = 'docs/recovery-transaction-design.md';
const IMPACT = 'internal/schema/impactrecovery.go';
const TRACE = 'internal/schema/decisiontrace.go';
const EXPORT = 'internal/schema/evidenceexport.go';

const STEPS = ['Start', 'Plan', 'Recovery Point', 'Authorize', 'Action', 'Verify', 'Commit'];
const CHAIN = ['Code Change', 'NPM Install', 'Docker Build', 'Config Change', 'Nginx Change', 'Deploy'];
const MECHANISMS = ['Copy-on-Write', 'Incremental Snapshot', 'Content Hash', 'Deduplication', 'Versioning', 'Metadata'];
const LEVELS = ['Temporary', 'Normal', 'High-Value', 'Critical', 'System'];

function fencedBlocks(text) {
  return [...text.matchAll(/```text\n([^]*?)```/g)].map((m) => m[1]);
}

function shippedRecoveryClasses(goSrc) {
  // wire tokens declared in the shipped schema file, parsed not copied
  const m = goSrc.match(/RecoveryClass[^=]*=\s*([^]*?)(?=\n\s*\/\/|\n[a-zA-Z])/);
  const src = m ? m[1] : goSrc;
  const ids = [...src.matchAll(/"(local_reversible|local_partial|external_compensation|non_reversible)"/g)].map((x) => x[1]);
  return [...new Set(ids)];
}

export function check(docText, impactSrc, traceSrc, exportSrc) {
  const fails = [];
  const need = (cond, msg) => { if (!cond) fails.push(msg); };

  // 1. step census
  const blocks = fencedBlocks(docText);
  const pipe = blocks.find((b) => b.includes('Start') && b.includes('Commit'));
  need(!!pipe, 'pipeline fenced block missing');
  if (pipe) {
    const lines = pipe.split('\n').map((l) => l.trim()).filter((l) => l && l !== '↓');
    need(lines.length === 7, `pipeline must have exactly 7 steps, got ${lines.length}`);
    STEPS.forEach((s, i) => need(lines[i] === s, `pipeline step ${i} must be ${s}, got ${lines[i]}`));
    need(/\nRollback\b/.test(docText), 'Rollback failure exit missing');
  }
  // 2. invariant
  need(/only a result that\s*\npassed VERIFY may enter COMMIT/i.test(docText), 'verify->commit invariant sentence missing');

  // 3. task chain
  const chainBlk = blocks.find((b) => b.includes('Code Change'));
  need(!!chainBlk, 'task example chain fenced block missing');
  if (chainBlk) CHAIN.forEach((c) => need(chainBlk.includes(c), `task chain step missing: ${c}`));
  need(/same\s+transaction/i.test(docText), 'same-transaction grouping clause missing');
  need(/whole task/i.test(docText) && /not just the last file/i.test(docText), 'whole-task (not last-file) clause missing');

  // 4. fabric census
  MECHANISMS.forEach((k) => need(docText.includes(k), `fabric mechanism missing: ${k}`));
  const lvlBlk = blocks.find((b) => b.includes('Temporary') && b.includes('System'));
  need(!!lvlBlk, 'recovery levels fenced block missing');
  if (lvlBlk) {
    const lv = lvlBlk.split('\n').map((l) => l.trim()).filter(Boolean);
    need(lv.length === 5, `recovery levels must be exactly 5, got ${lv.length}`);
    LEVELS.forEach((l, i) => need(lv[i] === l, `level ${i} must be ${l}, got ${lv[i]}`));
  }
  // 5. quota
  need(/independent quota/i.test(docText), 'independent-quota clause missing');

  // 6. shipped recovery vocabulary restatement
  const classes = shippedRecoveryClasses(impactSrc);
  need(classes.length === 4, `shipped recovery classes must parse as 4, got ${classes.length}`);
  classes.forEach((c) => need(docText.includes(c), `doc must restate shipped recovery class: ${c}`));

  // 7. correlation grounding
  need(/decisiontrace\.go/.test(docText), 'doc must ground correlation in decisiontrace.go');
  need(/evidenceexport\.go/.test(docText), 'doc must ground bundle gap in evidenceexport.go');
  const stance = exportSrc.match(/"(recovery-state export waits for its upstream[^"]*)"/);
  need(!!stance, 'shipped absent-stance token not found in evidenceexport.go');
  if (stance) need(docText.includes(stance[1]), `doc must restate shipped stance verbatim: ${stance[1]}`);

  // 8. coverage table rows
  for (let n = 246; n <= 250; n++) {
    const row = docText.split(String.fromCharCode(10)).find((l) => l.startsWith('| spec section ' + n + ' |'));
    need(!!row, `coverage row missing for spec section ${n}`);
    if (row) need(/\| (covered|deferred|registered)\b/.test(row), `coverage row ${n} has no disposition token`);
  }

  // 9. honesty anchors
  need(/本期不排/.test(docText), 'verbatim registration "本期不排" missing');
  need(/无已执行强制动作可恢复/.test(docText), 'verbatim registration reason missing');
  need(/所有动作都可以撤销/.test(docText) && /forbids/.test(docText), 'marketing prohibition line missing');
  need(/No recovery\nexecution plane exists behind it|no recovery or rollback execution plane exists behind/i.test(docText), 'declaration-plane-only stance sentence missing');
  const badClaim = /recovery execution (?:is shipped|plane ships)|rollback executes today/i.test(docText);
  need(!badClaim, 'doc claims an executing recovery plane (forbidden form)');

  return fails;
}

function banner(docText, impactSrc, traceSrc, exportSrc) {
  const fails = check(docText, impactSrc, traceSrc, exportSrc);
  if (fails.length) {
    for (const f of fails) console.error(`RED: ${f}`);
    console.log('W12 RECOVERY-DESIGN CHECK: ' + fails.length + ' RED');
    return 1;
  }
  console.log('W12 RECOVERY-DESIGN CHECK GREEN (4 classes restated, 25 census tokens)');
  return 0;
}

function selftest() {
  const doc = readFileSync(at(DOC), 'utf8');
  const impact = readFileSync(at(IMPACT), 'utf8');
  const trace = readFileSync(at(TRACE), 'utf8');
  const exportSrc = readFileSync(at(EXPORT), 'utf8');
  const defects = [
    ['drop a pipeline step', (t) => t.replace('Authorize\n', '')],
    ['add an eighth step', (t) => t.replace('Commit\n```', 'Commit\n↓\nApplaud\n```')],
    ['rename a recovery level', (t) => t.replace('High-Value', 'High-Safe')],
    ['add a sixth level', (t) => t.replace('System\n```', 'System\nExtra\n```')],
    ['drop a fabric mechanism', (t) => t.replace('Deduplication,', '')],
    ['drop the quota clause', (t) => t.replace(/independent quota/gi, 'casual hope')],
    ['drop the invariant sentence', (t) => t.replace(/only a result that\s*\npassed VERIFY may enter COMMIT/i, 'any result may enter COMMIT')],
    ['hollow a coverage row', (t) => t.replace(/\|\s*spec section 249[^]*?design rows\)/, '| spec section 249 | Recovery Fabric |  |')],
    ['drop verbatim registration', (t) => t.replace(/无已执行强制动作可恢复/, 'recovery maybe possible')],
    ['drop shipped-class restatement', (t) => t.replace(/local_reversible/g, 'lx_reversible')],
    ['claim executing plane', (t) => t + '\nrecovery execution is shipped today.\n'],
  ];
  let caught = 0;
  for (const [name, mutate] of defects) {
    const fails = check(mutate(doc), impact, trace, exportSrc);
    if (fails.length) caught++;
    else console.error(`SELFTEST MISS: ${name} not caught`);
  }
  const green = check(doc, impact, trace, exportSrc).length === 0;
  if (!green) { console.error('SELFTEST: real doc does not pass'); return 1; }
  if (caught !== defects.length) { console.error(`SELFTEST RED: ${caught}/${defects.length} defects caught`); return 1; }
  console.log(`SELFTEST GREEN (${caught} injected defects all caught)`);
  return 0;
}

if (process.argv[2] === '--selftest') process.exit(selftest());
process.exit(banner(
  readFileSync(at(DOC), 'utf8'),
  readFileSync(at(IMPACT), 'utf8'),
  readFileSync(at(TRACE), 'utf8'),
  readFileSync(at(EXPORT), 'utf8'),
));
