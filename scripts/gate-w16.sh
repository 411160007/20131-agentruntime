#!/usr/bin/env bash
# gate-w16.sh - machine-assertion battery for the task record-form and
# documentation-register wave (four slices: W1 additive task record schema,
# W2 release/canary/rollback pre-plan, W3 PRD + UX spec register skeletons,
# W4 Agent-to-Agent abuse observation on the threat documentation face).
#
# First edition, parameter-changed from the gate-w15 skeleton (whole-file
# authoring discipline; shared scaffolding kept byte-shape: TMPDIR residue
# cleanup, out-of-repo log directory, PATH fallback, closed-set doc
# re-parsing at run time, foreign-cwd control, inherited chain, tripwire
# closer). Unlike the previous wave, this wave moves exactly one runtime Go
# file (internal/schema/event.go joins the shipped vocabulary with the
# additive task.record type and the task_id/task_status pair) plus two
# taskrecord fixtures and the doc-register trio; the standing pins below
# fence that shape: zero testdata/golden bytes, zero non-test internal bytes
# outside schema/event.go, zero runtime Go elsewhere.
#
# Census numbers are re-taken from shipped artifacts on every run, never
# copied from a planning document: the seven-token task status vocabulary is
# cross-checked between the design doc, the Go constant face, the validator
# closed set and the fixture streams; the PRD matrix cell count and the
# acceptance clause fleet are loop-derived from the shipped docs; the wild
# fixture rejection is a run-time positive control. The wave-end endpoints
# are immutable history asserted as ancestors. Any red exits non-zero.
set -euo pipefail
cd "$(dirname "$0")/.."

export TMPDIR="$(pwd)/.tmptest"
mkdir -p "$TMPDIR"
find "$TMPDIR" -mindepth 1 -maxdepth 1 -name 'go-build*' -exec rm -rf {} + 2>/dev/null || true
command -v go >/dev/null 2>&1 || export PATH="$HOME/.local/go/bin:$PATH"
LOGDIR=/home/node/gate-logs
mkdir -p "$LOGDIR"

step() { printf '\n=== %s ===\n' "$*"; }

step '00 preflight: wave artifacts repo-visible; task-status closed set cross-checked between doc, Go constants, validator and fixtures at run time'
for f in docs/release-plan-v0.md docs/prd-v0.md docs/ux-spec-v0.md docs/threat-model.md \
         docs/evals-coverage-v0.md docs/task-primitive-design.md docs/schema-v2.md \
         internal/schema/event.go internal/schema/taskrecord_test.go internal/schema/sync_test.go \
         internal/schema/apicontract_sync_test.go testdata/taskrecord/good.jsonl testdata/taskrecord/wild.jsonl \
         scripts/validate-jsonl.mjs scripts/task-primitive-check.mjs scripts/apicontract-check.mjs \
         scripts/schema-v2-check.mjs scripts/coverage-map-check.mjs scripts/threatmodel-check.mjs \
         scripts/gate-d5.sh testdata/golden/coverage-map.json; do
  test -f "$f" || { echo "RED: wave artifact missing: $f"; exit 1; }
done
test -f scripts/gate-w16.sh || { echo 'RED: this gate not on disk'; exit 1; }
test -f scripts/gate-w15.sh || { echo 'RED: inherited gate-w15 missing from disk'; exit 1; }
# The seven status tokens are the contract. Four independent faces must
# agree right now: the design doc line, the Go constant declarations, the
# validator closed set, and the vocabulary re-parsed token by token.
sv=$(grep -m1 '^task_status_vocabulary:' docs/task-primitive-design.md | sed 's/^task_status_vocabulary: //' | tr ',' '\n' | wc -l | tr -cd '0-9')
[ "$sv" -eq 7 ] || { echo "RED: doc status vocabulary census reads $sv, want 7"; exit 1; }
goc=$(grep -cE $'\tTask[A-Za-z]+ +TaskStatus = "' internal/schema/event.go)
[ "$goc" -eq 7 ] || { echo "RED: Go TaskStatus constant face declares $goc, want 7"; exit 1; }
vwc=$(sed -n '/^const TASK_STATUS/,/^\];/p' scripts/validate-jsonl.mjs | grep -oE "'[a-z_]+',?" | tr -d "'," | wc -l | tr -cd '0-9')
[ "$vwc" -eq 7 ] || { echo "RED: validator closed set declares $vwc, want 7"; exit 1; }
for t in open active completed failed abandoned blocked unknown; do
  grep -m1 '^task_status_vocabulary:' docs/task-primitive-design.md | grep -qw "$t" \
    || { echo "RED: status token missing from doc: $t"; exit 1; }
  grep -qE "TaskStatus = \"$t\"" internal/schema/event.go \
    || { echo "RED: status token missing from Go constants: $t"; exit 1; }
done
grep -q 'TypeTaskRecord EventType = "task.record"' internal/schema/event.go \
  || { echo 'RED: schema no longer declares the task.record event type'; exit 1; }
grep -q "'task.record'" scripts/validate-jsonl.mjs \
  || { echo 'RED: validator dropped the task.record type from its vocabulary'; exit 1; }
echo "PREFLIGHT GREEN (closed set 7/7/7 across doc, Go constants and validator, all seven tokens double-pinned doc↔Go, task.record type present on both faces; wave artifacts visible)"

step '01 checker full runs: schema-v2 regression, task primitive, api contract, coverage map, threat model, and the validator over the shipped fixtures and the legacy streams'
node scripts/schema-v2-check.mjs > "$LOGDIR/gate-w16-v2check.out" 2>&1 \
  || { tail -40 "$LOGDIR/gate-w16-v2check.out"; echo RED: schema-v2-check; exit 1; }
grep -q 'SCHEMA-V2: ALL GREEN' "$LOGDIR/gate-w16-v2check.out" || { echo 'RED: v2-check banner missing'; exit 1; }
node scripts/task-primitive-check.mjs > "$LOGDIR/gate-w16-primitive.out" 2>&1 \
  || { tail -20 "$LOGDIR/gate-w16-primitive.out"; echo RED: task-primitive-check; exit 1; }
grep -q 'TASK-PRIMITIVE CHECK GREEN' "$LOGDIR/gate-w16-primitive.out" || { echo 'RED: primitive banner missing'; exit 1; }
node scripts/apicontract-check.mjs > "$LOGDIR/gate-w16-apicontract.out" 2>&1 \
  || { tail -20 "$LOGDIR/gate-w16-apicontract.out"; echo RED: apicontract-check; exit 1; }
grep -q 'API-CONTRACT OK' "$LOGDIR/gate-w16-apicontract.out" || { echo 'RED: apicontract banner missing'; exit 1; }
node scripts/coverage-map-check.mjs > "$LOGDIR/gate-w16-map.out" 2>&1 \
  || { tail -20 "$LOGDIR/gate-w16-map.out"; echo RED: coverage-map-check; exit 1; }
grep -q 'MAP OK' "$LOGDIR/gate-w16-map.out" || { echo 'RED: map banner missing'; exit 1; }
node scripts/threatmodel-check.mjs > "$LOGDIR/gate-w16-threatmodel.out" 2>&1 \
  || { tail -20 "$LOGDIR/gate-w16-threatmodel.out"; echo RED: threatmodel-check; exit 1; }
grep -q 'THREATMODEL OK' "$LOGDIR/gate-w16-threatmodel.out" || { echo 'RED: threatmodel banner missing'; exit 1; }
node scripts/validate-jsonl.mjs testdata/taskrecord/good.jsonl > "$LOGDIR/gate-w16-good.out" 2>&1 \
  || { cat "$LOGDIR/gate-w16-good.out"; echo RED: validator rejects its own good fixture; exit 1; }
grep -q 'VALIDATE OK' "$LOGDIR/gate-w16-good.out" || { echo 'RED: good fixture banner missing'; exit 1; }
gn=$(wc -l < testdata/taskrecord/good.jsonl | tr -cd '0-9')
[ "$gn" -ge 3 ] || { echo "RED: good fixture carries $gn lines, want at least 3"; exit 1; }
for s in testdata/golden/normal.jsonl testdata/golden/danger.jsonl; do
  node scripts/validate-jsonl.mjs "$s" > "$LOGDIR/gate-w16-legacy.out" 2>&1 \
    || { tail -20 "$LOGDIR/gate-w16-legacy.out"; echo "RED: legacy stream no longer validates: $s"; exit 1; }
  grep -q 'VALIDATE OK' "$LOGDIR/gate-w16-legacy.out" || { echo "RED: legacy validation banner missing: $s"; exit 1; }
done
echo "VALIDATOR RUNS GREEN (good fixture $gn lines, both legacy golden streams validate unchanged - the additive pair breaks nothing)"

step '02 positive controls: the validator catches every wild value, every selftest fleet catches its own injected defects - counts derived from this run'
if node scripts/validate-jsonl.mjs testdata/taskrecord/wild.jsonl > "$LOGDIR/gate-w16-wild.out" 2>&1; then
  echo 'RED: wild fixture passed validation - the closed-set guard is asleep'; exit 1
fi
wn=$(wc -l < testdata/taskrecord/wild.jsonl | tr -cd '0-9')
wlines=$(grep -oE '^\s*line [0-9]+' "$LOGDIR/gate-w16-wild.out" | grep -oE '[0-9]+' | sort -u | wc -l | tr -cd '0-9')
[ "$wlines" -eq "$wn" ] || { echo "RED: wild fixture has $wn lines but the validator reported rejections for $wlines - every wild line must be caught"; exit 1; }
grep -q 'task_status=' "$LOGDIR/gate-w16-wild.out" || { echo 'RED: wild report no longer names the task_status violation face'; exit 1; }
grep -q 'task.record requires' "$LOGDIR/gate-w16-wild.out" || { echo 'RED: wild report no longer names the task.record requirement face'; exit 1; }
pv=$(node scripts/task-primitive-check.mjs --selftest 2>&1 | tail -1)
printf '%s' "$pv" | grep -q 'SELFTEST GREEN' || { echo "RED: primitive selftest not green: $pv"; exit 1; }
pn=$(printf '%s' "$pv" | grep -oE '[0-9]+/[0-9]+' | head -1)
pa=${pn%/*}; pb=${pn#*/}
[ "$pa" = "$pb" ] && [ "$pa" -ge 12 ] || { echo "RED: primitive selftest fleet not fully caught or below floor: $pn"; exit 1; }
mv2=$(node scripts/schema-v2-check.mjs --selftest 2>&1 | tail -1)
printf '%s' "$mv2" | grep -q 'SELFTEST' || { echo "RED: schema-v2 selftest banner unreadable: $mv2"; exit 1; }
! printf '%s' "$mv2" | grep -q 'MISSED' || { echo "RED: schema-v2 selftest reports missed injections: $mv2"; exit 1; }
tv=$(node scripts/threatmodel-check.mjs --selftest 2>&1 | tail -1)
printf '%s' "$tv" | grep -qE 'SELFTEST.*(OK|GREEN)' || { echo "RED: threatmodel selftest banner: $tv"; exit 1; }
m1=$(node scripts/coverage-map-check.mjs --selftest 2>&1 | grep -c 'mutation rejected' || true)
[ "$m1" -ge 5 ] || { echo "RED: coverage-map selftest caught $m1 mutations, want at least 5"; exit 1; }
echo "POSITIVE CONTROLS GREEN (wild fixture: all $wn lines rejected with named faces; primitive fleet $pn fully caught, schema-v2 and threat-model selftests present, map mutations $m1 - all counted from this run)"

step '03 wave diff face at pinned endpoints plus registration reconciliation: four registered merges are ancestors and each registered headline artifact is on disk'
W16_BASE=1742d4313c2bc57a00b8aaa4e8bfa1f3b0202744
W16_END=a95a2b76c7b36424f273936c95c9a379c57c6949
[ "${#W16_BASE}" -eq 40 ] && [ "${#W16_END}" -eq 40 ] || { echo 'RED: wave endpoint pins malformed'; exit 1; }
git merge-base --is-ancestor "$W16_BASE" HEAD || { echo 'RED: wave base endpoint not an ancestor of HEAD'; exit 1; }
git merge-base --is-ancestor "$W16_END" HEAD || { echo 'RED: wave end endpoint not an ancestor of HEAD'; exit 1; }
stray=$(git diff --name-only "${W16_BASE}..${W16_END}" | grep -vE '^(docs/|scripts/|testdata/taskrecord/|internal/schema/[A-Za-z0-9._-]+_test\.go$|internal/schema/event\.go$)' || true)
[ -z "$stray" ] || { echo 'RED: wave diff face carries files outside the allowed categories:'; echo "$stray"; exit 1; }
rtgo=$(git diff --name-only "${W16_BASE}..${W16_END}" | grep -E '\.go$' | grep -v '_test\.go$' || true)
[ "$rtgo" = 'internal/schema/event.go' ] || { echo 'RED: runtime Go face is not exactly the schema event file:'; echo "$rtgo"; exit 1; }
inten=$(git diff --name-only "${W16_BASE}..${W16_END}" | grep -E '^internal/' | grep -vE '^internal/(schema/[A-Za-z0-9._-]+_test\.go|schema/event\.go)$' || true)
[ -z "$inten" ] || { echo 'RED: internal bytes outside the fenced schema shape:'; echo "$inten"; exit 1; }
gold=$(git diff --name-only "${W16_BASE}..${W16_END}" | grep -E '^testdata/golden/' || true)
[ -z "$gold" ] || { echo 'RED: the wave touched the golden corpus face (golden expansion is fenced out of this wave):'; echo "$gold"; exit 1; }
schdiff=$(git diff --name-only "${W16_BASE}..${W16_END}" | grep -E '^docs/schema-v2\.md$' || true)
[ -z "$schdiff" ] || { echo 'RED: docs/schema-v2.md carried wave diff (expected zero)'; exit 1; }
mrg=$(git log --format='%H %s' "${W16_BASE}..${W16_END}" | grep -c 'Merge pull request' || true)
[ "$mrg" -eq 4 ] || { echo "RED: registered wave carries $mrg merge commits, want the four registered PR merges"; exit 1; }
for sha in b99ca1c4c4b6bd9a4a26cf738f2887a5aefb419f 0bd10bb21acbd2bb63fec822f3f58c00fe048402 \
           2a049712148cde47d1a574358fb8acaf4e523b8f a95a2b76c7b36424f273936c95c9a379c57c6949; do
  git merge-base --is-ancestor "$sha" HEAD || { echo "RED: registered merge sha not an ancestor: $sha"; exit 1; }
done
grep -q 'TypeTaskRecord' internal/schema/event.go \
  || { echo 'RED: W1 registered artifact (task.record schema join) not on disk'; exit 1; }
grep -q '^## 3. Canary / staged rollout' docs/release-plan-v0.md \
  || { echo 'RED: W2 registered artifact (canary pre-plan section) not on disk'; exit 1; }
test -f docs/prd-v0.md && test -f docs/ux-spec-v0.md \
  || { echo 'RED: W3 registered artifacts (PRD + UX spec registers) missing'; exit 1; }
grep -q '^## Agent-to-Agent abuse observation v0' docs/threat-model.md \
  || { echo 'RED: W4 registered artifact (A2A observation section) not on disk'; exit 1; }
wc_files=$(git diff --name-only "${W16_BASE}..${W16_END}" | wc -l | tr -cd '0-9')
echo "WAVE DIFF FACE CLEAN + REGISTERED MERGES RECONCILED (${wc_files} file(s) between the pinned endpoints, $mrg registered merges all ancestors, docs+scripts+taskrecord fixtures+schema test files plus exactly one runtime Go file, zero golden bytes, zero other internal bytes, schema-v2 untouched, all four registered headline artifacts on disk)"

step '04 documentation-register machine checks: release pre-plan homes, PRD/UX closed-cell matrices, A2A counterpart cross-pin - all re-counted from the shipped docs at run time'
for h in '## 2. Release (train shape)' '## 3. Canary / staged rollout' '## 4. Rollback (pre-plan — not implemented)' '## 5. Freeze windows and emergency path' '## 6. Clause cross-reference table' '## 7. Known gaps'; do
  grep -qF "$h" docs/release-plan-v0.md || { echo "RED: release plan home section missing: $h"; exit 1; }
done
nip=$(grep -c 'not implemented' docs/release-plan-v0.md)
[ "$nip" -ge 3 ] || { echo "RED: release plan honest 'not implemented' markers read $nip, want at least 3 (pre-plans must not pretend to be shipped)"; exit 1; }
grep -qF '§221 item 21' docs/release-plan-v0.md || { echo 'RED: release plan lost its owning clause anchor'; exit 1; }
# PRD matrix: every priority band P0..P6 registers a table; data cells are
# everything that is not a header or separator line. Loop-derived, never copied.
cells=0
for p in P0 P1 P2 P3 P4 P5 P6; do
  grep -q "^### $p" docs/prd-v0.md || { echo "RED: PRD priority band section missing: $p"; exit 1; }
  c=$(awk -v pat="^### $p" '$0 ~ pat {on=1; next} on && /^##/ {exit} on && /^\|/ && $0 !~ /^\|---/ && $0 !~ /^\| Cell/ {n++} END {print n+0}' docs/prd-v0.md)
  cells=$((cells + c))
done
[ "$cells" -eq 57 ] || { echo "RED: PRD matrix re-counts $cells cells, want the 57 registered in the heading"; exit 1; }
ux15=$(awk '/^## 2\. Binding acceptance contract/,/^## 3\./' docs/ux-spec-v0.md | grep -cE '^\| *§?307|^\| *[0-9]+ *\|' || true)
[ "$ux15" -ge 1 ] || { echo 'RED: UX acceptance contract fleet unreadable'; exit 1; }
grep -qF '(§307, 15 clauses)' docs/ux-spec-v0.md || { echo 'RED: UX spec lost its 15-clause heading anchor'; exit 1; }
grep -q '^## 6. Clause reverse-lookup table' docs/ux-spec-v0.md || { echo 'RED: UX reverse-lookup table missing'; exit 1; }
grep -q '^## 6. Known gaps' docs/prd-v0.md || { echo 'RED: PRD honest zero list missing'; exit 1; }
# A2A two-face cross-pin: the coverage row names the section and the
# section names the coverage face; mirror values stay untouched.
grep -q 'explicit threat-model counterpart registered (threat-model.md, Agent-to-Agent abuse observation v0)' docs/evals-coverage-v0.md \
  || { echo 'RED: coverage row lost its counterpart registration carry'; exit 1; }
grep -q 'The evaluation coverage map (docs/evals-coverage-v0.md) carries' docs/threat-model.md \
  || { echo 'RED: A2A section lost its coverage-map back-reference'; exit 1; }
a2a=$(grep -m1 '^| Agent-to-Agent Abuse |' docs/evals-coverage-v0.md)
printf '%s' "$a2a" | grep -q 'semantics pending' || { echo 'RED: A2A status column moved out of pending'; exit 1; }
printf '%s' "$a2a" | grep -q '0 / 0' || { echo 'RED: A2A mirror count moved from 0 / 0 (golden expansion is fenced out of this wave)'; exit 1; }
echo "DOC REGISTERS GREEN (release plan six homes + $nip honest markers + clause anchor; PRD bands P0-P6 re-counted to $cells cells matching the heading; UX 15-clause fleet + reverse-lookup table; A2A counterpart pinned both directions with status and 0 / 0 mirror untouched)"

step '05 record-only invariants: decision closed set, enforcement-teeth negative scan over the wave docs, all-none plane census, positional task_id shape'
grep -qF '{allow, would_block}' docs/task-primitive-design.md || { echo 'RED: Phase 0 decision closed-set anchor lost'; exit 1; }
if grep -qiE '\b(auto_deny|deny_list|hard_block|enforce_now)\b' docs/release-plan-v0.md docs/prd-v0.md docs/ux-spec-v0.md docs/task-primitive-design.md; then
  echo 'RED: enforcement-teeth vocabulary inside wave docs'; exit 1;
fi
n=$(grep -cE '^[a-z_]+_(enforcement|execution)_plane: none-in-observation-phase$' docs/schema-v2.md)
t=$(grep -cE '^[a-z_]+_(enforcement|execution)_plane: ' docs/schema-v2.md)
[ "$n" -eq "$t" ] || { echo "RED: a declared plane was promoted out of observation stance ($n/$t)"; exit 1; }
[ "$n" -ge 25 ] || { echo "RED: plane census $n, want at least 25 none-lines"; exit 1; }
grep -q 'no sixth uplift row' docs/schema-v2.md || { echo 'RED: schema-v2 uplift census lock lost'; exit 1; }
# good fixture decision values must live inside the closed set, every line,
# re-derived at run time; task_id must carry the positional session:ordinal shape.
jqfree=$(node -e '
const fs=require("fs");
const set=new Set(["allow","would_block"]);
const re=/^sess-[A-Za-z0-9._-]+:[0-9]+$/;
let n=0;
for (const line of fs.readFileSync("testdata/taskrecord/good.jsonl","utf8").split("\n")) {
  if (!line.trim()) continue;
  const e=JSON.parse(line); n++;
  if (!set.has(e.decision)) { console.log("RED: decision outside closed set: "+e.decision); process.exit(1); }
  if (e.task_id !== undefined && !re.test(e.task_id)) { console.log("RED: task_id not positional: "+e.task_id); process.exit(1); }
}
console.log("OK "+n);
')
printf '%s' "$jqfree" | grep -q '^OK' || { echo "$jqfree"; exit 1; }
echo "RECORD-ONLY GREEN (closed set anchor + $jqfree good-fixture lines inside {allow,would_block} with positional task_id shape, teeth vocabulary absent from wave docs, plane census all-none $n/$t re-derived, uplift census lock in force)"

step '06 inherited chain: gate-w15 full battery (which itself re-runs gate-w14 down through gate-d1 serially, gate-d7 Go suite with the task record schema tests included, and the gate-d5 release assembly battery)'
bash scripts/gate-w15.sh > "$LOGDIR/gate-w15-inside-w16.out" 2>&1 \
  || { tail -30 "$LOGDIR/gate-w15-inside-w16.out"; echo RED: gate-w15 regression; exit 1; }
grep -q 'GATE-W15: ALL GREEN' "$LOGDIR/gate-w15-inside-w16.out" \
  || { echo 'RED: gate-w15 completion banner missing'; exit 1; }
echo 'GATE-W15 GREEN INSIDE GATE-W16 (zero regression on the full inherited chain down through gate-d1, shipped release assembly exercised end-to-end inside gate-d5, schema Go test face included)'

step '07 verbatim contract anchors from all wave slices plus tripwire standalone (internal-ledger negative scan)'
pre16() {
  grep -qF "$2" "$1" || { echo "RED: wave anchor lost in $1: $2"; exit 1; }
}
pre16 docs/task-primitive-design.md 'task_status_vocabulary: open, active, completed, failed, abandoned, blocked, unknown'
pre16 internal/schema/event.go 'TypeTaskRecord EventType = "task.record"'
pre16 scripts/validate-jsonl.mjs "if (e.type === 'task.record') {"
pre16 docs/release-plan-v0.md '## 3. Canary / staged rollout (pre-plan — not implemented)'
pre16 docs/release-plan-v0.md '## 4. Rollback (pre-plan — not implemented)'
pre16 docs/prd-v0.md '## 2. Priority-layer requirements matrix (§298, 57 cells)'
pre16 docs/ux-spec-v0.md '## 2. Binding acceptance contract (§307, 15 clauses)'
pre16 docs/threat-model.md '## Agent-to-Agent abuse observation v0 (documentation surface)'
pre16 docs/evals-coverage-v0.md 'explicit threat-model counterpart registered (threat-model.md, Agent-to-Agent abuse observation v0)'
pre16 testdata/taskrecord/good.jsonl '"type":"task.record"'
pre16 testdata/taskrecord/wild.jsonl '"task_status":"succeeded"'
bash scripts/tripwire.sh > "$LOGDIR/gate-w16-tripwire.out" 2>&1 \
  || { cat "$LOGDIR/gate-w16-tripwire.out"; echo RED: tripwire; exit 1; }
grep -q 'TRIPWIRE CLEAN' "$LOGDIR/gate-w16-tripwire.out" || { echo RED: tripwire banner; exit 1; }
twcount=$(grep -oE '[0-9]+ (entries|scanned|items)' "$LOGDIR/gate-w16-tripwire.out" | head -1 || true)
echo "ANCHORS GREEN (eleven verbatim tokens across the four wave slices; tripwire CLEAN ${twcount:-count not printed})"

printf '\nGATE-W16: ALL GREEN\n'
