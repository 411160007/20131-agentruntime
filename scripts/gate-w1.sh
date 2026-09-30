#!/usr/bin/env bash
# gate-w1.sh — machine-assertion battery closing the W1 evidence-trust-order
# series: the trust ladder runs in its two co-existing tracks (the narrative
# track pinning that self-description never decides alone, and the
# source_class field track pinning that the recorded provenance class is
# carried but never consumed, with stamped/unstamped twins forced to agree),
# the Phase 0 closed decision set re-pinned on the shipped table, and the
# complete inherited chain: gate-w0 (which re-runs gate-d7, which re-runs
# gate-d1..d6 serially) with zero regression. The golden-label conservation
# and the version-lock face are proven inside the inherited gate-w0 steps.
#
# Every gate runs against real artifacts and carries its own negative
# control (planted-shape controls must fire before a green scan is
# believed). Any red exits non-zero.
set -euo pipefail
cd "$(dirname "$0")/.."

export TMPDIR="$(pwd)/.tmptest"
mkdir -p "$TMPDIR"
find "$TMPDIR" -mindepth 1 -maxdepth 1 -name 'go-build*' -exec rm -rf {} + 2>/dev/null || true
command -v go >/dev/null 2>&1 || export PATH="$HOME/.local/go/bin:$PATH"

step() { printf '\n=== %s ===\n' "$*"; }

step '00 preflight: both tracks present in source form and repo-visible'
grep -q 'func TestEvidenceTrustOrder(' internal/rules/eval_test.go \
  || { echo 'RED: narrative track missing from the judgement test surface'; exit 1; }
grep -q 'func TestEvidenceTrustOrderDualTrack(' internal/rules/trustorder_dualtrack_test.go \
  || { echo 'RED: field track missing from the judgement test surface'; exit 1; }
git ls-files --error-unmatch internal/rules/trustorder_dualtrack_test.go >/dev/null 2>&1 \
  || { echo 'RED: field-track file not repo-visible'; exit 1; }
for c in native_os runtime tool_mcp agent_meta agent_self llm_interpretation; do
  grep -rq "\"$c\"" internal/schema/event.go || { echo "RED: class vocabulary lost: $c"; exit 1; }
done
echo 'BOTH TRACKS PRESENT (six-class vocabulary intact)'

step '01 dual-track battery: narrative zero-regression + field track with teeth'
go test -count=1 -v ./internal/rules -run 'TestEvidenceTrustOrder' > "$TMPDIR/gate-w1-trust.out" 2>&1 \
  || { tail -30 "$TMPDIR/gate-w1-trust.out"; echo RED: dual-track trust battery; exit 1; }
grep -q '^--- PASS: TestEvidenceTrustOrder ' "$TMPDIR/gate-w1-trust.out" \
  || { echo RED: narrative track pass line missing; exit 1; }
for sub in field-agent-self-alone field-llm-interpretation-alone \
           field-native-cmdline field-native-path field-runtime-path \
           control-forged-native-stamp-without-evidence control-low-stamp-over-native-evidence; do
  grep -q "^    --- PASS: TestEvidenceTrustOrderDualTrack/$sub" "$TMPDIR/gate-w1-trust.out" \
    || { echo "RED: field-track case missing or not green: $sub"; exit 1; }
done
echo 'DUAL TRACK GREEN (claim-only allow; native-backed would_block; forged-stamp and low-stamp controls agree; stamped/unstamped twins identical)'

step '02 decision values stay inside the Phase 0 closed set on the shipped table'
nwb=$(node -e 'const s=require("node:fs").readFileSync("internal/rules/rules.go","utf8");console.log((s.match(/"effect": "would_block"/g)||[]).length)')
[ "$nwb" -eq 12 ] || { echo "RED: built-in effects $nwb/12 would_block"; exit 1; }
if grep -rnE '"decision": ?"(block|denied|deny)"' --include='*.go' internal/rules/ | grep -v _test; then
  echo 'RED: blocking vocabulary appeared in the judgement package'; exit 1
fi
echo 'CLOSED SET PINNED (12/12 would_block effects; no blocking vocabulary)'

step '03 inherited chain: gate-w0 full battery (re-runs gate-d7 and gate-d1..d6 serially, version-lock and golden-label conservation included)'
bash scripts/gate-w0.sh > "$TMPDIR/gate-w1-reg-w0.out" 2>&1 \
  || { tail -30 "$TMPDIR/gate-w1-reg-w0.out"; echo RED: gate-w0 regression; exit 1; }
grep -q 'GATE-W0: ALL GREEN' "$TMPDIR/gate-w1-reg-w0.out" \
  || { echo RED: gate-w0 banner missing; tail -5 "$TMPDIR/gate-w1-reg-w0.out"; exit 1; }
echo 'GATE-W0 GREEN INSIDE GATE-W1 (zero regression on the full inherited chain)'

step '04 tripwire standalone (internal-ledger negative scan)'
bash scripts/tripwire.sh > "$TMPDIR/gate-w1-tripwire.out" 2>&1 \
  || { cat "$TMPDIR/gate-w1-tripwire.out"; echo RED: tripwire; exit 1; }
grep -q 'TRIPWIRE CLEAN' "$TMPDIR/gate-w1-tripwire.out" \
  || { echo RED: tripwire banner; exit 1; }

printf '\nGATE-W1: ALL GREEN\n'
