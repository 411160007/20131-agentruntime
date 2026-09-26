#!/usr/bin/env bash
# build-dist.sh — cross-compile release artifacts for all supported
# platforms into dist/. Deterministic flags: CGO off, buildid stripped
# for reproducibility. Usage: bash scripts/build-dist.sh [version]
set -euo pipefail
cd "$(dirname "$0")/.."

VERSION="${1:-0.2.0-d2}"
LDFLAGS="-s -w -X main.version=${VERSION}"
mkdir -p dist
# dist mirrors exactly one release set: drop stale artifacts from earlier
# versions so version-globbing gates cannot pick up an outdated binary.
rm -f dist/hello-collector-* dist/agent-collector-* dist/SHA256SUMS.txt

for target in linux/amd64 windows/amd64 darwin/amd64; do
  GOOS="${target%/*}"
  GOARCH="${target#*/}"
  ext=""
  [ "$GOOS" = "windows" ] && ext=".exe"
  for bin in hello-collector agent-collector; do
    out="dist/${bin}-${VERSION}-${GOOS}-${GOARCH}${ext}"
    echo "building ${out}"
    CGO_ENABLED=0 GOOS="$GOOS" GOARCH="$GOARCH" go build -trimpath -ldflags "$LDFLAGS" -o "$out" "./cmd/${bin}"
  done
done

( cd dist && sha256sum hello-collector-* agent-collector-* > SHA256SUMS.txt )
ls -la dist/
cat dist/SHA256SUMS.txt
echo "BUILD-DIST-OK"
