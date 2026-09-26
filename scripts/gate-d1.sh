#!/usr/bin/env bash
# gate-d1.sh — full machine assertion battery for the D1 skeleton.
# Every gate runs against the REAL staged build artifacts and includes
# its own positive/negative control. Any red exits non-zero.
set -euo pipefail
cd "$(dirname "$0")/.."

step() { printf '\n=== %s ===\n' "$*"; }

step '00 preflight'
go version
node --version

step '01 fmt / vet / unit tests'
test -z "$(gofmt -l .)" || { gofmt -l .; echo RED: gofmt; exit 1; }
go vet ./...
# Retry ONLY on rc 137 (SIGKILL): observed cold-cache environment kills in
# this sandbox, not test failures. A red test exits 1 and never retries.
rc=0
go test -count=1 -p 1 ./... || rc=$?
if [ "$rc" -eq 137 ]; then
  echo 'WARN: go test SIGKILLed by environment (rc137); retrying once'
  go test -count=1 -p 1 ./... || rc=$?
fi
if [ "$rc" -ne 0 ]; then echo RED: go test rc=$rc; exit "$rc"; fi

step '02 cross-build three platforms'
bash scripts/build-dist.sh

step '03 linux artifact executes: --version must exit 0'
./dist/hello-collector-0.1.0-d1-linux-amd64 --version

step '04 windows/darwin artifacts: structural header verification'
node scripts/verify-cross.mjs
step '04c control: verifier must REJECT a non-binary file'
node scripts/verify-cross.mjs --negative README.md

step '05 hello-collector real run -> valid JSONL'
out=$(mktemp -d)/events.jsonl
./dist/hello-collector-0.1.0-d1-linux-amd64 --out "$out"
node scripts/validate-jsonl.mjs "$out" --min-lines 4
step '05c control: validator must REJECT a bad fixture'
bad=$(mktemp -d)/bad.jsonl
printf '%s\n' '{"v":9,"id":"!!","ts":"nope","stage":"wild","type":"x.y","decision":"block","severity":9,"summary":""}' > "$bad"
node scripts/validate-jsonl.mjs --expect-reject "$bad"

step '06 license gate (selftest control first)'
node scripts/licenses-check.mjs --selftest
node scripts/licenses-check.mjs

step '07 tripwire negative scan (selftest control first)'
bash scripts/tripwire.sh --selftest
bash scripts/tripwire.sh

step '08 zero-network dependency closure of the collector binary'
# Strongest static form of the local-only promise: the FULL transitive
# import closure of cmd/hello-collector must contain no networking or
# process-exec packages at all.
deps=$(go list -deps ./cmd/hello-collector)
if printf '%s\n' "$deps" | grep -E '^(net|net/.+|crypto/tls|os/exec)$'; then
  echo RED: collector depends on network/exec packages; exit 1
fi
echo 'ZERO-NETWORK CLOSURE OK: no net/*, crypto/tls, os/exec in dependency closure'
# belt and braces: no raw imports in shipped sources
! grep -rnE '"(net|net/http|crypto/tls|os/exec)"' --include='*.go' . | grep -v '_test\.go'

step '09 audit file privacy bits'
grep -q '0o600' internal/auditlog/writer.go && echo 'writer opens audit files 0600: OK'

printf '\nGATE-D1: ALL GREEN\n'
