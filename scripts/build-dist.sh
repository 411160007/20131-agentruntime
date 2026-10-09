#!/usr/bin/env bash
# build-dist.sh v2 — cross-compile the release matrix and package every
# binary into a per-platform archive under dist/release/, plus one external
# SHA256SUMS.txt covering the release asset set.
#
# Version stamp has ONE source: the repo-root VERSION file. The release tag
# (v<ver>), CI workflow env, the ldflags below, artifact file names, and the
# checksums file entries all derive from that same string. An explicit first
# argument overrides it for local experiments only.
#
# Matrix (D5 release slice):
#   linux   amd64 arm64        -> <name>-<ver>-linux-<arch>.tar.gz
#   darwin  arm64(main) amd64  -> <name>-<ver>-darwin-<arch>.tar.gz
#   windows amd64              -> <name>-<ver>-windows-amd64.exe.zip
# Archives embed: the binary, LICENSE, NOTICE, CHECKSUM.txt (binary sha256).
# darwin/amd64 ships ONLY through the signed path described in
# docs/distribution.md; this builder always produces the raw cross-compiled
# binary, and the release assembly decides which darwin-amd64 bytes go out
# through an explicit branch choice (see --release-branch below). The
# builder never packages an unsigned Intel binary into dist/release/.
#
# Usage:
#   bash scripts/build-dist.sh [version]            # full build + package (dev default: full matrix)
#   bash scripts/build-dist.sh --package-only       # repackage binaries in dist/
#   bash scripts/build-dist.sh --release-branch=A --signed-dir DIR   # Intel pair from signed bytes
#   bash scripts/build-dist.sh --package-only --release-branch=B      # omit Intel archives, honest skip note
set -euo pipefail
cd "$(dirname "$0")/.."

# Release assembly branch: A = package the CI-signed darwin/amd64 bytes
# (each must pass scripts/macho-sig-check.mjs --present; the pair is
# all-or-none), B = omit the Intel archives from dist/release/ and write an
# honest BRANCH-B-INTEL-SKIP note outside the release dir. Empty = dev
# default (package the full cross matrix as before, for local builds/gates).
RELEASE=''
SIGNED_DIR=''
ARGS=()
for a in "$@"; do
  case "$a" in
    --release-branch=A) RELEASE=A ;;
    --release-branch=B) RELEASE=B ;;
    --release-branch=*) echo "RED: illegal --release-branch value '$a' (expect A(signed)|B(skip-amd64))" >&2; exit 1 ;;
    --signed-dir=*) SIGNED_DIR="${a#--signed-dir=}" ;;
    *) ARGS+=("$a") ;;
  esac
done
if [ "${#ARGS[@]}" -gt 0 ]; then set -- "${ARGS[@]}"; else set --; fi
if [ -n "$SIGNED_DIR" ] && [ "$RELEASE" != A ]; then
  echo 'RED: --signed-dir is only valid with --release-branch=A' >&2; exit 1
fi

if [ "${1:-}" = "--package-only" ]; then
  MODE=package
  VERSION="$(cat VERSION)"
else
  MODE=build
  VERSION="${1:-$(cat VERSION)}"
fi
case "$VERSION" in
  ''|*[!0-9A-Za-z.-]*) echo "RED: illegal version stamp '$VERSION'" >&2; exit 1 ;;
esac

# dist/ mirrors exactly one release set: drop stale trees from earlier
# versions so version-globbing gates cannot pick up an outdated binary.
if [ "$MODE" = build ]; then
  rm -rf dist
  mkdir -p dist dist/release
else
  [ -d dist ] || { echo 'RED: --package-only without a dist/ tree'; exit 1; }
  rm -rf dist/release
  mkdir -p dist/release
fi
rm -rf dist/.stage

TARGETS='linux/amd64 linux/arm64 darwin/arm64 darwin/amd64 windows/amd64'

if [ "$MODE" = build ]; then
  LDFLAGS="-s -w -X main.version=${VERSION}"
  for target in $TARGETS; do
    GOOS="${target%/*}"
    GOARCH="${target#*/}"
    ext=''
    if [ "$GOOS" = windows ]; then ext='.exe'; fi
    for bin in hello-collector agent-collector; do
      out="dist/${bin}-${VERSION}-${GOOS}-${GOARCH}${ext}"
      echo "building ${out}"
      CGO_ENABLED=0 GOOS="$GOOS" GOARCH="$GOARCH" go build -trimpath -ldflags "$LDFLAGS" -o "$out" "./cmd/${bin}"
    done
  done
fi

package_one() { # $1=bin $2=os $3=arch
  local bin="$1" os="$2" arch="$3"
  local ext='' rawbase arcname
  if [ "$os" = windows ]; then ext='.exe'; fi
  rawbase="${bin}-${VERSION}-${os}-${arch}${ext}"
  if [ "$os" = windows ]; then
    arcname="${bin}-${VERSION}-${os}-${arch}.exe.zip"
  else
    arcname="${bin}-${VERSION}-${os}-${arch}.tar.gz"
  fi
  if [ ! -f "dist/${rawbase}" ]; then
    echo "RED: missing raw artifact dist/${rawbase} for packaging" >&2; exit 1
  fi
  local s="dist/.stage/${bin}-${os}-${arch}"
  mkdir -p "$s"
  cp "dist/${rawbase}" "${s}/${rawbase}"
  chmod 0755 "${s}/${rawbase}"
  cp LICENSE NOTICE "$s/"
  ( cd "$s" && sha256sum "${rawbase}" > CHECKSUM.txt )
  if [ "$os" = windows ]; then
    python3 - "$s" "dist/release/${arcname}" "$rawbase" <<'PY'
import sys, zipfile, os
stage, arc, raw = sys.argv[1], sys.argv[2], sys.argv[3]
with zipfile.ZipFile(arc, 'w', zipfile.ZIP_DEFLATED) as z:
    for name in (raw, 'LICENSE', 'NOTICE', 'CHECKSUM.txt'):
        zi = zipfile.ZipInfo(name, date_time=(2020, 1, 1, 0, 0, 0))
        zi.external_attr = (0o755 if name == raw else 0o644) << 16
        with open(os.path.join(stage, name), 'rb') as f:
            z.writestr(zi, f.read(), zipfile.ZIP_DEFLATED)
PY
  else
    tar -C "$s" --format=ustar --owner=0 --group=0 --numeric-owner \
        --mtime='UTC 2020-01-01' -cf - \
        "$rawbase" LICENSE NOTICE CHECKSUM.txt | gzip -n > "dist/release/${arcname}"
  fi
  echo "packaged dist/release/${arcname}"
}

if [ "$RELEASE" = A ]; then
  [ -n "$SIGNED_DIR" ] || { echo 'RED: branch A needs --signed-dir with the CI darwin-amd64-signed bytes'; exit 1; }
  for bin in hello-collector agent-collector; do
    sb="$SIGNED_DIR/${bin}-${VERSION}-darwin-amd64"
    [ -f "$sb" ] || { echo "RED: branch A missing signed binary $sb (Intel pair is all-or-none)" >&2; exit 1; }
    node scripts/macho-sig-check.mjs --present "$sb" > /dev/null \
      || { echo "RED: $sb judged unsigned by macho-sig-check.mjs --present (branch A ships signed bytes only)" >&2; exit 1; }
  done
  for bin in hello-collector agent-collector; do
    cp "$SIGNED_DIR/${bin}-${VERSION}-darwin-amd64" "dist/${bin}-${VERSION}-darwin-amd64"
    echo "assembled signed bytes: dist/${bin}-${VERSION}-darwin-amd64"
  done
fi

for target in $TARGETS; do
  GOOS="${target%/*}"
  GOARCH="${target#*/}"
  if [ "$RELEASE" = B ] && [ "$GOOS/$GOARCH" = "darwin/amd64" ]; then continue; fi
  for bin in hello-collector agent-collector; do
    package_one "$bin" "$GOOS" "$GOARCH"
  done
done

( cd dist && sha256sum hello-collector-* agent-collector-* > SHA256SUMS.txt )
( cd dist/release && sha256sum *.tar.gz *.exe.zip > SHA256SUMS.txt )
rm -rf dist/.stage

if [ "$RELEASE" = B ]; then
  printf '%s\n' \
    'BRANCH-B-INTEL-SKIP: darwin/amd64 (Intel) is not part of this release set.' \
    'The CI signing branch did not produce verified signed bytes; the builder' \
    'never packages unsigned Intel binaries. See docs/distribution.md.' \
    'This note lives OUTSIDE dist/release/ and is never a release asset.' \
    > dist/INTEL-SKIP-NOTE.txt
  echo 'BRANCH=B(skip-amd64): Intel archives omitted, honest note at dist/INTEL-SKIP-NOTE.txt'
fi
if [ "$RELEASE" = A ]; then
  echo 'BRANCH=A(intel-signed): Intel pair assembled from verified signed bytes'
fi
if [ "$RELEASE" != B ]; then rm -f dist/INTEL-SKIP-NOTE.txt; fi

ls dist/release/
cat dist/release/SHA256SUMS.txt
echo "VERSION=${VERSION}"
echo "BUILD-DIST-OK"
