#!/usr/bin/env bash
# gate-d4.sh — full machine-assertion battery for the D4 security
# judgement slice: threat model ↔ rule table cross-check, the built-in
# 12-rule set with per-rule positive/negative fixtures, the golden
# evaluation thresholds (release pre-condition), the CLI timeline
# surface, Phase 0 semantics on the new emission paths, the frozen
# contract extended with the rule grammar, and every inherited D1/D2/D3
# guarantee (full regression at the tail).
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

step() { printf '\n=== %s ===\n' "$*"; }

step '00 preflight'
go version
node --version

step '01 fmt / vet (three GOOS) / unit tests, serial'
# format gate judges shipped sources only: tracked .go files (never the
# gate's own scratch — re-run chains must not false-red on leftovers)
fmt=$(git ls-files '*.go' | xargs -r gofmt -l)
test -z "$fmt" || { printf '%s\n' "$fmt"; echo RED: gofmt; exit 1; }
go vet ./...
GOOS=darwin GOARCH=amd64 go vet ./...
GOOS=windows GOARCH=amd64 go vet ./...
rc=0
go test -count=1 -p 1 -parallel 1 -v ./... > "$TMPDIR/gate-d4-tests.out" 2>&1 || rc=$?
if [ "$rc" -eq 137 ]; then
  echo 'WARN: go test SIGKILLed by environment (rc137); retrying once'
  go test -count=1 -p 1 -parallel 1 -v ./... > "$TMPDIR/gate-d4-tests.out" 2>&1 || rc=$?
fi
if [ "$rc" -ne 0 ]; then echo RED: go test rc=$rc; tail -30 "$TMPDIR/gate-d4-tests.out"; exit "$rc"; fi
fails=$(grep -c '^--- FAIL' "$TMPDIR/gate-d4-tests.out" || true)
skips=$(grep -c '^--- SKIP' "$TMPDIR/gate-d4-tests.out" || true)
tests=$(grep -c '^--- PASS' "$TMPDIR/gate-d4-tests.out" || true)
if [ "$fails" -ne 0 ] || [ "$skips" -ne 0 ]; then
  echo "RED: FAIL=$fails SKIP=$skips (slice end requires 0 FAIL / 0 SKIP)"; exit 1
fi
echo "UNIT GREEN: $tests top-level test(s), 0 FAIL, 0 SKIP"

step '02 cross-build six artifacts (2 binaries x 3 platforms)'
bash scripts/build-dist.sh

step '03 judgement + collector tests COMPILE on darwin/windows (cross-equivalent)'
GOOS=darwin GOARCH=amd64 go test -c -o "$TMPDIR/rules-darwin.test" ./internal/rules
GOOS=windows GOARCH=amd64 go test -c -o "$TMPDIR/rules-windows.test.exe" ./internal/rules
GOOS=darwin GOARCH=amd64 go test -c -o "$TMPDIR/collector-d4-darwin.test" ./cmd/agent-collector
GOOS=windows GOARCH=amd64 go test -c -o "$TMPDIR/collector-d4-windows.test.exe" ./cmd/agent-collector
echo 'CROSS-TEST-COMPILE OK (rules/collector build-tagged targets compile clean; native run = CI matrix)'

step '04 per-rule positive/negative fixtures + hard invariants (focused)'
go test -count=1 -p 1 -parallel 1 -v ./internal/rules -run 'TestEachBuiltinRule|TestHard|TestMultiMatch|TestBuiltinTables' > "$TMPDIR/gate-d4-rules.out" 2>&1 || { tail -20 "$TMPDIR/gate-d4-rules.out"; exit 1; }
pn=$(grep -c '^--- PASS' "$TMPDIR/gate-d4-rules.out" || true)
if [ "$pn" -lt 4 ]; then echo "RED: focused rule battery ran $pn tests, want >= 4"; exit 1; fi
grep -q '^ok' "$TMPDIR/gate-d4-rules.out" || { echo RED: rules battery not ok; exit 1; }
echo "RULE FIXTURES OK ($pn battery tests green: 12x positive+negative, hard shape, multi-match determinism, table cross-check)"

step '05 golden corpus integrity + frozen-grammar validation'
ng=$(wc -l < testdata/golden/normal.jsonl)
dg=$(wc -l < testdata/golden/danger.jsonl)
[ "$ng" -eq 20 ] && [ "$dg" -eq 20 ] || { echo "RED: golden corpus $ng/$dg, want 20/20"; exit 1; }
tracked=$(git ls-files --cached --others --exclude-standard testdata/golden | wc -l)
[ "$tracked" -ge 3 ] || { echo 'RED: golden files not repo-visible (gitignore swallowing evidence?)'; exit 1; }
node scripts/validate-jsonl.mjs testdata/golden/normal.jsonl --min-lines 20
node scripts/validate-jsonl.mjs testdata/golden/danger.jsonl --min-lines 20
echo 'GOLDEN CORPUS OK (20+20 tracked, every line passes the frozen JSONL validator)'

step '06 eval runner: thresholds are machine-judged twice (Go test + gate parser)'
go test ./internal/rules/ -run TestGoldenEvals -v -count=1 -p 1 -parallel 1 > "$TMPDIR/gate-d4-eval.out" 2>&1 || { tail -25 "$TMPDIR/gate-d4-eval.out"; echo 'RED: eval suite failed (a sick judgement layer may not ship)'; exit 1; }
summary=$(grep 'EVAL SUMMARY' "$TMPDIR/gate-d4-eval.out")
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
[ "$hit" -gt 0 ] && [ "$tot" -ge 20 ] || { echo "RED: eval ratio parse degenerate ($hit/$tot) — parser or summary drift"; exit 1; }
# parser control: a deliberately failing summary must be REJECTED by the
# same awk judgments above (gate-honesty positive control)
if awk -v d="70.0" 'BEGIN{exit !(d>=80.0)}' >/dev/null; then
  echo 'RED: threshold parser accepts a below-bar detection number'; exit 1
fi
echo "EVAL THRESHOLDS OK (detection ${det}% >= 80, worst FP $fp <= 0.05, credential $cred <= 0.02, alerts $alerts <= 1; parser self-rejects bad input)"

step '07 threat model ↔ rule table bidirectional cross-check'
node scripts/threatmodel-check.mjs --selftest
node scripts/threatmodel-check.mjs

step '08 CLI timeline on the real artifact: golden + filters + refusal + read-only'
agent=$(ls dist/agent-collector-*-linux-amd64 | head -1)
"$agent" timeline --out testdata/timeline-sample.jsonl > "$TMPDIR/tl.out"
diff "$TMPDIR/tl.out" testdata/timeline-golden.txt || { echo RED: timeline drifted from golden; exit 1; }
"$agent" timeline --out testdata/timeline-sample.jsonl --agent agi-tl0002 | head -1 | grep -q '^timeline: 2 event(s)$' || { echo RED: agent filter wrong; exit 1; }
"$agent" timeline --out testdata/timeline-sample.jsonl --since 2026-09-27T09:05:30Z | head -1 | grep -q '^timeline: 2 event(s)$' || { echo RED: since filter wrong; exit 1; }
if "$agent" timeline --out scripts/tripwire.sh >/dev/null 2>&1; then
  echo 'RED: timeline echoed a non-JSONL file as audit truth'; exit 1
fi
if "$agent" timeline --out testdata/timeline-sample.jsonl --since nonsense >/dev/null 2>&1; then
  echo 'RED: bad --since accepted'; exit 1
fi
before=$(sha256sum testdata/timeline-sample.jsonl | cut -d' ' -f1)
"$agent" timeline --out testdata/timeline-sample.jsonl >/dev/null
after=$(sha256sum testdata/timeline-sample.jsonl | cut -d' ' -f1)
[ "$before" = "$after" ] || { echo RED: timeline mutated its input file; exit 1; }
echo 'TIMELINE OK (golden exact, both filters, corrupt + bad-since refusal, read-only)'

step '09 Phase 0 semantics on the NEW judgement surface'
# no real-block vocabulary may enter any shipped decision path, and the
# deny-enforcement words must not exist at all in shipped code
if grep -rnE '"decision": ?"(block|denied|deny)"' --include='*.go' cmd/ internal/ | grep -v _test; then
  echo 'RED: real-blocking decision vocabulary in shipped code'; exit 1
fi
if grep -rniE '\bharddeny\b|\bhard_deny\b|\bdeny\b' --include='*.go' cmd/ internal/ | grep -v _test; then
  echo 'RED: enforcement vocabulary in shipped code (Phase 0 records only)'; exit 1
fi
# would_block may only be produced as the recorded decision value;
# assert the emitter set is exactly {allow, would_block} at runtime
go test -count=1 ./internal/rules -run 'TestDecisionEvent' -v > "$TMPDIR/gate-d4-guard.out" 2>&1 || { cat "$TMPDIR/gate-d4-guard.out"; exit 1; }
grep -q '^--- PASS: TestDecisionEvent' "$TMPDIR/gate-d4-guard.out"
# the built-in table itself: 12/12 would_block, >=5 hard critical
nwb=$(node -e 'const s=require("node:fs").readFileSync("internal/rules/rules.go","utf8");console.log((s.match(/"effect": "would_block"/g)||[]).length)')
[ "$nwb" -eq 12 ] || { echo "RED: built-in effects $nwb/12 would_block"; exit 1; }
nh=$(node -e 'const s=require("node:fs").readFileSync("internal/rules/rules.go","utf8");console.log((s.match(/"hard": true/g)||[]).length)')
[ "$nh" -ge 5 ] || { echo "RED: hard rules $nh < 5"; exit 1; }
echo "PHASE 0 SEMANTICS OK (would_block-only, ${nh} hard critical rules, deny vocabulary absent, emitter guard green)"

step '10 frozen contract extended: doc == validator == Go (+ negative control)'
node scripts/apicontract-check.mjs
node scripts/apicontract-check.mjs --negative
# rule grammar vocabularies are pinned doc<->Go by
# TestAPIDocContractMatchesGoEnums (ran in step 01); re-assert presence:
grep -q 'rule_fields: agent_id, type, tool, path, domain, cmdline, exe' docs/api-v0.md
grep -q 'rule_ops: equals, prefix, suffix, contains' docs/api-v0.md
grep -q '`timeline`' docs/api-v0.md
echo 'CONTRACT EXTENSION OK (rule grammar + timeline command frozen into api-v0.md, cross-checked)'

step '11 license gate + tripwire (selftest controls first)'
node scripts/licenses-check.mjs --selftest
node scripts/licenses-check.mjs
bash scripts/tripwire.sh --selftest
bash scripts/tripwire.sh

step '12 zero-network + stdlib-only closure: both binaries AND the judgement package, all three GOOS'
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
echo 'ZERO-NETWORK + STDLIB-ONLY OK (fast path holds for binaries and the judgement package, 3 GOOS)'

step '13 fast-path timing contract on real execution (linux artifact)'
go test -count=1 ./internal/rules -run TestFastPathBudget -v > "$TMPDIR/gate-d4-fast.out" 2>&1 || { cat "$TMPDIR/gate-d4-fast.out"; exit 1; }
grep -q 'FASTPATH' "$TMPDIR/gate-d4-fast.out" || { echo 'RED: fast-path log line missing'; exit 1; }
grep -q '^--- PASS: TestFastPathBudget' "$TMPDIR/gate-d4-fast.out"
echo 'FAST PATH OK (20k-event replay under the 1ms/event budget, 0 FAIL 0 SKIP)'

step '14 full inherited regression: gate-d1 + gate-d2 + gate-d3 serial'
bash scripts/gate-d1.sh  > "$TMPDIR/gate-d4-reg-d1.out"  2>&1 || { tail -25 "$TMPDIR/gate-d4-reg-d1.out";  echo RED: gate-d1 regression; exit 1; }
bash scripts/gate-d2.sh  > "$TMPDIR/gate-d4-reg-d2.out"  2>&1 || { tail -25 "$TMPDIR/gate-d4-reg-d2.out";  echo RED: gate-d2 regression; exit 1; }
bash scripts/gate-d3.sh  > "$TMPDIR/gate-d4-reg-d3.out"  2>&1 || { tail -25 "$TMPDIR/gate-d4-reg-d3.out";  echo RED: gate-d3 regression; exit 1; }
grep -q 'GATE-D1: ALL GREEN' "$TMPDIR/gate-d4-reg-d1.out"
grep -q 'GATE-D2: ALL GREEN' "$TMPDIR/gate-d4-reg-d2.out"
grep -q 'GATE-D3: ALL GREEN' "$TMPDIR/gate-d4-reg-d3.out"
echo 'D1+D2+D3 REGRESSION GREEN (serial, full batteries)'

printf '\nGATE-D4: ALL GREEN\n'
