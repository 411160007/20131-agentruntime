#!/usr/bin/env bash
# gate-w21.sh - machine-assertion battery for the multi-seat corpus wave
# (seven seats, seven merges: the capability-escalation first wave merged as
# PR#98 (four direct attempts gd-49 to gd-52 each mirroring one shipped
# primitive effect), the skill-abuse second wave as PR#99 (four attempts
# gd-53 to gd-56 behind the eval-exposing tool primitive in fresh shapes),
# the supply-chain second wave as PR#100 (four attempts gd-57 to gd-60 behind
# the credential-store and fetch-and-run primitives), the prompt-injection
# second wave as PR#101 (four attempts gd-61 to gd-64 mirroring four of the
# five shipped primitives with fresh injection-chain shapes), the
# capability-escalation second wave as PR#102 (four attempts gd-65 to gd-68
# mirroring four more shipped primitive effects), the data-exfiltration
# third wave as PR#103 (four attempts gd-69 to gd-72) and the
# data-exfiltration fourth wave as PR#104 (four attempts gd-73 to gd-76).
# One seat (the night leg between PR#102 and PR#103) stayed a registered
# zero-write seat: it shipped the wave's scoping decision document instead,
# which is exactly why the merge count below is seven and not eight.
#
# First edition, parameter-changed from the gate-w20 skeleton (whole-file
# authoring discipline; shared scaffolding kept byte-shape: TMPDIR residue
# cleanup, out-of-repo log directory, PATH fallback, closed-set re-parsing
# at run time, tripwire closer). The inherited ladder keeps the wave-base
# shape introduced at w20: it re-runs gate-w20 and below at a detached
# worktree of this wave's base commit, because the corpus re-counts asserted
# inside those older gates are wave-relative facts certified at their own
# wave tails - this wave again moved the danger stream, so replaying those
# absolute re-counts at the new head would be a checker shape artifact, not
# a regression. The moved face of this wave is certified at the real head by
# steps 01 to 05, and every old gate stays green exactly where it left it.
#
# RE-PIN DECLARATION (why this wave's golden face moves and exactly how far):
# the wave legitimately ships twenty-eight new danger-side direct-attempt
# cases (four plus four plus four plus four across the four day seats, then
# four plus four plus four across the three night write seats), so
#   - testdata/golden/danger.jsonl +28/-0 and labels.json +140/-0 are pure
#     appends; every byte of the pre-existing streams is fenced unchanged by
#     the append-only assertions below (the standing "old golden bytes stay
#     red" rule is enforced here, not waived - the 108-plus-80 corpus that
#     stood at the wave base keeps its bytes, and every byte added after it
#     is accounted for), and the normal stream is fenced byte-identical
#     because this wave ships no new normal case at all;
#   - coverage-map.json is the mirror carrier whose rows the shipped checker
#     re-derives against the freshly appended corpus (doc total 172 to 200,
#     danger 92 to 120; the Prompt Injection primitive row moves 10 to 14,
#     Skill Abuse 12 to 16, Supply Chain Attack 10 to 14, Capability
#     Escalation 8 to 16, Data Exfiltration 29 to 37, while the
#     memory-poisoning row stays rules-empty and cases-empty exactly as
#     before, and the covered rows stay full engine mirrors of the appended
#     ids as the map checker enforces);
#   - thresholds.json ledger readings are re-nailed from this wave's own
#     fresh eval-runner output because the denominator moved with the added
#     cases (detection 94/96 through 98/100, 102/104, 106/108, 110/112 and
#     114/116 up to 118/120, pct 98.3), with the missed set unchanged at the
#     two declared known gaps.
# Evidence for the re-pin: the wave handoff rows recorded in the build ledger
# (ledger file paths deliberately stay off the product face). The W15 through
# W20 pins inside the inherited gates stay exactly as they were.
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

step '00 preflight: wave artifacts repo-visible right now (all golden faces, docs table, both corpus checkers, the replay pin, the repaired ledger checker, three inherited gates)'
for f in cmd/agent-collector/replay_cli_test.go \
         scripts/coverage-map-check.mjs scripts/threshold-ledger-check.mjs scripts/threatmodel-check.mjs \
         scripts/apicontract-check.mjs scripts/schema-v2-check.mjs scripts/validate-jsonl.mjs scripts/tripwire.sh \
         internal/rules/rules.go internal/rules/eval_test.go \
         docs/evals-coverage-v0.md docs/threat-model.md docs/api-v0.md docs/schema-v2.md README.md \
         testdata/golden/danger.jsonl testdata/golden/normal.jsonl testdata/golden/labels.json \
         testdata/golden/coverage-map.json testdata/golden/thresholds.json; do
  test -f "$f" || { echo "RED: wave artifact missing: $f"; exit 1; }
done
test -f scripts/gate-w21.sh || { echo 'RED: this gate not on disk'; exit 1; }
test -f scripts/gate-w20.sh || { echo 'RED: inherited gate-w20 missing from disk'; exit 1; }
test -f scripts/gate-w19.sh || { echo 'RED: inherited gate-w19 missing from disk'; exit 1; }
test -f scripts/gate-w18.sh || { echo 'RED: inherited gate-w18 missing from disk'; exit 1; }
echo "PREFLIGHT GREEN (corpus faces, docs table, both checkers, replay pin and the inherited gate ladder all visible on disk)"

step '01 checker full runs: map lockstep against the freshly appended corpus, ledger bound to THIS run eval output, threat model, api contract, schema-v2 regression'
node scripts/coverage-map-check.mjs > "$LOGDIR/gate-w21-map.out" 2>&1 \
  || { tail -20 "$LOGDIR/gate-w21-map.out"; echo RED: coverage-map-check; exit 1; }
grep -q 'MAP OK' "$LOGDIR/gate-w21-map.out" || { echo 'RED: map banner missing'; exit 1; }
go test ./internal/rules/ -run TestGoldenEvals -v -count=1 -p 1 -parallel 1 > "$LOGDIR/gate-w21-eval.out" 2>&1 \
  || { tail -25 "$LOGDIR/gate-w21-eval.out"; echo 'RED: eval suite failed (ledger has no live source to bind to)'; exit 1; }
grep -q 'EVAL SUMMARY' "$LOGDIR/gate-w21-eval.out" || { echo 'RED: no EVAL SUMMARY in runner output'; exit 1; }
node scripts/threshold-ledger-check.mjs "$LOGDIR/gate-w21-eval.out" > "$LOGDIR/gate-w21-ledger.out" 2>&1 \
  || { tail -20 "$LOGDIR/gate-w21-ledger.out"; echo RED: threshold-ledger-check; exit 1; }
grep -q 'LEDGER OK' "$LOGDIR/gate-w21-ledger.out" || { echo 'RED: ledger banner missing'; exit 1; }
node scripts/threatmodel-check.mjs > "$LOGDIR/gate-w21-threat.out" 2>&1 \
  || { tail -20 "$LOGDIR/gate-w21-threat.out"; echo RED: threatmodel-check; exit 1; }
grep -q 'THREAT' "$LOGDIR/gate-w21-threat.out" || { echo 'RED: threat banner moved out of lockstep'; exit 1; }
node scripts/apicontract-check.mjs > "$LOGDIR/gate-w21-apicontract.out" 2>&1 \
  || { tail -20 "$LOGDIR/gate-w21-apicontract.out"; echo RED: apicontract-check; exit 1; }
grep -q 'API-CONTRACT OK' "$LOGDIR/gate-w21-apicontract.out" || { echo 'RED: apicontract banner missing'; exit 1; }
node scripts/schema-v2-check.mjs > "$LOGDIR/gate-w21-v2check.out" 2>&1 \
  || { tail -20 "$LOGDIR/gate-w21-v2check.out"; echo RED: schema-v2-check; exit 1; }
grep -q 'SCHEMA-V2: ALL GREEN' "$LOGDIR/gate-w21-v2check.out" || { echo 'RED: v2-check banner missing'; exit 1; }
echo "CHECKERS GREEN (map lockstep, ledger re-nailed against this run's eval output, threat/api/schema batteries in force)"

step '02 positive controls: both corpus checkers fully catch their own injections, both golden streams validate, the replay CLI total test passes at its pinned total'
mv=$(node scripts/coverage-map-check.mjs --selftest 2>&1 | tail -1)
printf '%s' "$mv" | grep -q 'SELFTEST OK' || { echo "RED: map selftest not green: $mv"; exit 1; }
lv=$(node scripts/threshold-ledger-check.mjs --selftest "$LOGDIR/gate-w21-eval.out" 2>&1 | tail -1)
printf '%s' "$lv" | grep -q 'SELFTEST OK' || { echo "RED: ledger selftest not green: $lv"; exit 1; }
node scripts/validate-jsonl.mjs testdata/golden/danger.jsonl > "$LOGDIR/gate-w21-dv.out" 2>&1 \
  || { tail -5 "$LOGDIR/gate-w21-dv.out"; echo RED: danger stream failed validation; exit 1; }
node scripts/validate-jsonl.mjs testdata/golden/normal.jsonl > "$LOGDIR/gate-w21-nv.out" 2>&1 \
  || { tail -5 "$LOGDIR/gate-w21-nv.out"; echo RED: normal stream failed validation; exit 1; }
go test ./cmd/agent-collector/ -run TestReplayCLIGoldenRun -count=1 > "$LOGDIR/gate-w21-replay.out" 2>&1 \
  || { tail -15 "$LOGDIR/gate-w21-replay.out"; echo RED: replay CLI golden run test; exit 1; }
echo "POSITIVE CONTROLS GREEN (map and ledger selftests fully caught, both streams valid, replay CLI golden run passes at its pinned total)"

step '03 wave diff face at pinned endpoints plus registration reconciliation: seven registered merges are ancestors and the face carries nothing outside the corpus-and-mirror categories'
W21_BASE=07296e1f2741d34b01cdea167855b1300881783b
W21_END=7decbe4791c24399858bb492b6456b274bc2e1dc
[ "${#W21_BASE}" -eq 40 ] && [ "${#W21_END}" -eq 40 ] || { echo 'RED: wave endpoint pins malformed'; exit 1; }
git merge-base --is-ancestor "$W21_BASE" HEAD || { echo 'RED: wave base endpoint not an ancestor of HEAD'; exit 1; }
git merge-base --is-ancestor "$W21_END" HEAD || { echo 'RED: wave end endpoint not an ancestor of HEAD'; exit 1; }
stray=$(git diff --name-only "${W21_BASE}..${W21_END}" | grep -vE '^(docs/|scripts/|cmd/agent-collector/replay_cli_test\.go$|testdata/golden/)' || true)
[ -z "$stray" ] || { echo 'RED: wave diff face carries files outside the allowed categories:'; echo "$stray"; exit 1; }
goldface=$(git diff --name-only "${W21_BASE}..${W21_END}" | grep -E '^testdata/golden/' | sort)
want='testdata/golden/coverage-map.json
testdata/golden/danger.jsonl
testdata/golden/labels.json
testdata/golden/thresholds.json'
[ "$goldface" = "$want" ] || { echo 'RED: golden face is not exactly the four declared carriers:'; echo "$goldface"; exit 1; }
git diff --quiet "${W21_BASE}..${W21_END}" -- testdata/golden/normal.jsonl || { echo 'RED: normal stream moved this wave - the corpus tickets ship no new normal case'; exit 1; }
delanger=$(git diff -U0 "${W21_BASE}..${W21_END}" -- testdata/golden/danger.jsonl | grep -c '^-[^-]' || true)
addanger=$(git diff -U0 "${W21_BASE}..${W21_END}" -- testdata/golden/danger.jsonl | grep -c '^+[^+]' || true)
[ "$delanger" = "0" ] || { echo "RED: danger.jsonl is not append-only ($delanger removed lines) - old golden bytes stay red"; exit 1; }
[ "$addanger" = "28" ] || { echo "RED: danger.jsonl appended $addanger lines, want exactly the twenty-eight registered case lines"; exit 1; }
dellabel=$(git diff -U0 "${W21_BASE}..${W21_END}" -- testdata/golden/labels.json | grep -c '^-[^-]' || true)
addlabel=$(git diff -U0 "${W21_BASE}..${W21_END}" -- testdata/golden/labels.json | grep -c '^+[^+]' || true)
[ "$dellabel" = "0" ] || { echo "RED: labels.json is not append-only ($dellabel removed lines) - the standing label face must be untouched"; exit 1; }
[ "$addlabel" = "140" ] || { echo "RED: labels.json appended $addlabel lines, want exactly five per each of the twenty-eight registered cases"; exit 1; }
git diff --quiet "${W21_BASE}..${W21_END}" -- internal/ || { echo 'RED: shipped Go code moved during the corpus wave - the tickets mirror shipped rules only'; exit 1; }
mrg=$(git log --format='%s' "${W21_BASE}..${W21_END}" | grep -c 'Merge pull request' || true)
[ "$mrg" -eq 7 ] || { echo "RED: wave carries $mrg merge commits, want exactly the seven registered PR merges (PR#98..PR#104, one night seat registered as zero-write)"; exit 1; }
for short in e973b7b2 44234fa7 b9dbcc1f 466af97b 6d219a15 632db1f8 7decbe47; do
  full=$(git rev-parse --verify "${short}^{commit}" 2>/dev/null || true)
  [ "${#full}" -eq 40 ] || { echo "RED: registered merge short pin $short does not resolve to a full commit"; exit 1; }
  git merge-base --is-ancestor "$full" HEAD || { echo "RED: registered merge sha not an ancestor: $short"; exit 1; }
done
for pr in 98 99 100 101 102 103 104; do
  git log --format='%s' "${W21_BASE}..${W21_END}" | grep -q "Merge pull request #$pr " || { echo "RED: registered merge PR#$pr title missing from the wave"; exit 1; }
done
grep -q '"id":"gd-76"' testdata/golden/danger.jsonl || { echo 'RED: wave corpus tail case gd-76 not on disk'; exit 1; }
grep -q 'realPct' scripts/threshold-ledger-check.mjs || { echo 'RED: the ledger checker lost its fresh-reading-derived negative controls'; exit 1; }
echo "WAVE DIFF FACE CLEAN + REGISTERED MERGES RECONCILED (${mrg} registered merges all resolve at run time, are ancestors, and no eighth; docs+replay pin plus exactly the four golden carriers, danger fenced append-only at exactly twenty-eight added lines, labels append-only at exactly one hundred forty, normal byte-identical, internal Go untouched, tail case and repaired checker present on disk)"

step '04 slice machine checks: four-source lock (corpus line, label key, mirror carrier, docs mirror) re-derived at run time for all twenty-eight appended cases; per-row primitive lockstep; missed set unchanged'
node -e '
const { execSync } = require("child_process");
const fs = require("fs");
const base = execSync("git show 07296e1f2741d34b01cdea167855b1300881783b:testdata/golden/danger.jsonl").toString().trim().split("\n").map(l=>JSON.parse(l).id);
const cur = fs.readFileSync("testdata/golden/danger.jsonl","utf8").trim().split("\n").map(l=>JSON.parse(l));
const ids = cur.map(r=>r.id);
const fresh = ids.filter(i=>!base.includes(i));
const want = [];
for (let n=49;n<=76;n++) want.push("gd-"+String(n));
if (fresh.length !== 28 || !want.every(w=>fresh.includes(w))) {
  console.log("RED: wave appended "+fresh.join(",")+" - want exactly the twenty-eight registered ids gd-49..gd-76"); process.exit(1);
}
const labels = JSON.parse(fs.readFileSync("testdata/golden/labels.json","utf8"));
for (const id of fresh) {
  const lab = labels[id];
  if (!lab) { console.log("RED: appended "+id+" has no label key (labels lockstep broken)"); process.exit(1); }
  if (lab.expect !== "would_block") { console.log("RED: "+id+" expect is not would_block (direct attempts must be booked as downstream-capture hits, never silent passes)"); process.exit(1); }
}
const mapObj = JSON.parse(fs.readFileSync("testdata/golden/coverage-map.json","utf8"));
const rowOf = (name)=>{const r=mapObj.security_classes.find(x=>x&&x.name===name); if(!r){console.log("RED: class row vanished: "+name);process.exit(1);} return (r.primitive_cases||[]);};
const pi=rowOf("Prompt Injection"), sa=rowOf("Skill Abuse"), sc=rowOf("Supply Chain Attack"), ce=rowOf("Capability Escalation"), de=rowOf("Data Exfiltration");
if (pi.length!==14 || sa.length!==16 || sc.length!==14 || ce.length!==16 || de.length!==37) { console.log("RED: primitive row counts moved out of lockstep (PI "+pi.length+" want 14, SA "+sa.length+" want 16, SCA "+sc.length+" want 14, CE "+ce.length+" want 16, DE "+de.length+" want 37)"); process.exit(1); }
for (const id of fresh) {
  const homes=[pi,sa,sc,ce,de].filter(r=>r.includes(id));
  if (homes.length !== 1) { console.log("RED: "+id+" sits in "+homes.length+" pending-seat rows, want exactly one (per-seat reverse lookup)"); process.exit(1); }
}
for (const [grp,lo,hi,row] of [["CE",49,52,ce],["SA",53,56,sa],["SC",57,60,sc],["PI",61,64,pi],["CE",65,68,ce],["DE",69,72,de],["DE",73,76,de]]) {
  for (let n=lo;n<=hi;n++){const id="gd-"+n; if(!row.includes(id)){console.log("RED: seat-row mismatch: "+id+" not in the "+grp+" row");process.exit(1);}}
}
const mp = mapObj.security_classes.find(x=>x && x.name === "Memory Poisoning");
if (!mp || (mp.rules||[]).length !== 0 || (mp.cases||[]).length !== 0 || (mp.primitive_cases||[]).length !== 0) {
  console.log("RED: memory-poisoning row moved out of its rules-empty cases-empty stance"); process.exit(1);
}
const docs = fs.readFileSync("docs/evals-coverage-v0.md","utf8");
for (const a of ["gd-49","gd-53","gd-57","gd-61","gd-65","gd-69","gd-73","gd-76"]) {
  if (!docs.includes(a)) { console.log("RED: docs mirror lost append anchor "+a); process.exit(1); }
}
const corpus = mapObj.corpus || {};
if (corpus.normal!==80 || corpus.danger!==120 || corpus.total!==200 || corpus.labels!==200) {
  console.log("RED: mirror carrier corpus block not re-nailed at 80/120/200/200"); process.exit(1);
}
console.log("LOCKSTEP OK (twenty-eight fresh ids gd-49..gd-76 present in corpus and labels with expect=would_block; each in exactly one pending-seat row and in its seat row (CE gd-49..gd-52 and gd-65..gd-68, SA gd-53..gd-56, SC gd-57..gd-60, PI gd-61..gd-64, DE gd-69..gd-76); row counts 14/16/14/16/37; memory-poisoning row fenced empty; docs anchors present; carrier corpus block 80/120/200/200)");
' || exit 1
grep -F 'missed=[gd-19 gd-20]' "$LOGDIR/gate-w21-eval.out" > /dev/null \
  || { echo 'RED: the runner missed set moved off the two declared known gaps - a wave case went undetected'; exit 1; }
dn=$(wc -l < testdata/golden/danger.jsonl | tr -cd '0-9')
nn=$(wc -l < testdata/golden/normal.jsonl | tr -cd '0-9')
tot=$((dn + nn))
[ "$dn" -eq 120 ] || { echo "RED: danger stream re-count $dn, want the pinned 120"; exit 1; }
[ "$nn" -eq 80 ] || { echo "RED: normal stream re-count $nn, want the pinned 80 (unchanged by this wave)"; exit 1; }
lk=$(grep -c '"expect"' testdata/golden/labels.json | tr -cd '0-9')
[ "$lk" -eq "$tot" ] || { echo "RED: label keys re-count $lk against corpus total $tot"; exit 1; }
grep -q 'want 200/0' cmd/agent-collector/replay_cli_test.go || { echo 'RED: replay CLI total pin no longer names the re-counted total 200'; exit 1; }
echo "SLICE CHECKS GREEN (four-source lock re-derived at run time; missed set pinned to the declared known gaps so every wave case is a real downstream-capture hit; totals re-counted $dn danger + $nn normal = $tot with labels in lockstep and matching the replay CLI pin)"

step '05 invariants: decision closed set, shipped rule table provably untouched by the wave with count re-derived, teeth vocabulary absent from the wave docs face, plane census all-none re-derived'
grep -qF '{allow, would_block}' docs/api-v0.md || { echo 'RED: Phase 0 decision closed-set anchor lost from the api contract'; exit 1; }
if grep -qiE '\b(auto_deny|deny_list|hard_block|enforce_now)\b' docs/evals-coverage-v0.md docs/threat-model.md docs/api-v0.md README.md; then
  echo 'RED: enforcement-teeth vocabulary inside the wave docs face or the README'; exit 1;
fi
rc=$(grep -c '"priority":' internal/rules/rules.go | tr -cd '0-9')
const=$(grep -oE 'BuiltinRuleCount = [0-9]+' internal/rules/rules.go | grep -oE '[0-9]+')
[ "$rc" -eq "$const" ] || { echo "RED: built-in rule rows re-count $rc but the pinned constant says $const"; exit 1; }
[ "$rc" -eq 13 ] || { echo "RED: rule re-count $rc moved off the thirteen shipped rules - the wave promised zero new matchers"; exit 1; }
git diff --quiet 07296e1f2741d34b01cdea167855b1300881783b..HEAD -- internal/rules/rules.go || { echo 'RED: the shipped rule table moved during the wave - the tickets mirror shipped rules only'; exit 1; }
n=$(grep -cE '^[a-z_]+_(enforcement|execution)_plane: none-in-observation-phase$' docs/schema-v2.md)
t=$(grep -cE '^[a-z_]+_(enforcement|execution)_plane: ' docs/schema-v2.md)
[ "$n" -eq "$t" ] || { echo "RED: a declared plane was promoted out of observation stance ($n/$t)"; exit 1; }
[ "$n" -ge 26 ] || { echo "RED: plane census $n, want at least the 26 none-lines in force"; exit 1; }
echo "INVARIANTS GREEN (closed-set anchor present, teeth vocabulary absent, rule table provably untouched at re-count $rc==$const==13, plane census all-none $n/$t re-derived)"

step '06 inherited chain: gate-w20 ladder at its certified wave-tail state via a detached worktree pinned at this wave base (the corpus re-counts inside w20 and below are wave-relative facts certified at their own tails; the ladder must stay green exactly where each gate left it, and the moved face of THIS wave is certified by steps 01 to 05; the W15 through W20 pins and the gate files themselves stay untouched)'
BASEWT=/home/node/w21-base-wt
git rev-parse --verify "${W21_BASE}^{commit}" > /dev/null || { echo 'RED: wave base pin unresolvable'; exit 1; }
git worktree remove --force "$BASEWT" 2>/dev/null || true
git worktree add --detach "$BASEWT" "$W21_BASE" > /dev/null \
  || { echo 'RED: cannot stage the base worktree'; exit 1; }
mkdir -p "$BASEWT/.tmpdir" || { echo 'RED: cannot stage the base worktree scratch directory'; exit 1; }
( cd "$BASEWT" && bash scripts/gate-w20.sh > "$LOGDIR/gate-w20-inside-w21.out" 2>&1 ) \
  || { tail -30 "$LOGDIR/gate-w20-inside-w21.out"; echo RED: gate-w20 regression; git worktree remove --force "$BASEWT" 2>/dev/null || true; exit 1; }
grep -q 'GATE-W20: ALL GREEN' "$LOGDIR/gate-w20-inside-w21.out" \
  || { echo 'RED: gate-w20 completion banner missing'; git worktree remove --force "$BASEWT" 2>/dev/null || true; exit 1; }
git worktree remove --force "$BASEWT" || { echo 'RED: base worktree cleanup failed'; exit 1; }
if git worktree list | grep -q "$BASEWT"; then echo 'RED: base worktree still registered'; exit 1; fi
echo 'GATE-W20 GREEN AT ITS CERTIFIED STATE (full inherited ladder re-runs at the wave base exactly where each gate left it, W15 through W20 pins untouched, the base worktree cleaned up behind the run)'

step '07 verbatim contract anchors from all wave slices plus tripwire standalone (internal-ledger negative scan)'
anch() {
  grep -qF "$2" "$1" || { echo "RED: wave anchor lost in $1: $2"; exit 1; }
}
anch docs/evals-coverage-v0.md 'still no capability-growth trajectory semantics on shipped rules'
anch docs/evals-coverage-v0.md 'not through intent-reading'
anch docs/evals-coverage-v0.md 'no skill-vocabulary matcher ships'
anch docs/evals-coverage-v0.md 'package-integrity semantics are still not observed by shipped rules'
anch testdata/golden/coverage-map.json 'caught as primitives'
anch testdata/golden/danger.jsonl '"id":"gd-49"'
anch testdata/golden/danger.jsonl '"id":"gd-76"'
anch cmd/agent-collector/replay_cli_test.go 'want 200/0'
anch scripts/threshold-ledger-check.mjs 'a vacuous control'
bash scripts/tripwire.sh > "$LOGDIR/gate-w21-tripwire.out" 2>&1 \
  || { cat "$LOGDIR/gate-w21-tripwire.out"; echo RED: tripwire; exit 1; }
grep -q 'TRIPWIRE CLEAN' "$LOGDIR/gate-w21-tripwire.out" || { echo RED: tripwire banner; exit 1; }
twcount=$(grep -oE '[0-9]+ (entries|scanned|items|files)' "$LOGDIR/gate-w21-tripwire.out" | head -1 || true)
echo "ANCHORS GREEN (nine verbatim tokens across the wave slices; honest-posture sentences kept verbatim in the docs table and the mirror carrier; tripwire CLEAN ${twcount:-count not printed})"

printf '\nGATE-W21: ALL GREEN\n'
