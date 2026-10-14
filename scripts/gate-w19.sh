#!/usr/bin/env bash
# gate-w19.sh - machine-assertion battery for the memory-provenance wave
# (four day seats, three merges: the schema seat merged as PR#89 (typed
# memory provenance observation record: docs contract section plus the Go
# record type, its test and the schema checker mirror), the README
# feedback-pin seat as PR#90 (a product-face pointer to the planned team
# edition interest page that records interest only and charges nothing),
# the corpus-and-wire seat as PR#91 (the optional event-wire field, four
# normal-side allow shapes and their five mirror carriers), and the
# remaining day seats of the wave registered as zero-write reconciliation
# reads that must show no extra merge anywhere in the wave face).
#
# First edition, parameter-changed from the gate-w18 skeleton (whole-file
# authoring discipline; shared scaffolding kept byte-shape: TMPDIR residue
# cleanup, out-of-repo log directory, PATH fallback, closed-set re-parsing
# at run time, inherited chain, tripwire closer).
#
# RE-PIN DECLARATION (why this wave's golden face moves and exactly how
# far): the wave legitimately ships four new normal-side allow cases, so
#   - testdata/golden/normal.jsonl +4/-0 and labels.json +20/-0 are pure
#     appends; every byte of the pre-existing 149-case streams is fenced
#     unchanged by the append-only assertions below (the standing "old
#     golden bytes stay red" rule is enforced here, not waived), and the
#     danger stream is fenced byte-identical because this wave ships no
#     new danger case at all;
#   - coverage-map.json is the mirror carrier whose rows the shipped
#     checker re-derives against the freshly appended corpus (doc total
#     149 to 153, normal 76 to 80) while the memory-poisoning class row
#     stays rules-empty and cases-empty exactly as before;
#   - thresholds.json ledger readings are re-nailed from this wave's own
#     fresh eval-runner output because the denominator moved with the
#     added cases (76 to 80 normal, 149 to 153 total).
# Evidence for the re-pin: the wave handoff rows recorded in the build
# ledger for the schema seat, the docs-pin seat and the corpus+wire seat
# (ledger file paths deliberately stay off the product face). The W15,
# W16, W17 and W18 pins inside the inherited gates stay exactly as they
# were. Any red exits non-zero.
set -euo pipefail
cd "$(dirname "$0")/.."

export TMPDIR="$(pwd)/.tmptest"
mkdir -p "$TMPDIR"
find "$TMPDIR" -mindepth 1 -maxdepth 1 -name 'go-build*' -exec rm -rf {} + 2>/dev/null || true
command -v go >/dev/null 2>&1 || export PATH="$HOME/.local/go/bin:$PATH"
LOGDIR=/home/node/gate-logs
mkdir -p "$LOGDIR"

step() { printf '\n=== %s ===\n' "$*"; }

step '00 preflight: wave artifacts repo-visible right now (record type, wire field, all golden faces, docs mirrors, checker, README pin, three inherited gates)'
for f in internal/schema/memoryprofile.go internal/schema/memoryprofile_test.go \
         internal/schema/event.go internal/schema/memorywire_test.go \
         cmd/agent-collector/replay_cli_test.go \
         scripts/coverage-map-check.mjs scripts/threshold-ledger-check.mjs scripts/threatmodel-check.mjs \
         scripts/apicontract-check.mjs scripts/schema-v2-check.mjs scripts/tripwire.sh \
         docs/evals-coverage-v0.md docs/threat-model.md docs/api-v0.md docs/schema-v2.md README.md \
         testdata/golden/danger.jsonl testdata/golden/normal.jsonl testdata/golden/labels.json \
         testdata/golden/coverage-map.json testdata/golden/thresholds.json; do
  test -f "$f" || { echo "RED: wave artifact missing: $f"; exit 1; }
done
test -f scripts/gate-w19.sh || { echo 'RED: this gate not on disk'; exit 1; }
test -f scripts/gate-w18.sh || { echo 'RED: inherited gate-w18 missing from disk'; exit 1; }
test -f scripts/gate-w17.sh || { echo 'RED: inherited gate-w17 missing from disk'; exit 1; }
test -f scripts/gate-w16.sh || { echo 'RED: inherited gate-w16 missing from disk'; exit 1; }
echo "PREFLIGHT GREEN (record type, wire field, checkers, golden faces, docs mirrors, README pin and the inherited gate ladder all visible on disk)"

step '01 checker full runs: map lockstep against the freshly appended corpus, ledger bound to THIS run eval output, threat model, api contract, schema-v2 regression (including the new record contract section)'
node scripts/coverage-map-check.mjs > "$LOGDIR/gate-w19-map.out" 2>&1 \
  || { tail -20 "$LOGDIR/gate-w19-map.out"; echo RED: coverage-map-check; exit 1; }
grep -q 'MAP OK' "$LOGDIR/gate-w19-map.out" || { echo 'RED: map banner missing'; exit 1; }
go test ./internal/rules/ -run TestGoldenEvals -v -count=1 -p 1 -parallel 1 > "$LOGDIR/gate-w19-eval.out" 2>&1 \
  || { tail -25 "$LOGDIR/gate-w19-eval.out"; echo 'RED: eval suite failed (ledger has no live source to bind to)'; exit 1; }
grep -q 'EVAL SUMMARY' "$LOGDIR/gate-w19-eval.out" || { echo 'RED: no EVAL SUMMARY in runner output'; exit 1; }
node scripts/threshold-ledger-check.mjs "$LOGDIR/gate-w19-eval.out" > "$LOGDIR/gate-w19-ledger.out" 2>&1 \
  || { tail -20 "$LOGDIR/gate-w19-ledger.out"; echo RED: threshold-ledger-check; exit 1; }
grep -q 'LEDGER OK' "$LOGDIR/gate-w19-ledger.out" || { echo 'RED: ledger banner missing'; exit 1; }
node scripts/threatmodel-check.mjs > "$LOGDIR/gate-w19-threat.out" 2>&1 \
  || { tail -20 "$LOGDIR/gate-w19-threat.out"; echo RED: threatmodel-check; exit 1; }
grep -q 'THREAT' "$LOGDIR/gate-w19-threat.out" || { echo 'RED: threat banner moved out of lockstep'; exit 1; }
node scripts/apicontract-check.mjs > "$LOGDIR/gate-w19-apicontract.out" 2>&1 \
  || { tail -20 "$LOGDIR/gate-w19-apicontract.out"; echo RED: apicontract-check; exit 1; }
grep -q 'API-CONTRACT OK' "$LOGDIR/gate-w19-apicontract.out" || { echo 'RED: apicontract banner missing'; exit 1; }
node scripts/schema-v2-check.mjs > "$LOGDIR/gate-w19-v2check.out" 2>&1 \
  || { tail -20 "$LOGDIR/gate-w19-v2check.out"; echo RED: schema-v2-check; exit 1; }
grep -q 'SCHEMA-V2: ALL GREEN' "$LOGDIR/gate-w19-v2check.out" || { echo 'RED: v2-check banner missing'; exit 1; }
echo "CHECKERS GREEN (map lockstep, ledger re-nailed against this run's eval output, threat/api/schema batteries in force)"

step '02 positive controls: both wave checkers fully catch their own injections, both golden streams validate, the replay CLI total test passes at its pinned total'
mv=$(node scripts/coverage-map-check.mjs --selftest 2>&1 | tail -1)
printf '%s' "$mv" | grep -q 'SELFTEST OK' || { echo "RED: map selftest not green: $mv"; exit 1; }
lv=$(node scripts/threshold-ledger-check.mjs --selftest "$LOGDIR/gate-w19-eval.out" 2>&1 | tail -1)
printf '%s' "$lv" | grep -q 'SELFTEST OK' || { echo "RED: ledger selftest not green: $lv"; exit 1; }
node scripts/validate-jsonl.mjs testdata/golden/danger.jsonl > "$LOGDIR/gate-w19-dv.out" 2>&1 \
  || { tail -5 "$LOGDIR/gate-w19-dv.out"; echo RED: danger stream failed validation; exit 1; }
node scripts/validate-jsonl.mjs testdata/golden/normal.jsonl > "$LOGDIR/gate-w19-nv.out" 2>&1 \
  || { tail -5 "$LOGDIR/gate-w19-nv.out"; echo RED: normal stream failed validation; exit 1; }
go test ./cmd/agent-collector/ -run TestReplayCLIGoldenRun -count=1 > "$LOGDIR/gate-w19-replay.out" 2>&1 \
  || { tail -15 "$LOGDIR/gate-w19-replay.out"; echo RED: replay CLI golden run test; exit 1; }
echo "POSITIVE CONTROLS GREEN (map and ledger selftests fully caught, both streams valid, replay CLI golden run passes at its pinned total)"

step '03 wave diff face at pinned endpoints plus registration reconciliation: three registered merges are ancestors and the zero-write seats show no extra merge'
W19_BASE=61f551f01d8b81fb3556674228c8552fe1382677
W19_END=a4c5f3628c2ffc40d8ccf5b86618067560dc29d9
[ "${#W19_BASE}" -eq 40 ] && [ "${#W19_END}" -eq 40 ] || { echo 'RED: wave endpoint pins malformed'; exit 1; }
git merge-base --is-ancestor "$W19_BASE" HEAD || { echo 'RED: wave base endpoint not an ancestor of HEAD'; exit 1; }
git merge-base --is-ancestor "$W19_END" HEAD || { echo 'RED: wave end endpoint not an ancestor of HEAD'; exit 1; }
stray=$(git diff --name-only "${W19_BASE}..${W19_END}" | grep -vE '^(docs/|internal/|scripts/|cmd/agent-collector/replay_cli_test\.go$|testdata/golden/|README\.md$)' || true)
[ -z "$stray" ] || { echo 'RED: wave diff face carries files outside the allowed categories:'; echo "$stray"; exit 1; }
goldface=$(git diff --name-only "${W19_BASE}..${W19_END}" | grep -E '^testdata/golden/' | sort)
want='testdata/golden/coverage-map.json
testdata/golden/labels.json
testdata/golden/normal.jsonl
testdata/golden/thresholds.json'
[ "$goldface" = "$want" ] || { echo 'RED: golden face is not exactly the four declared carriers:'; echo "$goldface"; exit 1; }
git diff --quiet "${W19_BASE}..${W19_END}" -- testdata/golden/danger.jsonl || { echo 'RED: danger stream moved this wave - the record-only wire ships no new danger case'; exit 1; }
delnorm=$(git diff -U0 "${W19_BASE}..${W19_END}" -- testdata/golden/normal.jsonl | grep -c '^-[^-]' || true)
dellabel=$(git diff -U0 "${W19_BASE}..${W19_END}" -- testdata/golden/labels.json | grep -c '^-[^-]' || true)
[ "$delnorm" = "0" ] || { echo "RED: normal.jsonl is not append-only ($delnorm removed lines) - old golden bytes stay red"; exit 1; }
[ "$dellabel" = "0" ] || { echo "RED: labels.json is not append-only ($dellabel removed lines) - the standing label face must be untouched"; exit 1; }
mrg=$(git log --format='%s' "${W19_BASE}..${W19_END}" | grep -c 'Merge pull request' || true)
[ "$mrg" -eq 3 ] || { echo "RED: wave carries $mrg merge commits, want exactly the three registered PR merges (PR#89 #90 #91; the other day seats are registered zero-write)"; exit 1; }
for sha in 6ad22313301e4f21cf3fe5ec7f77bdeec1ecf1af 0997c15f861f62024e228df8d1832881924548a9 \
           a4c5f3628c2ffc40d8ccf5b86618067560dc29d9; do
  git merge-base --is-ancestor "$sha" HEAD || { echo "RED: registered merge sha not an ancestor: $sha"; exit 1; }
done
grep -q 'func BuildMemoryProvenance' internal/schema/memoryprofile.go || { echo 'RED: ticket-1 registered artifact (the typed record constructor) not on disk'; exit 1; }
grep -q '"id":"g7-m04"' testdata/golden/normal.jsonl || { echo 'RED: wave corpus tail case g7-m04 not on disk'; exit 1; }
echo "WAVE DIFF FACE CLEAN + REGISTERED MERGES RECONCILED (${mrg} registered merges all ancestors and no fourth, docs+internal+scripts+README plus the replay pin and exactly the four golden carriers, normal and labels fenced append-only, danger byte-identical, constructor and corpus tail present on disk)"

step '04 slice machine checks: four-source lock (corpus line, label key, mirror carrier, docs mirror) re-derived at run time; memory-poisoning row stays rules-empty and cases-empty; totals re-counted from disk'
node -e '
const { execSync } = require("child_process");
const fs = require("fs");
const base = execSync("git show 61f551f01d8b81fb3556674228c8552fe1382677:testdata/golden/normal.jsonl").toString().trim().split("\n").map(l=>JSON.parse(l).id);
const cur = fs.readFileSync("testdata/golden/normal.jsonl","utf8").trim().split("\n").map(l=>JSON.parse(l));
const ids = cur.map(r=>r.id);
const fresh = ids.filter(i=>!base.includes(i));
if (fresh.length !== 4) { console.log("RED: wave appended "+fresh.length+" normal ids, want the four registered (g7-m01..g7-m04)"); process.exit(1); }
for (const want of ["g7-m01","g7-m02","g7-m03","g7-m04"]) {
  if (!fresh.includes(want)) { console.log("RED: registered id missing from the append face: "+want); process.exit(1); }
}
const labels = JSON.parse(fs.readFileSync("testdata/golden/labels.json","utf8"));
const mapObj = JSON.parse(fs.readFileSync("testdata/golden/coverage-map.json","utf8"));
const mapRaw = fs.readFileSync("testdata/golden/coverage-map.json","utf8");
for (const id of fresh) {
  const lab = labels[id];
  if (!lab) { console.log("RED: appended "+id+" has no label key (labels lockstep broken)"); process.exit(1); }
  if (lab.expect !== "allow") { console.log("RED: "+id+" expect is not allow (the wire ships no judgement teeth - allow shapes only)"); process.exit(1); }
  if (!mapRaw.includes("\""+id+"\"")) { console.log("RED: "+id+" absent from the mirror carrier"); process.exit(1); }
}
const mp = (mapObj.security_classes||[]).find(x=>x && x.name === "Memory Poisoning");
if (!mp) { console.log("RED: memory-poisoning class row vanished from the mirror carrier"); process.exit(1); }
if ((mp.rules||[]).length !== 0 || (mp.cases||[]).length !== 0 || (mp.primitive_cases||[]).length !== 0) {
  console.log("RED: memory-poisoning row moved out of its rules-empty cases-empty stance (no detection semantics shipped)"); process.exit(1);
}
const docs = fs.readFileSync("docs/evals-coverage-v0.md","utf8");
for (const a of ["g7-m01","g7-m04"]) {
  if (!docs.includes(a)) { console.log("RED: docs mirror lost append anchor "+a); process.exit(1); }
}
if (!/no shipped rule|matches no shipped rule/.test(mp.note||"")) {
  console.log("RED: the record-not-authorization note lost its shipped-rule sentence"); process.exit(1);
}
console.log("LOCKSTEP OK (four fresh ids present in corpus, labels and mirror carrier with expect=allow; docs anchors present; memory-poisoning row fenced empty)");
' || exit 1
if grep -r 'memory_provenance' internal/rules/ 2>/dev/null; then
  echo 'RED: a shipped rule reads the memory-provenance field (records are evidence, never authorization)'; exit 1;
fi
surfaces=$(grep -c 'MemorySurface = "' internal/schema/memoryprofile.go | tr -cd '0-9')
[ "$surfaces" -eq 6 ] || { echo "RED: memory surface closed set re-count $surfaces, want the six shipped tokens"; exit 1; }
dn=$(wc -l < testdata/golden/danger.jsonl | tr -cd '0-9')
nn=$(wc -l < testdata/golden/normal.jsonl | tr -cd '0-9')
tot=$((dn + nn))
[ "$dn" -eq 73 ] || { echo "RED: danger stream re-count $dn, want the pinned 73 (unchanged by this wave)"; exit 1; }
[ "$nn" -eq 80 ] || { echo "RED: normal stream re-count $nn, want the pinned 80"; exit 1; }
grep -q "$tot" cmd/agent-collector/replay_cli_test.go || { echo "RED: replay CLI total pin no longer names the re-counted total $tot"; exit 1; }
echo "SLICE CHECKS GREEN (four-source lock re-derived at run time: g7-m01..m04 across corpus, labels (allow-only), mirror and docs; no rule reads the wire field; six-surface closed set; totals re-counted $dn danger + $nn normal = $tot matching the replay CLI pin)"

step '05 record-only invariants: decision closed set, built-in rule count re-derived and untouched by the wave, teeth vocabulary absent from the wave docs face plus the README pin, plane census all-none re-derived'
grep -qF '{allow, would_block}' docs/api-v0.md || { echo 'RED: Phase 0 decision closed-set anchor lost from the api contract'; exit 1; }
if grep -qiE '\b(auto_deny|deny_list|hard_block|enforce_now)\b' docs/evals-coverage-v0.md docs/threat-model.md docs/api-v0.md README.md; then
  echo 'RED: enforcement-teeth vocabulary inside the wave docs face or the README pin'; exit 1;
fi
rc=$(grep -c '"priority":' internal/rules/rules.go | tr -cd '0-9')
const=$(grep -oE 'BuiltinRuleCount = [0-9]+' internal/rules/rules.go | grep -oE '[0-9]+')
[ "$rc" -eq "$const" ] || { echo "RED: built-in rule rows re-count $rc but the pinned constant says $const"; exit 1; }
git diff --quiet 61f551f01d8b81fb3556674228c8552fe1382677..HEAD -- internal/rules/rules.go || { echo 'RED: the shipped rule table moved during the wave - this wave is record-only'; exit 1; }
n=$(grep -cE '^[a-z_]+_(enforcement|execution)_plane: none-in-observation-phase$' docs/schema-v2.md)
t=$(grep -cE '^[a-z_]+_(enforcement|execution)_plane: ' docs/schema-v2.md)
[ "$n" -eq "$t" ] || { echo "RED: a declared plane was promoted out of observation stance ($n/$t)"; exit 1; }
[ "$n" -ge 25 ] || { echo "RED: plane census $n, want at least 25 none-lines"; exit 1; }
echo "RECORD-ONLY GREEN (closed-set anchor present, teeth vocabulary absent from the wave docs plus README, rule table provably untouched by the wave at re-count $rc==$const, plane census all-none $n/$t re-derived)"

step '06 inherited chain: gate-w18 full battery (which itself re-runs gate-w17 down through gate-d1 serially, the gate-d7 Go suite and the gate-d5 release assembly, with the W15/W16/W17/W18 pins untouched)'
bash scripts/gate-w18.sh > "$LOGDIR/gate-w18-inside-w19.out" 2>&1 \
  || { tail -30 "$LOGDIR/gate-w18-inside-w19.out"; echo RED: gate-w18 regression; exit 1; }
grep -q 'GATE-W18: ALL GREEN' "$LOGDIR/gate-w18-inside-w19.out" \
  || { echo 'RED: gate-w18 completion banner missing'; exit 1; }
echo 'GATE-W18 GREEN INSIDE GATE-W19 (zero regression on the full inherited chain down through gate-d1 with the W15/W16/W17/W18 pins untouched)'

step '07 verbatim contract anchors from all wave slices plus tripwire standalone (internal-ledger negative scan)'
anch() {
  grep -qF "$2" "$1" || { echo "RED: wave anchor lost in $1: $2"; exit 1; }
}
anch internal/schema/event.go 'memory_provenance,omitempty'
anch internal/schema/event.go 'Memory is not Authorization'
anch docs/api-v0.md 'memory_provenance'
anch docs/schema-v2.md '## 41. Memory provenance observation record contract (slice W12.2)'
anch README.md 'records interest only'
anch testdata/golden/labels.json '"g7-m01"'
anch testdata/golden/labels.json '"g7-m04"'
anch testdata/golden/coverage-map.json 'Memory is not Authorization'
bash scripts/tripwire.sh > "$LOGDIR/gate-w19-tripwire.out" 2>&1 \
  || { cat "$LOGDIR/gate-w19-tripwire.out"; echo RED: tripwire; exit 1; }
grep -q 'TRIPWIRE CLEAN' "$LOGDIR/gate-w19-tripwire.out" || { echo RED: tripwire banner; exit 1; }
twcount=$(grep -oE '[0-9]+ (entries|scanned|items|files)' "$LOGDIR/gate-w19-tripwire.out" | head -1 || true)
echo "ANCHORS GREEN (eight verbatim tokens across the wave slices; tripwire CLEAN ${twcount:-count not printed})"

printf '\nGATE-W19: ALL GREEN\n'
