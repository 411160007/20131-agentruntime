#!/usr/bin/env bash
# gate-w4.sh - machine-assertion battery closing the W4 data-boundary
# wave: the nine data action classes of slice W4.1 (closed vocabulary
# mirrored docs<->Go<->api contract with the three non-equivalences),
# the trust domain mapping of slice W4.2 (five levels, seven run
# domains, legacy projection pinned pair by pair, lift as a window
# never a single value), the EXPORT re-judgement and packaging word
# family of slice W4.3 (eight packaging words, independence and
# sensitivity rules mirrored docs<->Go, golden facet pair recorded
# side by side), the full v2-check battery with its selftest
# positive-control fleet, a decision-plane reachability grep over
# every wave symbol whose teeth are proven by a plant/remove control
# on the command tree, and the complete inherited chain (gate-w3,
# which re-runs gate-w2, gate-w1, gate-w0, gate-d7, and gate-d1..d6
# serially) with zero regression. Vocabulary admission recorded here
# is the pre-gate artifact for the phase-one entry gate.
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

step '00 preflight: W4 surface repo-visible with sixteen rule keys over sections 15-17'
test -f internal/schema/dataaction.go || { echo 'RED: W4.1 surface missing'; exit 1; }
test -f internal/schema/trustdomain.go || { echo 'RED: W4.2 surface missing'; exit 1; }
test -f internal/schema/exportwrap.go || { echo 'RED: W4.3 surface missing'; exit 1; }
test -f scripts/gate-w4.sh || { echo 'RED: this gate not on disk'; exit 1; }
grep -q '^## 15. Data action vocabulary contract' docs/schema-v2.md \
  || { echo 'RED: section 15 header missing'; exit 1; }
grep -q '^## 16. Trust domain mapping contract' docs/schema-v2.md \
  || { echo 'RED: section 16 header missing'; exit 1; }
grep -q '^## 17. EXPORT re-judgement and packaging annotation contract' docs/schema-v2.md \
  || { echo 'RED: section 17 header missing'; exit 1; }
n=$(grep -cE '^(data_action_vocabulary|data_action_nonequivalence_rule|data_action_export_rejudgement_rule|data_action_absent_default|data_action_enforcement_plane|trust_level_vocabulary|run_domain_vocabulary|trust_domain_legacy_projection|trust_domain_crossing_rule|trust_domain_absent_default|trust_domain_enforcement_plane|export_packaging_word_vocabulary|export_packaging_sensitivity_rule|export_rejudgement_independence_rule|export_rejudgement_absent_default|export_rejudgement_enforcement_plane): ' docs/schema-v2.md)
[ "$n" -eq 16 ] || { echo "RED: W4 rule key census $n/16"; exit 1; }
echo 'W4 SURFACE VISIBLE (vocabulary + mapping + rejudgement, sixteen keys pinned)'

step '01 v2-check full battery (slot, template, element, vocabularies, anchors, planeLeak, master table, W4 contracts)'
node scripts/schema-v2-check.mjs > "$TMPDIR/gate-w4-v2check.out" 2>&1 \
  || { tail -40 "$TMPDIR/gate-w4-v2check.out"; echo RED: schema-v2-check; exit 1; }
grep -q 'SCHEMA-V2: ALL GREEN' "$TMPDIR/gate-w4-v2check.out" \
  || { echo 'RED: v2-check banner missing'; exit 1; }
grep -q 'PASS data action' "$TMPDIR/gate-w4-v2check.out" \
  || { echo 'RED: data action predicate did not run'; exit 1; }
grep -q 'PASS trust domain' "$TMPDIR/gate-w4-v2check.out" \
  || { echo 'RED: trust domain predicate did not run'; exit 1; }
grep -q 'PASS export wrap' "$TMPDIR/gate-w4-v2check.out" \
  || { echo 'RED: export wrap predicate did not run'; exit 1; }
grep -q 'PASS untrusted consumption' "$TMPDIR/gate-w4-v2check.out" \
  || { echo 'RED: inherited consume predicate did not run'; exit 1; }
grep -q 'PASS master table' "$TMPDIR/gate-w4-v2check.out" \
  || { echo 'RED: master table predicate did not run'; exit 1; }
echo 'V2-CHECK FULL RUN GREEN (data action, trust domain, and export wrap predicates included)'

step '02 selftest positive controls: every mutation caught, shipped file silent'
node scripts/schema-v2-check.mjs --selftest > "$TMPDIR/gate-w4-selftest.out" 2>&1 \
  || { tail -20 "$TMPDIR/gate-w4-selftest.out"; echo RED: selftest; exit 1; }
grep -q 'SELFTEST OK: all 44 mutations caught' "$TMPDIR/gate-w4-selftest.out" \
  || { echo 'RED: selftest banner (want all 44 caught)'; tail -5 "$TMPDIR/gate-w4-selftest.out"; exit 1; }
MISSES=$(grep -c 'SELFTEST MISS' "$TMPDIR/gate-w4-selftest.out" || true)
[ "$MISSES" -eq 0 ] || { echo "RED: $MISSES selftest mutations missed"; exit 1; }
echo 'SELFTEST 44/44 CAUGHT (shipped predicates have teeth)'

step '03 Go: full schema package plus the named W4 batteries'
go test -count=1 ./internal/schema > "$TMPDIR/gate-w4-go.out" 2>&1 \
  || { tail -30 "$TMPDIR/gate-w4-go.out"; echo RED: schema package tests; exit 1; }
go test -count=1 ./internal/schema -run 'TestDataAction' -v > "$TMPDIR/gate-w4-vocab.out" 2>&1 \
  || { tail -30 "$TMPDIR/gate-w4-vocab.out"; echo RED: vocabulary battery; exit 1; }
for t in TestDataActionVocabularyAndRulesPin TestDataActionGoodFixturesRoundTrip TestDataActionWildFixturesRejected TestDataActionAbsentMeansUnclassified TestDataActionCapabilityVocabularyUntouched TestDataActionSymbolsStayOffTheDecisionPlane TestDataActionDocsSync; do
  grep -q "^--- PASS: $t" "$TMPDIR/gate-w4-vocab.out" || { echo "RED: $t did not pass"; exit 1; }
done
echo 'VOCABULARY BATTERY GREEN (nine words pinned, wild shapes rejected, capability tokens untouched, docs sync pinned)'

go test -count=1 ./internal/schema -run 'TestTrustDomain|TestLegacyProjection|TestLift|TestProjection' -v > "$TMPDIR/gate-w4-domain.out" 2>&1 \
  || { tail -30 "$TMPDIR/gate-w4-domain.out"; echo RED: trust domain battery; exit 1; }
for t in TestTrustDomainVocabulariesAndRulesPin TestLegacyProjectionTableExact TestLiftIsAWindowNeverASingleValue TestProjectionLiftRoundTrip TestTrustDomainAbsentMeansUnclassified TestTrustDomainSymbolsStayOffTheDecisionPlane TestTrustDomainDocsSync; do
  grep -q "^--- PASS: $t" "$TMPDIR/gate-w4-domain.out" || { echo "RED: $t did not pass"; exit 1; }
done
echo 'TRUST DOMAIN BATTERY GREEN (projection exact, lift is a window, round trip closed, docs sync pinned)'

go test -count=1 ./internal/schema -run 'TestExport|TestPackaging|TestBuildExport|TestEachPackagingWord' -v > "$TMPDIR/gate-w4-export.out" 2>&1 \
  || { tail -30 "$TMPDIR/gate-w4-export.out"; echo RED: export battery; exit 1; }
for t in TestExportWrapVocabularyAndRulesPin TestPackagingWordValidity TestBuildExportRejudgementGoldenFacet TestEachPackagingWordHasProAndConFixtures TestExportRejudgementRejectionsLeaveNoRecord TestExportRejudgementFieldSetCarriesNoDecision TestExportRejudgementSymbolsStayOffTheDecisionPlane; do
  grep -q "^--- PASS: $t" "$TMPDIR/gate-w4-export.out" || { echo "RED: $t did not pass"; exit 1; }
done
echo 'EXPORT BATTERY GREEN (golden facet pair side by side, per-word pro and con fixtures, rejections leave no bytes, docs sync pinned)'

step '04 decision-plane reachability grep: no decision or command file consumes the W4 symbols (plant/remove control)'
LEAKRE='AllDataActions|DataAction|AllTrustLevels|TrustLevel|AllRunDomains|RunDomain|LegacyClassOf|LiftCandidates|AllPackagingWords|PackagingWord|BuildExportRejudgement|ExportRejudgementRecord'
probe() {
  grep -rlE "$LEAKRE" --include='*.go' cmd internal/policy internal/rules internal/bus internal/auditlog 2>/dev/null | grep -v '_test\.go' || true
}
hits=$(probe)
[ -z "$hits" ] || { echo "RED: W4 symbols reachable in decision/command files:"; echo "$hits"; exit 1; }
PLANT=cmd/agentruntime/w4-teeth-plant.go
if [ ! -d cmd/agentruntime ]; then
  d=$(find cmd -mindepth 1 -maxdepth 1 -type d | head -1)
  PLANT="$d/w4-teeth-plant.go"
fi
printf 'package main\n\n// probe\nvar _ = BuildExportRejudgement\n' > "$PLANT"
hits=$(probe)
rm -f "$PLANT"
[ -n "$hits" ] || { echo 'RED: planted reference did not fire (gate has no teeth)'; exit 1; }
hits=$(probe)
[ -z "$hits" ] || { echo 'RED: removal did not clear the probe'; exit 1; }
echo 'DECISION-PLANE GREP ZERO HITS (planted shape fired, removal cleared)'

step '05 inherited chain: gate-w3 full battery (re-runs gate-w2, gate-w1, gate-w0, gate-d7 and gate-d1..d6 serially)'
bash scripts/gate-w3.sh > "$TMPDIR/gate-w4-reg-w3.out" 2>&1 \
  || { tail -30 "$TMPDIR/gate-w4-reg-w3.out"; echo RED: gate-w3 regression; exit 1; }
grep -q 'GATE-W3: ALL GREEN' "$TMPDIR/gate-w4-reg-w3.out" \
  || { echo 'RED: gate-w3 banner missing'; exit 1; }
echo 'GATE-W3 GREEN INSIDE GATE-W4 (zero regression on the full inherited chain)'

step '06 tripwire standalone (internal-ledger negative scan)'
bash scripts/tripwire.sh > "$TMPDIR/gate-w4-tripwire.out" 2>&1 \
  || { cat "$TMPDIR/gate-w4-tripwire.out"; echo RED: tripwire; exit 1; }
grep -q 'TRIPWIRE CLEAN' "$TMPDIR/gate-w4-tripwire.out" || { echo RED: tripwire banner; exit 1; }

printf '\nGATE-W4: ALL GREEN\n'
