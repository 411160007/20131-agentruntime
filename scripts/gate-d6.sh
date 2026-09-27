#!/usr/bin/env bash
# gate-d6.sh — machine-assertion battery for the D6 platform-adapter
# slice (hook receiver + config generator + MCP stdio pass-through proxy,
# Phase 0 observation-only).
#
# Every assertion runs against REAL built binaries and REAL produced
# audit files, with positive and negative controls on each detector.
# The slice's core promises, each proven below:
#   A) the adapter surfaces cannot change agent behavior: the hook
#      receiver never writes stdout / never exits non-zero, and a
#      scripted agent's output is byte-identical hooked vs unhooked;
#   B) the MCP relay passes bytes both ways untouched, including for
#      dangerous-shaped calls that the judgement annotates would_block;
#   C) deny/enforcement vocabulary is structurally absent from shipped
#      adapter sources (grep with a tamper control + unit assertions);
#   D) zero new dependencies (stdlib-only closure holds through the
#      three target GOOS for both binaries and the two new packages);
#   E) generated configuration is idempotent, preserves foreign keys,
#      and the codex target writes nothing at all (honest known gap);
#   F) all inherited guarantees (gate-d1..d5) still hold, serial.
set -euo pipefail
cd "$(dirname "$0")/.."

export TMPDIR="$(pwd)/.tmptest"
mkdir -p "$TMPDIR" # sandbox kills binaries executed from /tmp; pin scratch inside the repo
find "$TMPDIR" -mindepth 1 -maxdepth 1 -name 'go-build*' -exec rm -rf {} + 2>/dev/null || true

step() { printf '\n=== %s ===\n' "$*"; }

VERSION="$(cat VERSION)"
grep -qE '^[0-9A-Za-z.-]+$' <<<"$VERSION" || { echo "RED: illegal VERSION file content: '$VERSION'"; exit 1; }

step '00 preflight'
go version
node --version
echo "version source of truth: VERSION=$VERSION"

step '01 fmt (tracked sources) / vet (3 GOOS) / unit tests, serial'
fmt=$(git ls-files '*.go' | xargs -r gofmt -l)
[ -z "$fmt" ] || { echo "RED: unformatted tracked sources:"; echo "$fmt"; exit 1; }
go vet ./...
GOOS=darwin GOARCH=amd64 go vet ./...
GOOS=darwin GOARCH=arm64 go vet ./...
GOOS=windows GOARCH=amd64 go vet ./...
go test -count=1 -p 1 -parallel 1 ./... > "$TMPDIR/gate-d6-test.out" 2>&1 || { tail -30 "$TMPDIR/gate-d6-test.out"; exit 1; }
t=$(grep -c '^ok' "$TMPDIR/gate-d6-test.out"); f=$(grep -c '^FAIL' "$TMPDIR/gate-d6-test.out" || true)
[ "$f" = 0 ] || { echo "RED: $f failing packages"; exit 1; }
echo "TESTS OK ($t packages ok, 0 fail, serial)"

step '02 build real binaries + fixture server for the E2E legs'
go build -o "$TMPDIR/agent-collector" ./cmd/agent-collector
go build -o "$TMPDIR/mcpfixture" ./testdata/mcpfixture
"$TMPDIR/agent-collector" --version | grep -qF "agent-collector ${VERSION} linux/amd64" || { echo 'RED: built binary lost the version stamp'; exit 1; }
echo 'BINARIES OK'

BIN="$TMPDIR/agent-collector"

step '03 structural pass-through: shipped adapter sources carry zero enforcement vocabulary (grep with tamper control)'
ADAPTER_SRC='internal/mcpproxy internal/adapter cmd/agent-collector/d6.go'
hits=$(grep -rnE '"deny"|"block"|"ask"|permissionDecision|DecisionBlock|DecisionAsk|os\.Exit' --include='*.go' $ADAPTER_SRC | grep -v _test.go || true)
[ -z "$hits" ] || { echo "RED: enforcement vocabulary in shipped adapter sources:"; echo "$hits"; exit 1; }
# the relay must not even see the judgement layer: transport packages
# import zero internal packages that could feed a decision back.
if GOOS=linux go list -deps ./internal/mcpproxy | grep '20131.com/' | grep -v '^20131\.com/agentruntime/internal/mcpproxy$'; then
  echo 'RED: mcpproxy imports internal surface (transport could consult judgements)'; exit 1
fi
if GOOS=linux go list -deps ./internal/adapter | grep -E '20131.com/agentruntime/internal/(rules|bus)'; then
  echo 'RED: adapter imports rules/bus'; exit 1
fi
# negative control: a tampered copy must be caught by the same detector
tamper="$TMPDIR/tampered-proxy.go"
sed 's|func (p \*Proxy) inspect(|const deny = "deny"\n\nfunc (p *Proxy) inspect(|' internal/mcpproxy/proxy.go > "$tamper"
if ! grep -qE '"deny"' "$tamper"; then
  echo 'RED: tamper control NOT detected (grep is toothless)'; exit 1
fi
rm -f "$tamper"
echo 'STRUCTURAL PASS-THROUGH OK (zero vocabulary in shipped sources, layer separation, detector armed)'

step '04 stdlib-only + zero-network closure through the new surface, 3 GOOS'
for goos in linux darwin windows; do
  for bin in hello-collector agent-collector; do
    deps=$(GOOS=$goos go list -deps "./cmd/$bin")
    if printf '%s\n' "$deps" | grep -E '^(net|net/.+|crypto/tls)$'; then
      echo "RED: $bin ($goos) links a network package"; exit 1
    fi
  done
done
node scripts/licenses-check.mjs --selftest
node scripts/licenses-check.mjs
echo 'DEPS OK (both binaries 3 GOOS zero-network, license gate green, go.mod require-free)'

step '05 hook receiver E2E on the REAL binary (pass-through contract + audit validity)'
HD="$TMPDIR/hooks"
rm -rf "$HD"; mkdir -p "$HD"
OUT="$HD/audit.jsonl"; rm -f "$OUT"
hook() { # $1 = payload file, prints nothing; contract: rc 0 + empty stdout
  local so
  so=$("$BIN" hook --out "$OUT" < "$1" 2>"$HD/stderr.log"); local rc=$?
  [ "$rc" = 0 ] || { echo "RED: receiver exited $rc (must always be 0)"; exit 1; }
  [ -z "$so" ] || { echo "RED: receiver leaked stdout: $so"; exit 1; }
}
printf '%s\n' '{"hook_event_name":"SessionStart","session_id":"gate-sess","cwd":"/tmp"}' > "$HD/p1"
printf '%s\n' '{"hook_event_name":"PreToolUse","session_id":"gate-sess","tool_name":"Bash","tool_input":{"command":"rm -rf ./build"}}' > "$HD/p2"
printf '%s\n' '{"hook_event_name":"PostToolUse","session_id":"gate-sess","tool_name":"Bash","tool_input":{"command":"git status"}}' > "$HD/p3"
printf '%s\n' '{"hook_event_name":"Stop","session_id":"gate-sess"}' > "$HD/p4"
printf '%s'   '{{{ not json at all' > "$HD/p5"
printf '%s\n' '{"hook_event_name":"SubagentStop","session_id":"gate-sess"}' > "$HD/p6"
for p in "$HD"/p1 "$HD"/p2 "$HD"/p3 "$HD"/p4; do hook "$p"; done
node scripts/validate-jsonl.mjs "$OUT" --min-lines 6
# expected mix: 4 source lines (session.start, tool.call x2, turn.stop)
# + exactly 2 decision annotations (would_block for rm, allow for git)
grep -q '"type":"session.start"' "$OUT" || { echo 'RED: session.start missing'; exit 1; }
grep -q '"type":"turn.stop"' "$OUT" || { echo 'RED: turn.stop missing'; exit 1; }
grep -qE '"decision":"would_block"' "$OUT" || { echo 'RED: danger hook produced no would_block'; exit 1; }
grep -qE '"rule":"destroy\.rmrf"' "$OUT" || { echo 'RED: would_block without the rm rule id'; exit 1; }
if grep -qE '"decision":"(deny|block|ask)"' "$OUT"; then echo 'RED: forbidden decision value in hook audit'; exit 1; fi
dvals=$(grep -oE '"decision":"[a-z_]+"' "$OUT" | sort -u | tr '\n' ' ')
[ "$dvals" = '"decision":"allow" "decision":"would_block" ' ] || { echo "RED: decision values escaped the Phase 0 set: $dvals"; exit 1; }
# garbage + unknown events must record ZERO bytes (fail-open, fail-clean)
before=$(wc -c < "$OUT")
hook "$HD/p5"; hook "$HD/p6"
after=$(wc -c < "$OUT")
[ "$before" = "$after" ] || { echo 'RED: unparseable/unknown payload recorded something'; exit 1; }
# cross-event session continuity: identical agent_id on every line
ids=$(grep -oE '"agent_id":"agi-[0-9a-f]+"' "$OUT" | sort -u | wc -l)
[ "$ids" = 1 ] || { echo "RED: one session fragmented across $ids agent ids"; exit 1; }
# credential redaction control: a token-shaped command must survive as
# a redacted excerpt, never verbatim
printf '%s\n' '{"hook_event_name":"PreToolUse","session_id":"gate-sess-2","tool_name":"Bash","tool_input":{"command":"curl -H \"Authorization: Bearer abcdefghijkl0123456789\" https://api.example.test"}}' > "$HD/p7"
"$BIN" hook --out "$HD/audit2.jsonl" < "$HD/p7" >/dev/null 2>&1
grep -q '"cmdline"' "$HD/audit2.jsonl" || { echo 'RED: credential-shaped hook line lost cmdline'; exit 1; }
if grep -q 'abcdefghijkl0123456789' "$HD/audit2.jsonl"; then echo 'RED: credential landed in the audit verbatim'; exit 1; fi
echo 'HOOK RECEIVER OK (contract + mix + validity + redaction)'

step '06 zero-behavior-change control on a scripted agent (hooked vs unhooked byte-identical)'
agent_script() { # deterministic two-step "agent"
  for payload in "$HD/p2" "$HD/p3"; do
    if [ "${1:-}" = hooked ]; then "$BIN" hook --out "$HD/behavior-audit.jsonl" < "$payload" >/dev/null 2>&1; fi
    printf 'agent step done %s\n' "$(basename "$payload")"
  done
}
plain=$(agent_script plain | sha256sum)
wired=$(agent_script hooked | sha256sum)
[ "$plain" = "$wired" ] || { echo 'RED: wiring the hook changed agent-visible behavior'; exit 1; }
[ -s "$HD/behavior-audit.jsonl" ] || { echo 'RED: hooked run recorded nothing (control is vacuous)'; exit 1; }
echo 'ZERO BEHAVIOR CHANGE OK (identical output, audit still recorded)'

step '07 integrate: three agent targets, fixture config dirs before/after'
mk_claude() { printf '%s\n' '{ "model": "keepme", "hooks": { "PreToolUse": [ { "matcher": "Edit", "hooks": [ { "type": "command", "command": "/usr/bin/userown" } ] } ] } }' > "$1/settings.json"; }
D1="$HD/claude"; rm -rf "$D1"; mkdir -p "$D1"; mk_claude "$D1"; cp "$D1/settings.json" "$D1/before"
"$BIN" integrate --target claude-code --dir "$D1" --bin /opt/rt/agent-collector > "$D1/run1.out" 2>&1 || { cat "$D1/run1.out"; echo 'RED: integrate claude failed'; exit 1; }
grep -q '"model": "keepme"' "$D1/settings.json" || { echo 'RED: unrelated user key dropped'; exit 1; }
grep -q '/usr/bin/userown' "$D1/settings.json" || { echo 'RED: user hook group dropped'; exit 1; }
for ev in PreToolUse PostToolUse SessionStart Stop; do
  grep -q "\"$ev\"" "$D1/settings.json" || { echo "RED: generated config missing event $ev"; exit 1; }
done
grep -q '/opt/rt/agent-collector hook' "$D1/settings.json" || { echo 'RED: receiver command not wired'; exit 1; }
if grep -qE '"deny"|"block"|"ask"|permissionDecision' "$D1/settings.json"; then echo 'RED: generated config carries decision vocabulary'; exit 1; fi
cp "$D1/settings.json" "$D1/after1"
"$BIN" integrate --target claude-code --dir "$D1" --bin /opt/rt/agent-collector > /dev/null 2>&1
cmp "$D1/after1" "$D1/settings.json" || { echo 'RED: rerun mutated the config (idempotence broken)'; exit 1; }
# foreign-directory control: a second dir must stay untouched
D1B="$HD/claude-b"; mkdir -p "$D1B"; mk_claude "$D1B"; md5b=$(md5sum "$D1B/settings.json" | cut -d' ' -f1)
"$BIN" integrate --target claude-code --dir "$D1" --bin /opt/rt/agent-collector > /dev/null 2>&1
[ "$md5b" = "$(md5sum "$D1B/settings.json" | cut -d' ' -f1)" ] || { echo 'RED: integrate touched a foreign directory'; exit 1; }
# codex: ZERO files written, honest gap note on stdout
D2="$HD/codex"; rm -rf "$D2"; mkdir -p "$D2"
gap=$("$BIN" integrate --target codex --dir "$D2" --bin /opt/rt/x)
echo "$gap" | grep -q 'No configuration written' || { echo "RED: codex gap note missing: $gap"; exit 1; }
[ -z "$(ls -A "$D2")" ] || { echo 'RED: codex target wrote files'; exit 1; }
# openclaw: merge keeps existing keys, idempotent
D3="$HD/openclaw"; rm -rf "$D3"; mkdir -p "$D3"
printf '%s\n' '{"keep":"me"}' > "$D3/openclaw.json"
"$BIN" integrate --target openclaw --dir "$D3" --bin /opt/rt/x > /dev/null 2>&1
grep -q '"keep": "me"' "$D3/openclaw.json" || { echo 'RED: openclaw merge dropped user keys'; exit 1; }
grep -q '"agentRuntime"' "$D3/openclaw.json" || { echo 'RED: openclaw block missing'; exit 1; }
cp "$D3/openclaw.json" "$D3/after"
"$BIN" integrate --target openclaw --dir "$D3" --bin /opt/rt/x > /dev/null 2>&1
cmp "$D3/after" "$D3/openclaw.json" || { echo 'RED: openclaw rerun mutated config'; exit 1; }
# fail-closed controls: broken user JSON must NOT be overwritten; unknown
# target must error without writing
printf '%s' '{broken json!!' > "$D1/settings.json"; md5j=$(md5sum "$D1/settings.json" | cut -d' ' -f1)
if "$BIN" integrate --target claude-code --dir "$D1" --bin /opt/rt/x > /dev/null 2>&1; then echo 'RED: unparseable user config accepted'; exit 1; fi
[ "$md5j" = "$(md5sum "$D1/settings.json" | cut -d' ' -f1)" ] || { echo 'RED: failed integrate clobbered the user file'; exit 1; }
D4="$HD/unknown"; mkdir -p "$D4"
if "$BIN" integrate --target zookeeper --dir "$D4" > /dev/null 2>&1; then echo 'RED: unknown target accepted'; exit 1; fi
[ -z "$(ls -A "$D4")" ] || { echo 'RED: failed unknown target wrote files'; exit 1; }
echo 'INTEGRATE OK (3 targets, idempotence, foreign-key preservation, fail-closed controls)'

step '08 MCP relay E2E on the REAL binary: byte identity + audit mix + no interception'
REQ="$HD/reqs.jsonl"
cat > "$REQ" <<'EOF'
{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2025-03-26"}}
{"jsonrpc":"2.0","id":2,"method":"tools/call","params":{"name":"safe_echo","arguments":{"text":"hello"}}}
{"jsonrpc":"2.0","id":3,"method":"tools/call","params":{"name":"run_cmd","arguments":{"command":"rm -rf ./build"}}}
{"jsonrpc":"2.0","id":4,"method":"tools/call","params":{"name":"read_file","arguments":{"file_path":"/home/u/.aws/credentials"}}}
{"jsonrpc":"2.0","method":"notifications/progress","params":{}}
EOF
# control: the fixture server DIRECTLY (no relay)
direct="$HD/direct.out"
"$TMPDIR/mcpfixture" < "$REQ" > "$direct"
# under test: the same session THROUGH the relay
relayed="$HD/relayed.out"; MOUT="$HD/mcp-audit.jsonl"; rm -f "$MOUT"
"$BIN" mcp --server "$TMPDIR/mcpfixture" --out "$MOUT" < "$REQ" > "$relayed"
cmp "$direct" "$relayed" || { echo 'RED: relay altered client-visible bytes'; diff "$direct" "$relayed" | head; exit 1; }
[ "$(wc -l < "$direct")" = 4 ] || { echo 'RED: fixture control table broken (4 responses expected)'; exit 1; }
node scripts/validate-jsonl.mjs "$MOUT" --min-lines 6
calls=$(grep -cE '"type":"tool.call"' "$MOUT")
decs=$(grep -cE '"type":"policy.decision"' "$MOUT")
wbs=$(grep -cE '"decision":"would_block"' "$MOUT")
[ "$calls" = 3 ] || { echo "RED: tool.call source lines=$calls (want 3)"; exit 1; }
[ "$decs" = 3 ] || { echo "RED: decision annotation lines=$decs (want 3 round trips)"; exit 1; }
[ "$wbs" = 2 ] || { echo "RED: would_block lines=$wbs (rm + credential calls = want 2)"; exit 1; }
# the dangerous calls completed: responses for id 3 and 4 exist (the
# relay answered nothing on their behalf — interception is impossible)
grep -q '"id":3' "$relayed" && grep -q '"id":4' "$relayed" || { echo 'RED: dangerous call responses vanished'; exit 1; }
if grep -qE '"decision":"(deny|block|ask)"' "$MOUT"; then echo 'RED: forbidden decision value in mcp audit'; exit 1; fi
# args leave as hash + redacted excerpt only, sizes + latency recorded
grep -q '"args_h":"' "$MOUT" || { echo 'RED: missing args hash attribute'; exit 1; }
grep -q '"lat_ms":"' "$MOUT" && grep -q '"req_b":"' "$MOUT" && grep -q '"resp_b":"' "$MOUT" || { echo 'RED: transport metrics missing'; exit 1; }
# oversized-payload robustness: a 2MB request line must be forwarded
# to the server untouched; the fixture table has no id 9, so zero
# response bytes back is itself byte-identity for the response
# direction — and neither end may hang or corrupt the stream.
{ printf '%s' '{"jsonrpc":"2.0","id":9,"method":"tools/call","params":{"name":"big","arguments":{"blob":"'; head -c 2000000 /dev/zero | tr '\0' 'A'; printf '%s\n' '"}}}' ; } > "$HD/big-reqs"
"$BIN" mcp --server "$TMPDIR/mcpfixture" --out "$HD/big-audit.jsonl" < "$HD/big-reqs" > "$HD/big.out"
[ ! -s "$HD/big.out" ] || { echo 'RED: unexpected bytes for a no-table request'; exit 1; }
echo 'MCP RELAY OK (byte identity both directions, would_block annotation without interference, metrics present)'

step '09 frozen contract still holds (dual-source + negative control)'
node scripts/apicontract-check.mjs
node scripts/apicontract-check.mjs --negative
grep -q 'session.start' docs/api-v0.md && grep -q 'turn.stop' docs/api-v0.md || { echo 'RED: adapter types absent from the contract doc'; exit 1; }
echo 'CONTRACT OK'

step '10 tripwire (selftest control first) — internal identifiers never ship'
bash scripts/tripwire.sh --selftest
bash scripts/tripwire.sh
echo 'TRIPWIRE OK'

step '11 full inherited regression: gate-d1 + d2 + d3 + d4 + d5 serial'
for n in 1 2 3 4 5; do
  bash "scripts/gate-d$n.sh" > "$TMPDIR/gate-d6-reg-d$n.out" 2>&1 || { tail -25 "$TMPDIR/gate-d6-reg-d$n.out"; echo "RED: gate-d$n regression"; exit 1; }
  grep -q "GATE-D$n: ALL GREEN" "$TMPDIR/gate-d6-reg-d$n.out"
done
echo 'D1+D2+D3+D4+D5 REGRESSION GREEN (serial, full batteries)'

printf '\nGATE-D6: ALL GREEN\n'
