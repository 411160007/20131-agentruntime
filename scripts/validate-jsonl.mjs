// validate-jsonl.mjs — schema validator for collector JSONL output.
// Independently re-implements the field contract of the Go schema package
// (second source of truth for the gate; divergence is caught by the
// cross-check fixtures and the enum sync tests below).
//
// Usage:
//   node scripts/validate-jsonl.mjs <file> [--min-lines N]
//   node scripts/validate-jsonl.mjs --expect-reject <file>   # control mode
import { readFileSync } from 'node:fs';

const STAGES = ['proposed', 'evaluated', 'enforcement', 'action', 'observation'];
const TYPES = [
  'command.proposed', 'tool.call', 'file.access', 'network.intent',
  'policy.decision', 'enforce.action', 'collector.start', 'collector.stop',
  'agent.detected', 'agent.scan',
];
const DECISIONS = ['allow', 'ask', 'would_block'];
// TIER mirrors the Go schema Tier enum (internal/schema/event.go); the
// sync test fails if the two sources diverge. Absent key = L1 default.
const TIER = ['L0', 'L1', 'L2', 'L3'];
// CAPS mirrors internal/schema/capability.go; RES_CLASSES the three
// sensitivity levels. attrs["cap"]/attrs["res_class"] are validated here
// exactly as Event.Validate() validates them in Go.
const CAPS = [
  'file.read', 'file.write', 'file.delete', 'net.outbound', 'net.listen',
  'proc.spawn', 'shell.exec', 'mcp.tool', 'credential.access', 'env.read',
  'process.inspect',
];
const RES_CLASSES = ['low', 'medium', 'high'];
const ID_RE = /^[A-Za-z0-9][A-Za-z0-9._-]{0,63}$/;
const SCHEMA_VERSION = 1;

function lineErrors(e) {
  const errs = [];
  if (typeof e !== 'object' || e === null) return ['not an object'];
  if (e.v !== SCHEMA_VERSION) errs.push(`v=${JSON.stringify(e.v)}`);
  if (typeof e.id !== 'string' || !ID_RE.test(e.id)) errs.push(`id=${JSON.stringify(e.id)}`);
  if (typeof e.agent_id !== 'string' || !ID_RE.test(e.agent_id)) errs.push(`agent_id=${JSON.stringify(e.agent_id)}`);
  if (typeof e.ts !== 'string' || Number.isNaN(Date.parse(e.ts))) errs.push(`ts=${JSON.stringify(e.ts)}`);
  else {
    const year = new Date(e.ts).getUTCFullYear();
    if (year < 2020 || year > 2100) errs.push(`ts year ${year} out of range`);
  }
  if (!STAGES.includes(e.stage)) errs.push(`stage=${JSON.stringify(e.stage)}`);
  if (!TYPES.includes(e.type)) errs.push(`type=${JSON.stringify(e.type)}`);
  if (!DECISIONS.includes(e.decision)) errs.push(`decision=${JSON.stringify(e.decision)}`);
  if (!Number.isInteger(e.severity) || e.severity < 0 || e.severity > 4) errs.push(`severity=${JSON.stringify(e.severity)}`);
  if (e.tier !== undefined && !TIER.includes(e.tier)) errs.push(`tier=${JSON.stringify(e.tier)}`);
  if (typeof e.summary !== 'string' || e.summary.length === 0 || Buffer.byteLength(e.summary) > 512) errs.push('summary invalid');
  if (e.attrs !== undefined) {
    if (typeof e.attrs !== 'object' || e.attrs === null || Array.isArray(e.attrs)) errs.push('attrs not a map');
    else for (const [k, v] of Object.entries(e.attrs)) {
      if (!k || k.length > 64) errs.push(`attr key ${JSON.stringify(k)} invalid`);
      if (typeof v !== 'string' || Buffer.byteLength(v) > 1024) errs.push(`attr ${k} value invalid`);
      if (k === 'cap' && !CAPS.includes(v)) errs.push(`attr cap ${JSON.stringify(v)} not in vocabulary`);
      if (k === 'res_class' && !RES_CLASSES.includes(v)) errs.push(`attr res_class ${JSON.stringify(v)} not in vocabulary`);
    }
  }
  return errs;
}

function validateFile(file) {
  const raw = readFileSync(file, 'utf8');
  if (raw.trim() === '') return { lines: [], errors: ['file is empty'] };
  const lines = raw.split('\n');
  if (lines.at(-1) === '') lines.pop();
  else return { lines, errors: [`${file}: missing trailing newline (JSONL requires \\n-terminated records)`] };
  const errors = [];
  lines.forEach((ln, i) => {
    let obj;
    try { obj = JSON.parse(ln); } catch (e) { errors.push(`line ${i + 1}: JSON parse: ${e.message}`); return; }
    for (const msg of lineErrors(obj)) errors.push(`line ${i + 1}: ${msg}`);
  });
  return { lines, errors };
}

const argv = process.argv.slice(2);
if (argv[0] === '--expect-reject') {
  // Control mode: fixture file containing bad records must be rejected.
  const file = argv[1];
  if (!file) { console.error('usage: --expect-reject <file>'); process.exit(2); }
  const { errors } = validateFile(file);
  if (errors.length === 0) { console.error('CONTROL FAIL: bad fixture passed validation!'); process.exit(1); }
  console.log(`CONTROL OK: fixture rejected with ${errors.length} error(s)`);
  for (const e of errors.slice(0, 5)) console.log('  ' + e);
  process.exit(0);
}

const file = argv[0];
if (!file) { console.error('usage: validate-jsonl.mjs <file> [--min-lines N] | --expect-reject <file>'); process.exit(2); }
const minIdx = argv.indexOf('--min-lines');
const minLines = minIdx >= 0 ? parseInt(argv[minIdx + 1], 10) : 1;
const { lines, errors } = validateFile(file);
if (errors.length) {
  console.error(`VALIDATE FAIL ${file}: ${errors.length} error(s)`);
  for (const e of errors.slice(0, 20)) console.error('  ' + e);
  process.exit(1);
}
if (lines.length < minLines) {
  console.error(`VALIDATE FAIL ${file}: ${lines.length} lines < required ${minLines}`);
  process.exit(1);
}
console.log(`VALIDATE OK ${file}: ${lines.length} valid JSONL event(s)`);
