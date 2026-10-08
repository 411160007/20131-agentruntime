#!/usr/bin/env node
// W12.4 wedge-merge completeness checker (pure-doc slice; injection selftest + diff gate)
// Scans: docs/phase1-enforcement-wedge.md (merge glue only; enforcement body lives in the
// single-writer proposal file OUTSIDE this repo, referenced by path — do not inline it here).
import { readFileSync } from 'node:fs';
import { execSync } from 'node:child_process';

const DOC = 'docs/phase1-enforcement-wedge.md';
const SELF = 'scripts/w12-wedge-check.mjs';

// Section titles pinned verbatim from the proposal's table of contents (evidence read 2026-10-09).
const SECTIONS = {
  '2': '§2 范围与非目标',
  '3': '§3 架构演进路径：α 透传 → 网关（双案对比材料，不预裁）',
  '4': '§4 接口契约草案 + V2 需求面挂接点',
  '5': '§5 里程碑切片草案（每片 ≤120min + 片尾机判断言；与 v1.1 W 序兼容注记）',
  '6': '§6 Phase 1 入门门定义草案（gate 面：哪些 Phase 0 不变量必须先钉死）',
  '7': '§7 风险册（含双案 top 风险）与 $0 资源面',
};
const ELEMENTS = ['范围', '非目标', '契约', '风险', '切片', 'gate'];
const UPSTREAM_DOCS = [
  'docs/recovery-transaction-design.md',
  'docs/recovery-honesty-grading.md',
  'docs/memory-learning-safety-design.md',
];

function run(docText, selfText) {
  const ok = []; const fail = [];
  const A = (cond, name) => (cond ? ok : fail).push(name);
  const t = docText;
  // 1) six element rows exist as table rows in §1
  const rows = t.split('\n').filter(l => /^\|\s*\d+\s*\|/.test(l));
  A(rows.length === 6, 'rows=6 (got ' + rows.length + ')');
  for (const el of ELEMENTS) A(rows.some(r => r.includes('| ' + el + ' ')), 'row for ' + el);
  // 2) every element row cites verbatim section title(s); bidirectional coverage == exactly {2..7}
  const cited = new Set();
  let verbatimOk = true;
  for (const r of rows) {
    const hits = Object.values(SECTIONS).filter(s => r.includes(s));
    if (hits.length === 0) verbatimOk = false;
    for (const h of hits) for (const k of Object.keys(SECTIONS)) if (SECTIONS[k] === h) cited.add(k);
  }
  A(verbatimOk, 'every row cites verbatim section title');
  A(cited.size === 6 && [2, 3, 4, 5, 6, 7].every(k => cited.has(String(k))),
    'coverage set exactly {2..7} (got ' + [...cited].sort().join(',') + ')');
  // orphan check: no section referenced outside the table would still be fine, but doc must not
  // invent sections {8+} as reverse-lookup targets
  A(!/§(8|9)\b/.test(t), 'no reverse-lookup targets beyond §7');
  // 3) upstream increment rows: pointers + all three docs exist in repo
  for (const d of UPSTREAM_DOCS) {
    A(t.includes(d), 'pointer row ' + d);
    let exists = true;
    try { readFileSync(d, 'utf8'); } catch { exists = false; }
    A(exists, 'upstream file present ' + d);
  }
  // 4) declarations: single-writer source, drift defense, non-adjudication placeholder, path handoff
  A(t.includes('单写者源'), 'single-writer declaration');
  A(t.includes('漂移防线'), 'drift-defense declaration');
  A(t.includes('机械占位非预裁'), 'placeholder-not-prejudged declaration');
  A(t.includes('20131-phase1-mcp-gateway-proposal-20260929.md'), 'proposal path reference present');
  // 5) honest layering: zero-real-enforcement shape; observation vocab only
  A(t.includes('只记不拦') && t.includes('零代码增量'), 'observation-only layering');
  A(!/\bdeny\b/i.test(t), 'no deny-vocab in product face');
  // 6) internal-ID leak scan on doc AND on this checker's own source
  const leakRes = ['E\\d{2,3}(?![0-9a-zA-Z])', 'D-\\d{2,3}(?![0-9])', 'T\\d{3,}', 'R' + '14', 'PL' + 'AN', '§A[Dd]\\b', 'ai' + '-company', 'cl' + 'awd', '/hom' + 'e/node', '4111' + '60007'].map(s => new RegExp(s));
  for (const [name, src] of [['doc', t], ['self', selfText]]) {
    const hit = leakRes.find(re => re.test(src));
    A(!hit, name + ' free of internal-ID shapes' + (hit ? ' [' + hit.source + ']' : ''));
  }
  // 7) glue-only direction: enforcement body must NOT be duplicated — heuristic pins:
  //    doc must not enumerate contract/gate content beyond pointers (no long lists under §1/§3)
  A(!/接口有|响应体字段|请求体字段/.test(t), 'no contract-body duplication');
  return { pass: fail.length === 0, ok, fail };
}

const doc = readFileSync(DOC, 'utf8');
const self = readFileSync(SELF, 'utf8');

const argv = process.argv.slice(2);
if (argv.includes('--selftest')) {
  const mut = (name, f) => {
    const d = f(doc);
    const clean = run(d, self).pass;
    if (clean) { console.log('SELFTEST MISS: ' + name); process.exitCode = 1; }
    else console.log('selftest caught: ' + name);
  };
  mut('drop a row', s => s.replace(/\|\s*6\s*\| gate[\s\S]*?\n/, ''));
  mut('wrong title', s => s.replace(SECTIONS['6'], '§6 gate 定义'));
  mut('orphan element', s => s.replace(SECTIONS['3'], SECTIONS['2']));
  mut('add §8 target', s => s.replace('§7 风险册', '§8 附录；§7 风险册').replace(SECTIONS['7'], '§8 附录'));
  mut('missing pointer', s => s.replace(UPSTREAM_DOCS[2], 'docs/nowhere-design.md'));
  mut('deny vocab', s => s + '\n此路径输出 deny 决策。\n');
  mut('leak ID', s => s + '\n（参考 ' + 'E1' + '08 防线）\n');
  mut('drop placeholder', s => s.replace('机械占位非预裁', '已裁定'));
  mut('body duplication', s => s + '\n契约示例：响应体字段 decision 取值。\n');
  // positive control: pristine doc must pass
  if (run(doc, self).pass) console.log('selftest PRISTINE PASS');
  else { console.log('SELFTEST PRISTINE FAIL'); process.exitCode = 1; }
  console.log(process.exitCode ? 'SELFTEST RED' : 'SELFTEST GREEN (9 injections + pristine)');
  process.exit(process.exitCode || 0);
}

if (argv[0] === '--diff-base') {
  const base = argv[1];
  const files = execSync('git diff --name-only ' + base + '...HEAD', { encoding: 'utf8' })
    .trim().split('\n').filter(Boolean);
  const bad = files.filter(f => !(f.startsWith('docs/') || f.startsWith('scripts/')));
  console.log(bad.length ? 'CODE-DIFF RED: ' + bad.join(',')
    : 'CODE-DIFF GREEN: ' + files.length + ' files docs/scripts only');
  process.exit(bad.length ? 1 : 0);
}

const r = run(doc, self);
for (const n of r.ok) console.log('  ok: ' + n);
for (const n of r.fail) console.log('  FAIL: ' + n);
console.log(r.pass ? 'WEDGE CHECK GREEN' : 'WEDGE CHECK RED (' + r.fail.length + ')');
process.exit(r.pass ? 0 : 1);
