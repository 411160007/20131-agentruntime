#!/usr/bin/env bash
# gate-w20.sh - machine-assertion battery for the downstream-capture corpus wave
# (four night seats, four merges: the data-exfiltration first wave merged as
# PR#93 (seven direct attempts gd-30 to gd-36 each mirroring one shipped
# primitive effect), the data-exfiltration second wave as PR#94 (four further
# attempts gd-37 to gd-40 on the same shipped vocabulary), the skill-abuse
# first wave as PR#95 (four attempts gd-41 to gd-44 behind the eval-exposing
# tool primitive) and the supply-chain first wave as PR#96 (four attempts
# gd-45 to gd-48 behind the credential-store and fetch-and-run primitives).
# The same PR family also carries one mechanism-side repair: the ledger
# checker's own self-test negative controls are now derived at a fixed offset
# from the fresh runner reading instead of being hand-tweaked literals, after
# a wave whose legitimate percentage collided with an old hardcoded control
# would have made that rejection vacuous.
#
# First edition, parameter-changed from the gate-w19 skeleton (whole-file
# authoring discipline; shared scaffolding kept byte-shape: TMPDIR residue
# cleanup, out-of-repo log directory, PATH fallback, closed-set re-parsing
# at run time, tripwire closer). One deliberate shape change: the inherited
# ladder re-runs gate-w19 and below at a detached worktree of this wave's
# base commit, because the corpus re-counts asserted inside those older
# gates are wave-relative facts certified at their own wave tails - this
# wave legitimately moved the danger stream, so replaying those absolute
# re-counts at the new head would be a checker shape artifact, not a
# regression. The moved face of this wave is certified at the real head by
# steps 01 to 05, and every old gate stays green exactly where it left it.
#
# RE-PIN DECLARATION (why this wave's golden face moves and exactly how far):
# the wave legitimately ships nineteen new danger-side direct-attempt cases
# (seven plus four plus four plus four across the four seats), so
#   - testdata/golden/danger.jsonl +19/-0 and labels.json +95/-0 are pure
#     appends; every byte of the pre-existing streams is fenced unchanged by
#     the append-only assertions below (the standing "old golden bytes stay
#     red" rule is enforced here, not waived), and the normal stream is
#     fenced byte-identical because this wave ships no new normal case at all;
#   - coverage-map.json is the mirror carrier whose rows the shipped checker
#     re-derives against the freshly appended corpus (doc total 153 to 172,
#     danger 73 to 92; the Data Exfiltration primitive row moves 18 to 29,
#     Skill Abuse 8 to 12, Supply Chain Attack 6 to 10, while the
#     memory-poisoning row stays rules-empty and cases-empty exactly as
#     before);
#   - thresholds.json ledger readings are re-nailed from this wave's own
#     fresh eval-runner output because the denominator moved with the added
#     cases (detection 78/80 through 82/84 and 86/88 up to 90/92, pct 97.8),
#     with the missed set unchanged at the two declared known gaps.
# Evidence for the re-pin: the wave handoff rows recorded in the build ledger
# (ledger file paths deliberately stay off the product face). The W15, W16,
# W17, W18 and W19 pins inside the inherited gates stay exactly as they were.
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
test -f scripts/gate-w20.sh || { echo 'RED: this gate not on disk'; exit 1; }
test -f scripts/gate-w19.sh || { echo 'RED: inherited gate-w19 missing from disk'; exit 1; }
test -f scripts/gate-w18.sh || { echo 'RED: inherited gate-w18 missing from disk'; exit 1; }
test -f scripts/gate-w17.sh || { echo 'RED: inherited gate-w17 missing from disk'; exit 1; }
echo "PREFLIGHT GREEN (corpus faces, docs table, both checkers, replay pin and the inherited gate ladder all visible on disk)"

step '01 checker full runs: map lockstep against the freshly appended corpus, ledger bound to THIS run eval output, threat model, api contract, schema-v2 regression'
node scripts/coverage-map-check.mjs > "$LOGDIR/gate-w20-map.out" 2>&1 \
  || { tail -20 "$LOGDIR/gate-w20-map.out"; echo RED: coverage-map-check; exit 1; }
grep -q 'MAP OK' "$LOGDIR/gate-w20-map.out" || { echo 'RED: map banner missing'; exit 1; }
go test ./internal/rules/ -run TestGoldenEvals -v -count=1 -p 1 -parallel 1 > "$LOGDIR/gate-w20-eval.out" 2>&1 \
  || { tail -25 "$LOGDIR/gate-w20-eval.out"; echo 'RED: eval suite failed (ledger has no live source to bind to)'; exit 1; }
grep -q 'EVAL SUMMARY' "$LOGDIR/gate-w20-eval.out" || { echo 'RED: no EVAL SUMMARY in runner output'; exit 1; }
node scripts/threshold-ledger-check.mjs "$LOGDIR/gate-w20-eval.out" > "$LOGDIR/gate-w20-ledger.out" 2>&1 \
  || { tail -20 "$LOGDIR/gate-w20-ledger.out"; echo RED: threshold-ledger-check; exit 1; }
grep -q 'LEDGER OK' "$LOGDIR/gate-w20-ledger.out" || { echo 'RED: ledger banner missing'; exit 1; }
node scripts/threatmodel-check.mjs > "$LOGDIR/gate-w20-threat.out" 2>&1 \
  || { tail -20 "$LOGDIR/gate-w20-threat.out"; echo RED: threatmodel-check; exit 1; }
grep -q 'THREAT' "$LOGDIR/gate-w20-threat.out" || { echo 'RED: threat banner moved out of lockstep'; exit 1; }
node scripts/apicontract-check.mjs > "$LOGDIR/gate-w20-apicontract.out" 2>&1 \
  || { tail -20 "$LOGDIR/gate-w20-apicontract.out"; echo RED: apicontract-check; exit 1; }
grep -q 'API-CONTRACT OK' "$LOGDIR/gate-w20-apicontract.out" || { echo 'RED: apicontract banner missing'; exit 1; }
node scripts/schema-v2-check.mjs > "$LOGDIR/gate-w20-v2check.out" 2>&1 \
  || { tail -20 "$LOGDIR/gate-w20-v2check.out"; echo RED: schema-v2-check; exit 1; }
grep -q 'SCHEMA-V2: ALL GREEN' "$LOGDIR/gate-w20-v2check.out" || { echo 'RED: v2-check banner missing'; exit 1; }
echo "CHECKERS GREEN (map lockstep, ledger re-nailed against this run's eval output, threat/api/schema batteries in force)"

step '02 positive controls: both corpus checkers fully catch their own injections, both golden streams validate, the replay CLI total test passes at its pinned total'
mv=$(node scripts/coverage-map-check.mjs --selftest 2>&1 | tail -1)
printf '%s' "$mv" | grep -q 'SELFTEST OK' || { echo "RED: map selftest not green: $mv"; exit 1; }
lv=$(node scripts/threshold-ledger-check.mjs --selftest "$LOGDIR/gate-w20-eval.out" 2>&1 | tail -1)
printf '%s' "$lv" | grep -q 'SELFTEST OK' || { echo "RED: ledger selftest not green: $lv"; exit 1; }
node scripts/validate-jsonl.mjs testdata/golden/danger.jsonl > "$LOGDIR/gate-w20-dv.out" 2>&1 \
  || { tail -5 "$LOGDIR/gate-w20-dv.out"; echo RED: danger stream failed validation; exit 1; }
node scripts/validate-jsonl.mjs testdata/golden/normal.jsonl > "$LOGDIR/gate-w20-nv.out" 2>&1 \
  || { tail -5 "$LOGDIR/gate-w20-nv.out"; echo RED: normal stream failed validation; exit 1; }
go test ./cmd/agent-collector/ -run TestReplayCLIGoldenRun -count=1 > "$LOGDIR/gate-w20-replay.out" 2>&1 \
  || { tail -15 "$LOGDIR/gate-w20-replay.out"; echo RED: replay CLI golden run test; exit 1; }
echo "POSITIVE CONTROLS GREEN (map and ledger selftests fully caught, both streams valid, replay CLI golden run passes at its pinned total)"

step '03 wave diff face at pinned endpoints plus registration reconciliation: four registered merges are ancestors and the face carries nothing outside the corpus-and-mirror categories'
W20_BASE=912db0a6d9c1ef34ca4fdf81bb6b842568b57cfc
W20_END=910de6e63150b3df1c87cb56ed5cf3011cba78bc
[ "${#W20_BASE}" -eq 40 ] && [ "${#W20_END}" -eq 40 ] || { echo 'RED: wave endpoint pins malformed'; exit 1; }
git merge-base --is-ancestor "$W20_BASE" HEAD || { echo 'RED: wave base endpoint not an ancestor of HEAD'; exit 1; }
git merge-base --is-ancestor "$W20_END" HEAD || { echo 'RED: wave end endpoint not an ancestor of HEAD'; exit 1; }
stray=$(git diff --name-only "${W20_BASE}..${W20_END}" | grep -vE '^(docs/|scripts/|cmd/agent-collector/replay_cli_test\.go$|testdata/golden/)' || true)
[ -z "$stray" ] || { echo 'RED: wave diff face carries files outside the allowed categories:'; echo "$stray"; exit 1; }
goldface=$(git diff --name-only "${W20_BASE}..${W20_END}" | grep -E '^testdata/golden/' | sort)
want='testdata/golden/coverage-map.json
testdata/golden/danger.jsonl
testdata/golden/labels.json
testdata/golden/thresholds.json'
[ "$goldface" = "$want" ] || { echo 'RED: golden face is not exactly the four declared carriers:'; echo "$goldface"; exit 1; }
git diff --quiet "${W20_BASE}..${W20_END}" -- testdata/golden/normal.jsonl || { echo 'RED: normal stream moved this wave - the corpus ticket ships no new normal case'; exit 1; }
delanger=$(git diff -U0 "${W20_BASE}..${W20_END}" -- testdata/golden/danger.jsonl | grep -c '^-[^-]' || true)
addanger=$(git diff -U0 "${W20_BASE}..${W20_END}" -- testdata/golden/danger.jsonl | grep -c '^+[^+]' || true)
[ "$delanger" = "0" ] || { echo "RED: danger.jsonl is not append-only ($delanger removed lines) - old golden bytes stay red"; exit 1; }
[ "$addanger" = "19" ] || { echo "RED: danger.jsonl appended $addanger lines, want exactly the nineteen registered case lines"; exit 1; }
dellabel=$(git diff -U0 "${W20_BASE}..${W20_END}" -- testdata/golden/labels.json | grep -c '^-[^-]' || true)
[ "$dellabel" = "0" ] || { echo "RED: labels.json is not append-only ($dellabel removed lines) - the standing label face must be untouched"; exit 1; }
git diff --quiet "${W20_BASE}..${W20_END}" -- internal/ || { echo 'RED: shipped Go code moved during the corpus wave - the tickets mirror shipped rules only'; exit 1; }
mrg=$(git log --format='%s' "${W20_BASE}..${W20_END}" | grep -c 'Merge pull request' || true)
[ "$mrg" -eq 4 ] || { echo "RED: wave carries $mrg merge commits, want exactly the four registered PR merges (PR#93 #94 #95 #96)"; exit 1; }
for sha in dc81950dbbfb78df3cdd165561345bb8c01ae198 b8647b24f9a0cf5f600c015176770e2c3a311055 \
           08ae23ef3a6883b481e02b5471f26763d558b6b2 910de6e63150b3df1c87cb56ed5cf3011cba78bc; do
  git merge-base --is-ancestor "$sha" HEAD || { echo "RED: registered merge sha not an ancestor: $sha"; exit 1; }
done
grep -q '"id":"gd-48"' testdata/golden/danger.jsonl || { echo 'RED: wave corpus tail case gd-48 not on disk'; exit 1; }
grep -q 'realPct' scripts/threshold-ledger-check.mjs || { echo 'RED: the ledger checker lost its fresh-reading-derived negative controls'; exit 1; }
echo "WAVE DIFF FACE CLEAN + REGISTERED MERGES RECONCILED (${mrg} registered merges all ancestors and no fifth, docs+scripts+replay pin plus exactly the four golden carriers, danger fenced append-only at exactly nineteen added lines, labels append-only, normal byte-identical, internal Go untouched, tail case and repaired checker present on disk)"

step '04 slice machine checks: four-source lock (corpus line, label key, mirror carrier, docs mirror) re-derived at run time for all nineteen appended cases; per-row primitive lockstep; missed set unchanged'
node -e '
const { execSync } = require("child_process");
const fs = require("fs");
const base = execSync("git show 912db0a6d9c1ef34ca4fdf81bb6b842568b57cfc:testdata/golden/danger.jsonl").toString().trim().split("\n").map(l=>JSON.parse(l).id);
const cur = fs.readFileSync("testdata/golden/danger.jsonl","utf8").trim().split("\n").map(l=>JSON.parse(l));
const ids = cur.map(r=>r.id);
const fresh = ids.filter(i=>!base.includes(i));
const want = [];
for (let n=30;n<=48;n++) want.push("gd-"+String(n));
if (fresh.length !== 19 || !want.every(w=>fresh.includes(w))) {
  console.log("RED: wave appended "+fresh.join(",")+" - want exactly the nineteen registered ids gd-30..gd-48"); process.exit(1);
}
const labels = JSON.parse(fs.readFileSync("testdata/golden/labels.json","utf8"));
const mapObj = JSON.parse(fs.readFileSync("testdata/golden/coverage-map.json","utf8"));
for (const id of fresh) {
  const lab = labels[id];
  if (!lab) { console.log("RED: appended "+id+" has no label key (labels lockstep broken)"); process.exit(1); }
  if (lab.expect !== "would_block") { console.log("RED: "+id+" expect is not would_block (direct attempts must be booked as downstream-capture hits, never silent passes)"); process.exit(1); }
}
const rowOf = (name)=>{const r=mapObj.security_classes.find(x=>x&&x.name===name); if(!r){console.log("RED: class row vanished: "+name);process.exit(1);} return (r.primitive_cases||[]);};
const de=rowOf("Data Exfiltration"), sa=rowOf("Skill Abuse"), sc=rowOf("Supply Chain Attack");
if (de.length!==29 || sa.length!==12 || sc.length!==10) { console.log("RED: primitive row counts moved out of lockstep (DE "+de.length+" want 29, SA "+sa.length+" want 12, SCA "+sc.length+" want 10)"); process.exit(1); }
for (const id of fresh) {
  const homes=[de,sa,sc].filter(r=>r.includes(id));
  if (homes.length !== 1) { console.log("RED: "+id+" sits in "+homes.length+" mirror rows, want exactly one (per-seat reverse lookup)"); process.exit(1); }
}
for (const [grp,lo,hi,row] of [["DE",30,40,de],["SA",41,44,sa],["SC",45,48,sc]]) {
  for (let n=lo;n<=hi;n++){const id="gd-"+n; if(!row.includes(id)){console.log("RED: seat-row mismatch: "+id+" not in the "+grp+" row");process.exit(1);}}
}
const mp = mapObj.security_classes.find(x=>x && x.name === "Memory Poisoning");
if (!mp || (mp.rules||[]).length !== 0 || (mp.cases||[]).length !== 0 || (mp.primitive_cases||[]).length !== 0) {
  console.log("RED: memory-poisoning row moved out of its rules-empty cases-empty stance"); process.exit(1);
}
const docs = fs.readFileSync("docs/evals-coverage-v0.md","utf8");
for (const a of ["gd-30","gd-37","gd-41","gd-45","gd-48"]) {
  if (!docs.includes(a)) { console.log("RED: docs mirror lost append anchor "+a); process.exit(1); }
}
const corpus = mapObj.corpus || {};
if (corpus.normal!==80 || corpus.danger!==92 || corpus.total!==172 || corpus.labels!==172) {
  console.log("RED: mirror carrier corpus block not re-nailed at 80/92/172/172"); process.exit(1);
}
console.log("LOCKSTEP OK (nineteen fresh ids gd-30..gd-48 present in corpus and labels with expect=would_block; each in exactly one mirror row and in its seat row (DE gd-30..gd-40, SA gd-41..gd-44, SC gd-45..gd-48); row counts 29/12/10; memory-poisoning row fenced empty; docs anchors present; carrier corpus block 80/92/172/172)");
' || exit 1
grep -F 'missed=[gd-19 gd-20]' "$LOGDIR/gate-w20-eval.out" > /dev/null \
  || { echo 'RED: the runner missed set moved off the two declared known gaps - a wave case went undetected'; exit 1; }
dn=$(wc -l < testdata/golden/danger.jsonl | tr -cd '0-9')
nn=$(wc -l < testdata/golden/normal.jsonl | tr -cd '0-9')
tot=$((dn + nn))
[ "$dn" -eq 92 ] || { echo "RED: danger stream re-count $dn, want the pinned 92"; exit 1; }
[ "$nn" -eq 80 ] || { echo "RED: normal stream re-count $nn, want the pinned 80 (unchanged by this wave)"; exit 1; }
lk=$(grep -c '"expect"' testdata/golden/labels.json | tr -cd '0-9')
[ "$lk" -eq "$tot" ] || { echo "RED: label keys re-count $lk against corpus total $tot"; exit 1; }
grep -q "$tot" cmd/agent-collector/replay_cli_test.go || { echo "RED: replay CLI total pin no longer names the re-counted total $tot"; exit 1; }
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
git diff --quiet 912db0a6d9c1ef34ca4fdf81bb6b842568b57cfc..HEAD -- internal/rules/rules.go || { echo 'RED: the shipped rule table moved during the wave - the tickets mirror shipped rules only'; exit 1; }
n=$(grep -cE '^[a-z_]+_(enforcement|execution)_plane: none-in-observation-phase$' docs/schema-v2.md)
t=$(grep -cE '^[a-z_]+_(enforcement|execution)_plane: ' docs/schema-v2.md)
[ "$n" -eq "$t" ] || { echo "RED: a declared plane was promoted out of observation stance ($n/$t)"; exit 1; }
[ "$n" -ge 26 ] || { echo "RED: plane census $n, want at least the 26 none-lines in force"; exit 1; }
echo "INVARIANTS GREEN (closed-set anchor present, teeth vocabulary absent, rule table provably untouched at re-count $rc==$const==13, plane census all-none $n/$t re-derived)"

step '06 inherited chain: gate-w19 ladder at its certified wave-tail state via a detached worktree pinned at this wave base (the corpus re-counts inside w19 and below are wave-relative facts certified at their own tails; the ladder must stay green exactly where each gate left it, and the moved face of THIS wave is certified by steps 01 to 05; the W15 through W19 pins and the gate files themselves stay untouched)'
BASEWT=/home/node/w20-base-wt
git rev-parse --verify "${W20_BASE}^{commit}" > /dev/null || { echo 'RED: wave base pin unresolvable'; exit 1; }
git worktree remove --force "$BASEWT" 2>/dev/null || true
git worktree add --detach "$BASEWT" "$W20_BASE" > /dev/null \
  || { echo 'RED: cannot stage the base worktree'; exit 1; }
( cd "$BASEWT" && bash scripts/gate-w19.sh > "$LOGDIR/gate-w19-inside-w20.out" 2>&1 ) \
  || { tail -30 "$LOGDIR/gate-w19-inside-w20.out"; echo RED: gate-w19 regression; git worktree remove --force "$BASEWT" 2>/dev/null || true; exit 1; }
grep -q 'GATE-W19: ALL GREEN' "$LOGDIR/gate-w19-inside-w20.out" \
  || { echo 'RED: gate-w19 completion banner missing'; git worktree remove --force "$BASEWT" 2>/dev/null || true; exit 1; }
git worktree remove --force "$BASEWT" || { echo 'RED: base worktree cleanup failed'; exit 1; }
if git worktree list | grep -q "$BASEWT"; then echo 'RED: base worktree still registered'; exit 1; fi
echo 'GATE-W19 GREEN AT ITS CERTIFIED STATE (full inherited ladder re-runs at the wave base exactly where each gate left it, W15 through W19 pins untouched, the base worktree cleaned up behind the run)'

step '07 verbatim contract anchors from all wave slices plus tripwire standalone (internal-ledger negative scan)'
anch() {
  grep -qF "$2" "$1" || { echo "RED: wave anchor lost in $1: $2"; exit 1; }
}
anch docs/evals-coverage-v0.md 'no exfiltration-semantics matcher ships'
anch docs/evals-coverage-v0.md 'no skill-vocabulary matcher ships'
anch docs/evals-coverage-v0.md 'package-integrity semantics are still not observed by shipped rules'
anch testdata/golden/coverage-map.json 'still no exfiltration-semantics matcher ships'
anch testdata/golden/danger.jsonl '"id":"gd-30"'
anch testdata/golden/danger.jsonl '"id":"gd-44"'
anch cmd/agent-collector/replay_cli_test.go 'want 172/0'
anch scripts/threshold-ledger-check.mjs 'a vacuous control'
bash scripts/tripwire.sh > "$LOGDIR/gate-w20-tripwire.out" 2>&1 \
  || { cat "$LOGDIR/gate-w20-tripwire.out"; echo RED: tripwire; exit 1; }
grep -q 'TRIPWIRE CLEAN' "$LOGDIR/gate-w20-tripwire.out" || { echo RED: tripwire banner; exit 1; }
twcount=$(grep -oE '[0-9]+ (entries|scanned|items|files)' "$LOGDIR/gate-w20-tripwire.out" | head -1 || true)
echo "ANCHORS GREEN (eight verbatim tokens across the wave slices; honest-posture sentences kept verbatim in the docs table and the mirror carrier; tripwire CLEAN ${twcount:-count not printed})"

printf '\nGATE-W20: ALL GREEN\n'
