#!/usr/bin/env node
// releaseleg-check.mjs — machine gate for the release-slice close-out.
//
// Cross-asserts the release surface across the sources of truth:
//   1. VERSION file                 — the single stamp source
//   2. docs/distribution.md         — the authoritative contract prose
//   3. .github/workflows/build.yml  — CI build + darwin signing branches
//   4. scripts/build-dist.sh        — the release builder incl. its Intel
//      release-assembly branches (A: signed bytes gate, B: honest skip)
//   5. live release assets (JSON)   — GitHub release API response, optional
//   6. website download page (HTML) — snapshot file, optional
// Any drift between surfaces is RED. --selftest injects named corruptions,
// must catch every one, and must keep the pristine tree and the two honest
// live shapes (fully signed / honest Intel skip) GREEN.
//
// Usage:
//   node scripts/releaseleg-check.mjs [--root DIR]
//       [--assets release-assets.json] [--site download-page.html]
//   node scripts/releaseleg-check.mjs --selftest
// Exit: 0 GREEN, 1 RED. Prints rule verdicts only, never file bodies.

import fs from 'node:fs';
import path from 'node:path';
import { fileURLToPath } from 'node:url';

const BINS = ['hello-collector', 'agent-collector'];
const CORE_TARGETS = ['linux/amd64', 'linux/arm64', 'darwin/arm64', 'windows/amd64'];
const INTEL = 'darwin/amd64';
const CONTRACT_RE = /^(hello-collector|agent-collector)-([0-9A-Za-z.-]+)-(linux|darwin|windows)-(amd64|arm64)((\.tar\.gz)|(\.exe\.zip)|(\.exe))?$/;

function red(rules, msg) { rules.push(['RED', msg]); }
function parseVersionFile(text) {
  const v = text.trim();
  if (!v || !/^[0-9A-Za-z.-]+$/.test(v)) return { error: `illegal VERSION content '${v}'` };
  return { version: v };
}

function parseDocs(text) {
  const targets = new Map();
  for (const m of text.matchAll(/\|\s*(linux|darwin|windows)\/(amd64|arm64)[^|]*\|\s*([^|]*?)\s*\|/g)) {
    targets.set(`${m[1]}/${m[2]}`, m[3]);
  }
  return {
    targets,
    stampSingleSource: /`VERSION` file is the single source/.test(text),
    contractBlock: /<name>-<version>-<os>-<arch>\[?\.exe\]?/.test(text),
    tarGzRule: /<name>-<version>-<os>-<arch>\.tar\.gz/.test(text),
    winZipRule: /<name>-<version>-windows-amd64\.exe\.zip/.test(text),
    intelHonestNote: /pending verification/.test(text),
    signingBranchNote: /signing branch/.test(text),
    checkerNamed: /scripts\/releaseleg-check\.mjs/.test(text),
    assemblyNamed: (text.match(/--release-branch=[AB]/g) || []).length >= 2,
  };
}

function parseWorkflow(text) {
  const crossTargets = new Set();
  for (const m of text.matchAll(/goos:\s*(linux|darwin|windows)\s*\n\s*goarch:\s*(amd64|arm64)/g)) {
    crossTargets.add(`${m[1]}/${m[2]}`);
  }
  const ldLines = [...text.matchAll(/-X main\.version=(\S+)/g)].map((m) => m[1]);
  return {
    crossTargets,
    signArm: /GOOS=darwin GOARCH=arm64/.test(text),
    signAmd: /GOOS=darwin GOARCH=amd64/.test(text),
    tagAssert: /test "v\$\(cat VERSION\)" = "\$\{GITHUB_REF_NAME\}"/.test(text),
    ldUsesVar: ldLines.length > 0 && ldLines.every((v) => /\$\{?VERSION\}?/.test(v)),
    contractNamed: (text.match(/-\$\{?VERSION\}?-\$\{\{\s*matrix\.goos\s*\}\}-\$\{\{\s*matrix\.goarch\s*\}\}/g) || []).length >= 2,
    signingNamed: /dist\/\$\{bin\}-\$\{VERSION\}-darwin-arm64/.test(text) && /dist\/\$\{bin\}-\$\{VERSION\}-darwin-amd64/.test(text),
    branchB: /BRANCH=B\(skip-amd64\)/.test(text),
    branchA: /name:\s*darwin-amd64-signed/.test(text),
  };
}

function parseBuilder(text) {
  const tm = text.match(/^TARGETS='([^']+)'/m);
  return {
    targets: tm ? new Set(tm[1].split(/\s+/).filter(Boolean)) : new Set(),
    versionFromVar: /VERSION="\$\{1:-\$\(cat VERSION\)\}"/.test(text) && /VERSION="\$\(cat VERSION\)"/.test(text),
    shapeGuard: /case "\$VERSION" in/.test(text),
    tarRule: /arcname="\$\{bin\}-\$\{VERSION\}-\$\{os\}-\$\{arch\}\.tar\.gz"/.test(text),
    zipRule: /arcname="\$\{bin\}-\$\{VERSION\}-\$\{os\}-\$\{arch\}\.exe\.zip"/.test(text),
    ld: /-X main\.version=\$\{VERSION\}/.test(text),
    sums: /sha256sum \*\.tar\.gz \*\.exe\.zip > SHA256SUMS\.txt/.test(text),
    signedDirGate: /branch A needs --signed-dir/.test(text),
    allOrNone: /Intel pair is all-or-none/.test(text),
    sigVerify: /macho-sig-check\.mjs --present "\$sb"/.test(text),
    bSkipNote: /BRANCH-B-INTEL-SKIP:/.test(text),
    bSkipArchive: /\[ "\$RELEASE" = B \] && \[ "\$GOOS\/\$GOARCH" = "darwin\/amd64" \]/.test(text),
  };
}

function checkSurfaces({ version, docs, wf, builder }) {
  const rules = [];
  if (!version) { red(rules, 'A0 illegal or missing VERSION'); return rules; }
  const eq = (a, b) => a.size === b.size && [...a].every((x) => b.has(x));
  const docTargets = new Set([...docs.targets.keys()]);
  // A1 builder target set must equal the documented matrix exactly
  if (!eq(docTargets, builder.targets)) red(rules, `A1 platform set drift: docs{${[...docTargets].sort()}} vs builder{${[...builder.targets].sort()}}`);
  // A2 CI native-test matrix must be a subset of the documented matrix
  const outside = [...wf.crossTargets].filter((t) => !docTargets.has(t));
  if (outside.length) red(rules, `A2 CI builds target absent from the doc matrix: ${outside}`);
  // A3 every documented row ships; Intel keeps its honest notes
  const notShipped = [...docs.targets.entries()].filter(([, s]) => !/shipped/.test(s));
  if (notShipped.length) red(rules, `A3 doc matrix rows not shipped: ${notShipped.map(([t, s]) => `${t}=${s}`).join(', ')}`);
  if (!docs.intelHonestNote) red(rules, 'A3 Intel real-hardware honesty note missing');
  if (!docs.signingBranchNote) red(rules, 'A3 darwin/amd64 signing-branch note missing');
  // A4 single stamp consumed everywhere, never a literal
  if (!docs.stampSingleSource) red(rules, 'A4 docs no longer name the single stamp source');
  if (!wf.tagAssert) red(rules, 'A4 tag-vs-VERSION equality assert missing in workflow');
  if (!wf.ldUsesVar) red(rules, 'A4 workflow ldflags must use the stamp variable, never a literal');
  if (!builder.versionFromVar) red(rules, 'A4 builder lost the VERSION-file source');
  if (!builder.shapeGuard) red(rules, 'A4 builder illegal-stamp shape guard missing');
  if (!builder.ld) red(rules, 'A4 builder ldflags not consuming the stamp variable');
  // A5 artifact-name contract tokens agree across surfaces
  if (!docs.contractBlock || !docs.tarGzRule || !docs.winZipRule) red(rules, 'A5 name-contract block incomplete in docs');
  if (!builder.tarRule || !builder.zipRule) red(rules, 'A5 builder archive naming diverged from the contract');
  if (!wf.contractNamed || !wf.signingNamed) red(rules, 'A5 workflow artifact naming diverged from the contract');
  if (!builder.sums) red(rules, 'A5 external SHA256SUMS assembly missing');
  if (!docs.checkerNamed) red(rules, 'A5 docs do not name this cross-surface checker');
  // A6 signing two-branch plan intact
  if (!wf.signArm || !wf.signAmd) red(rules, 'A6 signing job no longer builds both darwin arches');
  if (!wf.branchA || !wf.branchB) red(rules, 'A6 signing branch A/B shapes missing (skip path unauditable)');
  // A9 Intel release assembly is a real builder path, not prose: branch A
  // gates on the signature detector, branch B omits the archives honestly,
  // and the docs name both commands.
  if (!builder.signedDirGate || !builder.allOrNone || !builder.sigVerify) red(rules, 'A9 builder branch A shapes missing (signed-dir requirement / all-or-none pair / signature verification before packaging)');
  if (!builder.bSkipArchive || !builder.bSkipNote) red(rules, 'A9 builder branch B shapes missing (Intel archives omitted / honest skip note)');
  if (!docs.assemblyNamed) red(rules, 'A9 docs no longer name the release-assembly branch commands');
  if (!rules.some((r) => r[0] === 'RED')) rules.push(['OK', `A1-A6+A9 GREEN: release contract synced across VERSION/docs/workflow/builder (${version})`]);
  return rules;
}

function expectedCoreAssets(version) {
  const set = new Set(['SHA256SUMS.txt']);
  for (const bin of BINS) {
    for (const t of CORE_TARGETS) {
      const [os, arch] = t.split('/');
      set.add(`${bin}-${version}-${os}-${arch}${os === 'windows' ? '.exe.zip' : '.tar.gz'}`);
    }
  }
  return set;
}

function checkLive({ assetsJson, siteHtml, version }) {
  const rules = [];
  const names = assetsJson.map((a) => a.name);
  const got = new Set(names);
  const intel = names.filter((n) => n.endsWith('-darwin-amd64.tar.gz'));
  if (intel.length === 1) { red(rules, 'A7 Intel half-shipped: exactly one darwin/amd64 asset (signing must be all-or-none)'); return rules; }
  for (const n of names) if (n !== 'SHA256SUMS.txt' && !CONTRACT_RE.test(n)) red(rules, `A7 live asset violates the name contract: ${n}`);
  const exp = expectedCoreAssets(version);
  const extra = [...got].filter((n) => !exp.has(n) && !intel.includes(n));
  const missing = [...exp].filter((n) => !got.has(n));
  if (!rules.some((r) => r[0] === 'RED')) {
    if (missing.length || extra.length) red(rules, `A7 live asset set drift: missing=[${missing}] extra=[${extra}]`);
    else if (intel.length === 0) rules.push(['WARN', 'A7 honest skip shape: Intel pair omitted this release (signing branch B)']);
    else rules.push(['OK', 'A7 live assets == contract set incl. signed Intel pair']);
  }
  if (siteHtml && !rules.some((r) => r[0] === 'RED')) {
    const html = fs.readFileSync(siteHtml, 'utf8');
    // Only links count as the download surface; prose mentions of the inner
    // binary name ("unzip and run X.exe") are contract-correct and not assets.
    const found = new Set();
    for (const m of html.matchAll(/(?:href|src)\s*=\s*["']([^"']+)["']/g)) {
      const base = m[1].split('/').pop().split('?')[0];
      if (CONTRACT_RE.test(base)) found.add(base);
    }
    const siteMissing = [...got].filter((n) => n !== 'SHA256SUMS.txt' && !found.has(n));
    const siteStale = [...found].filter((n) => !got.has(n));
    if (siteMissing.length || siteStale.length) red(rules, `A8 download page drift: missing=[${siteMissing}] stale=[${siteStale}]`);
    else rules.push(['OK', `A8 download page links == live asset set (${found.size} names)`]);
  }
  if (!rules.some((r) => r[0] === 'RED') && !rules.some((r) => r[0] === 'WARN')) rules.push(['OK', 'A8 live surfaces synced']);
  return rules;
}

// ---------- selftest ----------

function surfaces(f) {
  const v = parseVersionFile(f.version);
  if (v.error) return { version: null };
  return { version: v.version, docs: parseDocs(f.docs), wf: parseWorkflow(f.wf), builder: parseBuilder(f.builder) };
}

function selftest(repoRoot) {
  const real = {
    version: fs.readFileSync(path.join(repoRoot, 'VERSION'), 'utf8'),
    docs: fs.readFileSync(path.join(repoRoot, 'docs/distribution.md'), 'utf8'),
    wf: fs.readFileSync(path.join(repoRoot, '.github/workflows/build.yml'), 'utf8'),
    builder: fs.readFileSync(path.join(repoRoot, 'scripts/build-dist.sh'), 'utf8'),
  };
  const baseRules = checkSurfaces(surfaces(real));
  if (baseRules.some((r) => r[0] === 'RED')) { console.log('MISS C00 pristine must be GREEN:', baseRules.filter((r) => r[0] === 'RED')); process.exit(1); }
  console.log('pass  C00 pristine surfaces GREEN');
  let bad = 0;
  const mut = (text, from, to, label) => { if (!text.includes(from)) { console.log(`ANCHOR MISSING ${label}`); process.exit(1); } return text.split(from).join(to); };
  const expectRed = (label, rules) => {
    if (!rules.some((r) => r[0] === 'RED')) { console.log(`MISS ${label}: no RED produced`); bad++; } else console.log(`pass  ${label}`);
  };
  const sc = (label, patch) => expectRed(label, checkSurfaces(surfaces({ ...real, ...patch })));

  sc('C01 illegal VERSION', { version: '0.5.0 beta!' });
  sc('C02 docs drop a matrix row', { docs: mut(real.docs, '| linux/arm64 | shipped |', '| (removed) |', 'c02') });
  sc('C03 docs status flip vs builder', { docs: mut(real.docs, '| linux/arm64 | shipped', '| linux/arm64 | planned', 'c03') });
  sc('C04 Intel honesty note removed', { docs: mut(real.docs, 'pending verification', 'fully verified', 'c04') });
  sc('C05 workflow literal stamp', { wf: mut(real.wf, '-X main.version=${VERSION}', '-X main.version=0.9.9-lit', 'c05') });
  sc('C06 builder target added', { builder: mut(real.builder, "windows/amd64'", "windows/amd64 windows/arm64'", 'c06') });
  sc('C07 builder naming diverged', { builder: mut(real.builder, '${arch}.tar.gz', '${arch}.tgz', 'c07') });
  sc('C08 builder VERSION-file source lost', { builder: mut(real.builder, '${1:-$(cat VERSION)}', '${1:-9.9.9}', 'c08') });
  sc('C09 docs contract block broken', { docs: mut(real.docs, '<name>-<version>-<os>-<arch>[.exe]', '<name>-<version>-<os>', 'c09') });
  sc('C10 signing branch B shape gone', { wf: mut(real.wf, 'BRANCH=B(skip-amd64)', 'BRANCH=X', 'c10') });
  sc('C11 CI builds undocumented target', { wf: mut(real.wf, 'goos: windows\n            goarch: amd64', 'goos: windows\n            goarch: arm64', 'c11') });
  sc('C11b workflow tag assert removed', { wf: mut(real.wf, 'test "v$(cat VERSION)" = "${GITHUB_REF_NAME}"', ':', 'c11b') });
  sc('C16 builder drops the signature gate on branch A bytes', { builder: mut(real.builder, 'macho-sig-check.mjs --present "$sb"', 'test -f "$sb"', 'c16') });
  sc('C17 builder branch B note disappears', { builder: mut(real.builder, 'BRANCH-B-INTEL-SKIP:', 'BRANCH-C-NOTE:', 'c17') });
  sc('C18 builder silently packages Intel on branch B (no archive skip)', { builder: mut(real.builder, '[ "$RELEASE" = B ] && [ "$GOOS/$GOARCH" = "darwin/amd64" ]', '[ -n "" ]', 'c18') });
  sc('C19 builder branch A accepts a missing signed dir', { builder: mut(real.builder, 'branch A needs --signed-dir', 'branch A optionally takes a dir', 'c19') });
  sc('C20 docs stop naming the release-assembly commands', { docs: mut(real.docs, '--release-branch=', '--release-lane=', 'c20') });

  const V = '9.9.9';
  const fullSet = [...expectedCoreAssets(V), `${BINS[0]}-${V}-darwin-amd64.tar.gz`, `${BINS[1]}-${V}-darwin-amd64.tar.gz`].map((name) => ({ name }));
  expectRed('C12 live half-shipped Intel', checkLive({ assetsJson: fullSet.filter((a) => a.name !== `${BINS[0]}-${V}-darwin-amd64.tar.gz`), version: V }));
  expectRed('C13 live missing core asset', checkLive({ assetsJson: fullSet.filter((a) => a.name !== 'SHA256SUMS.txt'), version: V }));
  expectRed('C14 live bad name', checkLive({ assetsJson: [{ name: 'agent-collector-linux.tar.gz' }, ...fullSet], version: V }));
  const staleFile = path.join(repoRoot, `.selftest-site-${process.pid}.html`);
  fs.writeFileSync(staleFile, fullSet.map((a) => `<a href="/x/${a.name}">d</a>`).join('') + `<a href="${BINS[0]}-${V}-windows-arm64.exe.zip">stale</a>`);
  expectRed('C15 site stale link', checkLive({ assetsJson: fullSet, siteHtml: staleFile, version: V }));
  fs.unlinkSync(staleFile);

  const posA = checkLive({ assetsJson: fullSet, version: V });
  if (posA.some((r) => r[0] === 'RED')) { console.log('MISS P01 full signed set must be GREEN'); bad++; } else console.log('pass  P01 complete asset set GREEN');
  const posB = checkLive({ assetsJson: fullSet.filter((a) => !/-darwin-amd64\.tar\.gz$/.test(a.name)), version: V });
  if (posB.some((r) => r[0] === 'RED') || !posB.some((r) => r[0] === 'WARN')) { console.log('MISS P02 honest skip must be GREEN with WARN'); bad++; } else console.log('pass  P02 honest Intel-skip shape GREEN + WARN');
  if (bad) { console.log(`SELFTEST FAIL (${bad} controls)`); process.exit(1); }
  console.log('SELFTEST OK (1 control pristine + 21 injected catches + 2 positive shapes)');
}

// ---------- main ----------

const args = process.argv.slice(2);
const getArg = (flag) => { const i = args.indexOf(flag); return i >= 0 ? args[i + 1] : null; };
const root = getArg('--root') || path.resolve(path.dirname(fileURLToPath(import.meta.url)), '..');
if (args.includes('--selftest')) { selftest(root); process.exit(0); }

const f = {
  version: fs.readFileSync(path.join(root, 'VERSION'), 'utf8'),
  docs: fs.readFileSync(path.join(root, 'docs/distribution.md'), 'utf8'),
  wf: fs.readFileSync(path.join(root, '.github/workflows/build.yml'), 'utf8'),
  builder: fs.readFileSync(path.join(root, 'scripts/build-dist.sh'), 'utf8'),
};
const s = surfaces(f);
const rules = checkSurfaces(s);
if (s.version && getArg('--assets')) {
  const j = JSON.parse(fs.readFileSync(getArg('--assets'), 'utf8'));
  rules.push(...checkLive({ assetsJson: Array.isArray(j) ? j : j.assets, siteHtml: getArg('--site'), version: s.version }));
} else if (getArg('--site')) {
  red(rules, '--site given without --assets');
}
for (const [k, m] of rules) console.log(`${k.padEnd(4)} ${m}`);
process.exit(rules.some((r) => r[0] === 'RED') ? 1 : 0);
