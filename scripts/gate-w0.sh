#!/usr/bin/env bash
# gate-w0.sh — machine-assertion battery for the W0 evaluations-closure
# residual slice: the sixteen-class / twelve-class coverage map in its two
# required forms (docs table + machine JSON), the threshold ledger in its
# two forms, both cross-checked against INDEPENDENT sources (corpus files,
# labels.json, the built-in rule table parsed from shipped source, and a
# fresh eval-runner execution), and the complete inherited chain: gate-d7
# (which itself re-runs gate-d1..d6 serially) with zero regression.
#
# Every gate runs against real artifacts and carries its own negative
# control (--selftest batteries must fire before the real scan is believed).
# Any red exits non-zero.
set -euo pipefail
cd "$(dirname "$0")/.."

export TMPDIR="$(pwd)/.tmptest"
mkdir -p "$TMPDIR"
find "$TMPDIR" -mindepth 1 -maxdepth 1 -name 'go-build*' -exec rm -rf {} + 2>/dev/null || true
command -v go >/dev/null 2>&1 || export PATH="$HOME/.local/go/bin:$PATH"

step() { printf '\n=== %s ===\n' "$*"; }

step '00 preflight'
go version
node --version
for f in testdata/golden/coverage-map.json testdata/golden/thresholds.json docs/evals-coverage-v0.md scripts/coverage-map-check.mjs scripts/threshold-ledger-check.mjs; do
  [ -f "$f" ] || { echo "RED: slice artifact missing: $f"; exit 1; }
done
git ls-files --error-unmatch testdata/golden/coverage-map.json testdata/golden/thresholds.json docs/evals-coverage-v0.md >/dev/null 2>&1 \
  || { echo 'RED: map/ledger/docs not repo-visible (gitignore swallowing evidence?)'; exit 1; }

step '01 coverage map: negative controls, then the real dual-source re-count'
node scripts/coverage-map-check.mjs --selftest
node scripts/coverage-map-check.mjs

step '02 threshold ledger: fresh runner execution, controls, then the cross-check'
go test ./internal/rules/ -run TestGoldenEvals -v -count=1 -p 1 -parallel 1 > "$TMPDIR/gate-w0-eval.out" 2>&1 \
  || { tail -25 "$TMPDIR/gate-w0-eval.out"; echo 'RED: eval suite failed (ledger has no live source to bind to)'; exit 1; }
grep -q 'EVAL SUMMARY' "$TMPDIR/gate-w0-eval.out" || { echo 'RED: no EVAL SUMMARY in runner output'; exit 1; }
node scripts/threshold-ledger-check.mjs --selftest "$TMPDIR/gate-w0-eval.out"
node scripts/threshold-ledger-check.mjs "$TMPDIR/gate-w0-eval.out"

step '03 map and ledger carry the version-lock line (rule vocabulary / labels / map evolve in one PR)'
grep -q 'Version lock:' testdata/golden/coverage-map.json
grep -q 'Version lock:' testdata/golden/thresholds.json
grep -q 'Version lock:' docs/evals-coverage-v0.md
echo 'VERSION LOCK PRESENT IN ALL THREE ARTIFACTS'

step '04 inherited chain: gate-d7 full battery (includes gate-d1..d6 serial regression)'
bash scripts/gate-d7.sh > "$TMPDIR/gate-w0-reg-d7.out" 2>&1 || { tail -30 "$TMPDIR/gate-w0-reg-d7.out"; echo RED: gate-d7 regression; exit 1; }
grep -q 'GATE-D7: ALL GREEN' "$TMPDIR/gate-w0-reg-d7.out" || { echo RED: gate-d7 banner missing; tail -5 "$TMPDIR/gate-w0-reg-d7.out"; exit 1; }
echo 'GATE-D7 GREEN INSIDE GATE-W0 (zero regression on the expanded chain)'

step '05 tripwire standalone (internal-ledger negative scan)'
bash scripts/tripwire.sh > "$TMPDIR/gate-w0-tripwire.out" 2>&1 || { cat "$TMPDIR/gate-w0-tripwire.out"; echo RED: tripwire; exit 1; }
grep -q 'TRIPWIRE CLEAN' "$TMPDIR/gate-w0-tripwire.out" || { echo RED: tripwire banner; exit 1; }

printf '\nGATE-W0: ALL GREEN\n'
