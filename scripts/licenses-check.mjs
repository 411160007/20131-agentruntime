// licenses-check.mjs — dependency license gate (company copyleft-veto policy).
//
// Rules enforced mechanically:
//  1. External module set from `go list -m all` (minus the main module)
//     must exactly match the `- module:` lines of docs/licenses.md
//     (line count == dependency count, plus set equality).
//  2. Every license listed must be in the allow-list (MIT / BSD family /
//     Apache-2.0). GPL / AGPL / LGPL and anything unknown = hard reject.
//  3. --selftest exercises the judge with synthetic allow/deny fixtures
//     (positive control: the deny path must actually fire).
//
// Usage:
//   node scripts/licenses-check.mjs
//   node scripts/licenses-check.mjs --selftest
import { readFileSync } from 'node:fs';
import { execFileSync } from 'node:child_process';

const ALLOW = new Set([
  'MIT', 'MIT-0',
  'Apache-2.0',
  'BSD-2-Clause', 'BSD-3-Clause', 'BSD-3-Clause-Clear',
  'ISC',
]);
const COPyleft_MARKERS = ['GPL', 'AGPL', 'LGPL', 'MPL', 'CC-BY-SA'];

// judge returns {ok, problems[]} for a list of {module, license}.
export function judge(deps) {
  const problems = [];
  for (const d of deps) {
    const up = d.license.toUpperCase();
    for (const m of COPyleft_MARKERS) {
      if (up.includes(m)) problems.push(`${d.module}: copyleft marker ${m} in "${d.license}" (hard veto)`);
    }
    if (!ALLOW.has(d.license)) problems.push(`${d.module}: license "${d.license}" not in allow-list`);
  }
  return { ok: problems.length === 0, problems };
}

function selftest() {
  let failures = 0;
  const expect = (name, got, want) => {
    if (got !== want) { console.error(`SELFTEST FAIL ${name}: got ${got}, want ${want}`); failures++; }
    else console.log(`SELFTEST OK ${name}`);
  };
  expect('allow-mixed', judge([{ module: 'a', license: 'MIT' }, { module: 'b', license: 'BSD-3-Clause' }, { module: 'c', license: 'Apache-2.0' }]).ok, true);
  expect('deny-gpl', judge([{ module: 'x', license: 'GPL-3.0' }]).ok, false);
  expect('deny-agpl', judge([{ module: 'y', license: 'AGPL-3.0' }]).ok, false);
  expect('deny-lgpl', judge([{ module: 'z', license: 'LGPL-2.1' }]).ok, false);
  expect('deny-unknown', judge([{ module: 'w', license: 'WTFPL' }]).ok, false);
  expect('deny-empty-license', judge([{ module: 'v', license: '' }]).ok, false);
  process.exit(failures ? 1 : 0);
}

function realCheck() {
  const goList = execFileSync('go', ['list', '-m', 'all'], { encoding: 'utf8' })
    .split('\n').map(s => s.trim()).filter(Boolean);
  const main = goList.find(l => l.startsWith('20131.com/agentruntime')) ?? goList[0];
  const external = goList.filter(l => l !== main).map(l => l.split(/\s+/)[0]);

  const doc = readFileSync('docs/licenses.md', 'utf8');
  const listed = [...doc.matchAll(/^- module: (\S+) \| license: (.+)$/gm)]
    .map(m => ({ module: m[1], license: m[2].trim() }));

  const problems = [];
  if (listed.length !== external.length)
    problems.push(`line count ${listed.length} != dependency count ${external.length}`);
  const listedSet = new Set(listed.map(l => l.module));
  for (const e of external) if (!listedSet.has(e)) problems.push(`module ${e} used but not listed`);
  for (const l of listed) if (!external.includes(l.module)) problems.push(`module ${l.module} listed but not used`);

  const j = judge(listed);
  problems.push(...j.problems);

  if (problems.length) {
    console.error('LICENSES FAIL:');
    for (const p of problems) console.error('  ' + p);
    process.exit(1);
  }
  console.log(`LICENSES OK: ${external.length} external dependency(ies), all allow-listed; docs/licenses.md lines reconciled`);
}

const mode = process.argv[2];
if (mode === '--selftest') selftest();
else realCheck();
