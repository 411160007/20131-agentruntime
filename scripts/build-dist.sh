#!/usr/bin/env bash
# build-dist.sh — cross-compile release artifacts for all supported
# platforms into dist/. Deterministic flags: CGO off, buildid stripped
# for reproducibility. Usage: bash scripts/build-dist.sh [version]
set -euo pipefail
cd "$(dirname "$0")/.."

VERSION="${1:-0.1.0-d1}"
LDFLAGS="-s -w -X main.version=${VERSION}"
mkdir -p dist

for target in linux/amd64 windows/amd64 darwin/amd64; do
  GOOS="${target%/*}"
  GOARCH="${target#*/}"
  ext=""
  [ "$GOOS" = "windows" ] && ext=".exe"
  out="dist/hello-collector-${VERSION}-${GOOS}-${GOARCH}${ext}"
  echo "building ${out}"
  CGO_ENABLED=0 GOOS="$GOOS" GOARCH="$GOARCH" go build -trimpath -ldflags "$LDFLAGS" -o "$out" ./cmd/hello-collector
done

( cd dist && sha256sum hello-collector-* > SHA256SUMS.txt )
ls -la dist/
cat dist/SHA256SUMS.txt
echo "BUILD-DIST-OK"
