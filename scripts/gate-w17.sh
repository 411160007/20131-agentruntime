#!/usr/bin/env bash
# gate-w17.sh - machine-assertion battery for the emission-and-reconcile
# wave (four seats, three writes: W1 tooling-side task.record emission v0,
# W2 recovery-transaction coverage-table reconcile, W3 external-intelligence
# contract seat confirmed already shipped with zero repo writes, W4 the
# shipped hook-intake receiver row joining the coverage-truthfulness matrix).
#
# First edition, parameter-changed from the gate-w16 skeleton (whole-file
# authoring discipline; shared scaffolding kept byte-shape: TMPDIR residue
# cleanup, out-of-repo log directory, PATH fallback, closed-set re-parsing at
# run time, inherited chain, tripwire closer). Unlike the previous wave this
# one moves NO runtime Go file at all and NO event-stream golden: the diff
# face is four docs, three scripts and exactly one mirror JSON
# (testdata/golden/coverage-truthfulness.json) which lives inside the golden
# directory but is the matrix mirror carrier, not a collector corpus sample -
# the standing pins below fence that shape precisely: zero *.jsonl bytes
# under testdata/golden, zero .go bytes anywhere in the wave.
#
# The wave registered four seats but only three merges: the W3 seat is a
# reconciliation read whose ledger row says zero writes, and the repository
# face must agree (exactly three merge commits between the pinned endpoints,
# no fourth). Census numbers are re-taken from disk on every run - matrix row
# counts parsed from both carriers, selftest fleet sizes read from this run's
# banners - never copied from a planning document. The wave-end endpoints are
# immutable history asserted as ancestors; the W15/W16 pins inside the
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

step '00 preflight: wave artifacts repo-visible; both matrix carriers and the emitter trio present on disk right now'
for f in scripts/task-record-emit.mjs scripts/task-map.mjs scripts/task-primitive-check.mjs \
         scripts/coverage-truth-check.mjs scripts/w12-recovery-check.mjs \
         docs/coverage-truthfulness.md docs/recovery-transaction-design.md \
         docs/recovery-honesty-grading.md docs/time-machine-v0.md docs/task-primitive-design.md docs/api-v0.md \
         testdata/golden/coverage-truthfulness.json testdata/taskrecord/good.jsonl testdata/taskrecord/wild.jsonl \
         scripts/validate-jsonl.mjs scripts/apicontract-check.mjs scripts/schema-v2-check.mjs \
         scripts/tripwire.sh testdata/golden/normal.jsonl testdata/golden/danger.jsonl; do
  test -f "$f" || { echo "RED: wave artifact missing: $f"; exit 1; }
done
test -f scripts/gate-w17.sh || { echo 'RED: this gate not on disk'; exit 1; }
test -f scripts/gate-w16.sh || { echo 'RED: inherited gate-w16 missing from disk'; exit 1; }
test -f scripts/gate-w15.sh || { echo 'RED: inherited gate-w15 missing from disk'; exit 1; }
echo "PREFLIGHT GREEN (wave artifacts visible; emitter, three checkers, five docs, mirror JSON and the untouched golden event streams all on disk)"

step '01 checker full runs: emission contract, truthfulness matrix lockstep, recovery-design reconcile, api contract, schema-v2 regression'
node scripts/task-primitive-check.mjs > "$LOGDIR/gate-w17-primitive.out" 2>&1 \
  || { tail -20 "$LOGDIR/gate-w17-primitive.out"; echo RED: task-primitive-check; exit 1; }
grep -q 'TASK-PRIMITIVE CHECK GREEN' "$LOGDIR/gate-w17-primitive.out" || { echo 'RED: primitive banner missing'; exit 1; }
node scripts/coverage-truth-check.mjs > "$LOGDIR/gate-w17-truth.out" 2>&1 \
  || { tail -20 "$LOGDIR/gate-w17-truth.out"; echo RED: coverage-truth-check; exit 1; }
grep -q 'TRUTH GREEN' "$LOGDIR/gate-w17-truth.out" || { echo 'RED: truth banner missing'; exit 1; }
node scripts/w12-recovery-check.mjs > "$LOGDIR/gate-w17-recovery.out" 2>&1 \
  || { tail -20 "$LOGDIR/gate-w17-recovery.out"; echo RED: w12-recovery-check; exit 1; }
grep -q 'W12 RECOVERY-DESIGN CHECK GREEN' "$LOGDIR/gate-w17-recovery.out" || { echo 'RED: recovery banner missing'; exit 1; }
grep -q '25 census tokens' "$LOGDIR/gate-w17-recovery.out" || { echo 'RED: recovery census phrasing moved out of lockstep'; exit 1; }
node scripts/apicontract-check.mjs > "$LOGDIR/gate-w17-apicontract.out" 2>&1 \
  || { tail -20 "$LOGDIR/gate-w17-apicontract.out"; echo RED: apicontract-check; exit 1; }
grep -q 'API-CONTRACT OK' "$LOGDIR/gate-w17-apicontract.out" || { echo 'RED: apicontract banner missing'; exit 1; }
node scripts/schema-v2-check.mjs > "$LOGDIR/gate-w17-v2check.out" 2>&1 \
  || { tail -20 "$LOGDIR/gate-w17-v2check.out"; echo RED: schema-v2-check; exit 1; }
grep -q 'SCHEMA-V2: ALL GREEN' "$LOGDIR/gate-w17-v2check.out" || { echo 'RED: v2-check banner missing'; exit 1; }
echo "CHECKERS GREEN (emission contract with check 8b reverse-lookup, matrix lockstep against shipped source, recovery reconcile with 25-token census, api contract untouched, schema-v2 regression)"

step '02 positive controls: every selftest fleet fully catches its own injections and the emitter replays byte-identical - counts read from this run'
pv=$(node scripts/task-primitive-check.mjs --selftest 2>&1 | tail -1)
printf '%s' "$pv" | grep -q 'SELFTEST GREEN' || { echo "RED: primitive selftest not green: $pv"; exit 1; }
pn=$(printf '%s' "$pv" | grep -oE '[0-9]+/[0-9]+' | head -1)
pa=${pn%/*}; pb=${pn#*/}
[ "$pa" = "$pb" ] && [ "$pa" -ge 17 ] || { echo "RED: primitive selftest fleet not fully caught or below floor: $pn"; exit 1; }
tv=$(node scripts/coverage-truth-check.mjs --selftest 2>&1 | tail -1)
tc=$(printf '%s' "$tv" | grep -oE '\([0-9]+ positive' | grep -oE '[0-9]+' | head -1)
printf '%s' "$tv" | grep -q 'SELFTEST GREEN' || { echo "RED: truth selftest not green: $tv"; exit 1; }
[ -n "$tc" ] && [ "$tc" -ge 14 ] || { echo "RED: truth selftest controls below floor: $tv"; exit 1; }
rv=$(node scripts/w12-recovery-check.mjs --selftest 2>&1 | tail -1)
printf '%s' "$rv" | grep -q 'SELFTEST GREEN' || { echo "RED: recovery selftest not green: $rv"; exit 1; }
rc=$(printf '%s' "$rv" | grep -oE '\([0-9]+ injected' | grep -oE '[0-9]+' | head -1)
[ -n "$rc" ] && [ "$rc" -ge 11 ] || { echo "RED: recovery selftest floor: $rv"; exit 1; }
ev=$(node scripts/task-record-emit.mjs --selftest 2>&1 | tail -1)
printf '%s' "$ev" | grep -qE 'SELFTEST GREEN.*records emitted' || { echo "RED: emitter selftest not green: $ev"; exit 1; }
printf '%s' "$ev" | grep -q 'input bytes identical' || { echo "RED: emitter selftest lost its input-immutability claim: $ev"; exit 1; }
if node scripts/validate-jsonl.mjs testdata/taskrecord/wild.jsonl > "$LOGDIR/gate-w17-wild.out" 2>&1; then
  echo 'RED: wild fixture passed validation - the closed-set guard is asleep'; exit 1
fi
echo "POSITIVE CONTROLS GREEN (primitive fleet $pn fully caught, truth controls $tc, recovery injections $rc all caught, emitter replay deterministic with inputs immutable, wild fixture still rejected)"

step '03 wave diff face at pinned endpoints plus registration reconciliation: three registered merges are ancestors and the zero-write seat shows no fourth merge'
W17_BASE=29f4addc0e10486eb7107845870f38490b33a810
W17_END=defb4708b2665ae341b25567fc597a21e6d14768
[ "${#W17_BASE}" -eq 40 ] && [ "${#W17_END}" -eq 40 ] || { echo 'RED: wave endpoint pins malformed'; exit 1; }
git merge-base --is-ancestor "$W17_BASE" HEAD || { echo 'RED: wave base endpoint not an ancestor of HEAD'; exit 1; }
git merge-base --is-ancestor "$W17_END" HEAD || { echo 'RED: wave end endpoint not an ancestor of HEAD'; exit 1; }
stray=$(git diff --name-only "${W17_BASE}..${W17_END}" | grep -vE '^(docs/|scripts/|testdata/golden/coverage-truthfulness\.json$)' || true)
[ -z "$stray" ] || { echo 'RED: wave diff face carries files outside the allowed categories:'; echo "$stray"; exit 1; }
goes=$(git diff --name-only "${W17_BASE}..${W17_END}" | grep -E '\.go$' || true)
[ -z "$goes" ] || { echo 'RED: the wave carried Go bytes but this wave is registered zero-runtime-Go:'; echo "$goes"; exit 1; }
goldjsonl=$(git diff --name-only "${W17_BASE}..${W17_END}" | grep -E '^testdata/golden/.*\.jsonl$' || true)
[ -z "$goldjsonl" ] || { echo 'RED: event-stream golden bytes moved (expansion is fenced out of this wave):'; echo "$goldjsonl"; exit 1; }
goldface=$(git diff --name-only "${W17_BASE}..${W17_END}" | grep -E '^testdata/golden/' || true)
[ "$goldface" = 'testdata/golden/coverage-truthfulness.json' ] || { echo 'RED: golden-directory face is not exactly the matrix mirror carrier:'; echo "$goldface"; exit 1; }
mrg=$(git log --format='%s' "${W17_BASE}..${W17_END}" | grep -c 'Merge pull request' || true)
[ "$mrg" -eq 3 ] || { echo "RED: wave carries $mrg merge commits, want the three registered PR merges (the W3 seat is registered zero-write - a fourth merge here would be unregistered)"; exit 1; }
for sha in afa96434821c80572bbeeddf157feb1bc5454a56 9160bea640d5c85492bc629ae40793764ff34d0c \
           defb4708b2665ae341b25567fc597a21e6d14768; do
  git merge-base --is-ancestor "$sha" HEAD || { echo "RED: registered merge sha not an ancestor: $sha"; exit 1; }
done
test -f scripts/task-record-emit.mjs \
  || { echo 'RED: W1 registered artifact (tooling-side emitter) not on disk'; exit 1; }
grep -q 'the deferral row is retired on this reconcile, no new semantics' docs/recovery-transaction-design.md \
  || { echo 'RED: W2 registered artifact (retired deferral row) not on disk'; exit 1; }
grep -q 'agent.hook-event-intake' docs/coverage-truthfulness.md testdata/golden/coverage-truthfulness.json \
  || { echo 'RED: W4 registered artifact (hook-intake row) missing from its two carriers'; exit 1; }
wc_files=$(git diff --name-only "${W17_BASE}..${W17_END}" | wc -l | tr -cd '0-9')
echo "WAVE DIFF FACE CLEAN + REGISTERED MERGES RECONCILED (${wc_files} file(s) between the pinned endpoints, $mrg registered merges all ancestors and no fourth merge, docs+scripts plus exactly the one mirror JSON, zero Go bytes, zero event-stream golden bytes, all three registered headline artifacts on disk)"

step '04 slice machine checks: emission face single-source contract, coverage matrix row census re-derived from both carriers at run time'
grep -q "task-map.mjs" scripts/task-record-emit.mjs \
  || { echo 'RED: emitter no longer consumes the shipped mapper (second-mapping-copy risk)'; exit 1; }
grep -q "validate-jsonl.mjs" scripts/task-record-emit.mjs \
  || { echo 'RED: emitter dropped its validator mirror'; exit 1; }
grep -q 'older thirteen types simply ignore the new one' docs/api-v0.md \
  || { echo 'RED: api-v0 additive-compatibility sentence moved'; exit 1; }
grep -q 'record-form landing v2 (emission side)' docs/task-primitive-design.md \
  || { echo 'RED: design doc lost the emission-side landing v2 row'; exit 1; }
grep -q 'reconciled there, never duplicated' docs/recovery-transaction-design.md \
  || { echo 'RED: recovery design lost its never-duplicated reconcile phrase'; exit 1; }
grep -q 'shipped-surface enumeration (reverse direction)' scripts/coverage-truth-check.mjs \
  || { echo 'RED: truth checker dropped the reverse enumeration gate (the structural blind spot this wave closed)'; exit 1; }
docrows=$(grep -cE '^\| `[a-z0-9._-]+` \|' docs/coverage-truthfulness.md)
jsonrows=$(node -e 'const j=require("./testdata/golden/coverage-truthfulness.json"); console.log((j.rows||j).length);')
[ "$docrows" -ge 11 ] || { echo "RED: docs matrix re-counts $docrows rows, want at least the registered 11"; exit 1; }
[ "$docrows" -eq "$jsonrows" ] || { echo "RED: carriers drifted out of lockstep: docs $docrows rows vs mirror $jsonrows rows"; exit 1; }
node -e '
const j=require("./testdata/golden/coverage-truthfulness.json");
const rows=j.rows||j;
const hook=rows.find(r=>(r.face||r.id||"").includes("agent.hook-event-intake"));
if(!hook){ console.log("RED: mirror JSON has no hook-intake row"); process.exit(1); }
const ev=JSON.stringify(hook);
if(!ev.includes("internal/adapter/hooks.go")){ console.log("RED: hook-intake row carries no evidence pointer into internal/adapter/hooks.go"); process.exit(1); }
' || exit 1
echo "SLICE CHECKS GREEN (emitter consumes the single shipped mapper and mirrors the validator; compatibility sentence intact; matrix re-counted at run time: docs $docrows == mirror $jsonrows rows; hook row names its shipped evidence file)"

step '05 record-only invariants: decision closed set, enforcement-teeth negative scan over the wave docs, plane census all-none re-derived'
grep -qF '{allow, would_block}' docs/task-primitive-design.md || { echo 'RED: Phase 0 decision closed-set anchor lost'; exit 1; }
if grep -qiE '\b(auto_deny|deny_list|hard_block|enforce_now)\b' docs/api-v0.md docs/coverage-truthfulness.md docs/recovery-transaction-design.md docs/task-primitive-design.md; then
  echo 'RED: enforcement-teeth vocabulary inside wave docs'; exit 1;
fi
n=$(grep -cE '^[a-z_]+_(enforcement|execution)_plane: none-in-observation-phase$' docs/schema-v2.md)
t=$(grep -cE '^[a-z_]+_(enforcement|execution)_plane: ' docs/schema-v2.md)
[ "$n" -eq "$t" ] || { echo "RED: a declared plane was promoted out of observation stance ($n/$t)"; exit 1; }
[ "$n" -ge 25 ] || { echo "RED: plane census $n, want at least 25 none-lines"; exit 1; }
grep -q 'no sixth uplift row' docs/schema-v2.md || { echo 'RED: schema-v2 uplift census lock lost'; exit 1; }
tw=$(grep -m1 '^task_status_vocabulary:' docs/task-primitive-design.md | sed 's/^task_status_vocabulary: //' | tr ',' '\n' | wc -l | tr -cd '0-9')
[ "$tw" -eq 7 ] || { echo "RED: status vocabulary re-census reads $tw, want 7"; exit 1; }
echo "RECORD-ONLY GREEN (closed set anchor present, teeth vocabulary absent from the four wave docs, plane census all-none $n/$t re-derived, uplift lock in force, status vocabulary re-counted $tw/7)"

step '06 inherited chain: gate-w16 full battery (which itself re-runs gate-w15 down through gate-d1 serially, the gate-d7 Go suite and the gate-d5 release assembly)'
bash scripts/gate-w16.sh > "$LOGDIR/gate-w16-inside-w17.out" 2>&1 \
  || { tail -30 "$LOGDIR/gate-w16-inside-w17.out"; echo RED: gate-w16 regression; exit 1; }
grep -q 'GATE-W16: ALL GREEN' "$LOGDIR/gate-w16-inside-w17.out" \
  || { echo 'RED: gate-w16 completion banner missing'; exit 1; }
echo 'GATE-W16 GREEN INSIDE GATE-W17 (zero regression on the full inherited chain down through gate-d1 with the W15/W16 pins untouched)'

step '07 verbatim contract anchors from all wave slices plus tripwire standalone (internal-ledger negative scan)'
pre17() {
  grep -qF "$2" "$1" || { echo "RED: wave anchor lost in $1: $2"; exit 1; }
}
pre17 scripts/task-record-emit.mjs "const MAPPER = path.join(root, 'scripts', 'task-map.mjs');"
pre17 docs/task-primitive-design.md 'record-form landing v2 (emission side)'
pre17 docs/api-v0.md 'older thirteen types simply ignore the new one'
pre17 docs/recovery-transaction-design.md 'the deferral row is retired on this reconcile, no new semantics'
pre17 docs/recovery-transaction-design.md 'this consolidated row stays the inherited cross-reference and is reconciled there, never duplicated'
pre17 docs/coverage-truthfulness.md '`agent.hook-event-intake`'
pre17 scripts/coverage-truth-check.mjs 'shipped-surface enumeration (reverse direction)'
pre17 testdata/golden/coverage-truthfulness.json 'agent.hook-event-intake'
pre17 docs/task-primitive-design.md 'task_status_vocabulary: open, active, completed, failed, abandoned, blocked, unknown'
bash scripts/tripwire.sh > "$LOGDIR/gate-w17-tripwire.out" 2>&1 \
  || { cat "$LOGDIR/gate-w17-tripwire.out"; echo RED: tripwire; exit 1; }
grep -q 'TRIPWIRE CLEAN' "$LOGDIR/gate-w17-tripwire.out" || { echo RED: tripwire banner; exit 1; }
twcount=$(grep -oE '[0-9]+ (entries|scanned|items)' "$LOGDIR/gate-w17-tripwire.out" | head -1 || true)
echo "ANCHORS GREEN (nine verbatim tokens across the wave slices; tripwire CLEAN ${twcount:-count not printed})"

printf '\nGATE-W17: ALL GREEN\n'
