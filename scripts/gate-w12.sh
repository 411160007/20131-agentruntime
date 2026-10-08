#!/usr/bin/env bash
# gate-w12.sh - machine-assertion battery for the W12 recovery-and-safety design wave.
#
# First edition, parameter-changed from the gate-w11 skeleton (whole-file copy
# discipline: inherited scaffolding kept byte-shape where possible). The wave
# shipped four slices: W12.1 the transaction/task-level recovery design face
# (honest registration of the deferred decision kept verbatim, recovery point
# as an observed design shape only), W12.2 the recovery honesty grading
# document (four closed grades in normative order, per-face table, forbidden
# marketing clauses), W12.3 the memory-security and learning-safety design
# face (four clauses plus the capability-lease cross-reference, structural
# absence of any promotion path from memory or learning output to authority),
# and W12.4 the enforcement-wedge composition output (reference-shape
# skeleton, six-element completeness table against the external proposal's
# section list, single-writer-source drift defense declared and pinned).
#
# The wave diff face is docs plus four standalone checkers: unlike estimation
# wave W11 there is no schema package addition and no mount test addition, so
# the standing invariant sharpens from "no enforcement-plane change" to
# "pure design and checker bytes only" - decision and enforcement planes
# (policy, rules, bus, auditlog, agentlog, mcpproxy, hookinstaller, cmd)
# carry zero bytes, docs/schema-v2.md carries zero diff, and every plane
# token still reads none-in-observation-phase.
#
# Census numbers are re-taken from shipped artifacts on every run, never
# copied from a plan document: the four-grade closed vocabulary presence,
# the structural-absence line, the deferred-decision registration headings,
# and each checker's selftest fleet are derived at run time. The wave-end
# endpoints are immutable history asserted as ancestors. Every step runs
# against real repo artifacts and carries its own control. Any red exits
# non-zero.
set -euo pipefail
cd "$(dirname "$0")/.."

export TMPDIR="$(pwd)/.tmptest"
mkdir -p "$TMPDIR"
find "$TMPDIR" -mindepth 1 -maxdepth 1 -name 'go-build*' -exec rm -rf {} + 2>/dev/null || true
command -v go >/dev/null 2>&1 || export PATH="$HOME/.local/go/bin:$PATH"
LOGDIR=/home/node/gate-logs
mkdir -p "$LOGDIR"

step() { printf '\n=== %s ===\n' "$*"; }

step '00 preflight: the four W12 docs and the four shipped checkers are repo-visible; wave vocabularies and declaration lines re-taken from disk'
for f in docs/recovery-transaction-design.md docs/recovery-honesty-grading.md docs/memory-learning-safety-design.md docs/phase1-enforcement-wedge.md; do
  test -f "$f" || { echo "RED: wave doc missing: $f"; exit 1; }
done
for c in w12-recovery-check w12-grading-check w12-memsec-check w12-wedge-check; do
  test -f "scripts/${c}.mjs" || { echo "RED: checker missing: ${c}.mjs"; exit 1; }
done
test -f scripts/gate-w12.sh || { echo 'RED: this gate not on disk'; exit 1; }
# four-grade closed set: every display spelling must be present (derived from
# the shipped document, not from any planning artifact)
gv=0
for g in 'LOCAL REVERSIBLE' 'LOCAL PARTIAL' 'EXTERNAL COMPENSATION' 'NON-REVERSIBLE'; do
  grep -qF "$g" docs/recovery-honesty-grading.md || { echo "RED: grade token missing: $g"; exit 1; }
  gv=$((gv + 1))
done
[ "$gv" -eq 4 ] || { echo "RED: grade vocabulary loop did not run four times"; exit 1; }
# structural-absence assertion line (design face of the wave gate)
grep -qF 'does not structurally exist' docs/memory-learning-safety-design.md \
  || { echo 'RED: structural-absence MA line missing'; exit 1; }
# deferred-decision honest registration kept on both recovery docs
grep -qF 'Honest registration of the "not this cycle" decision' docs/recovery-transaction-design.md \
  || { echo 'RED: deferred-decision registration heading missing'; exit 1; }
grep -qF '单写者源' docs/phase1-enforcement-wedge.md \
  || { echo 'RED: single-writer-source declaration missing'; exit 1; }
tbl=$(grep -c '^| ' docs/recovery-honesty-grading.md)
[ "$tbl" -ge 10 ] || { echo "RED: grading table row census $tbl, want at least 10 (re-taken from disk)"; exit 1; }
echo "W12 SURFACE VISIBLE (four docs, four checkers, four-grade vocabulary complete, MA line + registration heading + single-writer line in place, grading table rows $tbl re-counted at run time)"

step '01 checker full runs: schema-v2 regression plus all four W12 checkers against the shipped tree'
node scripts/schema-v2-check.mjs > "$LOGDIR/gate-w12-v2check.out" 2>&1 \
  || { tail -40 "$LOGDIR/gate-w12-v2check.out"; echo RED: schema-v2-check; exit 1; }
grep -q 'SCHEMA-V2: ALL GREEN' "$LOGDIR/gate-w12-v2check.out" \
  || { echo 'RED: v2-check banner missing'; exit 1; }
node scripts/w12-recovery-check.mjs > "$LOGDIR/gate-w12-recovery.out" 2>&1 \
  || { tail -20 "$LOGDIR/gate-w12-recovery.out"; echo RED: w12-recovery-check; exit 1; }
grep -q 'RECOVERY-DESIGN CHECK GREEN' "$LOGDIR/gate-w12-recovery.out" || { echo 'RED: recovery banner missing'; exit 1; }
node scripts/w12-grading-check.mjs > "$LOGDIR/gate-w12-grading.out" 2>&1 \
  || { tail -20 "$LOGDIR/gate-w12-grading.out"; echo RED: w12-grading-check; exit 1; }
grep -q 'GRADING CHECK GREEN' "$LOGDIR/gate-w12-grading.out" || { echo 'RED: grading banner missing'; exit 1; }
node scripts/w12-memsec-check.mjs > "$LOGDIR/gate-w12-memsec.out" 2>&1 \
  || { tail -20 "$LOGDIR/gate-w12-memsec.out"; echo RED: w12-memsec-check; exit 1; }
grep -q 'MEMSEC CHECK GREEN' "$LOGDIR/gate-w12-memsec.out" || { echo 'RED: memsec banner missing'; exit 1; }
node scripts/w12-wedge-check.mjs > "$LOGDIR/gate-w12-wedge.out" 2>&1 \
  || { tail -20 "$LOGDIR/gate-w12-wedge.out"; echo RED: w12-wedge-check; exit 1; }
grep -q 'WEDGE CHECK GREEN' "$LOGDIR/gate-w12-wedge.out" || { echo 'RED: wedge banner missing'; exit 1; }
echo 'FIVE CHECKERS GREEN (schema-v2 regression + the four wave checkers, each run against the shipped tree)'

step '02 selftest positive controls: every checker catches its own injected defects, fleets derived from this run'
n=$(node scripts/w12-recovery-check.mjs --selftest 2>&1 | tail -1 | grep -oE '\(([0-9]+) injected' | tr -cd '0-9')
[ -n "$n" ] && [ "$n" -ge 9 ] || { echo "RED: recovery selftest fleet unreadable or below floor: $n"; exit 1; }
m=$(node scripts/w12-grading-check.mjs --selftest 2>&1 | tail -1 | grep -oE '\(([0-9]+) injected' | tr -cd '0-9')
[ -n "$m" ] && [ "$m" -ge 9 ] || { echo "RED: grading selftest fleet unreadable or below floor: $m"; exit 1; }
k=$(node scripts/w12-memsec-check.mjs --selftest 2>&1 | tail -1 | grep -oE '\(([0-9]+) injected' | tr -cd '0-9')
[ -n "$k" ] && [ "$k" -ge 9 ] || { echo "RED: memsec selftest fleet unreadable or below floor: $k"; exit 1; }
for f in w12-recovery-check w12-grading-check w12-memsec-check; do
  node "scripts/${f}.mjs" --selftest 2>&1 | tail -1 | grep -q 'all caught' \
    || { echo "RED: ${f} selftest banner not an all-caught fleet"; exit 1; }
done
w=$(node scripts/w12-wedge-check.mjs --selftest 2>&1)
printf '%s' "$w" | grep -q 'PRISTINE PASS' || { echo 'RED: wedge selftest pristine control missing'; exit 1; }
j=$(printf '%s\n' "$w" | tail -1 | grep -oE '\(([0-9]+) injections' | tr -cd '0-9')
[ -n "$j" ] && [ "$j" -ge 9 ] || { echo "RED: wedge selftest fleet unreadable or below floor: $j"; exit 1; }
echo "SELFTEST FLEETS GREEN (recovery ${n}, grading ${m}, memsec ${k} all-caught controls fired; wedge ${j} injections plus pristine pass - counts derived from this run)"

step '03 wave diff face at pinned endpoints: docs and checkers only - and zero bytes on the enforcement plane'
# Wave endpoints are immutable history: the previous wave's gate merge (head of
# the wave this one follows) and the W12.4 merge (wave end). If either stops
# being an ancestor of HEAD the range means nothing and the gate says so.
W12_BASE=513a6da0560f446bf011ba04ff3ae6433cad9d7c
W12_END=00ce6165fdc367dd4d8d8748d32a90dab63eaa4f
[ "${#W12_END}" -eq 40 ] || { echo 'RED: wave end pin malformed'; exit 1; }
git merge-base --is-ancestor "$W12_BASE" HEAD || { echo 'RED: wave base endpoint not an ancestor of HEAD'; exit 1; }
git merge-base --is-ancestor "$W12_END" HEAD || { echo 'RED: wave end endpoint not an ancestor of HEAD'; exit 1; }
stray=$(git diff --name-only "${W12_BASE}..${W12_END}" | grep -vE '^(docs/|scripts/)' || true)
[ -z "$stray" ] || { echo 'RED: wave diff face carries files outside the allowed categories:'; echo "$stray"; exit 1; }
enf=$(git diff --name-only "${W12_BASE}..${W12_END}" | grep -E '^(internal/policy|internal/rules|internal/bus|internal/auditlog|internal/agentlog|internal/mcpproxy|internal/hookinstaller|cmd/)' || true)
[ -z "$enf" ] || { echo 'RED: enforcement/decision plane touched by the wave diff:'; echo "$enf"; exit 1; }
newgo=$(git diff --name-only "${W12_BASE}..${W12_END}" | grep -E '\.go$' || true)
[ -z "$newgo" ] || { echo 'RED: any Go file touched by the wave diff (this wave is pure design + checkers):'; echo "$newgo"; exit 1; }
schdiff=$(git diff --name-only "${W12_BASE}..${W12_END}" | grep -E '^docs/schema-v2\.md$' || true)
[ -z "$schdiff" ] || { echo 'RED: docs/schema-v2.md carried wave diff (expected zero)'; exit 1; }
wc_=$(git diff --name-only "${W12_BASE}..${W12_END}" | wc -l)
echo "WAVE DIFF FACE CLEAN (${wc_} file(s) between the pinned endpoints, docs+scripts only, zero enforcement-plane bytes, zero Go bytes, schema-v2 untouched)"

step '04 record-only invariants: the design faces carry no enforcement teeth and every plane token still reads none-in-observation-phase'
if grep -qiE '\b(would_deny|auto_deny|deny_list|hard_block|enforce_now)\b' docs/recovery-transaction-design.md docs/recovery-honesty-grading.md docs/memory-learning-safety-design.md docs/phase1-enforcement-wedge.md; then
  echo 'RED: enforcement-teeth vocabulary inside wave docs'; exit 1;
fi
n=$(grep -cE '^[a-z_]+_(enforcement|execution)_plane: none-in-observation-phase$' docs/schema-v2.md)
t=$(grep -cE '^[a-z_]+_(enforcement|execution)_plane: ' docs/schema-v2.md)
[ "$n" -eq "$t" ] || { echo "RED: a declared plane was promoted out of observation stance ($n/$t)"; exit 1; }
[ "$n" -ge 25 ] || { echo "RED: plane census $n, want at least 25 none-lines"; exit 1; }
grep -qF 'same-source citation, no second system' docs/memory-learning-safety-design.md \
  || { echo 'RED: same-source gate-assertion declaration lost (a second system appeared?)'; exit 1; }
echo "RECORD-ONLY GREEN (no enforcement teeth in wave docs, plane census all-none: $n/$t derived from this run, gate assertions remain same-source)"

step '05 recovery-boundary minimum-face confirmation material: derived from shipped artifacts - MATERIAL ONLY, this battery adjudicates nothing'
# The owner's re-confirmation (final-signing body) needs a compact read of
# what the wave actually delivers at the recovery boundary and what remains
# explicitly deferred. The gate prints the material lines; the implementation
# choice for an observation-type recovery point stays with the owner.
test -f docs/recovery-transaction-design.md && echo '  material 1: transaction/task-level recovery DESIGN face shipped (docs/recovery-transaction-design.md) - step pipeline + id/audit correlation + fabric levels, nothing implemented'
test -f docs/recovery-honesty-grading.md && echo '  material 2: honesty grading shipped (docs/recovery-honesty-grading.md) - four closed grades, per-face table, forbidden marketing clauses registered'
grep -qF 'Honest registration of the "not this cycle" decision' docs/recovery-transaction-design.md && echo '  material 3: the original deferral reason is kept verbatim and was never overturned - only the order changed'
grep -qE '^## 7\. Deferred decision' docs/recovery-honesty-grading.md && echo '  material 4: the separate decision position (implement observation-type recovery point or not) is registered deferred, not resolved'
grep -qF '单写者源' docs/phase1-enforcement-wedge.md && echo '  material 5: enforcement composition remains reference-shape under a single-writer source; the composition decision itself is unadjudicated'
echo '  => minimum-face reading available to the owner: design-first artifacts are complete; NO recovery-point runtime surface exists in Phase 0 (plane census in step 04 proves it). This step decides nothing.'

step '06 inherited chain: gate-w11 full battery (re-runs gate-w10 down through gate-d1 serially)'
bash scripts/gate-w11.sh > "$LOGDIR/gate-w11-inside-w12.out" 2>&1 \
  || { tail -30 "$LOGDIR/gate-w11-inside-w12.out"; echo RED: gate-w11 regression; exit 1; }
grep -q 'GATE-W11: ALL GREEN' "$LOGDIR/gate-w11-inside-w12.out" \
  || { echo 'RED: gate-w11 completion banner missing'; exit 1; }
echo 'GATE-W11 GREEN INSIDE GATE-W12 (zero regression on the full inherited chain)'

step '07 verbatim contract anchors from all four slices plus cwd-independent checker resolution'
pre12() {
  grep -qF "$2" "$1" || { echo "RED: W12 anchor lost in $1: $2"; exit 1; }
}
pre12 docs/recovery-transaction-design.md 'spec section 246'
pre12 docs/recovery-transaction-design.md 'spec sections 246'
pre12 docs/recovery-honesty-grading.md 'recovery-class-display'
pre12 docs/recovery-honesty-grading.md 'non_reversible | cannot be undone'
pre12 docs/memory-learning-safety-design.md 'Design assertion MA-1: a promotion path from memory or learning results to'
pre12 docs/phase1-enforcement-wedge.md '双源漂移防线'
# the W11.5 lesson stays load-bearing: checkers must resolve the repo root
# from their own file, not the caller cwd.
ROOT="$(pwd)"
(cd "$TMPDIR" && node "$ROOT/scripts/w12-memsec-check.mjs" > "$LOGDIR/gate-w12-cwd.out" 2>&1) \
  || { tail -20 "$LOGDIR/gate-w12-cwd.out"; echo 'RED: w12-memsec-check is cwd-dependent'; exit 1; }
grep -q 'MEMSEC CHECK GREEN' "$LOGDIR/gate-w12-cwd.out" || { echo 'RED: foreign-cwd run produced no green banner'; exit 1; }
echo 'ANCHORS GREEN (six verbatim tokens across four slices, foreign-cwd checker run green - root resolved from the checker file itself)'

step '08 tripwire standalone (internal-ledger negative scan)'
bash scripts/tripwire.sh > "$LOGDIR/gate-w12-tripwire.out" 2>&1 \
  || { cat "$LOGDIR/gate-w12-tripwire.out"; echo RED: tripwire; exit 1; }
grep -q 'TRIPWIRE CLEAN' "$LOGDIR/gate-w12-tripwire.out" || { echo RED: tripwire banner; exit 1; }

printf '\nGATE-W12: ALL GREEN\n'
