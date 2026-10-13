#!/usr/bin/env bash
# gate-w18.sh - machine-assertion battery for the golden-expansion wave
# (five seats, four merges: the day-build ticket-1 EXPORT engine slice
# merged as PR#84, the W1 ticket-2 network-exfiltration corpus slice as
# PR#85, the W1 tail inherited-fix gate seat merged on the W2 seat as
# PR#86 (built-in would_block effect pin following the shipped rule table
# 12 to 13), the W4 Prompt-Injection case slice as PR#87, and the W3 seat
# a registered zero-write reconciliation read that must show no merge of
# its own anywhere in the wave face).
#
# First edition, parameter-changed from the gate-w17 skeleton (whole-file
# authoring discipline; shared scaffolding kept byte-shape: TMPDIR residue
# cleanup, out-of-repo log directory, PATH fallback, closed-set re-parsing
# at run time, inherited chain, tripwire closer).
#
# RE-PIN DECLARATION (the standing reason this wave's diff face is
# wider than the previous one): the wave legitimately moves golden corpus
# bytes because it ships the EXPORT judgement row plus nine new direct
# danger cases. The golden shape fenced here is exactly:
#   - testdata/golden/danger.jsonl +9/-0 and labels.json +45/-0 are pure
#     appends; every byte of the pre-existing 140-case streams is fenced
#     unchanged by the append-only assertions below (the standing "old
#     golden bytes stay red" rule is enforced here, not waived);
#   - coverage-map.json is the mirror carrier whose rows the shipped
#     checker re-derives against the freshly appended corpus;
#   - thresholds.json ledger readings are re-nailed from this wave's own
#     fresh eval-runner output because the denominator moved with the
#     added cases (68 to 73 danger, 144 to 149 total).
# Evidence for the re-pin: the wave handoff rows recorded in the build
# ledger for the engine slice, the corpus slice and the injection-chain
# slice, plus the gate-fix merge commit f874761 sitting in this branch's
# base history (ledger file paths deliberately stay off the product
# face). The W15/W16/W17 pins inside the
# inherited gates stay exactly as they were. Any red exits non-zero.
set -euo pipefail
cd "$(dirname "$0")/.."

export TMPDIR="$(pwd)/.tmptest"
mkdir -p "$TMPDIR"
find "$TMPDIR" -mindepth 1 -maxdepth 1 -name 'go-build*' -exec rm -rf {} + 2>/dev/null || true
command -v go >/dev/null 2>&1 || export PATH="$HOME/.local/go/bin:$PATH"
LOGDIR=/home/node/gate-logs
mkdir -p "$LOGDIR"

step() { printf '\n=== %s ===\n' "$*"; }

step '00 preflight: wave artifacts repo-visible right now (engine row, both checkers, all four golden faces, three inherited gates)'
for f in internal/rules/rules.go internal/schema/policy.go internal/policy/evaluate.go \
         cmd/agent-collector/replay_cli_test.go \
         scripts/coverage-map-check.mjs scripts/threshold-ledger-check.mjs scripts/threatmodel-check.mjs \
         scripts/apicontract-check.mjs scripts/schema-v2-check.mjs scripts/tripwire.sh \
         docs/evals-coverage-v0.md docs/threat-model.md docs/api-v0.md docs/schema-v2.md \
         testdata/golden/danger.jsonl testdata/golden/normal.jsonl testdata/golden/labels.json \
         testdata/golden/coverage-map.json testdata/golden/thresholds.json; do
  test -f "$f" || { echo "RED: wave artifact missing: $f"; exit 1; }
done
test -f scripts/gate-w18.sh || { echo 'RED: this gate not on disk'; exit 1; }
test -f scripts/gate-w17.sh || { echo 'RED: inherited gate-w17 missing from disk'; exit 1; }
test -f scripts/gate-w16.sh || { echo 'RED: inherited gate-w16 missing from disk'; exit 1; }
test -f scripts/gate-w15.sh || { echo 'RED: inherited gate-w15 missing from disk'; exit 1; }
echo "PREFLIGHT GREEN (engine row, checkers, golden faces, docs mirrors and the inherited gate ladder all visible on disk)"

step '01 checker full runs: map lockstep against the freshly appended corpus, ledger bound to THIS run eval output, threat model, api contract, schema-v2 regression'
node scripts/coverage-map-check.mjs > "$LOGDIR/gate-w18-map.out" 2>&1 \
  || { tail -20 "$LOGDIR/gate-w18-map.out"; echo RED: coverage-map-check; exit 1; }
grep -q 'MAP OK' "$LOGDIR/gate-w18-map.out" || { echo 'RED: map banner missing'; exit 1; }
go test ./internal/rules/ -run TestGoldenEvals -v -count=1 -p 1 -parallel 1 > "$LOGDIR/gate-w18-eval.out" 2>&1 \
  || { tail -25 "$LOGDIR/gate-w18-eval.out"; echo 'RED: eval suite failed (ledger has no live source to bind to)'; exit 1; }
grep -q 'EVAL SUMMARY' "$LOGDIR/gate-w18-eval.out" || { echo 'RED: no EVAL SUMMARY in runner output'; exit 1; }
node scripts/threshold-ledger-check.mjs "$LOGDIR/gate-w18-eval.out" > "$LOGDIR/gate-w18-ledger.out" 2>&1 \
  || { tail -20 "$LOGDIR/gate-w18-ledger.out"; echo RED: threshold-ledger-check; exit 1; }
grep -q 'LEDGER OK' "$LOGDIR/gate-w18-ledger.out" || { echo 'RED: ledger banner missing'; exit 1; }
node scripts/threatmodel-check.mjs > "$LOGDIR/gate-w18-threat.out" 2>&1 \
  || { tail -20 "$LOGDIR/gate-w18-threat.out"; echo RED: threatmodel-check; exit 1; }
grep -q 'THREAT' "$LOGDIR/gate-w18-threat.out" || { echo 'RED: threat banner moved out of lockstep'; exit 1; }
node scripts/apicontract-check.mjs > "$LOGDIR/gate-w18-apicontract.out" 2>&1 \
  || { tail -20 "$LOGDIR/gate-w18-apicontract.out"; echo RED: apicontract-check; exit 1; }
grep -q 'API-CONTRACT OK' "$LOGDIR/gate-w18-apicontract.out" || { echo 'RED: apicontract banner missing'; exit 1; }
node scripts/schema-v2-check.mjs > "$LOGDIR/gate-w18-v2check.out" 2>&1 \
  || { tail -20 "$LOGDIR/gate-w18-v2check.out"; echo RED: schema-v2-check; exit 1; }
grep -q 'SCHEMA-V2: ALL GREEN' "$LOGDIR/gate-w18-v2check.out" || { echo 'RED: v2-check banner missing'; exit 1; }
echo "CHECKERS GREEN (map lockstep, ledger re-nailed against this run's eval output, threat/api/schema batteries in force)"

step '02 positive controls: both wave checkers fully catch their own injections, the two golden streams validate, the replay CLI total test passes'
mv=$(node scripts/coverage-map-check.mjs --selftest 2>&1 | tail -1)
printf '%s' "$mv" | grep -q 'SELFTEST OK' || { echo "RED: map selftest not green: $mv"; exit 1; }
lv=$(node scripts/threshold-ledger-check.mjs --selftest "$LOGDIR/gate-w18-eval.out" 2>&1 | tail -1)
printf '%s' "$lv" | grep -q 'SELFTEST OK' || { echo "RED: ledger selftest not green: $lv"; exit 1; }
node scripts/validate-jsonl.mjs testdata/golden/danger.jsonl > "$LOGDIR/gate-w18-dv.out" 2>&1 \
  || { tail -5 "$LOGDIR/gate-w18-dv.out"; echo RED: danger stream failed validation; exit 1; }
node scripts/validate-jsonl.mjs testdata/golden/normal.jsonl > "$LOGDIR/gate-w18-nv.out" 2>&1 \
  || { tail -5 "$LOGDIR/gate-w18-nv.out"; echo RED: normal stream failed validation; exit 1; }
go test ./cmd/agent-collector/ -run TestReplayCLIGoldenRun -count=1 > "$LOGDIR/gate-w18-replay.out" 2>&1 \
  || { tail -15 "$LOGDIR/gate-w18-replay.out"; echo RED: replay CLI golden run test; exit 1; }
echo "POSITIVE CONTROLS GREEN (map and ledger selftests fully caught, both streams valid, replay CLI golden run passes at its pinned total)"

step '03 wave diff face at pinned endpoints plus registration reconciliation: four registered merges are ancestors and the zero-write seat shows no extra merge'
W18_BASE=40cd3d4aa6788ebff42c2ac3e5ef4f2c73ceaa23
W18_END=5b10fa11d4cc0ef50473d83e6e726042ff80281a
[ "${#W18_BASE}" -eq 40 ] && [ "${#W18_END}" -eq 40 ] || { echo 'RED: wave endpoint pins malformed'; exit 1; }
git merge-base --is-ancestor "$W18_BASE" HEAD || { echo 'RED: wave base endpoint not an ancestor of HEAD'; exit 1; }
git merge-base --is-ancestor "$W18_END" HEAD || { echo 'RED: wave end endpoint not an ancestor of HEAD'; exit 1; }
stray=$(git diff --name-only "${W18_BASE}..${W18_END}" | grep -vE '^(docs/|scripts/|internal/|cmd/agent-collector/replay_cli_test\.go$|testdata/golden/)' || true)
[ -z "$stray" ] || { echo 'RED: wave diff face carries files outside the allowed categories:'; echo "$stray"; exit 1; }
goldface=$(git diff --name-only "${W18_BASE}..${W18_END}" | grep -E '^testdata/golden/' | sort)
want='testdata/golden/coverage-map.json
testdata/golden/danger.jsonl
testdata/golden/labels.json
testdata/golden/thresholds.json'
[ "$goldface" = "$want" ] || { echo 'RED: golden face is not exactly the four declared carriers:'; echo "$goldface"; exit 1; }
deldanger=$(git diff -U0 "${W18_BASE}..${W18_END}" -- testdata/golden/danger.jsonl | grep -c '^-[^-]' || true)
dellabel=$(git diff -U0 "${W18_BASE}..${W18_END}" -- testdata/golden/labels.json | grep -c '^-[^-]' || true)
[ "$deldanger" = "0" ] || { echo "RED: danger.jsonl is not append-only ($deldanger removed lines) - old golden bytes stay red"; exit 1; }
[ "$dellabel" = "0" ] || { echo "RED: labels.json is not append-only ($dellabel removed lines) - the standing 140-case label face must be untouched"; exit 1; }
mrg=$(git log --format='%s' "${W18_BASE}..${W18_END}" | grep -c 'Merge pull request' || true)
[ "$mrg" -eq 4 ] || { echo "RED: wave carries $mrg merge commits, want exactly the four registered PR merges (PR#84 #85 #86 #87; the W3 seat is registered zero-write)"; exit 1; }
for sha in 2638df7a9082f82d2277d3e2548cb2b49f8bd84f 53433fbca0f4a0334888a1185b4ebd7814bcbfb0 \
           29b0bf017e2a6d48140b982e1606aae095704877 5b10fa11d4cc0ef50473d83e6e726042ff80281a; do
  git merge-base --is-ancestor "$sha" HEAD || { echo "RED: registered merge sha not an ancestor: $sha"; exit 1; }
done
grep -q '"id": "egress.export"' internal/rules/rules.go || { echo 'RED: ticket-1 registered artifact (EXPORT judgement row) not on disk'; exit 1; }
grep -q '"id":"gd-29"' testdata/golden/danger.jsonl || { echo 'RED: ticket corpus tail case gd-29 not on disk'; exit 1; }
echo "WAVE DIFF FACE CLEAN + REGISTERED MERGES RECONCILED (${mrg} registered merges all ancestors and no fifth, docs+scripts+internal/cmd plus exactly the four golden carriers, danger and labels fenced append-only, engine row and corpus tail present on disk)"

step '04 slice machine checks: ticket three-source lock (corpus line, label key, mirror carrier) re-derived at run time; PI mirror reverse-lookup; totals re-counted from disk'
node -e '
const { execSync } = require("child_process");
const fs = require("fs");
const base = execSync("git show 40cd3d4a:testdata/golden/danger.jsonl").toString().trim().split("\n").map(l=>JSON.parse(l).id);
const cur = fs.readFileSync("testdata/golden/danger.jsonl","utf8").trim().split("\n").map(l=>JSON.parse(l));
const ids = cur.map(r=>r.id);
const fresh = ids.filter(i=>!base.includes(i));
if (fresh.length !== 9) { console.log("RED: wave appended "+fresh.length+" danger ids, want the nine registered (gd-21..gd-29)"); process.exit(1); }
const labels = JSON.parse(fs.readFileSync("testdata/golden/labels.json","utf8"));
const mapRaw = fs.readFileSync("testdata/golden/coverage-map.json","utf8");
for (const id of fresh) {
  const lab = labels[id];
  if (!lab) { console.log("RED: appended "+id+" has no label key (labels lockstep broken)"); process.exit(1); }
  if (lab.expect !== "would_block") { console.log("RED: "+id+" expect is not would_block"); process.exit(1); }
  if (!mapRaw.includes("\""+id+"\"")) { console.log("RED: "+id+" absent from the mirror carrier"); process.exit(1); }
}
const docs = fs.readFileSync("docs/evals-coverage-v0.md","utf8");
for (const a of ["gd-21","gd-24","gd-25","gd-29"]) {
  if (!docs.includes(a)) { console.log("RED: docs mirror lost range anchor "+a); process.exit(1); }
}
console.log("LOCKSTEP OK (nine fresh ids present in corpus, labels and mirror carrier; docs range anchors present)");
' || exit 1
grep -q 'not through intent-reading' docs/evals-coverage-v0.md \
  || { echo 'RED: the honest-stance sentence moved out of lockstep (no intent-reading matcher is allowed and none was shipped)'; exit 1; }
for rid in exec.remotepipe cred.dotenv mcp.eval net.egress destroy.rmrf; do
  grep -q "\"id\": \"$rid\"" internal/rules/rules.go || { echo "RED: PI mirror primitive row missing from the shipped rule table: $rid"; exit 1; }
done
dn=$(wc -l < testdata/golden/danger.jsonl | tr -cd '0-9')
nn=$(wc -l < testdata/golden/normal.jsonl | tr -cd '0-9')
tot=$((dn + nn))
[ "$dn" -eq 73 ] || { echo "RED: danger stream re-count $dn, want the pinned 73"; exit 1; }
grep -q "$tot" cmd/agent-collector/replay_cli_test.go || { echo "RED: replay CLI total pin no longer names the re-counted total $tot"; exit 1; }
echo "SLICE CHECKS GREEN (ticket three-source lock re-derived at run time: nine fresh ids locked across corpus, labels and mirror; docs range anchors and honest-stance sentence present verbatim; all five PI mirror primitives shipped; totals re-counted $dn danger + $nn normal = $tot matching the replay CLI pin)"

step '05 record-only invariants: decision closed set, built-in rule count re-derived, effects pin lockstep across the inherited gate scripts, teeth vocabulary absent from wave docs, plane census all-none re-derived'
grep -qF '{allow, would_block}' docs/api-v0.md || { echo 'RED: Phase 0 decision closed-set anchor lost from the api contract'; exit 1; }
if grep -qiE '\b(auto_deny|deny_list|hard_block|enforce_now)\b' docs/evals-coverage-v0.md docs/threat-model.md docs/api-v0.md; then
  echo 'RED: enforcement-teeth vocabulary inside wave docs'; exit 1;
fi
rc=$(grep -c '"priority":' internal/rules/rules.go | tr -cd '0-9')
const=$(grep -oE 'BuiltinRuleCount = [0-9]+' internal/rules/rules.go | grep -oE '[0-9]+')
[ "$rc" -eq "$const" ] || { echo "RED: built-in rule rows re-count $rc but the pinned constant says $const"; exit 1; }
for g in scripts/gate-d4.sh scripts/gate-d7.sh scripts/gate-w1.sh; do
  grep -q "$const" "$g" || { echo "RED: inherited gate script $g still pins a stale effect count (12 to 13 fix not in lockstep)"; exit 1; }
done
n=$(grep -cE '^[a-z_]+_(enforcement|execution)_plane: none-in-observation-phase$' docs/schema-v2.md)
t=$(grep -cE '^[a-z_]+_(enforcement|execution)_plane: ' docs/schema-v2.md)
[ "$n" -eq "$t" ] || { echo "RED: a declared plane was promoted out of observation stance ($n/$t)"; exit 1; }
[ "$n" -ge 25 ] || { echo "RED: plane census $n, want at least 25 none-lines"; exit 1; }
echo "RECORD-ONLY GREEN (closed-set anchor present, teeth vocabulary absent from the wave docs, rule count re-derived $rc==$const and the three inherited gate scripts in lockstep with it, plane census all-none $n/$t re-derived)"

step '06 inherited chain: gate-w17 full battery (which itself re-runs gate-w16 down through gate-d1 serially, the gate-d7 Go suite and the gate-d5 release assembly, with the W15/W16/W17 pins untouched)'
bash scripts/gate-w17.sh > "$LOGDIR/gate-w17-inside-w18.out" 2>&1 \
  || { tail -30 "$LOGDIR/gate-w17-inside-w18.out"; echo RED: gate-w17 regression; exit 1; }
grep -q 'GATE-W17: ALL GREEN' "$LOGDIR/gate-w17-inside-w18.out" \
  || { echo 'RED: gate-w17 completion banner missing'; exit 1; }
echo 'GATE-W17 GREEN INSIDE GATE-W18 (zero regression on the full inherited chain down through gate-d1 with the W15/W16/W17 pins untouched)'

step '07 verbatim contract anchors from all wave slices plus tripwire standalone (internal-ledger negative scan)'
anch() {
  grep -qF "$2" "$1" || { echo "RED: wave anchor lost in $1: $2"; exit 1; }
}
anch internal/rules/rules.go '"id": "egress.export",  "priority": 45'
anch internal/schema/policy.go 'FieldDataAction MatchField = "data_action"'
anch docs/evals-coverage-v0.md 'not through intent-reading'
anch testdata/golden/labels.json '"gd-21"'
anch testdata/golden/labels.json '"gd-29"'
anch scripts/threshold-ledger-check.mjs 'FRESH eval-runner output'
bash scripts/tripwire.sh > "$LOGDIR/gate-w18-tripwire.out" 2>&1 \
  || { cat "$LOGDIR/gate-w18-tripwire.out"; echo RED: tripwire; exit 1; }
grep -q 'TRIPWIRE CLEAN' "$LOGDIR/gate-w18-tripwire.out" || { echo RED: tripwire banner; exit 1; }
twcount=$(grep -oE '[0-9]+ (entries|scanned|items|files)' "$LOGDIR/gate-w18-tripwire.out" | head -1 || true)
echo "ANCHORS GREEN (six verbatim tokens across the wave slices; tripwire CLEAN ${twcount:-count not printed})"

printf '\nGATE-W18: ALL GREEN\n'
