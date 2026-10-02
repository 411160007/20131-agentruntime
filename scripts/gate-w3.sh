#!/usr/bin/env bash
# gate-w3.sh - machine-assertion battery closing the W3 intent-contract
# wave: the UNTRUSTED consumption contract of slice W3.3 (five rule
# lines mirrored docs<->Go, yin-yang sticky-propagation tests, red
# shapes rejected before bytes), the slice W3.2 alignment record
# surface (inherited through gate-w2), the full v2-check battery with
# its selftest positive-control fleet, an escalation-path
# red-shape grep whose teeth are proven by a plant/remove control on
# the command tree, and the complete inherited chain (gate-w2, which
# re-runs gate-w1, gate-w0, gate-d7, and gate-d1..d6 serially) with
# zero regression.
#
# Every step runs against real repo artifacts and carries its own
# control. Any red exits non-zero.
set -euo pipefail
cd "$(dirname "$0")/.."

export TMPDIR="$(pwd)/.tmptest"
mkdir -p "$TMPDIR"
find "$TMPDIR" -mindepth 1 -maxdepth 1 -name 'go-build*' -exec rm -rf {} + 2>/dev/null || true
command -v go >/dev/null 2>&1 || export PATH="$HOME/.local/go/bin:$PATH"

step() { printf '\n=== %s ===\n' "$*"; }

step '00 preflight: W3 surface repo-visible with the five consume rules and section 13'
test -f internal/schema/actionalignment.go || { echo 'RED: W3.2 surface missing'; exit 1; }
test -f internal/schema/untrustedconsume.go || { echo 'RED: W3.3 surface missing'; exit 1; }
test -f scripts/gate-w3.sh || { echo 'RED: this gate not on disk'; exit 1; }
grep -q '^## 13. UNTRUSTED consumption contract' docs/schema-v2.md \
  || { echo 'RED: section 13 header missing'; exit 1; }
n=$(grep -cE '^(untrusted_consume_rule|authority_preservation_rule|origin_forge_rule|sticky_clearance_rule|consume_enforcement_plane): ' docs/schema-v2.md)
[ "$n" -eq 5 ] || { echo "RED: consume rule line census $n/5"; exit 1; }
echo 'W3 SURFACE VISIBLE (alignment + consumption, five rules pinned)'

step '01 v2-check full battery (slot, template, element, vocabularies, anchors, planeLeak, master table, consume contract)'
node scripts/schema-v2-check.mjs > "$TMPDIR/gate-w3-v2check.out" 2>&1 \
  || { tail -40 "$TMPDIR/gate-w3-v2check.out"; echo RED: schema-v2-check; exit 1; }
grep -q 'SCHEMA-V2: ALL GREEN' "$TMPDIR/gate-w3-v2check.out" \
  || { echo 'RED: v2-check banner missing'; exit 1; }
grep -q 'PASS untrusted consumption' "$TMPDIR/gate-w3-v2check.out" \
  || { echo 'RED: consume predicate did not run'; exit 1; }
grep -q 'PASS alignment' "$TMPDIR/gate-w3-v2check.out" \
  || { echo 'RED: alignment predicate did not run'; exit 1; }
grep -q 'PASS inflow' "$TMPDIR/gate-w3-v2check.out" \
  || { echo 'RED: inflow predicate did not run'; exit 1; }
grep -q 'PASS data action' "$TMPDIR/gate-w3-v2check.out" \
  || { echo 'RED: data action predicate did not run'; exit 1; }
grep -q 'PASS trust domain' "$TMPDIR/gate-w3-v2check.out" \
  || { echo 'RED: trust domain predicate did not run'; exit 1; }
echo 'V2-CHECK FULL RUN GREEN (consume, inflow, data action, and trust domain predicates included)'

step '02 selftest positive controls: every mutation caught, shipped file silent'
node scripts/schema-v2-check.mjs --selftest > "$TMPDIR/gate-w3-selftest.out" 2>&1 \
  || { tail -20 "$TMPDIR/gate-w3-selftest.out"; echo RED: selftest; exit 1; }
grep -q 'SELFTEST OK: all 38 mutations caught' "$TMPDIR/gate-w3-selftest.out" \
  || { echo 'RED: selftest banner (want all 38 caught)'; tail -5 "$TMPDIR/gate-w3-selftest.out"; exit 1; }
MISSES=$(grep -c 'SELFTEST MISS' "$TMPDIR/gate-w3-selftest.out" || true)
[ "$MISSES" -eq 0 ] || { echo "RED: $MISSES selftest mutations missed"; exit 1; }
echo 'SELFTEST 38/38 CAUGHT (shipped predicates have teeth)'

step '03 Go: full schema package plus the named W3.3 consumption battery'
go test -count=1 ./internal/schema > "$TMPDIR/gate-w3-go.out" 2>&1 \
  || { tail -30 "$TMPDIR/gate-w3-go.out"; echo RED: schema package tests; exit 1; }
go test -count=1 ./internal/schema -run 'TestUntrustedConsume' -v > "$TMPDIR/gate-w3-consume.out" 2>&1 \
  || { tail -30 "$TMPDIR/gate-w3-consume.out"; echo RED: consume battery; exit 1; }
for t in TestUntrustedConsumeYinYang TestUntrustedConsumeStickyNeverLaundered TestUntrustedConsumeRedShapes TestUntrustedConsumeLowLayerNeverAloneMovesUserBoundary TestUntrustedConsumeGoldenRoundTrip TestUntrustedConsumeDocsSync; do
  grep -q "^--- PASS: $t" "$TMPDIR/gate-w3-consume.out" || { echo "RED: $t did not pass"; exit 1; }
done
echo 'CONSUMPTION BATTERY GREEN (yin-yang sticky, red shapes zero-byte, docs sync pinned)'

go test -count=1 ./internal/schema -run 'TestInflow' -v > "$TMPDIR/gate-w3-inflow.out" 2>&1 \
  || { tail -30 "$TMPDIR/gate-w3-inflow.out"; echo RED: inflow battery; exit 1; }
for t in TestInflowVocabularyAndRulesPin TestInflowGoodFixturesRoundTrip TestInflowWildFixturesRejected TestInflowHookChannelCannotReachAuthority TestInflowAbsentMeansKnownGap TestInflowSymbolsStayOffTheDecisionPlane TestInflowDocsSync; do
  grep -q "^--- PASS: $t" "$TMPDIR/gate-w3-inflow.out" || { echo "RED: $t did not pass"; exit 1; }
done
echo 'INFLOW BATTERY GREEN (channel vocabulary pinned, hook task->goal only, absent means known gap, red shapes zero-record, docs sync pinned)'

step '04 escalation-path red-shape grep: no decision or command file consumes the new symbols (plant/remove control)'
LEAKRE='ApplyIntentModification|IntentModification|UntrustedConsumeRule|AuthorityPreservationRule|OriginForgeRule|StickyClearanceRule|ConsumeEnforcementPlane'
probe() {
  grep -rlE "$LEAKRE" --include='*.go' cmd internal/policy internal/rules internal/bus internal/auditlog 2>/dev/null | grep -v '_test\.go' || true
}
hits=$(probe)
[ -z "$hits" ] || { echo "RED: consumption symbols reachable in decision/command files:"; echo "$hits"; exit 1; }
PLANT=cmd/agentruntime/w33-teeth-plant.go
if [ ! -d cmd/agentruntime ]; then
  d=$(find cmd -mindepth 1 -maxdepth 1 -type d | head -1)
  PLANT="$d/w33-teeth-plant.go"
fi
printf 'package main\n\n// probe\nvar _ = ApplyIntentModification\n' > "$PLANT"
hits=$(probe)
rm -f "$PLANT"
[ -n "$hits" ] || { echo 'RED: planted reference did not fire (gate has no teeth)'; exit 1; }
hits=$(probe)
[ -z "$hits" ] || { echo 'RED: removal did not clear the probe'; exit 1; }
echo 'ESCALATION-PATH GREP ZERO HITS (planted shape fired, removal cleared)'

step '05 inherited chain: gate-w2 full battery (re-runs gate-w1, gate-w0, gate-d7 and gate-d1..d6 serially)'
bash scripts/gate-w2.sh > "$TMPDIR/gate-w3-reg-w2.out" 2>&1 \
  || { tail -30 "$TMPDIR/gate-w3-reg-w2.out"; echo RED: gate-w2 regression; exit 1; }
grep -q 'GATE-W2: ALL GREEN' "$TMPDIR/gate-w3-reg-w2.out" \
  || { echo 'RED: gate-w2 banner missing'; exit 1; }
echo 'GATE-W2 GREEN INSIDE GATE-W3 (zero regression on the full inherited chain)'

step '06 tripwire standalone (internal-ledger negative scan)'
bash scripts/tripwire.sh > "$TMPDIR/gate-w3-tripwire.out" 2>&1 \
  || { cat "$TMPDIR/gate-w3-tripwire.out"; echo RED: tripwire; exit 1; }
grep -q 'TRIPWIRE CLEAN' "$TMPDIR/gate-w3-tripwire.out" || { echo RED: tripwire banner; exit 1; }

printf '\nGATE-W3: ALL GREEN\n'
