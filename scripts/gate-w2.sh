#!/usr/bin/env bash
# gate-w2.sh - machine-assertion battery closing the W2 schema contract
# wave: the twenty-eight cell master table (seven wave schemas x four
# template elements, closed three-state census, every pointer resolving to
# a substantive element section under the shared sufficiency predicate,
# dual implementation same thresholds), the full v2-check run with all of
# its vocabulary/anchor/planeLeak predicates and its positive-control
# selftest, and the complete inherited chain (gate-w1, which re-runs
# gate-w0, gate-d7, and gate-d1..d6 serially) with zero regression.
#
# Every step runs against real repo artifacts and carries its own control
# (planted-shape controls must fire before a green scan is believed).
# Any red exits non-zero.
set -euo pipefail
cd "$(dirname "$0")/.."

export TMPDIR="$(pwd)/.tmptest"
mkdir -p "$TMPDIR"
find "$TMPDIR" -mindepth 1 -maxdepth 1 -name 'go-build*' -exec rm -rf {} + 2>/dev/null || true
command -v go >/dev/null 2>&1 || export PATH="$HOME/.local/go/bin:$PATH"

step() { printf '\n=== %s ===\n' "$*"; }

step '00 preflight: wave surface repo-visible with a full 28-cell census'
git ls-files --error-unmatch docs/schema-v2.md scripts/schema-v2-check.mjs scripts/gate-w2.sh >/dev/null \
  || { echo 'RED: wave surface not repo-visible'; exit 1; }
grep -q '^## 11. Wave master table' docs/schema-v2.md \
  || { echo 'RED: master table section missing'; exit 1; }
n=$(grep -c '^master_cell: ' docs/schema-v2.md)
[ "$n" -eq 28 ] || { echo "RED: master cell census $n/28"; exit 1; }
for s in decision intent authority impact recovery evidence profile; do
  c=$(grep -c "^master_cell: $s/" docs/schema-v2.md)
  [ "$c" -eq 4 ] || { echo "RED: schema $s carries $c/4 cells"; exit 1; }
done
echo 'MASTER TABLE VISIBLE (28 cells, seven x four complete cross product)'

step '01 v2-check full battery (slot, template, element, vocabularies, anchors, planeLeak, master table)'
node scripts/schema-v2-check.mjs > "$TMPDIR/gate-w2-v2check.out" 2>&1 \
  || { tail -40 "$TMPDIR/gate-w2-v2check.out"; echo RED: schema-v2-check; exit 1; }
grep -q 'SCHEMA-V2: ALL GREEN' "$TMPDIR/gate-w2-v2check.out" \
  || { echo 'RED: v2-check banner missing'; exit 1; }
grep -q 'PASS master table' "$TMPDIR/gate-w2-v2check.out" \
  || { echo 'RED: master table predicate did not run'; exit 1; }
echo 'V2-CHECK FULL RUN GREEN (master table predicate included)'

step '02 selftest positive controls: every mutation caught, shipped file silent'
node scripts/schema-v2-check.mjs --selftest > "$TMPDIR/gate-w2-selftest.out" 2>&1 \
  || { tail -20 "$TMPDIR/gate-w2-selftest.out"; echo RED: selftest; exit 1; }
grep -q 'SELFTEST OK: all 44 mutations caught' "$TMPDIR/gate-w2-selftest.out" \
  || { echo 'RED: selftest banner (want all 44 caught)'; tail -5 "$TMPDIR/gate-w2-selftest.out"; exit 1; }
MISSES=$(grep -c 'SELFTEST MISS' "$TMPDIR/gate-w2-selftest.out" || true)
[ "$MISSES" -eq 0 ] || { echo "RED: $MISSES selftest mutations missed"; exit 1; }
echo 'SELFTEST 44/44 CAUGHT (shipped predicates have teeth)'

step '03 Go second implementation: full schema package plus master battery'
go test -count=1 ./internal/schema > "$TMPDIR/gate-w2-go.out" 2>&1 \
  || { tail -30 "$TMPDIR/gate-w2-go.out"; echo RED: schema package tests; exit 1; }
go test -count=1 ./internal/schema -run 'TestSchemaMaster' -v > "$TMPDIR/gate-w2-master.out" 2>&1 \
  || { tail -30 "$TMPDIR/gate-w2-master.out"; echo RED: Go master battery; exit 1; }
for t in TestSchemaMasterCellCensus TestSchemaMasterCellSufficiency TestSchemaMasterFifteenNameCoverage; do
  grep -q "^--- PASS: $t" "$TMPDIR/gate-w2-master.out" || { echo "RED: $t did not pass"; exit 1; }
done
echo 'DUAL IMPLEMENTATION AGREES (same census, same thresholds, creator map pinned)'

step '04 inherited chain: gate-w1 full battery (re-runs gate-w0, gate-d7 and gate-d1..d6 serially)'
bash scripts/gate-w1.sh > "$TMPDIR/gate-w2-reg-w1.out" 2>&1 \
  || { tail -30 "$TMPDIR/gate-w2-reg-w1.out"; echo RED: gate-w1 regression; exit 1; }
grep -q 'GATE-W1: ALL GREEN' "$TMPDIR/gate-w2-reg-w1.out" \
  || { echo 'RED: gate-w1 banner missing'; exit 1; }
echo 'GATE-W1 GREEN INSIDE GATE-W2 (zero regression on the full inherited chain)'

step '05 tripwire standalone (internal-ledger negative scan)'
bash scripts/tripwire.sh > "$TMPDIR/gate-w2-tripwire.out" 2>&1 \
  || { cat "$TMPDIR/gate-w2-tripwire.out"; echo RED: tripwire; exit 1; }
grep -q 'TRIPWIRE CLEAN' "$TMPDIR/gate-w2-tripwire.out" || { echo RED: tripwire banner; exit 1; }

printf '\nGATE-W2: ALL GREEN\n'
