#!/usr/bin/env bash
# gate-w6.sh - machine-assertion battery closing the W6 observation wave:
# the agency guard observation counters of slice W6.1 (closed ten-word
# section 261 field vocabulary with a 10/10 coverage table, three counters
# over per-agent/per-task streams, count conservation, and the LIMIT/HOLD
# action-enum zero-write gate) and the cost guard observation records of
# slice W6.2 (five consumption proxies in count/duration form with a 5/3
# coverage registration, the known_gap wire shape that omits the value key
# entirely, the single-door gate-evidenced zero, and the cost-truth record
# structurally without any value entrance), the full v2-check battery with
# its 56-mutation selftest fleet, a decision-plane reachability grep over
# every wave symbol whose teeth are proven by a plant/remove control, the
# section 307 observation-form pre-verification fleet for items
# 2/4/6/8/9/12 (kept verbatim, zero enforcement-form promotion), and the
# complete inherited chain (gate-w5, which re-runs gate-w4, gate-w3,
# gate-w2, gate-w1, gate-w0, gate-d7 and gate-d1..d6 serially) with zero
# regression. Wave close per the taskbook W6 hard-judgement line:
# gate-w6 rc=0 with the 307 precheck fleet inside, nothing pre-borrowed.
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

step '00 preflight: W5, W6.1 and W6.2 surface repo-visible with eighteen rule keys over sections 18-20 plus seven agency keys over section 21 plus eight cost keys over section 22'
test -f internal/schema/actionnecessity.go || { echo 'RED: W5.1 surface missing'; exit 1; }
test -f internal/schema/chaindimensions.go || { echo 'RED: W5.2 surface missing'; exit 1; }
test -f internal/schema/delegationobservation.go || { echo 'RED: W5.3 surface missing'; exit 1; }
test -f internal/schema/agencyguard.go || { echo 'RED: W6.1 surface missing'; exit 1; }
test -f internal/schema/costguard.go || { echo 'RED: W6.2 surface missing'; exit 1; }
test -f scripts/gate-w6.sh || { echo 'RED: this gate not on disk'; exit 1; }
grep -q '^## 18. Action necessity record contract' docs/schema-v2.md \
  || { echo 'RED: section 18 header missing'; exit 1; }
grep -q '^## 19. Behavior chain dimension record contract' docs/schema-v2.md \
  || { echo 'RED: section 19 header missing'; exit 1; }
grep -q '^## 20. Delegation observation record contract' docs/schema-v2.md \
  || { echo 'RED: section 20 header missing'; exit 1; }
grep -q '^## 21. Agency guard observation record contract' docs/schema-v2.md \
  || { echo 'RED: section 21 header missing'; exit 1; }
grep -q '^## 22. Cost guard observation record contract' docs/schema-v2.md \
  || { echo 'RED: section 22 header missing'; exit 1; }
m=$(grep -cE '^(agency_guard_field_vocabulary|agency_counter_vocabulary|agency_counter_scope_vocabulary|agency_limit_state_vocabulary|agency_count_conservation_rule|agency_action_enum_gate|agency_enforcement_plane): ' docs/schema-v2.md)
[ "$m" -eq 7 ] || { echo "RED: W6.1 agency rule key census $m/7"; exit 1; }
q=$(grep -cE '^(cost_proxy_field_vocabulary|cost_proxy_shape_vocabulary|cost_value_state_vocabulary|cost_collector_standing_vocabulary|cost_field_shape_mapping|cost_absence_rule|cost_truth_stance|cost_enforcement_plane): ' docs/schema-v2.md)
[ "$q" -eq 8 ] || { echo "RED: W6.2 cost rule key census $q/8"; exit 1; }
n=$(grep -cE '^(action_necessity_question_vocabulary|action_necessity_answer_vocabulary|action_necessity_danger_only_rule|action_necessity_evidence_rule|action_necessity_absent_default|action_necessity_enforcement_plane|chain_dimension_vocabulary|chain_dimension_observation_states|chain_dimension_coverage_rule|chain_dimension_unclassified_rule|chain_dimension_reversibility_source_rule|chain_dimension_enforcement_plane|delegation_subject_vocabulary|delegation_revalidation_states|delegation_no_auto_inheritance_rule|delegation_reevaluation_registry|delegation_revalidation_obligation_rule|delegation_enforcement_plane): ' docs/schema-v2.md)
[ "$n" -eq 18 ] || { echo "RED: W5 rule key census $n/18"; exit 1; }
echo 'W5, W6.1 AND W6.2 SURFACE VISIBLE (necessity + chain dimensions + delegation + agency guard counters + cost guard proxies, eighteen plus seven plus eight keys pinned)'

step '01 v2-check full battery (slot, template, element, vocabularies, anchors, planeLeak, master table, W4 and W5 contracts)'
node scripts/schema-v2-check.mjs > "$TMPDIR/gate-w6-v2check.out" 2>&1 \
  || { tail -40 "$TMPDIR/gate-w6-v2check.out"; echo RED: schema-v2-check; exit 1; }
grep -q 'SCHEMA-V2: ALL GREEN' "$TMPDIR/gate-w6-v2check.out" \
  || { echo 'RED: v2-check banner missing'; exit 1; }
grep -q 'PASS data action' "$TMPDIR/gate-w6-v2check.out" \
  || { echo 'RED: data action predicate did not run'; exit 1; }
grep -q 'PASS trust domain' "$TMPDIR/gate-w6-v2check.out" \
  || { echo 'RED: trust domain predicate did not run'; exit 1; }
grep -q 'PASS export wrap' "$TMPDIR/gate-w6-v2check.out" \
  || { echo 'RED: export wrap predicate did not run'; exit 1; }
grep -q 'PASS action necessity' "$TMPDIR/gate-w6-v2check.out" \
  || { echo 'RED: action necessity predicate did not run'; exit 1; }
grep -q 'PASS chain dimensions' "$TMPDIR/gate-w6-v2check.out" \
  || { echo 'RED: chain dimensions predicate did not run'; exit 1; }
grep -q 'PASS delegation observation' "$TMPDIR/gate-w6-v2check.out" \
  || { echo 'RED: delegation observation predicate did not run'; exit 1; }
grep -q 'PASS agency guard counters' "$TMPDIR/gate-w6-v2check.out" \
  || { echo 'RED: agency guard predicate did not run'; exit 1; }
grep -q 'PASS cost guard proxies' "$TMPDIR/gate-w6-v2check.out" \
  || { echo 'RED: cost guard predicate did not run'; exit 1; }
grep -q 'PASS untrusted consumption' "$TMPDIR/gate-w6-v2check.out" \
  || { echo 'RED: inherited consume predicate did not run'; exit 1; }
grep -q 'PASS master table' "$TMPDIR/gate-w6-v2check.out" \
  || { echo 'RED: master table predicate did not run'; exit 1; }
echo 'V2-CHECK FULL RUN GREEN (necessity, chain dimensions, delegation, and agency guard predicates included)'

step '02 selftest positive controls: every mutation caught, shipped file silent'
node scripts/schema-v2-check.mjs --selftest > "$TMPDIR/gate-w6-selftest.out" 2>&1 \
  || { tail -20 "$TMPDIR/gate-w6-selftest.out"; echo RED: selftest; exit 1; }
grep -q 'SELFTEST OK: all 56 mutations caught' "$TMPDIR/gate-w6-selftest.out" \
  || { echo 'RED: selftest banner (want all 56 caught)'; tail -5 "$TMPDIR/gate-w6-selftest.out"; exit 1; }
MISSES=$(grep -c 'SELFTEST MISS' "$TMPDIR/gate-w6-selftest.out" || true)
[ "$MISSES" -eq 0 ] || { echo "RED: $MISSES selftest mutations missed"; exit 1; }
echo 'SELFTEST 50/50 CAUGHT (shipped predicates have teeth)'

step '03 Go: full schema package plus the named W5 and W6 batteries'
go test -count=1 ./internal/schema > "$TMPDIR/gate-w6-go.out" 2>&1 \
  || { tail -30 "$TMPDIR/gate-w6-go.out"; echo RED: schema package tests; exit 1; }
go test -count=1 ./internal/schema -run 'TestNecessity|TestActionNecessity|TestBuildActionNecessity' -v > "$TMPDIR/gate-w6-necessity.out" 2>&1 \
  || { tail -30 "$TMPDIR/gate-w6-necessity.out"; echo RED: action necessity battery; exit 1; }
for t in TestNecessityVocabularyAndRulesPin TestNecessityQuestionAndAnswerValidity TestBuildActionNecessityGoldenDeployChain TestBuildActionNecessityGoldenUnrelatedKeyRead TestBuildActionNecessityAbsentMeansUnassessed TestNecessityRejectionsLeaveNoRecord TestActionNecessityFieldSetCarriesNoDecision TestActionNecessitySymbolsStayOffTheDecisionPlane TestActionNecessityDocsSync; do
  grep -q "^--- PASS: $t" "$TMPDIR/gate-w6-necessity.out" || { echo "RED: $t did not pass"; exit 1; }
done
echo 'ACTION NECESSITY BATTERY GREEN (three questions and three answers pinned, golden pair recorded, rejections leave no record, docs sync pinned)'

go test -count=1 ./internal/schema -run 'TestChainDimension|TestIntentDeviation|TestTrustDomainCrossing|TestReversibilityReduction' -v > "$TMPDIR/gate-w6-chain.out" 2>&1 \
  || { tail -30 "$TMPDIR/gate-w6-chain.out"; echo RED: chain dimensions battery; exit 1; }
for t in TestChainDimensionVocabularyPin TestChainDimensionCoverageRows TestIntentDeviationGolden TestTrustDomainCrossingGolden TestReversibilityReductionGolden TestChainDimensionsRejectionsLeaveNoRecord TestChainDimensionsRecordCarriesNoDecision TestChainDimensionsSymbolsStayOffTheDecisionPlane TestChainDimensionsDocsSync; do
  grep -q "^--- PASS: $t" "$TMPDIR/gate-w6-chain.out" || { echo "RED: $t did not pass"; exit 1; }
done
echo 'CHAIN DIMENSIONS BATTERY GREEN (eight words pinned, 8/8 coverage rows matched against Go registration, three computable projections golden, docs sync pinned)'

go test -count=1 ./internal/schema -run 'TestDelegation|TestBuildDelegationObservation' -v > "$TMPDIR/gate-w6-delegation.out" 2>&1 \
  || { tail -30 "$TMPDIR/gate-w6-delegation.out"; echo RED: delegation battery; exit 1; }
for t in TestDelegationSubjectVocabularyPin TestDelegationRuleConstantsPin TestBuildDelegationObservationSpawnGolden TestDelegationRejectionsLeaveNoRecord TestDelegationRecordCarriesNoDecision TestDelegationSymbolsStayOffTheDecisionPlane TestDelegationDocsSync; do
  grep -q "^--- PASS: $t" "$TMPDIR/gate-w6-delegation.out" || { echo "RED: $t did not pass"; exit 1; }
done
echo 'DELEGATION BATTERY GREEN (five subjects pinned, no-inheritance constant constructor-held, rejections leave no record, docs sync pinned)'

go test -count=1 ./internal/schema -run 'TestAgency' -v > "$TMPDIR/gate-w6-agency.out" 2>&1 \
  || { tail -30 "$TMPDIR/gate-w6-agency.out"; echo RED: agency guard battery; exit 1; }
for t in TestAgencyGuardFieldVocabularyPin TestAgencyGuardCoverageRows TestAgencyCounterConservationGolden TestAgencyLimitRecordGoldenPair TestAgencyActionTokensNeverRecorded TestAgencyRejectionsLeaveNoRecord TestAgencyRecordsCarryNoDecision TestAgencySymbolsStayOffTheDecisionPlane TestAgencyGuardDocsSync; do
  grep -q "^--- PASS: $t" "$TMPDIR/gate-w6-agency.out" || { echo "RED: $t did not pass"; exit 1; }
done
echo 'AGENCY GUARD BATTERY GREEN (ten fields pinned with 10/10 coverage rows matched against Go registration, conservation golden, tripped ceiling builds byte-identical counters with nil error, ladder tokens never recorded, docs sync pinned)'

go test -count=1 ./internal/schema -run 'TestCost' -v > "$TMPDIR/gate-w6-cost.out" 2>&1 \
  || { tail -30 "$TMPDIR/gate-w6-cost.out"; echo RED: cost guard battery; exit 1; }
for t in TestCostProxyFieldVocabularyPin TestCostProxyCoverageRows TestCostStatedRecordGolden TestCostKnownGapAbsenceNotZero TestCostRejectionsLeaveNoRecord TestCostLadderTokensNeverStated TestCostLLMZeroGateRecord TestCostTruthRecordHasNoValueEntrance TestCostRecordsCarryNoDecision TestCostSymbolsStayOffTheDecisionPlane TestCostSurfaceIsStdlibOnly TestCostGuardDocsSync; do
  grep -q "^--- PASS: $t" "$TMPDIR/gate-w6-cost.out" || { echo "RED: $t did not pass"; exit 1; }
done
echo 'COST GUARD BATTERY GREEN (five proxies pinned with the 5/3 registration matched against Go, known_gap wire shapes omit the value key, half records rejected with nothing left behind, the gate zero keeps one door, the truth line has no value entrance, docs sync pinned)'

step '04 decision-plane reachability grep: no decision or command file consumes the W5 symbols (plant/remove control)'
LEAKRE='ActionNecessity|AllNecessity|NecessityQuestion|NecessityAnswer|BuildActionNecessity|ChainDimension|ChainStep|ChainObservation|ChainPhase|ComputableChain|AllChain|DelegationObservation|DelegationSubject|DelegationRevalidation|DelegationRecord|AllDelegation|BuildDelegation|AgencyGuardField|AgencyCounterKind|AgencyLimitState|AgencyLimitRecord|BuildAgencyCounterAggregate|BuildAgencyLimitRecord|AllAgencyGuard|AllAgencyCounter|AllAgencyLimit|AgencyForbiddenActionTokens|AgencyLadderRegistry|CostProxyField|CostProxyShape|CostValueState|CostCollectorStanding|CostProxyCoverageRow|CostProxyRecord|CostTruthRecord|BuildCostProxyRecord|BuildCostLLMZeroRecord|BuildCostTruthRecord|AllCostProxy|AllCostValueStates|AllCostCollectorStandings|CostAbsenceRule|CostTruthStance|CostGuardResponseRule|CostGuardEnforcementPlane|CostZeroLLMStatement|TraceField|AllTraceFields|TraceStance|AllTraceStances|TraceCoverageRow|AllTraceCoverage|TraceStanceOf|DecisionTrace|BuildDecisionTrace|TraceAbsenceRule|TraceCorrelationRule|TraceEnforcementModeValue|TraceEnforcementPlane|TraceDefaultApplied|EncodeTraceChecked|TraceDefaultAppliedRule'
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

step '05 inherited chain: gate-w5 full battery (re-runs gate-w4, gate-w3, gate-w2, gate-w1, gate-w0, gate-d7 and gate-d1..d6 serially)'
bash scripts/gate-w5.sh > "$TMPDIR/gate-w6-reg-w5.out" 2>&1 \
  || { tail -30 "$TMPDIR/gate-w6-reg-w5.out"; echo RED: gate-w5 regression; exit 1; }
grep -q 'GATE-W5: ALL GREEN' "$TMPDIR/gate-w6-reg-w5.out" \
  || { echo 'RED: gate-w5 banner missing'; exit 1; }
echo 'GATE-W5 GREEN INSIDE GATE-W6 (zero regression on the full inherited chain)'

step '06 section 307 observation-form pre-verification fleet (items 2/4/6/8/9/12, verbatim kept, zero promotion)'
pre307() {
  grep -qF "$1" docs/schema-v2.md || { echo "RED: 307 precheck anchor lost: $1"; exit 1; }
}
# item 2: high-risk pre-control observed as the danger-only necessity rule
pre307 'action_necessity_danger_only_rule: danger-judgement-alone-never-suffices'
# item 4: hard deny cannot be overwritten - observation form bans profile promotion
pre307 'Banned mutations: promoting `hard_deny`'
# item 6: untrusted consumption stays recorded-marked-never-auto-escalate
pre307 'untrusted_consume_rule: untrusted-source-modifications-are-recorded-marked-never-auto-escalate'
# item 8: recovery classes and the never-imply-reversible absence semantics
pre307 'recovery_class_vocabulary: local_reversible, local_partial, external_compensation, non_reversible'
pre307 'recovery_unclassified_semantics: absent-record-means-unknown-never-imply-reversible'
# item 9: irreversibility truthfulness and the reversibility source rail
pre307 'recovery_truthfulness_rule: never-claim-fully-reversible'
pre307 'chain_dimension_reversibility_source_rule: reversibility-reduction-reads-recovery-classes-only'
# item 12: the security core speaks to no cloud or UI plane - every declared
# enforcement/execution plane stays none-in-observation-phase (census pin:
# the none-line count equals the total declared-plane line count).
c=$(grep -cE '^[a-z_]+_(enforcement|execution)_plane: none-in-observation-phase$' docs/schema-v2.md)
t=$(grep -cE '^[a-z_]+_(enforcement|execution)_plane: ' docs/schema-v2.md)
[ "$c" -eq "$t" ] || { echo "RED: a declared plane was promoted out of observation stance ($c/$t)"; exit 1; }
[ "$c" -ge 16 ] || { echo "RED: plane census $c, want at least 16 none-lines"; exit 1; }
echo '307 PRECHECK FLEET GREEN (seven anchors verbatim, plane census all-none: no enforcement form pre-borrowed)'

step '07 tripwire standalone (internal-ledger negative scan)'
bash scripts/tripwire.sh > "$TMPDIR/gate-w6-tripwire.out" 2>&1 \
  || { cat "$TMPDIR/gate-w6-tripwire.out"; echo RED: tripwire; exit 1; }
grep -q 'TRIPWIRE CLEAN' "$TMPDIR/gate-w6-tripwire.out" || { echo RED: tripwire banner; exit 1; }

printf '\nGATE-W6: ALL GREEN\n'
