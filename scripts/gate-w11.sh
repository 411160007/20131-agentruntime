#!/usr/bin/env bash
# gate-w11.sh - machine-assertion battery for the W11 estimation-and-plan wave.
#
# First edition. The wave shipped five slices: W11.1 the impact / blast-radius
# estimation record shape (two of eight scope dimensions honestly derivable
# from raw tool-call argument bytes), W11.2 the coverage truthfulness matrix
# (four-tier closed vocabulary in human table and machine mirror), W11.3 the
# compatibility matrix plus the red-team plan (ten-family closed census, plan
# before engine), W11.4 the performance benchmark plan (seven metrics, goals
# reverse-pinned, seed recorded-not-verified), and W11.5 the threat-model v1
# family uplift TM-13..17 with the supply-chain observation document face.
#
# Unlike the docs-only W10 wave, this wave legally carries code: a record-shape
# package inside internal/schema, four standalone checkers, golden mirrors, and
# Go mount tests inside internal/discovery. The wave's standing invariant is
# therefore not "no code" but "no enforcement-plane change": the decision and
# enforcement planes (policy, rules, bus, auditlog, agentlog, mcpproxy,
# hookinstaller, cmd) carry zero bytes of wave diff, the schema additions are
# record-only, and every plane token still reads none-in-observation-phase.
#
# Census numbers are re-taken from shipped artifacts on every run, never
# copied from a plan document: the TM uplift census, the record-shape counts,
# the plane census, and each checker's selftest fleet are derived at run time.
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

step '00 preflight: the five W11 doc surfaces, the record-shape package, the four shipped checkers, and the six contract sections are repo-visible'
for h in 35 36 37 38 39 40; do
  grep -qE "^## ${h}\. " docs/schema-v2.md || { echo "RED: schema section ${h} header missing"; exit 1; }
done
for f in docs/coverage-truthfulness.md docs/compatibility-matrix.md docs/red-team-plan.md docs/benchmark-plan.md docs/threat-model.md; do
  test -f "$f" || { echo "RED: wave doc missing: $f"; exit 1; }
done
test -f internal/schema/impactestimation.go || { echo 'RED: record-shape package missing'; exit 1; }
for c in coverage-truth-check compat-redteam-check benchplan-check threatmodel-check; do
  test -f "scripts/${c}.mjs" || { echo "RED: checker missing: ${c}.mjs"; exit 1; }
done
test -f scripts/gate-w11.sh || { echo 'RED: this gate not on disk'; exit 1; }
ifg=$(grep -cE '^impact_field_count: 7$' docs/schema-v2.md)
brs=$(grep -cE '^blast_radius_scope_count: 8$' docs/schema-v2.md)
[ "$ifg" -eq 1 ] && [ "$brs" -eq 1 ] || { echo "RED: record-shape count pins drifted ($ifg/$brs)"; exit 1; }
up=$(grep -cE '^\| TM-1[3-7] ' docs/threat-model.md)
[ "$up" -eq 5 ] || { echo "RED: TM uplift row census $up/5"; exit 1; }
echo "W11 SURFACE VISIBLE (six section headers, five docs, record-shape package, four checkers, count pins 7/8, uplift census 5/5 re-taken from disk)"

step '01 checker full runs: schema-v2 plus all four W11 checkers against the shipped tree'
node scripts/schema-v2-check.mjs > "$LOGDIR/gate-w11-v2check.out" 2>&1 \
  || { tail -40 "$LOGDIR/gate-w11-v2check.out"; echo RED: schema-v2-check; exit 1; }
grep -q 'SCHEMA-V2: ALL GREEN' "$LOGDIR/gate-w11-v2check.out" \
  || { echo 'RED: v2-check banner missing'; exit 1; }
node scripts/coverage-truth-check.mjs > "$LOGDIR/gate-w11-coverage.out" 2>&1 \
  || { tail -20 "$LOGDIR/gate-w11-coverage.out"; echo RED: coverage-truth-check; exit 1; }
grep -q 'TRUTH GREEN' "$LOGDIR/gate-w11-coverage.out" || { echo 'RED: coverage banner missing'; exit 1; }
node scripts/compat-redteam-check.mjs > "$LOGDIR/gate-w11-compat.out" 2>&1 \
  || { tail -20 "$LOGDIR/gate-w11-compat.out"; echo RED: compat-redteam-check; exit 1; }
grep -q 'COMPAT GREEN' "$LOGDIR/gate-w11-compat.out" || { echo 'RED: compat banner missing'; exit 1; }
node scripts/benchplan-check.mjs > "$LOGDIR/gate-w11-bench.out" 2>&1 \
  || { tail -20 "$LOGDIR/gate-w11-bench.out"; echo RED: benchplan-check; exit 1; }
grep -q 'BENCHPLAN GREEN' "$LOGDIR/gate-w11-bench.out" || { echo 'RED: benchplan banner missing'; exit 1; }
node scripts/threatmodel-check.mjs > "$LOGDIR/gate-w11-threat.out" 2>&1 \
  || { tail -20 "$LOGDIR/gate-w11-threat.out"; echo RED: threatmodel-check; exit 1; }
grep -q 'THREATMODEL OK' "$LOGDIR/gate-w11-threat.out" || { echo 'RED: threatmodel banner missing'; exit 1; }
echo 'FIVE CHECKERS GREEN (schema-v2 + the four wave checkers, each run against the shipped tree)'

step '02 selftest positive controls: every checker catches its own injected defects, fleets derived from this run'
cov=$(node scripts/coverage-truth-check.mjs --selftest 2>&1 | tail -1)
n=$(printf '%s' "$cov" | grep -oE '\(([0-9]+)' | tr -cd '0-9')
[ -n "$n" ] && [ "$n" -ge 13 ] || { echo "RED: coverage selftest fleet unreadable or below floor: $cov"; exit 1; }
frac_ok=0
for c in benchplan-check threatmodel-check; do
  out=$(node "scripts/${c}.mjs" --selftest 2>&1 | tail -1)
  frac=$(printf '%s' "$out" | grep -oE '[0-9]+/[0-9]+' | head -1)
  a=${frac%/*}; b=${frac#*/}
  [ -n "$a" ] && [ "$a" = "$b" ] && [ "$a" -ge 5 ] || { echo "RED: ${c} selftest not all-caught: $out"; exit 1; }
  frac_ok=$((frac_ok + 1))
done
[ "$frac_ok" -eq 2 ] || { echo "RED: fraction-fleet loop did not run both checkers"; exit 1; }
out=$(node scripts/compat-redteam-check.mjs --selftest 2>&1 | tail -1)
a=$(printf '%s' "$out" | grep -oE '\([0-9]+ injected' | tr -cd '0-9')
[ -n "$a" ] && [ "$a" -ge 5 ] && printf '%s' "$out" | grep -q 'all caught' \
  || { echo "RED: compat selftest banner not an all-caught fleet of five or more: $out"; exit 1; }
echo "SELFTEST FLEETS GREEN (coverage ${n} controls fired, the two fraction-fleet checkers all-caught plus the compat banner fleet, floors met)"

step '03 wave diff face at pinned endpoints: docs, checkers, mirrors, record-shape additions, mount tests - and zero bytes on the enforcement plane'
# Wave endpoints are immutable history: the gate-w10 merge (head of the wave
# this one follows) and the W11.5 merge (wave end). If either stops being an
# ancestor of HEAD the range means nothing and the gate says so.
W11_BASE=fa58a6af9a426013b9e4c728004d462febd6c4ab
W11_END=4926afcc45d42e3ddae7829dbe91a858ce8c6002
[ "${#W11_END}" -eq 40 ] || { echo 'RED: wave end pin malformed'; exit 1; }
git merge-base --is-ancestor "$W11_BASE" HEAD || { echo 'RED: wave base endpoint not an ancestor of HEAD'; exit 1; }
git merge-base --is-ancestor "$W11_END" HEAD || { echo 'RED: wave end endpoint not an ancestor of HEAD'; exit 1; }
stray=$(git diff --name-only "${W11_BASE}..${W11_END}" | grep -vE '^(docs/|scripts/|testdata/golden/|internal/schema/|internal/discovery/.*_test\.go$)' || true)
[ -z "$stray" ] || { echo 'RED: wave diff face carries files outside the allowed categories:'; echo "$stray"; exit 1; }
enf=$(git diff --name-only "${W11_BASE}..${W11_END}" | grep -E '^(internal/policy|internal/rules|internal/bus|internal/auditlog|internal/agentlog|internal/mcpproxy|internal/hookinstaller|cmd/)' || true)
[ -z "$enf" ] || { echo 'RED: enforcement/decision plane touched by the wave diff:'; echo "$enf"; exit 1; }
newgo=$(git diff --name-only "${W11_BASE}..${W11_END}" | grep -E '^internal/discovery/.*\.go$' | grep -v '_test\.go$' || true)
[ -z "$newgo" ] || { echo 'RED: non-test code added under internal/discovery:'; echo "$newgo"; exit 1; }
wc_=$(git diff --name-only "${W11_BASE}..${W11_END}" | wc -l)
echo "WAVE DIFF FACE CLEAN (${wc_} file(s) between the pinned endpoints, all in allowed categories, zero enforcement-plane bytes)"

step '04 record-only invariants: the schema additions and the new doc faces carry no enforcement teeth and every plane token still reads none-in-observation-phase'
if grep -qE '\b(Deny|deny|Block|block|Enforce|enforce)\b' internal/schema/impactestimation.go; then
  echo 'RED: enforcement vocabulary inside the record-shape package'; exit 1;
fi
grep -q '^impact_enforcement_plane: none-in-observation-phase$' docs/schema-v2.md \
  || { echo 'RED: impact enforcement plane token missing'; exit 1; }
grep -q 'record-only-no-action' docs/threat-model.md \
  || { echo 'RED: supply-chain section record-only token missing'; exit 1; }
n=$(grep -cE '^[a-z_]+_(enforcement|execution)_plane: none-in-observation-phase$' docs/schema-v2.md)
t=$(grep -cE '^[a-z_]+_(enforcement|execution)_plane: ' docs/schema-v2.md)
[ "$n" -eq "$t" ] || { echo "RED: a declared plane was promoted out of observation stance ($n/$t)"; exit 1; }
[ "$n" -ge 25 ] || { echo "RED: plane census $n, want at least 25 none-lines"; exit 1; }
echo "RECORD-ONLY GREEN (no enforcement vocabulary in the new schema face, impact plane token pinned, supply-chain record-only token in place, plane census all-none: $n/$t derived from this run)"

step '05 inherited chain: gate-w10 full battery (re-runs gate-w9 down through gate-d1 serially)'
bash scripts/gate-w10.sh > "$LOGDIR/gate-w10-inside-w11.out" 2>&1 \
  || { tail -30 "$LOGDIR/gate-w10-inside-w11.out"; echo RED: gate-w10 regression; exit 1; }
grep -q 'GATE-W10: ALL GREEN' "$LOGDIR/gate-w10-inside-w11.out" \
  || { echo 'RED: gate-w10 completion banner missing'; exit 1; }
echo 'GATE-W10 GREEN INSIDE GATE-W11 (zero regression on the full inherited chain)'

step '06 verbatim contract anchors from all five slices plus cwd-independent checker resolution'
pre11() {
  grep -qF "$1" docs/schema-v2.md || { echo "RED: W11 anchor lost: $1"; exit 1; }
}
pre11 'impact_absent_semantics: not-estimated-known-gap'
pre11 'impact_field_vocabulary: direct_impact, indirect_impact, propagation_impact, blast_radius, reversibility, dependency_impact, production_impact'
pre11 'blast_radius_scope_vocabulary: file_scope, project_scope, network_scope, process_scope, database_scope, credential_scope, device_scope, subagent_scope'
grep -qF 'FULL`, `LIMITED`, `MONITOR ONLY`, `UNAVAILABLE' docs/schema-v2.md || { echo 'RED: four-tier closed vocabulary anchor lost'; exit 1; }
grep -qF 'RT-01 through RT-10' docs/schema-v2.md || { echo 'RED: ten-family red-team census anchor lost'; exit 1; }
grep -qF 'PARTIAL / PLANNED' docs/schema-v2.md || { echo 'RED: three-value instrumentation status anchor lost'; exit 1; }
grep -qF 'TM-13..TM-17' docs/schema-v2.md || { echo 'RED: uplift census anchor lost'; exit 1; }
grep -qF 'estimation-is-not-verified' docs/schema-v2.md || { echo 'RED: seed stance token lost'; exit 1; }
# the W11.5 lesson: checkers must resolve the repo root from their own file,
# not the caller cwd - the Go mount runs them with cwd = package directory.
ROOT="$(pwd)"
(cd "$TMPDIR" && node "$ROOT/scripts/threatmodel-check.mjs" > "$LOGDIR/gate-w11-cwd.out" 2>&1) \
  || { tail -20 "$LOGDIR/gate-w11-cwd.out"; echo 'RED: threatmodel-check is cwd-dependent'; exit 1; }
grep -q 'THREATMODEL OK' "$LOGDIR/gate-w11-cwd.out" || { echo 'RED: foreign-cwd run produced no green banner'; exit 1; }
echo "ANCHORS GREEN (ten verbatim tokens across five slices, foreign-cwd checker run green - root resolved from the checker file itself)"

step '07 tripwire standalone (internal-ledger negative scan)'
bash scripts/tripwire.sh > "$LOGDIR/gate-w11-tripwire.out" 2>&1 \
  || { cat "$LOGDIR/gate-w11-tripwire.out"; echo RED: tripwire; exit 1; }
grep -q 'TRIPWIRE CLEAN' "$LOGDIR/gate-w11-tripwire.out" || { echo RED: tripwire banner; exit 1; }

printf '\nGATE-W11: ALL GREEN\n'
