// w12-grading-check.mjs — machine checker for the recovery honesty
// grading document (spec section 247 outward truthfulness face).
//
// Design-slice checker: it re-parses the four recovery grades, the
// execution-plane constant, and the truthfulness rule from the shipped
// schema source (internal/schema/impactrecovery.go) — parsed, never
// hand-copied — and pins the document against them. It also censuses
// the per-face grading table, enforces the closed forbidden-marketing
// clause list, and runs the forbidden word-family scan across the
// outward corpus (README + all docs), with a two-line soft-wrap window
// so that a prohibition marker on the previous line still licenses an
// honest quote. Documentation slice: zero Go/code change expected.
//
// Checks:
//   1. shipped source parses exactly four wire tokens in normative order;
//   2. display anchor block: four lines, back-mapping (lower-case,
//      spaces and hyphens to single underscores) equals the parsed
//      tokens in order;
//   3. every parsed wire token is restated in the document;
//   4. the execution-plane token and truthfulness-rule token parsed
//      from source are restated verbatim in the document;
//   5. the spec's verbatim prohibition line (不得 + quoted forbidden
//      Chinese clause on one line) is present;
//   6. the forbidden-marketing list block: at least six unique
//      entries, at least one Latin-only and at least one CJK entry;
//   7. per-face table: exactly six rows, each naming exactly one of
//      the four wire tokens (hollow row red, double-grade row red);
//   8. the class-qualified claims rule sentence is present;
//   9. the deferred-decision verbatim registration is present
//      (observation recovery point stays with its decision slot);
//  10. forbidden word-family scan across the outward corpus: any
//      listed claim line must carry (on its own line or the previous
//      line, soft-wrap aware) a prohibition marker; plus a targeted
//      never-say: no line may claim the execution plane ships unless
//      negated in the same line.
//
// --selftest re-runs every check against deliberately broken in-memory
// copies (positive control: each injected defect must be caught) and
// against the real document + corpus (must pass).
import { readFileSync, readdirSync } from 'node:fs';
import * as path from 'node:path';
import { fileURLToPath } from 'node:url';

// repo root from this script's own location, never from caller cwd
const root = path.resolve(path.dirname(fileURLToPath(import.meta.url)), '..');
const at = (rel) => path.join(root, rel);

const DOC = 'docs/recovery-honesty-grading.md';
const IMPACT = 'internal/schema/impactrecovery.go';

const parseImpact = (src) => {
  const classes = [...src.matchAll(/Recovery[A-Za-z]+\s+RecoveryClass = "([a-z_]+)"/g)]
    .map((m) => m[1]);
  const plane = (src.match(/const RecoveryExecutionPlane = "([^"]+)"/) || [])[1];
  const rule = (src.match(/const RecoveryTruthfulnessRule = "([^"]+)"/) || [])[1];
  return { classes, plane, rule };
};

const fencedBlock = (text, fence) => {
  const m = text.match(new RegExp('```' + fence + '\\n([\\s\\S]*?)```'));
  return m ? m[1].replace(/\n$/, '') : null;
};

const displayToToken = (line) =>
  line.trim().toLowerCase().replace(/[\s-]+/g, '_');

const hasToken = (text, token) => {
  const re = new RegExp('(?:^|[^A-Za-z0-9_])' + token + '(?:[^A-Za-z0-9_]|$)', 'm');
  return re.test(text);
};

const MARKER = /must never|must not|forbid|prohibit|never advertise|never-claim|prohibition|不得|禁/;
const EXEC_CLAIM = /execution plane ships|execution is shipped|recovery execution ships/;
const EXEC_NEGATE = /\bno\b|\bnone\b|\bnot\b|never/;

const check = (doc, impactSrc, corpus) => {
  const fails = [];
  const need = (ok, msg) => { if (!ok) fails.push(msg); };
  const { classes, plane, rule } = parseImpact(impactSrc);

  // 1. shipped vocabulary
  need(classes.length === 4,
    `shipped recovery classes must parse as 4, got ${classes.length}`);

  // 2. display anchor back-map
  const disp = fencedBlock(doc, 'recovery-class-display');
  need(disp !== null, 'display anchor block ```recovery-class-display missing');
  if (disp !== null) {
    const lines = disp.split('\n').filter((l) => l.trim() !== '');
    need(lines.length === 4, `display anchor must carry exactly 4 lines, got ${lines.length}`);
    lines.forEach((l, i) => {
      need(classes[i] !== undefined && displayToToken(l) === classes[i],
        `display line ${i + 1} "${l.trim()}" does not back-map to parsed token ${classes[i]}`);
    });
  }

  // 3. wire tokens restated
  for (const t of classes) need(hasToken(doc, t), `doc must restate shipped wire token ${t}`);

  // 4. plane + rule restated verbatim
  need(plane && doc.includes(plane), `doc must restate execution-plane token (${plane})`);
  need(rule && doc.includes(rule), `doc must restate truthfulness-rule token (${rule})`);

  // 5. verbatim prohibition line
  const prohibitLine = doc.split('\n')
    .some((l) => l.includes('不得') && l.includes('所有动作都可以撤销'));
  need(prohibitLine, 'doc must carry the spec verbatim prohibition line (不得宣传 + quoted clause)');

  // 6. forbidden marketing list census
  const listBlock = fencedBlock(doc, 'forbidden-marketing-lines');
  need(listBlock !== null, 'forbidden list block ```forbidden-marketing-lines missing');
  const forbids = listBlock === null ? [] : listBlock.split('\n').map((l) => l.trim()).filter((l) => l !== '');
  need(forbids.length >= 6, `forbidden list needs >= 6 entries, got ${forbids.length}`);
  need(new Set(forbids).size === forbids.length, 'forbidden list entries must be unique');
  need(forbids.some((f) => !/[\u4e00-\u9fff]/.test(f)), 'forbidden list needs at least one Latin entry');
  need(forbids.some((f) => /[\u4e00-\u9fff]/.test(f)), 'forbidden list needs at least one CJK entry');

  // 7. per-face table census
  const faceSec = doc.split('\n## ').find((s) => s.startsWith('3.')) || '';
  const rows = faceSec.split('\n').filter((l) => l.trim().startsWith('|'))
    .filter((l) => !/\|\s*face\s*\|/.test(l) && !/^\s*\|[-\s|:]+\|\s*$/.test(l));
  need(rows.length === 6, `per-face table must have exactly 6 rows, got ${rows.length}`);
  for (const r of rows) {
    const hits = classes.filter((t) => hasToken(r, t));
    need(hits.length === 1,
      `face row must name exactly one grade, got ${hits.length} in: ${r.slice(0, 60)}...`);
  }

  // 8. class-qualified claims rule
  need(doc.includes('class-qualified'), 'class-qualified claims rule sentence missing');

  // 9. deferred decision verbatim
  need(doc.includes('观测型 recovery point 实装与否') && doc.includes('另裁位保持'),
    'deferred-decision verbatim registration missing (observation recovery point stays with its decision slot)');

  // 10. forbidden word-family scan across outward corpus
  const inListFence = (lines, i, fence) => {
    let inside = false;
    for (let k = 0; k < i; k++) {
      if (lines[k].startsWith('```' + fence)) inside = true;
      else if (inside && /^```\s*$/.test(lines[k])) inside = false;
    }
    return inside;
  };
  for (const f of corpus) {
    const lines = f.text.split('\n');
    for (let i = 0; i < lines.length; i++) {
      const line = lines[i];
      if (inListFence(lines, i, 'forbidden-marketing-lines')) continue;
      const hit = forbids.find((fb) => line.includes(fb));
      if (hit) {
        const window = line + ' ' + (i > 0 ? lines[i - 1] : '');
        if (!MARKER.test(window)) {
          fails.push(`forbidden claim without prohibition context: ${f.name}:${i + 1} "${line.trim().slice(0, 70)}"`);
        }
      }
      if (EXEC_CLAIM.test(line) && !EXEC_NEGATE.test(line)) {
        fails.push(`execution-shipping claim not allowed: ${f.name}:${i + 1} "${line.trim().slice(0, 70)}"`);
      }
    }
  }

  return fails;
};

const buildCorpus = () => {
  const files = ['README.md'];
  for (const n of readdirSync(at('docs'))) if (n.endsWith('.md')) files.push(path.join('docs', n));
  return files.filter((rel) => { try { readFileSync(at(rel)); return true; } catch { return false; } })
    .map((rel) => ({ name: rel, text: readFileSync(at(rel), 'utf8') }));
};

const banner = (doc, impact, corpus) => {
  const fails = check(doc, impact, corpus);
  if (fails.length) {
    for (const f of fails) console.error('FAIL:', f);
    console.error('GRADING CHECK RED');
    return 1;
  }
  const { classes } = parseImpact(impact);
  const listBlock = fencedBlock(doc, 'forbidden-marketing-lines');
  const n = listBlock.split('\n').filter((l) => l.trim() !== '').length;
  console.log(`GRADING CHECK GREEN (${classes.length} classes restated, 6 faces censused, ${n} forbidden clauses, corpus ${corpus.length} files scanned)`);
  return 0;
};

const selftest = () => {
  const doc = readFileSync(at(DOC), 'utf8');
  const impact = readFileSync(at(IMPACT), 'utf8');
  const corpus = buildCorpus();
  if (check(doc, impact, corpus).length) {
    console.error('SELFTEST: real doc does not pass');
    return 1;
  }
  const defects = [
    ['rename a display line', (d) => d.replace('LOCAL PARTIAL', 'LOCAL PARTLY')],
    ['drop a shipped token restatement', (d) => d.replace(/local_partial/g, 'local_partiality')],
    ['alter the execution-plane token', (d) => d.replace(/none-in-observation-phase/g, 'none-in-later-phase')],
    ['alter the truthfulness rule', (d) => d.replace(/never-claim-fully-reversible/g, 'claim-fully-reversible')],
    ['invert the verbatim prohibition', (d) => d.replace('不得宣传', '可以宣传')],
    ['shrink the forbidden list below six', (d) => d.replace('all actions can be undone\nevery action is reversible\nfully reversible\n', '').replace('一键恢复任何操作\n', '')],
    ['hollow a face row', (d) => d.replace('| local_reversible | The byte-for-byte', '|  | The byte-for-byte')],
    ['double-grade a face row', (d) => d.replace('needs no outside cooperation', 'needs no outside cooperation local_partial')],
    ['drop the class-qualified rule', (d) => d.replace(/\*\*class-qualified\*\*/g, 'plain')],
    ['drop the deferred verbatim', (d) => d.replace(/观测型 recovery point 实装与否/, 'observation recovery point whether-to-build')],
    ['assert a forbidden claim in the corpus', (d, c) => ({ doc: d, corpus: c.concat([{ name: 'README.md#injected', text: 'Our runtime is fully reversible for every recorded change.\n' }]) })],
    ['claim the execution plane ships', (d, c) => ({ doc: d, corpus: c.concat([{ name: 'docs/injected.md', text: 'the execution plane ships in this release.\n' }]) })],
  ];
  let caught = 0;
  for (const [name, mutate] of defects) {
    let docMut = doc; let corpusMut = corpus;
    const r = mutate(doc, corpus);
    if (typeof r === 'object' && r && r.doc !== undefined) { docMut = r.doc; corpusMut = r.corpus; }
    else docMut = r;
    const fails = check(docMut, impact, corpusMut);
    if (fails.length) caught++;
    else console.error(`SELFTEST MISS: ${name} not caught`);
  }
  if (caught !== defects.length) {
    console.error(`SELFTEST RED: ${caught}/${defects.length} defects caught`);
    return 1;
  }
  console.log(`SELFTEST GREEN (${caught} injected defects all caught)`);
  return 0;
};

if (process.argv[2] === '--selftest') process.exit(selftest());
process.exit(banner(
  readFileSync(at(DOC), 'utf8'),
  readFileSync(at(IMPACT), 'utf8'),
  buildCorpus(),
));
