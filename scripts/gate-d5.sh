#!/usr/bin/env bash
# gate-d5.sh — machine-assertion battery for the D5 release slice:
# build-dist v2 (cross matrix + archives + external SHA256SUMS), the exact
# artifact-name contract, the single version stamp proven four ways, the
# Mach-O ad-hoc signature gate (with positive AND negative controls on the
# same detector), archive content/checksum cross-verification, reproducibility
# of repackaging, zero residue of the retired CI naming, and every inherited
# D1-D4 guarantee (full regression at the tail).
#
# Every assertion runs against REAL staged build artifacts. Any red exits non-zero.
set -euo pipefail
cd "$(dirname "$0")/.."

export TMPDIR="$(pwd)/.tmptest"
mkdir -p "$TMPDIR" # sandbox kills binaries executed from /tmp; pin scratch inside the repo
find "$TMPDIR" -mindepth 1 -maxdepth 1 -name 'go-build*' -exec rm -rf {} + 2>/dev/null || true

step() { printf '\n=== %s ===\n' "$*"; }

VERSION="$(cat VERSION)"

grep -qE '^[0-9A-Za-z.-]+$' <<<"$VERSION" || { echo "RED: illegal VERSION file content: '$VERSION'"; exit 1; }

step '00 preflight'
go version
node --version
echo "version source of truth: VERSION=$VERSION"

step '01 fmt (tracked sources) / vet (3 GOOS + darwin/arm64) / unit tests, serial'
fmt=$(git ls-files '*.go' | xargs -r gofmt -l)
[ -z "$fmt" ] || { echo "RED: unformatted tracked sources:"; echo "$fmt"; exit 1; }
go vet ./...
GOOS=darwin GOARCH=amd64 go vet ./...
GOOS=darwin GOARCH=arm64 go vet ./...
GOOS=windows GOARCH=amd64 go vet ./...
go test -count=1 -p 1 -parallel 1 ./... > "$TMPDIR/gate-d5-test.out" 2>&1 || { tail -30 "$TMPDIR/gate-d5-test.out"; exit 1; }
t=$(grep -c '^ok' "$TMPDIR/gate-d5-test.out"); f=$(grep -c '^FAIL' "$TMPDIR/gate-d5-test.out" || true)
[ "$f" = 0 ] || { echo "RED: $f failing packages"; exit 1; }
echo "TESTS OK ($t packages ok, 0 fail, serial)"

step '02 build-dist v2: full matrix build + packaging'
bash scripts/build-dist.sh > "$TMPDIR/gate-d5-build.out" 2>&1 || { tail -30 "$TMPDIR/gate-d5-build.out"; echo RED: build-dist failed; exit 1; }
grep -q 'BUILD-DIST-OK' "$TMPDIR/gate-d5-build.out"
grep -q "VERSION=${VERSION}" "$TMPDIR/gate-d5-build.out" || { echo 'RED: build ignored the VERSION file'; exit 1; }
echo 'BUILD-DIST OK (10 binaries built from the single stamp, 10 archives packaged)'

step '03 artifact matrix: exact inventory + name contract'
expected_raw=()
for target in linux-amd64 linux-arm64 darwin-arm64 darwin-amd64 windows-amd64; do
  for bin in hello-collector agent-collector; do
    ext=''
    [ "${target%%-*}" = windows ] && ext='.exe'
    expected_raw+=("${bin}-${VERSION}-${target}${ext}")
  done
done
(cd dist && ls | grep -v '^release$' | sort) > "$TMPDIR/gate-d5-raw-list"
: > "$TMPDIR/gate-d5-expected"
for f in "${expected_raw[@]}"; do printf '%s\n' "$f" >> "$TMPDIR/gate-d5-expected"; done
printf 'SHA256SUMS.txt\n' >> "$TMPDIR/gate-d5-expected"
sort -o "$TMPDIR/gate-d5-expected" "$TMPDIR/gate-d5-expected"
diff "$TMPDIR/gate-d5-raw-list" "$TMPDIR/gate-d5-expected" || { echo 'RED: dist raw inventory drifted from the contract set'; exit 1; }
badname=$( (cd dist && ls | grep -v '^release$') | grep -vE "^(hello-collector|agent-collector)-${VERSION}-(linux|darwin|windows)-(amd64|arm64)(\.exe)?$|^SHA256SUMS\.txt$" || true)
[ -z "$badname" ] || { echo "RED: names violate the contract:"; echo "$badname"; exit 1; }
(cd dist/release && ls | grep -v '^SHA256SUMS.txt$' | sort) > "$TMPDIR/gate-d5-arc-list"
: > "$TMPDIR/gate-d5-arcexp"
for target in linux-amd64 linux-arm64 darwin-arm64 darwin-amd64; do
  for bin in hello-collector agent-collector; do printf '%s-%s-%s.tar.gz\n' "$bin" "$VERSION" "$target" >> "$TMPDIR/gate-d5-arcexp"; done
done
for bin in hello-collector agent-collector; do printf '%s-%s-windows-amd64.exe.zip\n' "$bin" "$VERSION" >> "$TMPDIR/gate-d5-arcexp"; done
sort -o "$TMPDIR/gate-d5-arcexp" "$TMPDIR/gate-d5-arcexp"
diff "$TMPDIR/gate-d5-arc-list" "$TMPDIR/gate-d5-arcexp" || { echo 'RED: release archive inventory drifted (name = contract binary + cap)'; exit 1; }
echo 'MATRIX OK (10 raw + 10 archives exactly per contract, no strays)'

step '04 version stamp identity (VERSION file == binary --version == file names == SUMS entries)'
hello="dist/hello-collector-${VERSION}-linux-amd64"
agent="dist/agent-collector-${VERSION}-linux-amd64"
"$hello" --version | grep -qF "hello-collector ${VERSION} linux/amd64" || { echo 'RED: hello --version stamp mismatch'; exit 1; }
"$agent" --version | grep -qF "agent-collector ${VERSION} linux/amd64" || { echo 'RED: agent --version stamp mismatch'; exit 1; }
nraw=$( { (cd dist && ls | grep -- "-${VERSION}-" || true); (cd dist/release && ls | grep -- "-${VERSION}-" || true); } | wc -l )
[ "$nraw" -eq 20 ] || { echo "RED: only $nraw/20 artifacts carry the stamp in their name"; exit 1; }
sums_ok=$(awk -v v="-${VERSION}-" '$0 ~ v {c++} END{print c+0}' dist/SHA256SUMS.txt)
[ "$sums_ok" -eq 10 ] || { echo "RED: raw SUMS entries with stamp: $sums_ok/10"; exit 1; }
sums_rel=$(awk -v v="-${VERSION}-" '$0 ~ v {c++} END{print c+0}' dist/release/SHA256SUMS.txt)
[ "$sums_rel" -eq 10 ] || { echo "RED: release SUMS entries with stamp: $sums_rel/10"; exit 1; }
# the workflow must NOT hardcode any version literal (single source discipline)
if grep -nE 'main\.version=[0-9]' .github/workflows/build.yml; then
  echo 'RED: workflow hardcodes a version literal (must read VERSION/tag)'; exit 1
fi
grep -q 'cat VERSION' .github/workflows/build.yml || { echo 'RED: workflow no longer reads the VERSION file'; exit 1; }
echo 'STAMP OK (all four observation points agree on one string)'

step '05 Mach-O signature gate: same detector, positive + negative controls'
node scripts/macho-sig-check.mjs --present "dist/hello-collector-${VERSION}-darwin-arm64" "dist/agent-collector-${VERSION}-darwin-arm64"
node scripts/macho-sig-check.mjs --absent "dist/hello-collector-${VERSION}-darwin-amd64" "dist/agent-collector-${VERSION}-darwin-amd64"
node scripts/macho-sig-check.mjs --absent "dist/hello-collector-${VERSION}-linux-amd64" "dist/agent-collector-${VERSION}-linux-arm64" LICENSE
echo 'SIG GATE OK (arm64 auto ad-hoc PRESENT; unsigned amd64 + ELF inputs judged not-signed by the same detector)'

step '06 archive contents + inner checksums cross-verified against raw bytes'
for f in dist/release/*.tar.gz; do
  members=$(tar -tzf "$f" | sort | tr '\n' ' ')
  b=$(basename "$f" .tar.gz)
  [ "$members" = "CHECKSUM.txt LICENSE NOTICE $b " ] || { echo "RED: $f members: $members"; exit 1; }
done
for f in dist/release/*.exe.zip; do
  b=$(basename "$f" .zip)   # inner binary keeps the .exe contract name
  members=$(python3 -c 'import sys,zipfile;print(" ".join(sorted(zipfile.ZipFile(sys.argv[1]).namelist())))' "$f")
  [ "$members" = "CHECKSUM.txt LICENSE NOTICE $b" ] || { echo "RED: $f members: $members"; exit 1; }
done
mkdir -p "$TMPDIR/unpack"
for f in dist/release/*.tar.gz dist/release/*.exe.zip; do
  rm -rf "$TMPDIR/unpack/x"; mkdir -p "$TMPDIR/unpack/x"
  case "$f" in
    *.tar.gz) b=$(basename "$f" .tar.gz); tar -xzf "$f" -C "$TMPDIR/unpack/x" ;;
    *.zip)    b=$(basename "$f" .zip);   python3 -c 'import sys,zipfile;zipfile.ZipFile(sys.argv[1]).extractall(sys.argv[2])' "$f" "$TMPDIR/unpack/x" ;;
  esac
  bin="$TMPDIR/unpack/x/$b"
  [ -f "$bin" ] || { echo "RED: no $b unpacked from $f"; exit 1; }
  ( cd "$(dirname "$bin")" && sha256sum -c CHECKSUM.txt > /dev/null ) || { echo "RED: inner CHECKSUM.txt mismatch in $f"; exit 1; }
  inner=$(sha256sum "$bin" | cut -d' ' -f1)
  outer=$(awk -v n="$b" '$2==n {print $1}' dist/SHA256SUMS.txt)
  [ "$inner" = "$outer" ] || { echo "RED: $f binary differs from dist/$b"; exit 1; }
done
echo 'ARCHIVES OK (every archive holds binary+LICENSE+NOTICE+CHECKSUM.txt; hashes chain to the raw set)'

step '07 external SHA256SUMS verifies the release set'
( cd dist/release && sha256sum -c SHA256SUMS.txt > /dev/null )
[ "$(wc -l < dist/release/SHA256SUMS.txt)" -eq 10 ] || { echo RED: release SUMS line count; exit 1; }
echo 'RELEASE SUMS OK'

step '08 artifact-form three-source check (headers + checksums + execution) incl. controls'
node scripts/verify-cross.mjs
node scripts/verify-cross.mjs --negative LICENSE
( cd dist && sha256sum -c SHA256SUMS.txt > /dev/null ) && echo 'raw SUMS verify OK'
"$hello" --version > /dev/null && "$agent" --version > /dev/null && echo 'linux artifacts execute OK'
echo 'FORM OK (struct + hash + run on the real matrix)'

step '09 reproducibility: --package-only rebuilds byte-identical archives'
cp dist/release/SHA256SUMS.txt "$TMPDIR/gate-d5-sums-before.txt"
bash scripts/build-dist.sh --package-only > "$TMPDIR/gate-d5-repack.out" 2>&1 || { tail -20 "$TMPDIR/gate-d5-repack.out"; exit 1; }
diff "$TMPDIR/gate-d5-sums-before.txt" dist/release/SHA256SUMS.txt || { echo RED: repack changed archive bytes; exit 1; }
echo 'REPACK OK (deterministic packaging: same sums without a rebuild)'

step '10 stdlib-only + zero-network closure: both binaries, linux arm64 included'
for combo in linux/amd64 linux/arm64 darwin/amd64 windows/amd64; do
  goos="${combo%/*}"; goarch="${combo#*/}"
  for bin in hello-collector agent-collector; do
    deps=$(GOOS=$goos GOARCH=$goarch go list -deps "./cmd/$bin")
    if printf '%s\n' "$deps" | grep -E '^(net|net/.+|crypto/tls)$'; then
      echo "RED: $bin ($goos/$goarch) depends on network packages"; exit 1
    fi
  done
done
grep -qE '^require' go.mod && { echo 'RED: go.mod gained external requires'; exit 1; }
echo 'STDLIB-ONLY OK (4 platform combos x 2 binaries, zero requires)'

step '11 name-contract: retired CI naming has zero residue'
if git ls-files -z | xargs -0 grep -nE 'hello-collector-(linux|darwin|windows)\b|dist/hello-collector[".$ ]|dist/agent-collector[".$ ]' 2>/dev/null; then
  echo 'RED: retired bare-name artifact form still referenced'; exit 1
fi
git ls-files .github/workflows/build.yml | xargs grep -q 'collector-${VERSION}-'
echo 'NAMING OK (old hello-collector-<os> / bare dist paths fully retired in tracked files)'

step '12 doc + tripwire + license gates'
node scripts/licenses-check.mjs --selftest
node scripts/licenses-check.mjs
bash scripts/tripwire.sh --selftest
bash scripts/tripwire.sh
test -f docs/distribution.md || { echo 'RED: distribution docs missing'; exit 1; }
echo 'DOCS+TRIPWIRE OK'

step '13 frozen contract still holds'
node scripts/apicontract-check.mjs
node scripts/apicontract-check.mjs --negative
echo 'CONTRACT OK'

step '14 full inherited regression: gate-d1 + d2 + d3 + d4 serial'
bash scripts/gate-d1.sh > "$TMPDIR/gate-d5-reg-d1.out" 2>&1 || { tail -25 "$TMPDIR/gate-d5-reg-d1.out"; echo RED: gate-d1 regression; exit 1; }
bash scripts/gate-d2.sh > "$TMPDIR/gate-d5-reg-d2.out" 2>&1 || { tail -25 "$TMPDIR/gate-d5-reg-d2.out"; echo RED: gate-d2 regression; exit 1; }
bash scripts/gate-d3.sh > "$TMPDIR/gate-d5-reg-d3.out" 2>&1 || { tail -25 "$TMPDIR/gate-d5-reg-d3.out"; echo RED: gate-d3 regression; exit 1; }
bash scripts/gate-d4.sh > "$TMPDIR/gate-d5-reg-d4.out" 2>&1 || { tail -25 "$TMPDIR/gate-d5-reg-d4.out"; echo RED: gate-d4 regression; exit 1; }
grep -q 'GATE-D1: ALL GREEN' "$TMPDIR/gate-d5-reg-d1.out"
grep -q 'GATE-D2: ALL GREEN' "$TMPDIR/gate-d5-reg-d2.out"
grep -q 'GATE-D3: ALL GREEN' "$TMPDIR/gate-d5-reg-d3.out"
grep -q 'GATE-D4: ALL GREEN' "$TMPDIR/gate-d5-reg-d4.out"
echo 'D1+D2+D3+D4 REGRESSION GREEN (serial, full batteries)'

printf '\nGATE-D5: ALL GREEN\n'
