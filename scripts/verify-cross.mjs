// verify-cross.mjs — machine assertion for cross-compiled artifacts
// without a runner for the target OS: checks the executable container
// header (ELF / PE / Mach-O), target machine word, and minimum size.
//
// The linux artifact is additionally *executed* by the caller (we run on
// linux/amd64); windows/darwin are verified structurally here.
//
// Usage:
//   node scripts/verify-cross.mjs            # verify the three dist targets, all must pass
//   node scripts/verify-cross.mjs --negative <file>  # control mode: file must FAIL
import { readFileSync, existsSync, readdirSync } from 'node:fs';

const MIN_BYTES = 1_000_000; // a Go hello binary is ~1.5MB+ even stripped

function fail(msg) {
  console.error('VERIFY FAIL: ' + msg);
  return false;
}

function checkELF(buf, file) {
  if (buf.length < 64) return fail(`${file}: too small for ELF header`);
  if (!(buf[0] === 0x7f && buf[1] === 0x45 && buf[2] === 0x4c && buf[3] === 0x46))
    return fail(`${file}: not ELF magic`);
  if (buf[4] !== 2) return fail(`${file}: ELF class must be 64-bit`);
  if (buf[5] !== 1) return fail(`${file}: ELF must be little-endian`);
  const machine = buf.readUInt16LE(18);
  if (machine !== 0x3e && machine !== 0xb7) return fail(`${file}: ELF e_machine=0x${machine.toString(16)}, want 0x3e (x86-64) or 0xb7 (aarch64)`);
  const etype = buf.readUInt16LE(16);
  if (etype !== 2 && etype !== 3) return fail(`${file}: ELF type ${etype}, want EXEC|DYN`);
  return true;
}

function checkPE(buf, file) {
  if (buf.length < 0x40) return fail(`${file}: too small for PE header`);
  if (!(buf[0] === 0x4d && buf[1] === 0x5a)) return fail(`${file}: missing MZ dos magic`);
  const lfanew = buf.readUInt32LE(0x3c);
  if (lfanew + 6 > buf.length) return fail(`${file}: e_lfanew ${lfanew} out of bounds`);
  if (buf.readUInt32LE(lfanew) !== 0x00004550) return fail(`${file}: missing PE\0\0 signature`);
  const machine = buf.readUInt16LE(lfanew + 4);
  if (machine !== 0x8664) return fail(`${file}: PE machine=0x${machine.toString(16)}, want 0x8664 (amd64)`);
  return true;
}

function checkMachO(buf, file) {
  if (buf.length < 8) return fail(`${file}: too small for Mach-O header`);
  const magic = buf.readUInt32LE(0);
  // 0xfeedfacf = MH_MAGIC_64 little-endian bytes cf fa ed fe
  if (magic !== 0xfeedfacf) return fail(`${file}: not Mach-O 64-bit magic (got 0x${magic.toString(16)})`);
  const cputype = buf.readInt32LE(4);
  if (cputype !== 0x01000007 && cputype !== 0x0100000c) return fail(`${file}: Mach-O cputype=0x${cputype.toString(16)}, want x86_64 or arm64`);
  return true;
}

function verify(file, kind) {
  if (!existsSync(file)) return fail(`${file}: missing`);
  const buf = readFileSync(file);
  if (buf.length < MIN_BYTES) return fail(`${file}: ${buf.length} bytes < ${MIN_BYTES} (implausible for a Go binary)`);
  if (kind === 'linux') return checkELF(buf, file);
  if (kind === 'windows') return checkPE(buf, file);
  if (kind === 'darwin') return checkMachO(buf, file);
  return fail(`${file}: unknown kind ${kind}`);
}

const argv = process.argv.slice(2);
if (argv[0] === '--negative') {
  const file = argv[1];
  if (!file) { console.error('usage: --negative <file>'); process.exit(2); }
  // Control mode: verification of an arbitrary non-target file MUST fail.
  const kinds = ['linux', 'windows', 'darwin'];
  let anyFailed = false;
  for (const k of kinds) if (!verify(file, k)) anyFailed = true;
  if (!anyFailed) { console.error('CONTROL FAIL: negative file passed a header check!'); process.exit(1); }
  console.log('CONTROL OK: negative file rejected by all header checks');
  process.exit(0);
}

const dist = 'dist';
// Verify every raw collector binary in dist/ (hello-collector and
// agent-collector across the release matrix). Packaged archives and the
// checksums files live beside them and are checked elsewhere; the raw
// executable set must cover every target, so a builder that silently
// drops a binary fails here.
const all = readdirSync(dist).filter((f) => /-collector-/.test(f) && !/\.(tar\.gz|zip)$/.test(f));
const files = [];
for (const f of all) {
  if (f.includes('-windows-')) files.push(['windows', `${dist}/${f}`]);
  else if (f.includes('-darwin-')) files.push(['darwin', `${dist}/${f}`]);
  else if (f.includes('-linux-')) files.push(['linux', `${dist}/${f}`]);
  else { console.error(`VERIFY FAIL: ${f} matches no known platform pattern`); process.exit(1); }
}
const TARGETS = ['linux-amd64', 'linux-arm64', 'darwin-arm64', 'darwin-amd64', 'windows-amd64'];
for (const target of TARGETS) {
  for (const base of ['hello-collector', 'agent-collector']) {
    if (!files.some(([, f]) => f.includes(`/${base}-`) && f.includes(`-${target}`))) {
      console.error(`VERIFY FAIL: missing ${base} artifact for ${target}`);
      process.exit(1);
    }
  }
}
let ok = true;
for (const [kind, file] of files) {
  if (verify(file, kind)) console.log(`VERIFY PASS ${kind}: ${file}`);
  else ok = false;
}
process.exit(ok ? 0 : 1);
