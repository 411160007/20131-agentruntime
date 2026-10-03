#!/usr/bin/env bash
# gate-w5.sh - machine-assertion battery closing the W5 record wave:
# the action necessity record of slice W5.1 (closed three-question
# and three-answer vocabularies, danger-only and evidence and
# absent-default rules mirrored docs<->Go, golden deploy-chain and
# unrelated-key-read pair), the behavior chain dimension records of
# slice W5.2 (eight-word vocabulary derived from the section 242
# anchor bullets, 8/8 coverage table pinned row by row, three
# computable projections with the reversibility dimension reading
# only the W2.3 recovery classes), the delegation observation record
# of slice W5.3 (eight must-record fields, closed five-subject and
# three-state vocabularies, no-auto-inheritance pinned by the
# constructor), the full v2-check battery with its selftest
# positive-control fleet, a decision-plane reachability grep over
# every wave symbol whose teeth are proven by a plant/remove control
# on the command tree, and the complete inherited chain (gate-w4,
# which re-runs gate-w3, gate-w2, gate-w1, gate-w0, gate-d7, and
# gate-d1..d6 serially) with zero regression. Wave close per the
# taskbook W5 hard-judgement line: gate-w5 rc=0 with d1..d7 inside.
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

step '00 preflight: W5 surface repo-visible with eighteen rule keys over sections 18-20'
test -f internal/schema/actionnecessity.go || { echo 'RED: W5.1 surface missing'; exit 1; }
test -f internal/schema/chaindimensions.go || { echo 'RED: W5.2 surface missing'; exit 1; }
test -f internal/schema/delegationobservation.go || { echo 'RED: W5.3 surface missing'; exit 1; }
test -f scripts/gate-w5.sh || { echo 'RED: this gate not on disk'; exit 1; }
grep -q '^## 18. Action necessity record contract' docs/schema-v2.md \
  || { echo 'RED: section 18 header missing'; exit 1; }
grep -q '^## 19. Behavior chain dimension record contract' docs/schema-v2.md \
  || { echo 'RED: section 19 header missing'; exit 1; }
grep -q '^## 20. Delegation observation record contract' docs/schema-v2.md \
  || { echo 'RED: section 20 header missing'; exit 1; }
n=$(grep -cE '^(action_necessity_question_vocabulary|action_necessity_answer_vocabulary|action_necessity_danger_only_rule|action_necessity_evidence_rule|action_necessity_absent_default|action_necessity_enforcement_plane|chain_dimension_vocabulary|chain_dimension_observation_states|chain_dimension_coverage_rule|chain_dimension_unclassified_rule|chain_dimension_reversibility_source_rule|chain_dimension_enforcement_plane|delegation_subject_vocabulary|delegation_revalidation_states|delegation_no_auto_inheritance_rule|delegation_reevaluation_registry|delegation_revalidation_obligation_rule|delegation_enforcement_plane): ' docs/schema-v2.md)
[ "$n" -eq 18 ] || { echo "RED: W5 rule key census $n/18"; exit 1; }
echo 'W5 SURFACE VISIBLE (necessity + chain dimensions + delegation, eighteen keys pinned)'

step '01 v2-check full battery (slot, template, element, vocabularies, anchors, planeLeak, master table, W4 and W5 contracts)'
node scripts/schema-v2-check.mjs > "$TMPDIR/gate-w5-v2check.out" 2>&1 \
  || { tail -40 "$TMPDIR/gate-w5-v2check.out"; echo RED: schema-v2-check; exit 1; }
grep -q 'SCHEMA-V2: ALL GREEN' "$TMPDIR/gate-w5-v2check.out" \
  || { echo 'RED: v2-check banner missing'; exit 1; }
grep -q 'PASS data action' "$TMPDIR/gate-w5-v2check.out" \
  || { echo 'RED: data action predicate did not run'; exit 1; }
grep -q 'PASS trust domain' "$TMPDIR/gate-w5-v2check.out" \
  || { echo 'RED: trust domain predicate did not run'; exit 1; }
grep -q 'PASS export wrap' "$TMPDIR/gate-w5-v2check.out" \
  || { echo 'RED: export wrap predicate did not run'; exit 1; }
grep -q 'PASS action necessity' "$TMPDIR/gate-w5-v2check.out" \
  || { echo 'RED: action necessity predicate did not run'; exit 1; }
grep -q 'PASS chain dimensions' "$TMPDIR/gate-w5-v2check.out" \
  || { echo 'RED: chain dimensions predicate did not run'; exit 1; }
grep -q 'PASS delegation observation' "$TMPDIR/gate-w5-v2check.out" \
  || { echo 'RED: delegation observation predicate did not run'; exit 1; }
grep -q 'PASS untrusted consumption' "$TMPDIR/gate-w5-v2check.out" \
  || { echo 'RED: inherited consume predicate did not run'; exit 1; }
grep -q 'PASS master table' "$TMPDIR/gate-w5-v2check.out" \
  || { echo 'RED: master table predicate did not run'; exit 1; }
echo 'V2-CHECK FULL RUN GREEN (necessity, chain dimensions, and delegation predicates included)'

step '02 selftest positive controls: every mutation caught, shipped file silent'
node scripts/schema-v2-check.mjs --selftest > "$TMPDIR/gate-w5-selftest.out" 2>&1 \
  || { tail -20 "$TMPDIR/gate-w5-selftest.out"; echo RED: selftest; exit 1; }
grep -q 'SELFTEST OK: all 50 mutations caught' "$TMPDIR/gate-w5-selftest.out" \
  || { echo 'RED: selftest banner (want all 50 caught)'; tail -5 "$TMPDIR/gate-w5-selftest.out"; exit 1; }
MISSES=$(grep -c 'SELFTEST MISS' "$TMPDIR/gate-w5-selftest.out" || true)
[ "$MISSES" -eq 0 ] || { echo "RED: $MISSES selftest mutations missed"; exit 1; }
echo 'SELFTEST 50/50 CAUGHT (shipped predicates have teeth)'

step '03 Go: full schema package plus the named W5 batteries'
go test -count=1 ./internal/schema > "$TMPDIR/gate-w5-go.out" 2>&1 \
  || { tail -30 "$TMPDIR/gate-w5-go.out"; echo RED: schema package tests; exit 1; }
go test -count=1 ./internal/schema -run 'TestNecessity|TestActionNecessity|TestBuildActionNecessity' -v > "$TMPDIR/gate-w5-necessity.out" 2>&1 \
  || { tail -30 "$TMPDIR/gate-w5-necessity.out"; echo RED: action necessity battery; exit 1; }
for t in TestNecessityVocabularyAndRulesPin TestNecessityQuestionAndAnswerValidity TestBuildActionNecessityGoldenDeployChain TestBuildActionNecessityGoldenUnrelatedKeyRead TestBuildActionNecessityAbsentMeansUnassessed TestNecessityRejectionsLeaveNoRecord TestActionNecessityFieldSetCarriesNoDecision TestActionNecessitySymbolsStayOffTheDecisionPlane TestActionNecessityDocsSync; do
  grep -q "^--- PASS: $t" "$TMPDIR/gate-w5-necessity.out" || { echo "RED: $t did not pass"; exit 1; }
done
echo 'ACTION NECESSITY BATTERY GREEN (three questions and three answers pinned, golden pair recorded, rejections leave no record, docs sync pinned)'

go test -count=1 ./internal/schema -run 'TestChainDimension|TestIntentDeviation|TestTrustDomainCrossing|TestReversibilityReduction' -v > "$TMPDIR/gate-w5-chain.out" 2>&1 \
  || { tail -30 "$TMPDIR/gate-w5-chain.out"; echo RED: chain dimensions battery; exit 1; }
for t in TestChainDimensionVocabularyPin TestChainDimensionCoverageRows TestIntentDeviationGolden TestTrustDomainCrossingGolden TestReversibilityReductionGolden TestChainDimensionsRejectionsLeaveNoRecord TestChainDimensionsRecordCarriesNoDecision TestChainDimensionsSymbolsStayOffTheDecisionPlane TestChainDimensionsDocsSync; do
  grep -q "^--- PASS: $t" "$TMPDIR/gate-w5-chain.out" || { echo "RED: $t did not pass"; exit 1; }
done
echo 'CHAIN DIMENSIONS BATTERY GREEN (eight words pinned, 8/8 coverage rows matched against Go registration, three computable projections golden, docs sync pinned)'

go test -count=1 ./internal/schema -run 'TestDelegation|TestBuildDelegationObservation' -v > "$TMPDIR/gate-w5-delegation.out" 2>&1 \
  || { tail -30 "$TMPDIR/gate-w5-delegation.out"; echo RED: delegation battery; exit 1; }
for t in TestDelegationSubjectVocabularyPin TestDelegationRuleConstantsPin TestBuildDelegationObservationSpawnGolden TestDelegationRejectionsLeaveNoRecord TestDelegationRecordCarriesNoDecision TestDelegationSymbolsStayOffTheDecisionPlane TestDelegationDocsSync; do
  grep -q "^--- PASS: $t" "$TMPDIR/gate-w5-delegation.out" || { echo "RED: $t did not pass"; exit 1; }
done
echo 'DELEGATION BATTERY GREEN (five subjects pinned, no-inheritance constant constructor-held, rejections leave no record, docs sync pinned)'

step '04 decision-plane reachability grep: no decision or command file consumes the W5 symbols (plant/remove control)'
LEAKRE='ActionNecessity|AllNecessity|NecessityQuestion|NecessityAnswer|BuildActionNecessity|ChainDimension|ChainStep|ChainObservation|ChainPhase|ComputableChain|AllChain|DelegationObservation|DelegationSubject|DelegationRevalidation|DelegationRecord|AllDelegation|BuildDelegation'
probe() {
  grep -rlE "$LEAKRE" --include='*.go' cmd internal/policy internal/rules internal/bus internal/auditlog 2>/dev/null | grep -v '_test\.go' || true
}
hits=$(probe)
[ -z "$hits" ] || { echo "RED: W5 symbols reachable in decision/command files:"; echo "$hits"; exit 1; }
PLANT=cmd/agentruntime/w5-teeth-plant.go
if [ ! -d cmd/agentruntime ]; then
  d=$(find cmd -mindepth 1 -maxdepth 1 -type d | head -1)
  PLANT="$d/w5-teeth-plant.go"
fi
printf 'package main\n\n// probe\nvar _ = BuildDelegationObservation\n' > "$PLANT"
hits=$(probe)
rm -f "$PLANT"
[ -n "$hits" ] || { echo 'RED: planted reference did not fire (gate has no teeth)'; exit 1; }
hits=$(probe)
[ -z "$hits" ] || { echo 'RED: removal did not clear the probe'; exit 1; }
echo 'DECISION-PLANE GREP ZERO HITS (planted shape fired, removal cleared)'

step '05 inherited chain: gate-w4 full battery (re-runs gate-w3, gate-w2, gate-w1, gate-w0, gate-d7 and gate-d1..d6 serially)'
bash scripts/gate-w4.sh > "$TMPDIR/gate-w5-reg-w4.out" 2>&1 \
  || { tail -30 "$TMPDIR/gate-w5-reg-w4.out"; echo RED: gate-w4 regression; exit 1; }
grep -q 'GATE-W4: ALL GREEN' "$TMPDIR/gate-w5-reg-w4.out" \
  || { echo 'RED: gate-w4 banner missing'; exit 1; }
echo 'GATE-W4 GREEN INSIDE GATE-W5 (zero regression on the full inherited chain)'

step '06 tripwire standalone (internal-ledger negative scan)'
bash scripts/tripwire.sh > "$TMPDIR/gate-w5-tripwire.out" 2>&1 \
  || { cat "$TMPDIR/gate-w5-tripwire.out"; echo RED: tripwire; exit 1; }
grep -q 'TRIPWIRE CLEAN' "$TMPDIR/gate-w5-tripwire.out" || { echo RED: tripwire banner; exit 1; }

printf '\nGATE-W5: ALL GREEN\n'
