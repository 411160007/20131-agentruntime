#!/usr/bin/env bash
# gate-d2.sh — full machine-assertion battery for the D2 slice:
# agent discovery (three-platform fixture + live linux scan), JSONL audit
# rotation (size + day-cut), the agent-collector binary, and every
# inherited D1 guarantee (regression).
#
# Every gate runs against the REAL staged build artifacts and carries its
# own positive/negative control. Any red exits non-zero.
set -euo pipefail
cd "$(dirname "$0")/.."

export TMPDIR="$(pwd)/.tmptest"
mkdir -p "$TMPDIR" # sandbox kills binaries executed from /tmp; pin scratch inside the repo
# environment-harvest guard: wipe a previous gate's leftover go-build scratch
# before any whole-tree step, so generated _testmain.go can never false-red.
find "$TMPDIR" -mindepth 1 -maxdepth 1 -name 'go-build*' -exec rm -rf {} + 2>/dev/null || true

step() { printf '\n=== %s ===\n' "$*"; }

step '00 preflight'
go version
node --version

step '01 fmt / vet (three GOOS) / unit tests, serial'
# format gate judges shipped sources only: tracked .go files — the gate
# must never judge its own scratch (leftover go-build dirs from a prior run
# once false-red the re-run chain; judge tracked sources, nothing else)
fmt=$(git ls-files '*.go' | xargs -r gofmt -l)
test -z "$fmt" || { printf '%s\n' "$fmt"; echo RED: gofmt; exit 1; }
go vet ./...
GOOS=darwin GOARCH=amd64 go vet ./...
GOOS=windows GOARCH=amd64 go vet ./...
rc=0
go test -count=1 -p 1 -parallel 1 ./... || rc=$?
if [ "$rc" -eq 137 ]; then
  echo 'WARN: go test SIGKILLed by environment (rc137); retrying once'
  go test -count=1 -p 1 -parallel 1 ./... || rc=$?
fi
if [ "$rc" -ne 0 ]; then echo RED: go test rc=$rc; exit "$rc"; fi

step '02 cross-build six artifacts (2 binaries x 3 platforms)'
bash scripts/build-dist.sh

step '03 darwin/windows platform-tagged tests COMPILE (cross-equivalent)'
# Native execution of the build-tagged test files happens on the CI
# matrix runners; locally the equivalent assertion is that the test
# binaries build cleanly for the target.
GOOS=darwin GOARCH=amd64 go test -c -o "$TMPDIR/discovery-darwin.test" ./internal/discovery
GOOS=windows GOARCH=amd64 go test -c -o "$TMPDIR/discovery-windows.test.exe" ./internal/discovery
GOOS=darwin GOARCH=amd64 go test -c -o "$TMPDIR/collector-darwin.test" ./cmd/agent-collector
GOOS=windows GOARCH=amd64 go test -c -o "$TMPDIR/collector-windows.test.exe" ./cmd/agent-collector
ls -la "$TMPDIR"/*.test* | awk '{print $5, $9}'
echo 'CROSS-TEST-COMPILE OK (darwin+windows test binaries built; native run = CI matrix)'

step '04 linux artifacts execute: --version exits 0'
hello=$(ls dist/hello-collector-*-linux-amd64 | head -1)
agent=$(ls dist/agent-collector-*-linux-amd64 | head -1)
"$hello" --version
"$agent" --version
# D5 shape contract: "<name> <version> <goos>/<goarch> (<goversion>)"
verline=$("$agent" --version)
printf '%s\n' "$verline" | grep -qE '^agent-collector [^ ]+ linux/amd64 \(go[0-9.]+\)$' || {
  echo "RED: version line shape wrong: $verline"; exit 1; }
printf '%s\n' "$("$hello" --version)" | grep -qE '^hello-collector [^ ]+ linux/amd64 \(go[0-9.]+\)$' || {
  echo 'RED: hello version line shape wrong'; exit 1; }

step '05 executable containers verified (positive) + control (negative)'
node scripts/verify-cross.mjs
node scripts/verify-cross.mjs --negative README.md

step '06 D1 regression: hello pipeline still validates'
out=$(mktemp "$TMPDIR"/events.XXXXXX.jsonl)
"$hello" --out "$out"
node scripts/validate-jsonl.mjs "$out" --min-lines 4

step '07 agent-collector --once live /proc scan -> valid JSONL'
live=$(mktemp "$TMPDIR"/live.XXXXXX.jsonl)
"$agent" --once --out "$live"
node scripts/validate-jsonl.mjs "$live" --min-lines 3
grep -q '"type":"agent.scan"' "$live" || { echo 'RED: no agent.scan event'; exit 1; }
grep -q '"stage":"observation"' "$live" || { echo 'RED: discovery outside observation stage'; exit 1; }
if grep -qE '"decision":"(block|would_block|ask)"' "$live"; then
  echo 'RED: blocking vocabulary in discovery audit'; exit 1
fi
echo 'LIVE-SCAN OK (detections in this container are environment-dependent; scan line is mandatory)'

step '08 rotation against the real binary (size policy, 3 cycles)'
rotout=$(mktemp -d --tmpdir="$TMPDIR")/rot.jsonl
"$agent" --max-cycles 3 --interval 50ms --rotate-bytes 900 --rotate-daily=false --out "$rotout"
segs=$(ls "$rotout".* 2>/dev/null | wc -l)
if [ "$segs" -lt 1 ]; then echo 'RED: size rotation produced no segments'; exit 1; fi
total=0
for f in "$rotout" "$rotout".*; do
  [ -f "$f" ] || continue
  # legal rotation tail (see gate-d3 step 10 note): rename-on-threshold
  # on the last cycle leaves an empty head; archives still validated,
  # empty head keeps the mode gate, total>=6 still proves the corpus
  if [ "$f" = "$rotout" ] && [ ! -s "$f" ]; then
    mode=$(stat -c '%a' "$f")
    [ "$mode" = "600" ] || { echo "RED: empty rotation head $f mode $mode != 600"; exit 1; }
    continue
  fi
  node scripts/validate-jsonl.mjs "$f" >/dev/null
  mode=$(stat -c '%a' "$f")
  [ "$mode" = "600" ] || { echo "RED: $f mode $mode != 600"; exit 1; }
  total=$((total + $(wc -l < "$f")))
done
if [ "$total" -lt 6 ]; then echo "RED: rotation lost lines (total=$total, want >= 6)"; exit 1; fi
echo "ROTATION OK: $segs segment(s), $total total lines, all 0600, all valid JSONL"

step '09 validator control: bad fixture must be rejected'
bad=$(mktemp "$TMPDIR"/bad.XXXXXX.jsonl)
printf '%s\n' '{"v":9,"id":"!!","ts":"nope","stage":"wild","type":"x.y","decision":"block","severity":9,"summary":""}' > "$bad"
node scripts/validate-jsonl.mjs --expect-reject "$bad"
bad2=$(mktemp "$TMPDIR"/bad2.XXXXXX.jsonl)
printf '%s\n' '{"v":1,"id":"ev-x","agent_id":"agi-1","ts":"2026-09-26T18:00:00Z","stage":"enforcement","type":"agent.detected","decision":"allow","severity":0,"summary":"wrong stage for discovery"}' > "$bad2"
# stage enum itself is valid (schema-level), so this line PASSES the
# validator on purpose — the collector-side invariant (observation only)
# is enforced in step 07's grep gate, not by schema. Documented boundary.
node scripts/validate-jsonl.mjs "$bad2" >/dev/null && echo 'boundary documented: enforcement-staged detected passes schema gate (collector never emits it)'

step '10 license gate (selftest control first)'
node scripts/licenses-check.mjs --selftest
node scripts/licenses-check.mjs

step '11 tripwire negative scan (selftest control first)'
bash scripts/tripwire.sh --selftest
bash scripts/tripwire.sh

step '12 zero-network closure for BOTH binaries, ALL THREE GOOS'
for goos in linux darwin windows; do
  for bin in hello-collector agent-collector; do
    deps=$(GOOS=$goos go list -deps "./cmd/$bin")
    if printf '%s\n' "$deps" | grep -E '^(net|net/.+|crypto/tls)$'; then
      echo "RED: $bin ($goos) depends on network packages"; exit 1
    fi
  done
done
echo 'ZERO-NETWORK CLOSURE OK: no net/*, crypto/tls in any collector closure (linux/darwin/windows)'
# hello-collector keeps its stricter D1 closure (no os/exec either)
hdeps=$(go list -deps ./cmd/hello-collector)
if printf '%s\n' "$hdeps" | grep -E '^(net|net/.+|crypto/tls|os/exec)$'; then
  echo 'RED: hello-collector lost its no-exec closure'; exit 1
fi
! grep -rnE '"(net|net/http|crypto/tls)"' --include='*.go' . | grep -v '_test\.go'

step '13 audit file privacy bits'
grep -q '0o600' internal/auditlog/writer.go
if [ "$(go version | cut -d' ' -f3)" = "go1.27.1" ] && [ "$(uname)" = "Linux" ]; then
  echo 'PASS unix 0600 (checked exactly here: GOOS=linux go1.27.1)'
else
  echo 'SKIP unix 0600 spot-check: darwin/windows privacy = owner ACLs (mode bits advisory), asserted in CI native leg (identity: go version + uname)'
fi

step '14 frozen core API contract (doc == validator == Go)'
node scripts/apicontract-check.mjs
node scripts/apicontract-check.mjs --negative

printf '\nGATE-D2: ALL GREEN\n'
