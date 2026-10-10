#!/usr/bin/env bash
# gate-w15.sh - machine-assertion battery for the intent/hint close-out wave,
# extended with the North Star readings rail.
#
# First edition, parameter-changed from the gate-w14 skeleton (whole-file
# authoring discipline; shared scaffolding kept byte-shape: TMPDIR residue
# cleanup, out-of-repo log directory, PATH fallback, foreign-cwd control,
# tripwire closer). The wave shipped two slices: intent recorder stage two -
# the five-source corpus migration moving streams, labels, coverage map, the
# docs mirror, the thresholds ledger banner and the CLI fixture in lockstep
# so the normal rules-corpus stream carries a typed intent.record hint on
# disk with zero emission-path change - and the North Star v0 readings rail:
# a derive-only runner beside the death-line mother, its machine-written
# ledger row, and the REFUSED same-day guard with UNAVAILABLE non-zero
# honest-absence semantics.
#
# The wave diff face is docs, scripts, testdata and exactly two Go test
# files (the CLI fixture pin and the golden-freshness vocabulary inside
# internal/rules/eval_test.go). Unlike the previous wave, internal/ carries
# test-face bytes this time and the standing invariant pins that they are
# _test.go only: no runtime Go byte moved, no non-test internal byte moved,
# and docs/schema-v2.md carries zero diff.
#
# Census numbers are re-taken from shipped artifacts on every run, never
# copied from a planning document: the vocabulary counts, rule counts,
# selftest fleets, the corpus total across six independent sources and the
# ledger row reproduction are all derived at run time. The wave-end endpoints
# are immutable history asserted as ancestors. Every step runs against real
# repo artifacts and carries its own control. Any red exits non-zero.
set -euo pipefail
cd "$(dirname "$0")/.."

export TMPDIR="$(pwd)/.tmptest"
mkdir -p "$TMPDIR"
find "$TMPDIR" -mindepth 1 -maxdepth 1 -name 'go-build*' -exec rm -rf {} + 2>/dev/null || true
command -v go >/dev/null 2>&1 || export PATH="$HOME/.local/go/bin:$PATH"
LOGDIR=/home/node/gate-logs
mkdir -p "$LOGDIR"

step() { printf '\n=== %s ===\n' "$*"; }

step '00 preflight: wave artifacts repo-visible; closed-set censuses re-parsed from the shipped doc at run time'
for f in docs/task-primitive-design.md docs/distribution.md docs/evals-coverage-v0.md \
         scripts/task-primitive-check.mjs scripts/task-map.mjs scripts/releaseleg-check.mjs \
         scripts/coverage-map-check.mjs scripts/threshold-ledger-check.mjs scripts/build-dist.sh \
         scripts/deathline-check.mjs docs/death-line-ledger.csv docs/death-line-monitor.md \
         scripts/northstar-readings.mjs docs/north-star-ledger.csv \
         scripts/gate-d5.sh testdata/golden/coverage-map.json testdata/golden/danger.jsonl \
         testdata/golden/normal.jsonl testdata/golden/labels.json testdata/golden/thresholds.json; do
  test -f "$f" || { echo "RED: wave artifact missing: $f"; exit 1; }
done
test -f scripts/gate-w15.sh || { echo 'RED: this gate not on disk'; exit 1; }
test -f scripts/gate-w14.sh || { echo 'RED: inherited gate-w14 missing from disk'; exit 1; }
# The seven status tokens are the contract; the census is what the shipped
# doc line yields right now, and both must agree (loop-derived counts).
sv=$(grep -m1 '^task_status_vocabulary:' docs/task-primitive-design.md | sed 's/^task_status_vocabulary: //' | tr ',' '\n' | wc -l | tr -cd '0-9')
[ "$sv" -eq 7 ] || { echo "RED: doc status vocabulary census reads $sv, want 7"; exit 1; }
for t in open active completed failed abandoned blocked unknown; do
  grep -m1 '^task_status_vocabulary:' docs/task-primitive-design.md | grep -qw "$t" \
    || { echo "RED: status token missing from doc: $t"; exit 1; }
done
bv=$(grep -m1 '^boundary_signal_vocabulary:' docs/task-primitive-design.md | sed 's/^boundary_signal_vocabulary: //' | tr ',' '\n' | wc -l | tr -cd '0-9')
[ "$bv" -eq 4 ] || { echo "RED: doc boundary signal census reads $bv, want 4"; exit 1; }
mrc=0
for i in 1 2 3 4 5; do grep -q "MR-$i" docs/task-primitive-design.md || { echo "RED: mapping rule MR-$i missing"; exit 1; }; mrc=$((mrc + 1)); done
drc=0
for i in 1 2 3; do grep -q "DR-$i" docs/task-primitive-design.md || { echo "RED: denominator rule DR-$i missing"; exit 1; }; drc=$((drc + 1)); done
grep -qF 'denominator_rule: every task interval reaching any terminal state counts once in the denominator, including terminal unknown' docs/task-primitive-design.md \
  || { echo 'RED: denominator rule line lost'; exit 1; }
grep -q 'reserved, not emitted' docs/task-primitive-design.md \
  || { echo 'RED: reserved-not-emitted heading lost'; exit 1; }
head -1 docs/north-star-ledger.csv | grep -q '^date,numerator,denominator,printed_ratio,' \
  || { echo 'RED: north star ledger header malformed'; exit 1; }
echo "PREFLIGHT GREEN (doc re-parse: status $sv, boundary $bv, mapping $mrc, denominator $drc; artifacts visible; ledger header in shape)"

step '01 checker full runs: schema-v2 regression, the wave checkers, and the mapper over the shipped golden streams'
node scripts/schema-v2-check.mjs > "$LOGDIR/gate-w15-v2check.out" 2>&1 \
  || { tail -40 "$LOGDIR/gate-w15-v2check.out"; echo RED: schema-v2-check; exit 1; }
grep -q 'SCHEMA-V2: ALL GREEN' "$LOGDIR/gate-w15-v2check.out" || { echo 'RED: v2-check banner missing'; exit 1; }
node scripts/task-primitive-check.mjs > "$LOGDIR/gate-w15-primitive.out" 2>&1 \
  || { tail -20 "$LOGDIR/gate-w15-primitive.out"; echo RED: task-primitive-check; exit 1; }
grep -q 'TASK-PRIMITIVE CHECK GREEN' "$LOGDIR/gate-w15-primitive.out" || { echo 'RED: primitive banner missing'; exit 1; }
node scripts/task-map.mjs testdata/golden/normal.jsonl testdata/golden/danger.jsonl > "$LOGDIR/gate-w15-mapper.out" 2>&1 \
  || { tail -30 "$LOGDIR/gate-w15-mapper.out"; echo RED: task-map conservation; exit 1; }
grep -q '"conservation": "GREEN"' "$LOGDIR/gate-w15-mapper.out" || { echo 'RED: mapper conservation banner missing'; exit 1; }
grep -q '"emitted": false' "$LOGDIR/gate-w15-mapper.out" || { echo 'RED: mapper no longer declares zero emission'; exit 1; }
node scripts/task-map.mjs testdata/golden/normal.jsonl testdata/golden/danger.jsonl --evals testdata/golden/labels.json > "$LOGDIR/gate-w15-join.out" 2>&1 \
  || { tail -30 "$LOGDIR/gate-w15-join.out"; echo RED: task-map eval-join conservation; exit 1; }
grep -q '"conservation": "GREEN"' "$LOGDIR/gate-w15-join.out" || { echo 'RED: join-form conservation banner missing'; exit 1; }
grep -q '"eval_coverage"' "$LOGDIR/gate-w15-join.out" || { echo 'RED: join form no longer reports eval coverage'; exit 1; }
grep -q '"emitted": false' "$LOGDIR/gate-w15-join.out" || { echo 'RED: join form declares emission'; exit 1; }
node scripts/coverage-map-check.mjs > "$LOGDIR/gate-w15-map.out" 2>&1 \
  || { tail -20 "$LOGDIR/gate-w15-map.out"; echo RED: coverage-map-check; exit 1; }
grep -q 'MAP OK' "$LOGDIR/gate-w15-map.out" || { echo 'RED: map banner missing'; exit 1; }
go test ./internal/rules/ -run TestGoldenEvals -v -count=1 -p 1 -parallel 1 > "$LOGDIR/gate-w15-eval.out" 2>&1 \
  || { tail -30 "$LOGDIR/gate-w15-eval.out"; echo RED: golden evals runner; exit 1; }
grep -q 'EVAL SUMMARY' "$LOGDIR/gate-w15-eval.out" || { echo 'RED: no EVAL SUMMARY in runner output'; exit 1; }
node scripts/threshold-ledger-check.mjs "$LOGDIR/gate-w15-eval.out" > "$LOGDIR/gate-w15-ledger.out" 2>&1 \
  || { tail -20 "$LOGDIR/gate-w15-ledger.out"; echo RED: threshold-ledger-check; exit 1; }
grep -q 'LEDGER OK' "$LOGDIR/gate-w15-ledger.out" || { echo 'RED: ledger banner missing'; exit 1; }
node scripts/releaseleg-check.mjs > "$LOGDIR/gate-w15-releaseleg.out" 2>&1 \
  || { tail -30 "$LOGDIR/gate-w15-releaseleg.out"; echo RED: releaseleg-check; exit 1; }
grep -q 'A1-A6+A9 GREEN' "$LOGDIR/gate-w15-releaseleg.out" || { echo 'RED: releaseleg A9 cross-pin not in the green set'; exit 1; }
! grep -q '^RED' "$LOGDIR/gate-w15-releaseleg.out" || { echo 'RED: releaseleg surface carries RED rows'; exit 1; }
echo 'EIGHT RUNNERS GREEN (schema-v2 regression, primitive checker, mapper single-source and join forms over both shipped streams, coverage map, threshold ledger, release-leg checker with A9 in force)'

step '02 selftest positive controls: every wave checker catches its own injected defects, fleets derived from this run'
pv=$(node scripts/task-primitive-check.mjs --selftest 2>&1 | tail -1)
printf '%s' "$pv" | grep -q 'SELFTEST GREEN' || { echo "RED: primitive selftest not green: $pv"; exit 1; }
pn=$(printf '%s' "$pv" | grep -oE '[0-9]+/[0-9]+' | head -1)
pa=${pn%/*}; pb=${pn#*/}
[ "$pa" = "$pb" ] && [ "$pa" -ge 12 ] || { echo "RED: primitive selftest fleet not fully caught or below floor: $pn"; exit 1; }
mv2=$(node scripts/task-map.mjs --selftest 2>&1 | tail -1)
printf '%s' "$mv2" | grep -q 'SELFTEST GREEN' || { echo "RED: mapper selftest not green: $mv2"; exit 1; }
printf '%s' "$mv2" | grep -q 'missed: none' || { echo 'RED: mapper selftest reports missed injections'; exit 1; }
mn2=$(printf '%s' "$mv2" | grep -oE '[0-9]+/[0-9]+' | head -1)
ma=${mn2%/*}; mb=${mn2#*/}
[ "$ma" = "$mb" ] && [ "$ma" -ge 6 ] || { echo "RED: mapper selftest fleet below floor: $mn2"; exit 1; }
rv=$(node scripts/releaseleg-check.mjs --selftest 2>&1 | tail -1)
printf '%s' "$rv" | grep -q 'SELFTEST OK' || { echo "RED: releaseleg selftest banner: $rv"; exit 1; }
rb=$(printf '%s' "$rv" | sed -n 's/.*+ \([0-9][0-9]*\) injected.*/\1/p')
[ -n "$rb" ] && [ "$rb" -ge 20 ] || { echo "RED: releaseleg injected fleet unreadable or below floor: $rb"; exit 1; }
m1=$(node scripts/coverage-map-check.mjs --selftest 2>&1 | grep -c 'mutation rejected' || true)
[ "$m1" -ge 5 ] || { echo "RED: coverage-map selftest caught $m1 mutations, want at least 5"; exit 1; }
m2=$(node scripts/threshold-ledger-check.mjs --selftest "$LOGDIR/gate-w15-eval.out" 2>&1 | grep -c 'tamper rejected' || true)
[ "$m2" -ge 3 ] || { echo "RED: ledger selftest caught $m2 tampers, want at least 3"; exit 1; }
nv=$(node scripts/northstar-readings.mjs --selftest 2>&1 | tail -1)
printf '%s' "$nv" | grep -q 'NORTHSTAR READINGS SELFTEST OK' || { echo "RED: northstar selftest banner: $nv"; exit 1; }
nc=$(printf '%s' "$nv" | sed -n 's/.*(\([0-9][0-9]*\) controls.*/\1/p')
[ -n "$nc" ] && [ "$nc" -ge 15 ] || { echo "RED: northstar selftest controls unreadable or below floor: $nv"; exit 1; }
echo "SELFTEST FLEETS GREEN (primitive $pn, mapper $mn2 with none missed, release-leg ${rb} injected catches, map mutations $m1, ledger tampers $m2, northstar $nc controls - all counted from this run)"

step '03 wave diff face at pinned endpoints plus registration reconciliation: registered merges are ancestors and their artifacts are on disk'
W15_BASE=92e523f9796763cf2077758fc122ece56dfa84d5
W15_END=48576e256ec87baae258e42617bc8582e9431d86
[ "${#W15_BASE}" -eq 40 ] && [ "${#W15_END}" -eq 40 ] || { echo 'RED: wave endpoint pins malformed'; exit 1; }
git merge-base --is-ancestor "$W15_BASE" HEAD || { echo 'RED: wave base endpoint not an ancestor of HEAD'; exit 1; }
git merge-base --is-ancestor "$W15_END" HEAD || { echo 'RED: wave end endpoint not an ancestor of HEAD'; exit 1; }
# Registration reconciliation: the ledger rows for this wave register exactly
# two merged PRs; each registered merge sha must be an ancestor and its
# registered headline artifact must be verifiable on disk right now.
stray=$(git diff --name-only "${W15_BASE}..${W15_END}" | grep -vE '^(docs/|scripts/|testdata/|cmd/[A-Za-z0-9._-]+/[A-Za-z0-9._-]+_test\.go$|internal/[A-Za-z0-9._-]+/[A-Za-z0-9._-]+_test\.go$)' || true)
[ -z "$stray" ] || { echo 'RED: wave diff face carries files outside the allowed categories:'; echo "$stray"; exit 1; }
rtgo=$(git diff --name-only "${W15_BASE}..${W15_END}" | grep -E '\.go$' | grep -v '_test\.go$' || true)
[ -z "$rtgo" ] || { echo 'RED: runtime Go touched by the wave diff:'; echo "$rtgo"; exit 1; }
inten=$(git diff --name-only "${W15_BASE}..${W15_END}" | grep -E '^internal/' | grep -v '_test\.go$' || true)
[ -z "$inten" ] || { echo 'RED: non-test internal bytes touched by the wave diff:'; echo "$inten"; exit 1; }
schdiff=$(git diff --name-only "${W15_BASE}..${W15_END}" | grep -E '^docs/schema-v2\.md$' || true)
[ -z "$schdiff" ] || { echo 'RED: docs/schema-v2.md carried wave diff (expected zero)'; exit 1; }
mrg=$(git log --format='%H %s' "${W15_BASE}..${W15_END}" | grep -c 'Merge pull request' || true)
[ "$mrg" -eq 2 ] || { echo "RED: registered wave carries $mrg merge commits, want the two registered PR merges"; exit 1; }
for sha in 6824277cc9cf39dab4f21b4013309d0e807b29a1 48576e256ec87baae258e42617bc8582e9431d86; do
  git merge-base --is-ancestor "$sha" HEAD || { echo "RED: registered merge sha not an ancestor: $sha"; exit 1; }
done
grep -q '"type":"intent.record"' testdata/golden/normal.jsonl \
  || { echo 'RED: W1 registered artifact (typed intent.record hint line) not on disk'; exit 1; }
test "$(wc -l < docs/north-star-ledger.csv | tr -cd '0-9')" -ge 2 \
  || { echo 'RED: W4 registered artifact (machine-written ledger row) missing'; exit 1; }
wc_files=$(git diff --name-only "${W15_BASE}..${W15_END}" | wc -l | tr -cd '0-9')
echo "WAVE DIFF FACE CLEAN + REGISTERED MERGES RECONCILED (${wc_files} file(s) between the pinned endpoints, $mrg registered merges both ancestors, docs+scripts+testdata+two Go test files only, zero runtime Go, zero non-test internal bytes, schema-v2 untouched, both registered artifacts on disk)"

step '04 corpus-count reverse-lookup across every copy, docs mirror and Go test face included: six sources re-taken at run time'
dn=$(wc -l < testdata/golden/danger.jsonl | tr -cd '0-9')
nm=$(wc -l < testdata/golden/normal.jsonl | tr -cd '0-9')
tot=$((dn + nm))
lb=$(node -e 'const fs=require("fs");const j=JSON.parse(fs.readFileSync("testdata/golden/labels.json","utf8"));console.log(Array.isArray(j)?j.length:Object.keys(j).length)')
mt=$(node -e 'const fs=require("fs");const c=JSON.parse(fs.readFileSync("testdata/golden/coverage-map.json","utf8")).corpus;console.log(c.total,c.normal,c.danger)')
read -r mt_n mt_norm mt_dang <<< "$mt"
led=$(sed -n 's/.*corpus \([0-9][0-9]*\)+\([0-9][0-9]*\)=\([0-9][0-9]*\) re-count.*/\3/p' "$LOGDIR/gate-w15-ledger.out")
gofix=$(grep -oE 'Cases != [0-9]+' cmd/agent-collector/replay_cli_test.go | grep -oE '[0-9]+' | head -1)
docm=$(grep -m1 -- '+ `danger.jsonl`' docs/evals-coverage-v0.md | grep -oE '[0-9]+' | tr '\n' ' ')
read -r dm_norm dm_dang dm_tot <<< "$docm"
[ "$mt_norm" = "$nm" ] || { echo "RED: coverage map normal count $mt_norm != stream $nm"; exit 1; }
[ "$mt_dang" = "$dn" ] || { echo "RED: coverage map danger count $mt_dang != stream $dn"; exit 1; }
[ "$mt_n" = "$tot" ] || { echo "RED: coverage map total $mt_n != derived total $tot"; exit 1; }
[ "$lb" = "$tot" ] || { echo "RED: label ledger length $lb != derived total $tot"; exit 1; }
[ "$led" = "$tot" ] || { echo "RED: ledger banner total $led != derived total $tot"; exit 1; }
[ -n "$gofix" ] || { echo 'RED: Go CLI fixture total unreadable'; exit 1; }
[ "$gofix" = "$tot" ] || { echo "RED: Go CLI fixture pins $gofix != derived total $tot (stale copy)"; exit 1; }
[ "$dm_norm" = "$nm" ] || { echo "RED: docs mirror normal count $dm_norm != stream $nm"; exit 1; }
[ "$dm_dang" = "$dn" ] || { echo "RED: docs mirror danger count $dm_dang != stream $dn"; exit 1; }
[ "$dm_tot" = "$tot" ] || { echo "RED: docs mirror total $dm_tot != derived total $tot"; exit 1; }
echo "CORPUS REVERSE-LOOKUP LOCKED ($tot events agree across six run-time sources: streams ${nm}+${dn}, labels $lb, coverage map $mt_n, ledger banner $led, Go CLI fixture $gofix, docs mirror $dm_norm+$dm_dang)"
node scripts/deathline-check.mjs > "$LOGDIR/gate-w15-deathline.out" 2>&1 \
  || { tail -20 "$LOGDIR/gate-w15-deathline.out"; echo RED: deathline-check; exit 1; }
grep -q 'DEATHLINE CHECK GREEN' "$LOGDIR/gate-w15-deathline.out" || { echo 'RED: deathline banner missing'; exit 1; }
lastdate=$(tail -1 docs/death-line-ledger.csv | cut -d, -f1)
grep -qE "^${lastdate}," docs/death-line-ledger.csv || { echo 'RED: the latest rail reading row is not on the ledger'; exit 1; }
echo "DEATHLINE RAIL GREEN (check green + latest appended row $lastdate present)"

step '05 typed intent triangle and record-only invariants: schema constant, golden-freshness vocabulary, on-disk hint line, zero enforcement teeth'
grep -q 'TypeIntentRecord EventType = "intent.record"' internal/schema/event.go \
  || { echo 'RED: schema no longer defines the typed intent.record event'; exit 1; }
grep -q 'schema.TypeIntentRecord: true' internal/rules/eval_test.go \
  || { echo 'RED: golden-freshness closed vocabulary dropped the intent.record input shape'; exit 1; }
icn=$(grep -c '"type":"intent.record"' testdata/golden/normal.jsonl)
[ "$icn" -ge 1 ] || { echo 'RED: normal stream carries no typed intent.record hint line'; exit 1; }
idn=$(grep -c '"type":"intent.record"' testdata/golden/danger.jsonl || true)
[ "$idn" -eq 0 ] || { echo "RED: danger stream carries $idn intent.record lines; it stays hint-free by construction"; exit 1; }
if grep -qiE '\b(auto_deny|deny_list|hard_block|enforce_now)\b' docs/task-primitive-design.md docs/distribution.md docs/evals-coverage-v0.md docs/north-star-ledger.csv; then
  echo 'RED: enforcement-teeth vocabulary inside wave docs'; exit 1;
fi
n=$(grep -cE '^[a-z_]+_(enforcement|execution)_plane: none-in-observation-phase$' docs/schema-v2.md)
t=$(grep -cE '^[a-z_]+_(enforcement|execution)_plane: ' docs/schema-v2.md)
[ "$n" -eq "$t" ] || { echo "RED: a declared plane was promoted out of observation stance ($n/$t)"; exit 1; }
[ "$n" -ge 25 ] || { echo "RED: plane census $n, want at least 25 none-lines"; exit 1; }
grep -qF '{allow, would_block}' docs/task-primitive-design.md || { echo 'RED: Phase 0 decision closed-set anchor lost'; exit 1; }
grep -q 'task-primitive-design.md' scripts/task-map.mjs || { echo 'RED: mapper lost its single-source vocabulary citation'; exit 1; }
grep -q 'scripts/task-map.mjs' scripts/task-primitive-check.mjs || { echo 'RED: checker lost its mapper reverse-lookup section'; exit 1; }
grep -q -- '--release-branch=A' scripts/build-dist.sh || { echo 'RED: builder signed-assembly branch flag lost'; exit 1; }
grep -q 'BRANCH-B-INTEL-SKIP' scripts/build-dist.sh || { echo 'RED: builder honest-skip marker lost'; exit 1; }
grep -q -- '--evals' scripts/task-primitive-check.mjs || { echo 'RED: checker lost the eval-join reverse-lookup'; exit 1; }
if grep -qE '"gn[0-9]+"' scripts/task-map.mjs; then echo 'RED: a second corpus copy embedded itself into the mapper'; exit 1; fi
echo "INTENT TRIANGLE + RECORD-ONLY GREEN (schema constant, eval_test vocabulary and $icn on-disk hint line(s) agree, danger hint-free at $idn, no teeth in wave docs, plane census all-none $n/$t re-derived, closed set + single-source links + builder branches + join reverse-lookup in force, zero embedded corpus copy)"

step '06 north star rail: the machine-written ledger row re-derives byte-identical from shipped data at a foreign cwd - earned, never pasted'
ROOT="$(pwd)"
ldate=$(sed -n '$p' docs/north-star-ledger.csv | cut -d, -f1)
lrow=$(sed -n '$p' docs/north-star-ledger.csv)
[ -n "$ldate" ] && [ -n "$lrow" ] || { echo 'RED: no ledger row to re-derive'; exit 1; }
(cd "$TMPDIR" && node "$ROOT/scripts/northstar-readings.mjs" --dry-run --date "$ldate" > "$LOGDIR/gate-w15-northstar.out" 2>&1) \
  || { tail -20 "$LOGDIR/gate-w15-northstar.out"; echo 'RED: northstar rail dry-run is cwd-dependent or fails to derive'; exit 1; }
drun=$(tail -1 "$LOGDIR/gate-w15-northstar.out")
[ "$drun" = "$lrow" ] || { echo "RED: re-derived row drifts from the shipped ledger row:"; echo "  ledger:   $lrow"; echo "  re-derived: $drun"; exit 1; }
echo "$drun" | grep -qE ',(DERIVED|NA)$|,DERIVED,|,UNAVAILABLE' \
  || { echo 'RED: re-derived row carries no honest ratio_status token'; exit 1; }
grep -c 'writeFileSync' scripts/northstar-readings.mjs | grep -q '^[12]$' \
  || { echo 'RED: northstar writer surface grew beyond the ledger path (mother-script shape)'; exit 1; }
echo "NORTHSTAR RAIL GREEN (last ledger row $ldate re-derived byte-identical at foreign cwd: $drun - the numerator is earned from on-disk typed intent lines after the stage-two migration, zero emission)"

step '07 inherited chain: gate-w14 full battery (which itself re-runs gate-w13 down through gate-d1 serially, gate-d7 Go suite with the expanded corpus fixture and the gate-d5 release assembly battery included)'
bash scripts/gate-w14.sh > "$LOGDIR/gate-w14-inside-w15.out" 2>&1 \
  || { tail -30 "$LOGDIR/gate-w14-inside-w15.out"; echo RED: gate-w14 regression; exit 1; }
grep -q 'GATE-W14: ALL GREEN' "$LOGDIR/gate-w14-inside-w15.out" \
  || { echo 'RED: gate-w14 completion banner missing'; exit 1; }
echo 'GATE-W14 GREEN INSIDE GATE-W15 (zero regression on the full inherited chain down through gate-d1, shipped release assembly exercised end-to-end inside gate-d5)'

step '08 verbatim contract anchors from all wave slices plus tripwire standalone (internal-ledger negative scan)'
pre15() {
  grep -qF "$2" "$1" || { echo "RED: wave anchor lost in $1: $2"; exit 1; }
}
pre15 docs/task-primitive-design.md 'task_status_vocabulary: open, active, completed, failed, abandoned, blocked, unknown'
pre15 docs/task-primitive-design.md 'boundary_signal_vocabulary: session_start_edge, turn_stop_sequence, goal_hint, lease_expiry_edge'
pre15 docs/distribution.md 'Release assembly (Intel signing branches)'
pre15 scripts/build-dist.sh 'BRANCH-B-INTEL-SKIP: darwin/amd64 (Intel) is not part of this release set.'
pre15 testdata/golden/coverage-map.json '"labels"'
pre15 docs/task-primitive-design.md 'eval_assertion'
pre15 scripts/task-map.mjs "iv.evidence = laterTurnEvidence ? 'later_turn' : evalEvidence ? 'eval_assertion' : 'none'"
pre15 docs/death-line-monitor.md 'append'
pre15 docs/north-star-ledger.csv 'evidence_later_turn,evidence_eval_assertion,ratio_status'
pre15 scripts/northstar-readings.mjs 'REFUSED:'
pre15 internal/rules/eval_test.go 'intent.record line joins the benign vocabulary'
bash scripts/tripwire.sh > "$LOGDIR/gate-w15-tripwire.out" 2>&1 \
  || { cat "$LOGDIR/gate-w15-tripwire.out"; echo RED: tripwire; exit 1; }
grep -q 'TRIPWIRE CLEAN' "$LOGDIR/gate-w15-tripwire.out" || { echo RED: tripwire banner; exit 1; }
twcount=$(grep -oE '[0-9]+ (entries|scanned|items)' "$LOGDIR/gate-w15-tripwire.out" | head -1 || true)
echo "ANCHORS GREEN (eleven verbatim tokens across the wave plus the four inherited ones; tripwire CLEAN ${twcount:-count not printed})"

printf '\nGATE-W15: ALL GREEN\n'
