// apicontract-check.mjs — dual-source cross-check gate:
// docs/api-v0.md contract lists  ==  scripts/validate-jsonl.mjs lists
// (== Go enums, pinned separately by internal/schema sync tests).
//
// The contract document is the frozen external promise; any drift
// between what we SHIP (validator), what we CODE (Go enums), and what
// we DOCUMENT (api-v0.md) fails this gate.
//
// Usage:
//   node scripts/apicontract-check.mjs                 # real check
//   node scripts/apicontract-check.mjs --negative      # control: a
//   deliberately broken temp copy of the doc MUST be rejected.
import { readFileSync, writeFileSync, mkdtempSync, rmSync } from 'node:fs';
import { join } from 'node:path';
import { tmpdir } from 'node:os';

const KEYS = ['stages', 'types', 'decisions', 'tiers', 'caps', 'res_classes', 'states'];

function contractFromDoc(text) {
  const out = {};
  const re = /```contract\n([^\n]+)\n```/g;
  let m;
  while ((m = re.exec(text))) {
    const [head, rest] = m[1].split(':');
    const key = head.trim();
    out[key] = rest.split(',').map((s) => s.trim()).filter(Boolean);
  }
  const vline = text.match(/^```contract\nversion_regex: .+$/m);
  if (vline) {
    const vm = /version_regex: (.+)$/.exec(vline[0]);
    out.version_regex = vm[1];
  }
  return out;
}

function listsFromValidator(src) {
  const grab = (name) => {
    const m = new RegExp(`const ${name} = \\[([\\s\\S]*?)\\];`).exec(src);
    if (!m) throw new Error(`validator const ${name} not found`);
    const vals = [...m[1].matchAll(/'([^']+)'/g)].map((x) => x[1]);
    if (vals.length === 0) throw new Error(`validator const ${name} parsed empty`);
    return vals;
  };
  return {
    stages: grab('STAGES'),
    types: grab('TYPES'),
    decisions: grab('DECISIONS'),
    tiers: grab('TIER'),
    caps: grab('CAPS'),
    res_classes: grab('RES_CLASSES'),
  };
}

// The passport states enum lives in Go only (internal/identity); the
// doc is its external source; keep them pinned via the Go table here by
// parsing identity.go directly (third source shrinks drift surface).
function statesFromGo(src) {
  const vals = [...src.matchAll(/State(\w+)\s+State = "(\w+)"/g)].map((m) => m[2]);
  if (vals.length !== 3) throw new Error(`identity.go states parsed ${vals.length}, want 3`);
  return vals;
}

function sameList(a, b) {
  return a.length === b.length && a.every((v, i) => v === b[i]);
}

const docPath = 'docs/api-v0.md';
const doc = readFileSync(docPath, 'utf8');
const val = readFileSync('scripts/validate-jsonl.mjs', 'utf8');
const goIdentity = readFileSync('internal/identity/identity.go', 'utf8');

if (process.argv.includes('--negative')) {
  // Control: tamper ONE token in a temp copy; the checker must reject.
  const tmp = mkdtempSync(join(tmpdir(), 'apictl'));
  try {
    const broken = doc.replace('res_classes: low, medium, high', 'res_classes: low, medium, extreme');
    if (broken === doc) throw new Error('negative control failed to tamper the doc');
    writeFileSync(join(tmp, 'api-v0.md'), broken);
    const d = readFileSync(join(tmp, 'api-v0.md'), 'utf8');
    const bad = contractFromDoc(d);
    const good = listsFromValidator(val);
    if (sameList(bad.res_classes, good.res_classes)) {
      console.error('NEGATIVE CONTROL FAIL: tampered contract passed!');
      process.exit(1);
    }
    console.log('NEGATIVE CONTROL OK: tampered res_classes detected as drift');
  } finally {
    rmSync(tmp, { recursive: true, force: true });
  }
  process.exit(0);
}

const c = contractFromDoc(doc);
const v = listsFromValidator(val);
const st = statesFromGo(goIdentity);
let drift = 0;
for (const k of KEYS) {
  if (!c[k] || !Array.isArray(c[k]) || c[k].length === 0) {
    console.error(`DRIFT: contract list "${k}" missing/empty in ${docPath}`);
    drift++;
    continue;
  }
  if (k === 'states') {
    if (!sameList(c[k], st)) {
      console.error(`DRIFT: states doc=${c[k]} go=${st}`);
      drift++;
    }
    continue;
  }
  if (!sameList(c[k], v[k])) {
    console.error(`DRIFT: ${k} doc=${c[k]} validator=${v[k]}`);
    drift++;
  }
}
// version regex contract: must match shipped --version shapes exactly
if (!c.version_regex) {
  console.error('DRIFT: version_regex contract missing');
  drift++;
} else {
  const rx = new RegExp(c.version_regex);
  const goodA = 'agent-collector 0.4.0-d4 linux/amd64 (go1.27.1)';
  const goodH = 'hello-collector 0.4.0-d4 windows/arm64 (go1.27.1)';
  const badV = 'agent-collector v3 linux (go)';
  if (!rx.test(goodA) || rx.test(badV)) {
    console.error('DRIFT: version_regex contract does not pin the frozen shape');
    drift++;
  }
  // hello name is a documented sibling of the same shape
  const helloRx = new RegExp(String(c.version_regex).replace('^agent-collector', '^hello-collector'));
  if (!helloRx.test(goodH)) {
    console.error('DRIFT: version shape does not carry over to the hello binary');
    drift++;
  }
}
if (drift) {
  console.error(`API-CONTRACT CHECK FAIL: ${drift} drift item(s)`);
  process.exit(1);
}
console.log(`API-CONTRACT OK: ${docPath} == validator == Go (${KEYS.length} vocabularies + version shape cross-checked)`);
