#!/usr/bin/env bash
# gate-w9.sh - machine-assertion battery for the W9 replay-and-observation wave.
#
# First edition. It pins what the wave has actually shipped and nothing more:
# slice W9.1 historical replay (candidate rule file loaded with zero runtime
# side effects, corpus replay producing per-event verdicts, byte-deterministic
# double runs, held lines counted honestly, reserved enforcement vocabulary
# never emitted), slice W9.2 the three-gate report (security, false-positive,
# productivity and performance faces with stated known gaps instead of
# defaulted zeros and a record-only response line that promotes no decision),
# slice W9.3 the policy diff form and the promotion decision record (the diff
# renders only the documents it compares, the promotion record cites a replay
# digest or it is not recordable, and the pre-pinned stance line - absent a
# promoted record no enforcement plane is admissible - is carried as text and
# wired nowhere), and slice W9.4 the decision cache observation shape (eight
# constraints plus the context epoch slot, indefinite reuse rejected, every
# Phase 0 record carries a zero hit count because every decision is
# recomputed, and the counters reuse the section 261 source with no second
# collector).
#
# The wave-end census for the four documentation word families (gates, diff,
# promotion and cache) is re-taken from the shipped docs on every run rather
# than trusted from memory, and the fleet count is read from the shipped
# checker at run time so a mutation added by a later leg cannot pass a stale
# banner number.
#
# Every step runs against real repo artifacts and carries its own control.
# Any red exits non-zero.
set -euo pipefail
cd "$(dirname "$0")/.."

export TMPDIR="$(pwd)/.tmptest"
mkdir -p "$TMPDIR"
find "$TMPDIR" -mindepth 1 -maxdepth 1 -name 'go-build*' -exec rm -rf {} + 2>/dev/null || true
command -v go >/dev/null 2>&1 || export PATH="$HOME/.local/go/bin:$PATH"
LOGDIR=/home/node/gate-logs
mkdir -p "$LOGDIR"

step() { printf '\n=== %s ===\n' "$*"; }

step '00 preflight: W9.1 to W9.4 surfaces repo-visible with the four documentation word families censused from disk'
test -f internal/replay/replay.go || { echo 'RED: W9.1 replay surface missing'; exit 1; }
test -f internal/replay/gates.go || { echo 'RED: W9.2 gates surface missing'; exit 1; }
test -f internal/replay/diff.go || { echo 'RED: W9.3 diff surface missing'; exit 1; }
test -f internal/schema/policypromotion.go || { echo 'RED: W9.3 promotion record surface missing'; exit 1; }
test -f internal/schema/decisioncache.go || { echo 'RED: W9.4 decision cache surface missing'; exit 1; }
test -f scripts/gate-w9.sh || { echo 'RED: this gate not on disk'; exit 1; }
grep -q '^## 29. Historical replay and the three-gate report' docs/schema-v2.md \
  || { echo 'RED: section 29 header missing'; exit 1; }
grep -q '^## 30. Policy diff form and the promotion decision record' docs/schema-v2.md \
  || { echo 'RED: section 30 header missing'; exit 1; }
grep -q '^## 31. Decision cache observation shape' docs/schema-v2.md \
  || { echo 'RED: section 31 header missing'; exit 1; }
g=$(grep -cE '^gate_[a-z_]+: ' docs/schema-v2.md)
[ "$g" -eq 8 ] || { echo "RED: gates word family census $g/8"; exit 1; }
d=$(grep -cE '^diff_[a-z_]+: ' docs/schema-v2.md)
[ "$d" -eq 7 ] || { echo "RED: diff word family census $d/7"; exit 1; }
p=$(grep -cE '^promotion_[a-z_]+: ' docs/schema-v2.md)
[ "$p" -eq 7 ] || { echo "RED: promotion word family census $p/7"; exit 1; }
c=$(grep -cE '^decision_cache_[a-z_]+: ' docs/schema-v2.md)
[ "$c" -eq 10 ] || { echo "RED: cache word family census $c/10"; exit 1; }
# the closed decision vocabulary is one spelling on the page and in the code
grep -q '^promotion_decision_vocabulary: shadow,promoted,rejected,deferred$' docs/schema-v2.md \
  || { echo 'RED: promotion vocabulary not pinned on the page'; exit 1; }
grep -q '"shadow", "promoted", "rejected", "deferred"' internal/schema/policypromotion.go \
  || { echo 'RED: promotion vocabulary not pinned in code'; exit 1; }
echo "W9 SURFACE VISIBLE (replay + gates + diff + promotion + cache with tests, three headings, eight plus seven plus seven plus ten keys censused from disk, vocabulary dual-pinned)"

step '01 v2-check full battery (inherited run, zero new predicates hung by this wave)'
node scripts/schema-v2-check.mjs > "$LOGDIR/gate-w9-v2check.out" 2>&1 \
  || { tail -40 "$LOGDIR/gate-w9-v2check.out"; echo RED: schema-v2-check; exit 1; }
grep -q 'SCHEMA-V2: ALL GREEN' "$LOGDIR/gate-w9-v2check.out" \
  || { echo 'RED: v2-check banner missing'; exit 1; }
grep -q 'function storageGovProblems' scripts/schema-v2-check.mjs \
  || { echo 'RED: inherited storage predicate definition lost'; exit 1; }
echo 'V2-CHECK FULL RUN GREEN (inherited battery intact; this wave hung no predicate that the shipped checker does not define)'

step '02 selftest positive controls: every mutation caught, fleet size read from the shipped checker'
node scripts/schema-v2-check.mjs --selftest > "$LOGDIR/gate-w9-selftest.out" 2>&1 \
  || { tail -20 "$LOGDIR/gate-w9-selftest.out"; echo RED: selftest; exit 1; }
banner=$(grep -m1 'SELFTEST OK' "$LOGDIR/gate-w9-selftest.out" || true)
[ -n "$banner" ] || { echo 'RED: selftest banner missing'; tail -5 "$LOGDIR/gate-w9-selftest.out"; exit 1; }
fleet=$(printf '%s' "$banner" | tr -cd '0-9' | cut -c1-4)
cases=$(awk '/const cases = \[/,/^  \];/' scripts/schema-v2-check.mjs | grep -cE '^ *\[')
[ "$fleet" -eq "$cases" ] || { echo "RED: fleet banner $fleet != cases block $cases"; exit 1; }
[ "$fleet" -ge 69 ] || { echo "RED: fleet $fleet, want at least 69 (W8 floor inherited)"
  exit 1; }
MISSES=$(grep -c 'SELFTEST MISS' "$LOGDIR/gate-w9-selftest.out" || true)
[ "$MISSES" -eq 0 ] || { echo "RED: $MISSES selftest mutations missed"; exit 1; }
echo "SELFTEST $fleet/$cases CAUGHT (banner derived from the shipped cases block)"

step '03 Go: named W9 batteries, test names enumerated from the shipped files (full-package regression rides the CI matrix)'
go test -count=1 ./internal/replay -run 'TestReplay|TestCandidate|TestHeld|TestGolden|TestReserved|TestGates|TestDiff' -v > "$LOGDIR/gate-w9-replay.out" 2>&1 \
  || { tail -40 "$LOGDIR/gate-w9-replay.out"; echo RED: W9.1/W9.2/W9.3 replay-family battery; exit 1; }
rnames=$(grep -hoE '^func (Test[A-Za-z0-9_]+)' internal/replay/replay_test.go internal/replay/gates_test.go internal/replay/diff_test.go | sed 's/^func //' \
  | grep -E '^Test(Replay|Candidate|Held|Golden|Reserved|Gates|Diff)' || true)
[ -n "$rnames" ] || { echo 'RED: no replay-family tests enumerated'; exit 1; }
for t in $rnames; do
  grep -q "^--- PASS: $t" "$LOGDIR/gate-w9-replay.out" || { echo "RED: $t did not pass"; exit 1; }
done
for fam in TestReplay TestCandidate TestHeld TestGolden TestReserved TestGates TestDiff; do
  grep -q "^--- PASS: $fam" "$LOGDIR/gate-w9-replay.out" || { echo "RED: family $fam emitted no pass"; exit 1; }
done
rn=$(printf '%s\n' "$rnames" | wc -l)
echo "REPLAY-FAMILY BATTERY GREEN ($rn tests, all seven families present)"

go test -count=1 ./internal/schema -run 'TestPromotion|TestBuildPromotion|TestDecisionCache' -v > "$LOGDIR/gate-w9-schema.out" 2>&1 \
  || { tail -40 "$LOGDIR/gate-w9-schema.out"; echo RED: W9.3/W9.4 schema battery; exit 1; }
snames=$(grep -hoE '^func (Test[A-Za-z0-9_]+)' internal/schema/policypromotion_test.go internal/schema/decisioncache_test.go | sed 's/^func //' \
  | grep -E '^Test(Promotion|BuildPromotion|DecisionCache)' || true)
[ -n "$snames" ] || { echo 'RED: no schema-family tests enumerated'; exit 1; }
for t in $snames; do
  grep -q "^--- PASS: $t" "$LOGDIR/gate-w9-schema.out" || { echo "RED: $t did not pass"; exit 1; }
done
for fam in TestPromotion TestDecisionCache; do
  grep -q "^--- PASS: $fam" "$LOGDIR/gate-w9-schema.out" || { echo "RED: family $fam emitted no pass"; exit 1; }
done
sn=$(printf '%s\n' "$snames" | wc -l)
echo "SCHEMA-FAMILY BATTERY GREEN ($sn tests over promotion and decision cache)"

step '04 decision-plane reachability grep: no decision-plane package consumes the W9 symbols (plant/remove control)'
# The shipped read-only replay subcommand on the CLI surface is the admitted
# consumer of the replay package (slice W9.1 comment states it is rendering
# only, with no write API anywhere on that file). The decision plane itself -
# policy, rules, bus, auditlog - and the runtime command must never reference
# a W9 symbol.
LEAKRE9='PolicyDiff|DiffHead|ModifiedRule|DefaultEffectChange|GatesReport|SecurityGate|FalsePositiveGate|ProductivityGate|PerformanceGate|PolicyPromotionRecord|PromotionDecision|DecisionCacheKey|DecisionCacheObservation|LoadCandidate|LoadCorpus|LoadLabels|BuildGates|BuildPolicyPromotionRecord|BuildDecisionCacheKey|BuildDecisionCacheObservation|PromotionRecordStance|DecisionCacheRecordStance|20131\.com/agentruntime/internal/replay'
probe9() {
  grep -rlE "$LEAKRE9" --include='*.go' internal/policy internal/rules internal/bus internal/auditlog cmd/hello-collector 2>/dev/null | grep -v '_test\.go' || true
}
hits=$(probe9)
[ -z "$hits" ] || { echo "RED: W9 symbols reachable in decision-plane files:"; echo "$hits"; exit 1; }
PLANT=internal/policy/w9-teeth-plant.go
printf 'package policy\n\n// probe\nvar _ = DecisionCacheKey{}\n' > "$PLANT"
hits=$(probe9)
rm -f "$PLANT"
[ -n "$hits" ] || { echo 'RED: planted reference did not fire (gate has no teeth)'; exit 1; }
hits=$(probe9)
[ -z "$hits" ] || { echo 'RED: removal did not clear the probe'; exit 1; }
echo 'DECISION-PLANE GREP ZERO HITS (planted shape fired, removal cleared)'

step '05 inherited chain: gate-w8 full battery (re-runs gate-w7, gate-w5, gate-w4, gate-w3, gate-w2, gate-w1, gate-w0, gate-d7 and gate-d1..d6 serially)'
bash scripts/gate-w8.sh > "$LOGDIR/gate-w8-inside-w9.out" 2>&1 \
  || { tail -30 "$LOGDIR/gate-w8-inside-w9.out"; echo RED: gate-w8 regression; exit 1; }
grep -q 'GATE-W8: ALL GREEN' "$LOGDIR/gate-w8-inside-w9.out" \
  || { echo 'RED: gate-w8 completion banner missing'; exit 1; }
grep -q 'STORAGE OBSERVATION ANCHORS GREEN' "$LOGDIR/gate-w8-inside-w9.out" \
  || { echo 'RED: gate-w8 step 06 did not report'; exit 1; }
echo 'GATE-W8 GREEN INSIDE GATE-W9 (zero regression on the full inherited chain)'

step '06 W9 observation anchors verbatim (record-only, zero promotion) plus the wave-end plane census re-taken from disk'
pre9() {
  grep -qF "$1" docs/schema-v2.md || { echo "RED: W9 anchor lost: $1"; exit 1; }
}
pre9 'gate_status_pair: measured,known_gap'
pre9 'gate_absent_rule: section-without-source-states-known-gap-never-defaulted-zero'
pre9 'gate_response_rule: record-only-no-promotion-decision-in-this-report'
pre9 'diff_input_gate: both-documents-must-pass-policy-grammar-nil-or-invalid-rejects-with-no-half-diff'
pre9 'diff_response_rule: record-only-zero-enforcement-plane'
pre9 'promotion_replay_cite_rule: record-must-cite-64-hex-replay-digest-a-decision-without-evidence-is-not-recordable'
pre9 'decision_cache_finite_ttl_rule: ttl-must-be-positive-inside-the-record-bound-indefinite-reuse-is-not-recordable'
pre9 'decision_cache_phase_zero_hits: hits-recorded-zero-always-every-decision-recomputed-nonzero-rejects-construction'
pre9 'decision_cache_counter_source: event-rate-kind-and-agent-task-scope-reused-from-slice-w6-1-no-second-collector'
# the two constructor-pinned stance lines carry the wave discipline as text
grep -qF 'absent a promoted record no enforcement plane is admissible' internal/schema/policypromotion.go \
  || { echo 'RED: promotion stance discipline line lost from the record'; exit 1; }
grep -qF 'in Phase 0 every decision is recomputed and the cache is never consulted' internal/schema/decisioncache.go \
  || { echo 'RED: cache stance discipline line lost from the record'; exit 1; }
# no promotion decision in this report, no enforcement plane in this wave:
# the two W9-family plane declarations exist and both read the one shipped
# observation token.
grep -q '^promotion_enforcement_plane: none-in-observation-phase$' docs/schema-v2.md \
  || { echo 'RED: promotion plane line lost or re-spelled'; exit 1; }
grep -q '^decision_cache_enforcement_plane: none-in-observation-phase$' docs/schema-v2.md \
  || { echo 'RED: cache plane line lost or re-spelled'; exit 1; }
pe=$(grep -cE '^(gate|diff|promotion|decision_cache)[a-z_]*_(enforcement|execution)_plane: ' docs/schema-v2.md || true)
[ "$pe" -eq 2 ] || { echo "RED: W9-family plane declarations drifted from the shipped two ($pe)"; exit 1; }
n=$(grep -cE '^[a-z_]+_(enforcement|execution)_plane: none-in-observation-phase$' docs/schema-v2.md)
t=$(grep -cE '^[a-z_]+_(enforcement|execution)_plane: ' docs/schema-v2.md)
[ "$n" -eq "$t" ] || { echo "RED: a declared plane was promoted out of observation stance ($n/$t)"; exit 1; }
[ "$n" -ge 22 ] || { echo "RED: plane census $n, want at least 22 none-lines"; exit 1; }
# the section 6 promotion-precedent register line: re-grepped from the repo on
# every run and carried as a record-form count, never a hung value.
g8=$(grep -rniE 'promotion.*(precedent|register)|六.*G8' docs internal/schema/policypromotion.go 2>/dev/null | grep -c . || true)
echo "W9 OBSERVATION ANCHORS GREEN (nine verbatim anchors, both stance lines pinned in code, plane census all-none: $n/$t derived from this run, promotion-precedent register hits: $g8 recorded not asserted)"

step '07 tripwire standalone (internal-ledger negative scan)'
bash scripts/tripwire.sh > "$LOGDIR/gate-w9-tripwire.out" 2>&1 \
  || { cat "$LOGDIR/gate-w9-tripwire.out"; echo RED: tripwire; exit 1; }
grep -q 'TRIPWIRE CLEAN' "$LOGDIR/gate-w9-tripwire.out" || { echo RED: tripwire banner; exit 1; }

printf '\nGATE-W9: ALL GREEN\n'
