#!/usr/bin/env bash
# gate-w10.sh - machine-assertion battery for the W10 external-intelligence
# contract wave.
#
# First edition. The wave shipped documentation-only contracts and nothing
# else: slice W10.1 the six-function signature contract (section 290 names,
# structured output shapes, the never-required-path ban per section 266),
# slice W10.2 the fallback contract (six failure modes, allow-all and
# block-all both banned, malformed equals unavailable, and the deterministic
# core continues untouched - the section 7 iron law on the W10 face), and
# slice W10.3 the AI output poisoning guard (five untouchable boundaries,
# the four-step validation chain in normative order, the four must-not
# prohibitions mapped one-to-one onto guarded boundaries, the deterministic
# policy bypass ban, and the sixth trust tier's structural absence carried
# forward as a continuation proof).
#
# Because the wave is contract-only, this gate's central machine judgement is
# the zero-code-increment assertion: the wave diff face between the pinned
# wave endpoints contains docs paths and nothing else, no intel package
# exists, the six contract function names are unreachable from the decision
# plane (plant/remove control), and every wiring_state line reads
# contract-docs-only on disk. The word-family census is re-taken from the
# shipped docs on every run (thirty-eight family keys: base, fallback and
# guard families censused separately so neither of the later two can go
# missing) and the fleet count is read from the shipped checker at run time.
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

step '00 preflight: W10.1 to W10.3 contract surfaces repo-visible with the three intel word families censused from disk'
grep -q '^## 32. External intelligence function contract (slice W10.1)$' docs/schema-v2.md \
  || { echo 'RED: section 32 header missing'; exit 1; }
grep -q '^## 33. External intelligence fallback contract (slice W10.2)$' docs/schema-v2.md \
  || { echo 'RED: section 33 header missing'; exit 1; }
grep -q '^## 34. AI output poisoning guard contract (slice W10.3)$' docs/schema-v2.md \
  || { echo 'RED: section 34 header missing'; exit 1; }
test -f scripts/gate-w10.sh || { echo 'RED: this gate not on disk'; exit 1; }
t=$(grep -cE '^intel[a-z_0-9]*: ' docs/schema-v2.md)
[ "$t" -eq 38 ] || { echo "RED: intel family census $t/38"; exit 1; }
fb=$(grep -cE '^intel_fallback_[a-z_0-9]+: ' docs/schema-v2.md)
[ "$fb" -eq 12 ] || { echo "RED: fallback family census $fb/12"; exit 1; }
gd=$(grep -cE '^intel_guard_[a-z_0-9]+: ' docs/schema-v2.md)
[ "$gd" -eq 13 ] || { echo "RED: guard family census $gd/13"; exit 1; }
bs=$((t - fb - gd))
[ "$bs" -eq 13 ] || { echo "RED: base family census $bs/13 (derived total minus the two later families)"; exit 1; }
# the three closed sets are one spelling on the page
grep -q '^intel_function_closed_set: analyze_intent,analyze_plan,analyze_context,analyze_behavior,explain_decision,recommend_policy$' docs/schema-v2.md \
  || { echo 'RED: six-function closed set not pinned on the page'; exit 1; }
grep -q '^intel_guard_must_not_closed_set: grant-root,modify-user-hard-deny,modify-security-boundary,release-anti-tamper$' docs/schema-v2.md \
  || { echo 'RED: four-prohibition closed set not pinned on the page'; exit 1; }
grep -q '^intel_guard_validation_chain_closed_set: schema-validation,source-validation,risk-evaluation,policy-evaluation-four-steps-in-normative-order$' docs/schema-v2.md \
  || { echo 'RED: four-step chain closed set not pinned on the page'; exit 1; }
echo "W10 SURFACE VISIBLE (three headings, 38 family keys censused from disk as 13+12+13, three closed sets pinned)"

step '01 v2-check full battery (inherited run, zero new predicates hung by this wave)'
node scripts/schema-v2-check.mjs > "$LOGDIR/gate-w10-v2check.out" 2>&1 \
  || { tail -40 "$LOGDIR/gate-w10-v2check.out"; echo RED: schema-v2-check; exit 1; }
grep -q 'SCHEMA-V2: ALL GREEN' "$LOGDIR/gate-w10-v2check.out" \
  || { echo 'RED: v2-check banner missing'; exit 1; }
grep -q 'function storageGovProblems' scripts/schema-v2-check.mjs \
  || { echo 'RED: inherited storage predicate definition lost'; exit 1; }
echo 'V2-CHECK FULL RUN GREEN (inherited battery intact; this wave hung no predicate that the shipped checker does not define)'

step '02 selftest positive controls: every mutation caught, fleet size read from the shipped checker'
node scripts/schema-v2-check.mjs --selftest > "$LOGDIR/gate-w10-selftest.out" 2>&1 \
  || { tail -20 "$LOGDIR/gate-w10-selftest.out"; echo RED: selftest; exit 1; }
banner=$(grep -m1 'SELFTEST OK' "$LOGDIR/gate-w10-selftest.out" || true)
[ -n "$banner" ] || { echo 'RED: selftest banner missing'; tail -5 "$LOGDIR/gate-w10-selftest.out"; exit 1; }
fleet=$(printf '%s' "$banner" | tr -cd '0-9' | cut -c1-4)
cases=$(awk '/const cases = \[/,/^  \];/' scripts/schema-v2-check.mjs | grep -cE '^ *\[')
[ "$fleet" -eq "$cases" ] || { echo "RED: fleet banner $fleet != cases block $cases"; exit 1; }
[ "$fleet" -ge 69 ] || { echo "RED: fleet $fleet, want at least 69 (W8 floor inherited)"; exit 1; }
MISSES=$(grep -c 'SELFTEST MISS' "$LOGDIR/gate-w10-selftest.out" || true)
[ "$MISSES" -eq 0 ] || { echo "RED: $MISSES selftest mutations missed"; exit 1; }
echo "SELFTEST $fleet/$cases CAUGHT (banner derived from the shipped cases block)"

step '03 wave diff face zero-code-increment: the pinned W10 wave range touches docs paths only (fail-closed on endpoint ancestry)'
# Wave endpoints are immutable history: gate-w9 merge (wave base, head of the
# W9 wave this one follows) and the W10.3 merge (wave end). If either stops
# being an ancestor of HEAD the range means nothing and the gate says so.
W10_BASE=a58916b1c22346dee740564e5df70d2520d251ba
W10_END=70974590383441c66765d2e128ced4ee2cea63ba
git merge-base --is-ancestor "$W10_BASE" HEAD || { echo 'RED: wave base endpoint not an ancestor of HEAD'; exit 1; }
git merge-base --is-ancestor "$W10_END" HEAD || { echo 'RED: wave end endpoint not an ancestor of HEAD'; exit 1; }
nondocs=$(git diff --name-only "${W10_BASE}..${W10_END}" | grep -vE '^docs/' || true)
[ -z "$nondocs" ] || { echo 'RED: wave diff face carries non-docs files:'; echo "$nondocs"; exit 1; }
codefiles=$(git diff --name-only "${W10_BASE}..${W10_END}" | grep -E '\.(go|mod|sum|yml|yaml|json)$' || true)
[ -z "$codefiles" ] || { echo 'RED: wave diff face carries code/build files:'; echo "$codefiles"; exit 1; }
test ! -d internal/intel || { echo 'RED: an implementation package internal/intel appeared'; exit 1; }
wcount=$(git diff --name-only "${W10_BASE}..${W10_END}" | wc -l)
echo "WAVE DIFF FACE DOCS-ONLY ($wcount file(s) between the pinned endpoints, no code, no intel package on disk)"

step '04 decision-plane reachability grep: no decision-plane file references a W10 contract function (plant/remove control)'
LEAKRE10='analyze_intent|analyze_plan|analyze_context|analyze_behavior|explain_decision|recommend_policy|20131\.com/agentruntime/internal/intel'
probe10() {
  grep -rlE "$LEAKRE10" --include='*.go' internal/policy internal/rules internal/bus internal/auditlog cmd/hello-collector 2>/dev/null | grep -v '_test\.go' || true
}
hits=$(probe10)
[ -z "$hits" ] || { echo "RED: W10 contract symbols reachable in decision-plane files:"; echo "$hits"; exit 1; }
PLANT=internal/policy/w10-teeth-plant.go
printf 'package policy\n\n// probe\nfunc analyze_intent() {}\n' > "$PLANT"
hits=$(probe10)
rm -f "$PLANT"
[ -n "$hits" ] || { echo 'RED: planted reference did not fire (gate has no teeth)'; exit 1; }
hits=$(probe10)
[ -z "$hits" ] || { echo 'RED: removal did not clear the probe'; exit 1; }
echo 'DECISION-PLANE GREP ZERO HITS (six contract names unreachable; planted shape fired, removal cleared)'

step '05 inherited chain: gate-w9 full battery (re-runs gate-w8, gate-w7, gate-w5, gate-w4, gate-w3, gate-w2, gate-w1, gate-w0, gate-d7 and gate-d1..d6 serially)'
bash scripts/gate-w9.sh > "$LOGDIR/gate-w9-inside-w10.out" 2>&1 \
  || { tail -30 "$LOGDIR/gate-w9-inside-w10.out"; echo RED: gate-w9 regression; exit 1; }
grep -q 'GATE-W9: ALL GREEN' "$LOGDIR/gate-w9-inside-w10.out" \
  || { echo 'RED: gate-w9 completion banner missing'; exit 1; }
grep -q 'W9 OBSERVATION ANCHORS GREEN' "$LOGDIR/gate-w9-inside-w10.out" \
  || { echo 'RED: gate-w9 step 06 did not report'; exit 1; }
echo 'GATE-W9 GREEN INSIDE GATE-W10 (zero regression on the full inherited chain)'

step '06 W10 contract anchors verbatim (contract-only wiring on all three slices) plus the wave-end plane census re-taken from disk'
pre10() {
  grep -qF "$1" docs/schema-v2.md || { echo "RED: W10 anchor lost: $1"; exit 1; }
}
pre10 'intel_function_count_pin: exactly-six-names-any-add-or-drop-fails-the-contract'
pre10 'intel_input_gate: one-structured-observation-envelope-per-call-never-free-text-only'
pre10 'intel_free_text_policy_ban: free-text-output-never-becomes-policy-directly'
pre10 'intel_required_path_ban: intelligence-path-is-never-a-required-path-per-section-266'
pre10 'intel_fallback_mode_count_pin: exactly-six-modes-any-add-or-drop-fails-the-contract'
pre10 'intel_fallback_ban_allow_all: intelligence-absent-never-means-allow-all'
pre10 'intel_fallback_ban_block_all: intelligence-absent-never-means-block-all'
pre10 'intel_fallback_malformed_equals_unavailable: structurally-invalid-answer-classifies-as-unavailable-never-partially-parsed'
pre10 'intel_guard_boundary_count_pin: exactly-five-boundaries-any-add-or-drop-fails-the-contract'
pre10 'intel_guard_policy_bypass_banned: intelligence-answer-never-bypasses-deterministic-policy-evaluation'
pre10 'intel_guard_sixth_tier_structural_absence: no-shipped-producer-emits-llm-interpretation-source-class-zero-llm-current-state-continuation-proof'
# all three slices state the same wiring stance on disk
wn=$(grep -cE '^intel(_(fallback|guard))?_wiring_state: contract-docs-only-zero-implementation-zero-import-zero-call-site$' docs/schema-v2.md)
[ "$wn" -eq 3 ] || { echo "RED: contract-docs-only wiring lines drifted from the shipped three ($wn)"; exit 1; }
# the sixth trust tier stays a schema declaration with zero producers outside the schema package
prod=$(grep -rn 'SrcLLMInterpretation' --include='*.go' internal cmd 2>/dev/null | grep -v '_test\.go' | grep -v 'internal/schema/' | grep -c . || true)
[ "$prod" -eq 0 ] || { echo "RED: an outside-schema producer/consumer of the sixth tier appeared ($prod hits)"; exit 1; }
# zero-cost standing order: the decision plane carries no model client and no transport
sdk=$(grep -rliE 'openai|anthropic|bedrock|ollama|gemini|inference' --include='*.go' internal/policy internal/rules internal/bus internal/auditlog cmd/hello-collector 2>/dev/null | grep -v '_test\.go' | grep -c . || true)
[ "$sdk" -eq 0 ] || { echo "RED: a model-client reference entered the decision plane ($sdk files)"; exit 1; }
net=$(grep -rln '"net/http"' internal/policy internal/rules internal/bus internal/auditlog cmd/hello-collector 2>/dev/null | grep -c . || true)
[ "$net" -eq 0 ] || { echo "RED: a network import entered the decision plane ($net files)"; exit 1; }
# wave-end plane census: every declared plane still reads the one shipped observation token
n=$(grep -cE '^[a-z_]+_(enforcement|execution)_plane: none-in-observation-phase$' docs/schema-v2.md)
t2=$(grep -cE '^[a-z_]+_(enforcement|execution)_plane: ' docs/schema-v2.md)
[ "$n" -eq "$t2" ] || { echo "RED: a declared plane was promoted out of observation stance ($n/$t2)"; exit 1; }
[ "$n" -ge 25 ] || { echo "RED: plane census $n, want at least 25 none-lines"; exit 1; }
echo "W10 CONTRACT ANCHORS GREEN (eleven verbatim anchors, three contract-docs-only wiring lines, sixth tier zero producers, zero-cost-order zero SDK/zero network on the decision plane, plane census all-none: $n/$t2 derived from this run)"

step '07 tripwire standalone (internal-ledger negative scan)'
bash scripts/tripwire.sh > "$LOGDIR/gate-w10-tripwire.out" 2>&1 \
  || { cat "$LOGDIR/gate-w10-tripwire.out"; echo RED: tripwire; exit 1; }
grep -q 'TRIPWIRE CLEAN' "$LOGDIR/gate-w10-tripwire.out" || { echo RED: tripwire banner; exit 1; }

printf '\nGATE-W10: ALL GREEN\n'
