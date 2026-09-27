// macho-sig-check.mjs — machine gate for the LC_CODE_SIGNATURE load command
// on Mach-O artifacts (read-only header parse, no macOS tooling needed).
//
// Why: on Apple Silicon, arm64 Mach-O images must carry at least an ad-hoc
// signature to execute. The Go linker writes one automatically for
// darwin/arm64 binaries (blob = embedded CodeDirectory, no CMS, no TeamID),
// so our primary mac artifact is signed for free. It does NOT sign
// darwin/amd64 cross builds; those go through the codesign branch in CI
// (see docs/distribution.md).
//
// Usage:
//   node scripts/macho-sig-check.mjs --present <files...>
//       every file must be a 64-bit LE Mach-O WITH LC_CODE_SIGNATURE, else exit 1
//   node scripts/macho-sig-check.mjs --absent <files...>
//       control mode: every file must be judged WITHOUT a signature
//       (a plain non-Mach-O input, e.g. an ELF, correctly reads as absent —
//        this is the detector's negative control)
//
// Exit codes: 0 verdicts all match, 1 any mismatch, 2 usage error.
import { readFileSync } from 'node:fs';

const LC_CODE_SIGNATURE = 0x1d;
const MH_MAGIC_64_LE = 0xfeedfacf;

function inspect(file) {
  const b = readFileSync(file);
  if (b.length < 32) return { file, macho: false, note: 'too small' };
  if (b.readUInt32LE(0) !== MH_MAGIC_64_LE) return { file, macho: false, note: 'not 64-bit LE Mach-O' };
  const ncmds = b.readUInt32LE(16); // header size for 64-bit Mach-O is 32
  let off = 32;
  let sig = null;
  for (let i = 0; i < ncmds && off + 8 <= b.length; i++) {
    const cmd = b.readUInt32LE(off);
    const size = b.readUInt32LE(off + 4);
    if (size < 8) return { file, macho: true, error: `load command ${i} size ${size} < 8 (corrupt)` };
    if (cmd === LC_CODE_SIGNATURE) {
      sig = { dataoff: b.readUInt32LE(off + 8), datasize: b.readUInt32LE(off + 12) };
    }
    off += size;
  }
  if (!sig) return { file, macho: true, signed: false, note: 'LC_CODE_SIGNATURE=ABSENT' };
  if (sig.dataoff + 4 > b.length) return { file, macho: true, error: 'LC_CODE_SIGNATURE points past EOF' };
  const blobMagic = b.readUInt32BE(sig.dataoff);
  // 0xfade0cc0 = embedded signature superblob, 0xfade0c00 = bare CodeDirectory
  const known = blobMagic === 0xfade0cc0 || blobMagic === 0xfade0c00;
  const hasCMS = b.indexOf(Buffer.from([0xfa, 0xde, 0x71, 0x71]), sig.dataoff) >= 0; // Developer ID
  const kind = !known ? `unknown-blob-0x${blobMagic.toString(16)}` : hasCMS ? 'developer-id' : 'ad-hoc (no CMS)';
  return { file, macho: true, signed: true, note: `LC_CODE_SIGNATURE=PRESENT blob=0x${blobMagic.toString(16)} ${kind}` };
}

const argv = process.argv.slice(2);
const mode = argv[0];
const files = argv.slice(1);
if ((mode !== '--present' && mode !== '--absent') || files.length === 0) {
  console.error('usage: macho-sig-check.mjs --present|--absent <files...>');
  process.exit(2);
}
let bad = 0;
for (const f of files) {
  const r = inspect(f);
  const signed = r.signed === true;
  const ok = mode === '--present' ? signed : !signed;
  const line = `${f}: macho=${r.macho} ${r.error || r.note} => ${signed ? 'SIGNED' : 'UNSIGNED'}`;
  if (ok) console.log(`${mode === '--absent' ? 'CONTROL OK' : 'PASS'}: ${line}`);
  else { console.error(`${mode === '--absent' ? 'CONTROL' : 'CHECK'} FAIL: ${line}`); bad++; }
  if (r.error) bad++;
}
if (bad) { console.error(`macho-sig-check: ${bad} file(s) violated the expected verdict (${mode})`); process.exit(1); }
console.log(`macho-sig-check: ${files.length} file(s) match ${mode}`);
