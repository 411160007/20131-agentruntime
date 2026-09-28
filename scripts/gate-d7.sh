#!/usr/bin/env bash
# gate-d7.sh — full machine-assertion battery for the D7+ evaluations
# expansion slice: the ten-dimension golden corpus (size floor, seed
# zero-regression byte diff against the frozen baseline, dimension
# coverage cross-check between labels.json and the runner output), the
# pinned thresholds re-judged over the expanded corpus, the evidence
# trust-order assertions (self-description alone never raises a finding),
# corpus validation through the independent JSONL validator, the
# inherited Phase 0 / stdlib / zero-network invariants, and the complete
# D1..D6 regression chain (serial).
#
# Every gate runs against REAL staged build artifacts and carries its
# own positive/negative control. Any red exits non-zero.
set -euo pipefail
cd "$(dirname "$0")/.."

export TMPDIR="$(pwd)/.tmptest"
mkdir -p "$TMPDIR" # sandbox kills binaries executed from /tmp; pin scratch inside the repo
# environment-harvest guard: wipe a previous gate's leftover go-build scratch
# before any whole-tree step, so generated _testmain.go can never false-red.
find "$TMPDIR" -mindepth 1 -maxdepth 1 -name 'go-build*' -exec rm -rf {} + 2>/dev/null || true

# Seed-corpus baseline: the commit whose golden files are the frozen 40.
# The zero-regression assertions compare the first 20 lines of each
# corpus file against this baseline byte-for-byte. If this commit ever
# stops resolving, the gate must be re-anchored deliberately (a loud,
# reviewed act) — never silently relaxed.
GOLDEN_BASE=b454f89

step() { printf '\n=== %s ===\n' "$*"; }

step '00 preflight'
go version
node --version
git cat-file -e "${GOLDEN_BASE}^{commit}" || { echo 'RED: golden baseline commit not reachable — re-anchor deliberately'; exit 1; }

step '01 fmt / vet (three GOOS) / unit tests, serial'
fmt=$(git ls-files '*.go' | xargs -r gofmt -l)
test -z "$fmt" || { printf '%s\n' "$fmt"; echo RED: gofmt; exit 1; }
go vet ./...
GOOS=darwin GOARCH=amd64 go vet ./...
GOOS=windows GOARCH=amd64 go vet ./...
rc=0
go test -count=1 -p 1 -parallel 1 -v ./... > "$TMPDIR/gate-d7-tests.out" 2>&1 || rc=$?
if [ "$rc" -eq 137 ]; then
  echo 'WARN: go test SIGKILLed by environment (rc137); retrying once'
  go test -count=1 -p 1 -parallel 1 -v ./... > "$TMPDIR/gate-d7-tests.out" 2>&1 || rc=$?
fi
if [ "$rc" -ne 0 ]; then echo RED: go test rc=$rc; tail -30 "$TMPDIR/gate-d7-tests.out"; exit "$rc"; fi
fails=$(grep -c '^--- FAIL' "$TMPDIR/gate-d7-tests.out" || true)
skips=$(grep -c '^--- SKIP' "$TMPDIR/gate-d7-tests.out" || true)
tests=$(grep -c '^--- PASS' "$TMPDIR/gate-d7-tests.out" || true)
if [ "$fails" -ne 0 ] || [ "$skips" -ne 0 ]; then
  echo "RED: FAIL=$fails SKIP=$skips (slice end requires 0 FAIL / 0 SKIP)"; exit 1
fi
echo "UNIT GREEN: $tests top-level test(s), 0 FAIL, 0 SKIP"

step '02 cross-build six artifacts (2 binaries x 3 platforms)'
bash scripts/build-dist.sh

step '03 golden seed zero-regression: first 20 lines byte-identical to baseline'
for f in normal.jsonl danger.jsonl; do
  git show "${GOLDEN_BASE}:testdata/golden/$f" | head -n 20 > "$TMPDIR/seed-$f"
  head -n 20 "testdata/golden/$f" > "$TMPDIR/cur-$f"
  cmp "$TMPDIR/seed-$f" "$TMPDIR/cur-$f" || { echo "RED: seed lines drifted in $f"; exit 1; }
done
# labels: every baseline key keeps its expect/class values (dim may be
# added; nothing existing may change meaning)
git show "${GOLDEN_BASE}:testdata/golden/labels.json" > "$TMPDIR/seed-labels.json"
node -e '
const base = JSON.parse(require("node:fs").readFileSync(process.argv[1], "utf8"));
const cur  = JSON.parse(require("node:fs").readFileSync(process.argv[2], "utf8"));
for (const [id, v] of Object.entries(base)) {
  const c = cur[id];
  if (!c) { console.error("RED: seed label lost " + id); process.exit(1); }
  if (c.expect !== v.expect || c.class !== v.class) {
    console.error("RED: seed label drifted " + id); process.exit(1);
  }
}
console.log("SEED LABELS UNCHANGED (" + Object.keys(base).length + " keys, expect+class identical)");
' "$TMPDIR/seed-labels.json" testdata/golden/labels.json
echo 'SEED 40 ZERO-REGRESSION OK (corpus lines byte-identical, label semantics unchanged)'

step '04 expanded corpus integrity + frozen-grammar validation'
ng=$(wc -l < testdata/golden/normal.jsonl)
dg=$(wc -l < testdata/golden/danger.jsonl)
total=$((ng + dg))
[ "$ng" -ge 20 ] && [ "$dg" -ge 20 ] || { echo "RED: golden corpus $ng/$dg, want >= 20/20"; exit 1; }
[ "$total" -ge 120 ] || { echo "RED: golden total $total < 120 expansion floor"; exit 1; }
tracked=$(git ls-files --cached --others --exclude-standard testdata/golden | wc -l)
[ "$tracked" -ge 3 ] || { echo 'RED: golden files not repo-visible (gitignore swallowing evidence?)'; exit 1; }
node scripts/validate-jsonl.mjs testdata/golden/normal.jsonl --min-lines 72
node scripts/validate-jsonl.mjs testdata/golden/danger.jsonl --min-lines 57
echo "CORPUS OK ($ng normal + $dg danger = $total events, all tracked-visible, validator-clean)"

step '05 ten-dimension coverage: labels.json re-count must agree with the runner table'
node -e '
const fs = require("node:fs");
const labels = JSON.parse(fs.readFileSync("testdata/golden/labels.json", "utf8"));
const DIMS = ["security","functional","recovery","compatibility","performance","ux","localization","security_regression","agent_behavior","untrusted"];
const ids = new Set();
for (const f of ["normal.jsonl","danger.jsonl"]) {
  for (const l of fs.readFileSync("testdata/golden/" + f, "utf8").split("\n").filter(Boolean)) ids.add(JSON.parse(l).id);
}
const per = {}; let ghosts = 0;
for (const [id, v] of Object.entries(labels)) {
  if (!ids.has(id)) { console.error("GHOST label " + id); ghosts++; }
  if (!DIMS.includes(v.dim)) { console.error("WILD dim on " + id + ": " + v.dim); process.exit(1); }
  (per[v.dim] ||= []).push(id);
}
if (ghosts) process.exit(1);
for (const d of DIMS) {
  const n = (per[d] || []).length;
  console.log("DIM " + d + " n=" + n);
  if (n < 8) { console.error("RED: dimension " + d + " carries " + n + " (<8, empty-shell floor)"); process.exit(1); }
}
console.log("DIM TABLE OK (10 dims, every dim >= 8, no ghosts)");
' > "$TMPDIR/gate-d7-dimtable.txt" || { echo RED: dimension coverage; exit 1; }
cat "$TMPDIR/gate-d7-dimtable.txt"
# cross-check the Go runner table against the node table (two sources)
go test -count=1 ./internal/rules -run TestGoldenTenDimCoverage -v > "$TMPDIR/gate-d7-coverage.out" 2>&1 || { tail -20 "$TMPDIR/gate-d7-coverage.out"; exit 1; }
for d in security functional recovery compatibility performance ux localization security_regression agent_behavior untrusted; do
  n=$(grep -oE "dim=$d n=[0-9]+" "$TMPDIR/gate-d7-coverage.out" | grep -oE '[0-9]+$' | head -1)
  m=$(grep -E "^DIM $d " "$TMPDIR/gate-d7-dimtable.txt" | grep -oE 'n=[0-9]+' | grep -oE '[0-9]+')
  [ "$n" = "$m" ] || { echo "RED: dim $d table mismatch go=$n node=$m"; exit 1; }
done
echo 'COVERAGE CROSS-CHECK OK (Go runner table == node re-count, 10/10 dims)'

step '06 eval runner: thresholds are machine-judged twice (Go test + gate parser)'
go test ./internal/rules/ -run TestGoldenEvals -v -count=1 -p 1 -parallel 1 > "$TMPDIR/gate-d7-eval.out" 2>&1 || { tail -25 "$TMPDIR/gate-d7-eval.out"; echo 'RED: eval suite failed (a sick judgement layer may not ship)'; exit 1; }
summary=$(grep 'EVAL SUMMARY' "$TMPDIR/gate-d7-eval.out")
det=$(printf '%s' "$summary" | sed -E 's/.*detection=([0-9]+)\/([0-9]+) \(([0-9.]+)%%?\).*/\3/' | sed 's/%//')
hit=$(printf '%s' "$summary" | sed -E 's/.*detection=([0-9]+)\/.*/\1/')
tot=$(printf '%s' "$summary" | sed -E 's#.*/([0-9]+) \([0-9.]+%.*#\1#')
fp=$(printf '%s' "$summary" | sed -E 's/.*worst_fp=([0-9.]+).*/\1/')
cred=$(printf '%s' "$summary" | sed -E 's/.*credential_fp=([0-9.]+).*/\1/')
alerts=$(printf '%s' "$summary" | sed -E 's/.*normal_alerts=([0-9]+).*/\1/')
awk -v d="$det" 'BEGIN{exit !(d>=80.0)}' || { echo "RED: detection ${det}% < 80%"; exit 1; }
awk -v f="$fp" 'BEGIN{exit !(f<=0.05)}' || { echo "RED: worst per-class FP $f > 0.05"; exit 1; }
awk -v c="$cred" 'BEGIN{exit !(c<=0.02)}' || { echo "RED: credential-class FP $c > 0.02"; exit 1; }
[ "$alerts" -le 1 ] || { echo "RED: zero-disturbance breach ($alerts alerts on normals)"; exit 1; }
[ "$hit" -gt 0 ] && [ "$tot" -ge 57 ] || { echo "RED: eval ratio parse degenerate ($hit/$tot) — parser or summary drift"; exit 1; }
# per-class detail lines must be present for BOTH sides (分项出数)
ncls=$(grep -c '^    eval_test.go.*EVAL: class=' "$TMPDIR/gate-d7-eval.out" || true)
ndet=$(grep -c '^    eval_test.go.*EVAL: danger_class=' "$TMPDIR/gate-d7-eval.out" || true)
[ "$ncls" -ge 6 ] && [ "$ndet" -ge 6 ] || { echo "RED: per-class breakdown thin (normal classes $ncls, danger classes $ndet)"; exit 1; }
# parser control: a deliberately failing summary must be REJECTED by the
# same awk judgments above (gate-honesty positive control)
if awk -v d="70.0" 'BEGIN{exit !(d>=80.0)}' >/dev/null; then
  echo 'RED: threshold parser accepts a below-bar detection number'; exit 1
fi
echo "EVAL THRESHOLDS OK (detection ${det}% >= 80 over $tot dangers, worst FP $fp <= 0.05, credential $cred <= 0.02, alerts $alerts <= 1; per-class lines: $ncls normal-side, $ndet danger-side; parser self-rejects bad input)"

step '07 evidence trust order: self-description never decides alone'
go test -count=1 ./internal/rules -run TestEvidenceTrustOrder -v > "$TMPDIR/gate-d7-trust.out" 2>&1 || { tail -25 "$TMPDIR/gate-d7-trust.out"; echo RED: evidence trust order; exit 1; }
grep -q '^--- PASS: TestEvidenceTrustOrder' "$TMPDIR/gate-d7-trust.out"
echo 'TRUST ORDER OK (claim-only allow; native-evidence controls would_block; no built-in rule matches summary)'

step '08 judgement fixtures + threat model ↔ rule table cross-check'
go test -count=1 -p 1 -parallel 1 -v ./internal/rules -run 'TestEachBuiltinRule|TestHard|TestMultiMatch|TestBuiltinTables' > "$TMPDIR/gate-d7-rules.out" 2>&1 || { tail -20 "$TMPDIR/gate-d7-rules.out"; exit 1; }
pn=$(grep -c '^--- PASS' "$TMPDIR/gate-d7-rules.out" || true)
if [ "$pn" -lt 4 ]; then echo "RED: focused rule battery ran $pn tests, want >= 4"; exit 1; fi
node scripts/threatmodel-check.mjs --selftest
node scripts/threatmodel-check.mjs
echo 'RULE FIXTURES + THREAT-MODEL CROSS-CHECK OK'

step '09 Phase 0 semantics on the judgement surface (inherited shape)'
if grep -rnE '"decision": ?"(block|denied|deny)"' --include='*.go' cmd/ internal/ | grep -v _test; then
  echo 'RED: real-blocking decision vocabulary in shipped code'; exit 1
fi
if grep -rniE '\bharddeny\b|\bhard_deny\b|\bdeny\b' --include='*.go' cmd/ internal/ | grep -v _test; then
  echo 'RED: enforcement vocabulary in shipped code (Phase 0 records only)'; exit 1
fi
go test -count=1 ./internal/rules -run 'TestDecisionEvent' -v > "$TMPDIR/gate-d7-guard.out" 2>&1 || { cat "$TMPDIR/gate-d7-guard.out"; exit 1; }
grep -q '^--- PASS: TestDecisionEvent' "$TMPDIR/gate-d7-guard.out"
nwb=$(node -e 'const s=require("node:fs").readFileSync("internal/rules/rules.go","utf8");console.log((s.match(/"effect": "would_block"/g)||[]).length)')
[ "$nwb" -eq 12 ] || { echo "RED: built-in effects $nwb/12 would_block"; exit 1; }
echo 'PHASE 0 SEMANTICS OK (would_block-only emitter set, deny vocabulary absent)'

step '10 frozen contract + license + tripwire (controls first)'
node scripts/apicontract-check.mjs
node scripts/apicontract-check.mjs --negative
node scripts/licenses-check.mjs --selftest
node scripts/licenses-check.mjs
bash scripts/tripwire.sh --selftest
bash scripts/tripwire.sh
echo 'CONTRACT + LICENSE + TRIPWIRE OK'

step '11 zero-network + stdlib-only closure: binaries AND the judgement package, all three GOOS'
for goos in linux darwin windows; do
  for bin in hello-collector agent-collector; do
    deps=$(GOOS=$goos go list -deps "./cmd/$bin")
    if printf '%s\n' "$deps" | grep -E '^(net|net/.+|crypto/tls)$'; then
      echo "RED: $bin ($goos) depends on network packages"; exit 1
    fi
  done
  rdeps=$(GOOS=$goos go list -deps ./internal/rules)
  if printf '%s\n' "$rdeps" | grep -E '^(net|net/.+|crypto/tls|os/exec|database/sql)$'; then
    echo "RED: rules package ($goos) left the pure fast path"; exit 1
  fi
done
grep -qE '^require' go.mod && { echo 'RED: go.mod gained external requires'; exit 1; }
echo 'ZERO-NETWORK + STDLIB-ONLY OK (fast path holds, 3 GOOS, go.mod still requires nothing)'

step '12 fast-path timing contract on real execution'
go test -count=1 ./internal/rules -run TestFastPathBudget -v > "$TMPDIR/gate-d7-fast.out" 2>&1 || { cat "$TMPDIR/gate-d7-fast.out"; exit 1; }
grep -q 'FASTPATH' "$TMPDIR/gate-d7-fast.out" || { echo 'RED: fast-path log line missing'; exit 1; }
grep -q '^--- PASS: TestFastPathBudget' "$TMPDIR/gate-d7-fast.out"
echo 'FAST PATH OK (replay under the per-event budget)'

step '13 full inherited regression: gate-d1 + gate-d2 + gate-d3 + gate-d4 + gate-d5 + gate-d6 serial'
bash scripts/gate-d1.sh  > "$TMPDIR/gate-d7-reg-d1.out"  2>&1 || { tail -25 "$TMPDIR/gate-d7-reg-d1.out";  echo RED: gate-d1 regression; exit 1; }
bash scripts/gate-d2.sh  > "$TMPDIR/gate-d7-reg-d2.out"  2>&1 || { tail -25 "$TMPDIR/gate-d7-reg-d2.out";  echo RED: gate-d2 regression; exit 1; }
bash scripts/gate-d3.sh  > "$TMPDIR/gate-d7-reg-d3.out"  2>&1 || { tail -25 "$TMPDIR/gate-d7-reg-d3.out";  echo RED: gate-d3 regression; exit 1; }
bash scripts/gate-d4.sh  > "$TMPDIR/gate-d7-reg-d4.out"  2>&1 || { tail -25 "$TMPDIR/gate-d7-reg-d4.out";  echo RED: gate-d4 regression; exit 1; }
bash scripts/gate-d5.sh  > "$TMPDIR/gate-d7-reg-d5.out"  2>&1 || { tail -25 "$TMPDIR/gate-d7-reg-d5.out";  echo RED: gate-d5 regression; exit 1; }
bash scripts/gate-d6.sh  > "$TMPDIR/gate-d7-reg-d6.out"  2>&1 || { tail -25 "$TMPDIR/gate-d7-reg-d6.out"; echo RED: gate-d6 regression; exit 1; }
grep -q 'GATE-D1: ALL GREEN' "$TMPDIR/gate-d7-reg-d1.out"
grep -q 'GATE-D2: ALL GREEN' "$TMPDIR/gate-d7-reg-d2.out"
grep -q 'GATE-D3: ALL GREEN' "$TMPDIR/gate-d7-reg-d3.out"
grep -q 'GATE-D4: ALL GREEN' "$TMPDIR/gate-d7-reg-d4.out"
grep -q 'GATE-D5: ALL GREEN' "$TMPDIR/gate-d7-reg-d5.out"
grep -q 'GATE-D6: ALL GREEN' "$TMPDIR/gate-d7-reg-d6.out"
echo 'D1..D6 REGRESSION GREEN (serial, full batteries)'

printf '\nGATE-D7: ALL GREEN\n'
