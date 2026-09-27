package main

import (
	"bufio"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"20131.com/agentruntime/internal/schema"
)

// --- hook receiver E2E -------------------------------------------------

const hookDanger = `{"hook_event_name":"PreToolUse","session_id":"e2e-sess","tool_name":"Bash","tool_input":{"command":"rm -rf /tmp/demo"}}` + "\n"
const hookHarmless = `{"hook_event_name":"PostToolUse","session_id":"e2e-sess","tool_name":"Bash","tool_input":{"command":"git status"}}` + "\n"
const hookGarbage = `{{{ not json at all`

// TestHookReceiverEndToEnd runs the receiver in a child process whose
// stdin is a hook payload (the receiver reads os.Stdin, so an in-process
// call cannot prove the contract). Pass-through is asserted as: exit
// code 0 and ZERO bytes on stdout, for dangerous input too.
func TestHookReceiverEndToEnd(t *testing.T) {
	if os.Getenv("AC_HOOK_ROLE") == "run" {
		cfg := config{out: os.Getenv("AC_HOOK_OUT")}
		_ = runHookSub(cfg)
		os.Exit(0) // the receiver's contract: unconditional 0
	}
	dir := t.TempDir()
	out := filepath.Join(dir, "audit.jsonl")

	run := func(payload string) (string, int) {
		cmd := exec.Command(os.Args[0], "-test.run=^TestHookReceiverEndToEnd$")
		cmd.Env = append(os.Environ(), "AC_HOOK_ROLE=run", "AC_HOOK_OUT="+out)
		cmd.Stdin = strings.NewReader(payload)
		var stdout, stderr strings.Builder
		cmd.Stdout = &stdout
		cmd.Stderr = &stderr
		err := cmd.Run()
		code := 0
		if ee, ok := err.(*exec.ExitError); ok {
			code = ee.ExitCode()
		} else if err != nil {
			t.Fatalf("child: %v", err)
		}
		return stdout.String(), code
	}

	if got, code := run(hookDanger); code != 0 || got != "" {
		t.Fatalf("dangerous hook: stdout=%q rc=%d (pass-through requires empty+0)", got, code)
	}
	if got, code := run(hookHarmless); code != 0 || got != "" {
		t.Fatalf("harmless hook: stdout=%q rc=%d", got, code)
	}
	before := statAudit(t, out)
	if got, code := run(hookGarbage); code != 0 || got != "" {
		t.Fatalf("garbage hook: stdout=%q rc=%d", got, code)
	}
	if got, code := run(`{"hook_event_name":"SubagentStop","session_id":"x"}` + "\n"); code != 0 || got != "" {
		t.Fatalf("unknown event hook: stdout=%q rc=%d", got, code)
	}
	if after := statAudit(t, out); after != before {
		t.Fatalf("garbage/unknown payloads must record ZERO bytes: size %d -> %d", before, after)
	}

	lines := readAudit(t, out)
	if len(lines) != 4 { // 2 source lines + 2 decision annotations
		t.Fatalf("audit lines: %d, want 4:\n%s", len(lines), strings.Join(lines, "\n"))
	}
	var toolCall, decisionAllow, decisionWB int
	for _, ln := range lines {
		var e schema.Event
		if err := json.Unmarshal([]byte(ln), &e); err != nil {
			t.Fatalf("undecodable audit line: %v", err)
		}
		if err := e.Validate(); err != nil {
			t.Fatalf("schema-invalid audit line: %v", err)
		}
		switch {
		case e.Type == schema.TypeToolCall && e.Stage == schema.StageProposed:
			toolCall++
			if e.Attrs["cmdline"] == "" || e.Attrs["session"] == "" {
				t.Errorf("tool.call attrs thin: %v", e.Attrs)
			}
		case e.Type == schema.TypeToolCall && e.Stage == schema.StageAction:
			toolCall++
		case e.Type == schema.TypePolicyDecision && e.Decision == schema.DecisionWouldBlock:
			decisionWB++
			if e.Attrs["rule"] != "destroy.rmrf" {
				t.Errorf("danger line must cite the rm rule, got %v", e.Attrs)
			}
		case e.Type == schema.TypePolicyDecision && e.Decision == schema.DecisionAllow:
			decisionAllow++
		}
	}
	if toolCall != 2 || decisionWB != 1 || decisionAllow != 1 {
		t.Fatalf("line mix wrong: calls=%d allow=%d wb=%d", toolCall, decisionAllow, decisionWB)
	}
	// Cross-agent continuity: both lines share one stable session agent id.
	ids := map[string]bool{}
	for _, ln := range lines {
		var e schema.Event
		_ = json.Unmarshal([]byte(ln), &e)
		ids[e.AgentID] = true
	}
	if len(ids) != 1 {
		t.Fatalf("same session produced %d agent ids: %v", len(ids), ids)
	}
}

// TestHookZeroBehaviorChangeControl asserts the E2E behavioural control
// from the slice spec: a scripted agent subprocess that invokes the hook
// before each "tool" produces IDENTICAL stdout whether the hook is wired
// or not (the receiver contributes nothing back).
func TestHookZeroBehaviorChangeControl(t *testing.T) {
	if os.Getenv("AC_AGENT_SCRIPT") == "1" {
		hooked := os.Getenv("AC_HOOKED") == "1"
		for _, p := range []string{hookDanger, hookHarmless} {
			if hooked {
				c := exec.Command(os.Args[0], "-test.run=^TestHookReceiverEndToEnd$")
				c.Env = append(os.Environ(), "AC_HOOK_ROLE=run", "AC_HOOK_OUT="+os.Getenv("AC_HOOK_OUT"))
				c.Stdin = strings.NewReader(p)
				outBytes, err := c.Output()
				if err != nil || len(outBytes) != 0 {
					// Output() captures stdout: the receiver must yield none
					t.Errorf("receiver leaked stdout: %q err=%v", outBytes, err)
				}
			}
			// The agent's own behaviour: deterministic local work + report.
			os.Stdout.WriteString("agent did tool step\n")
		}
		os.Exit(0)
	}
	dir := t.TempDir()
	out := filepath.Join(dir, "audit.jsonl")
	runScript := func(hooked bool) string {
		cmd := exec.Command(os.Args[0], "-test.run=^TestHookZeroBehaviorChangeControl$")
		cmd.Env = append(os.Environ(), "AC_AGENT_SCRIPT=1", "AC_HOOK_OUT="+out)
		if hooked {
			cmd.Env = append(cmd.Env, "AC_HOOKED=1")
		}
		b, err := cmd.Output()
		if err != nil {
			t.Fatal(err)
		}
		return string(b)
	}
	plain := runScript(false)
	wired := runScript(true)
	if plain != wired {
		t.Fatalf("hooking the agent changed its output:\nplain=%q\nwired=%q", plain, wired)
	}
	if plain != "agent did tool step\nagent did tool step\n" {
		t.Fatalf("script itself drifted: %q", plain)
	}
}

func statAudit(t *testing.T, path string) int64 {
	t.Helper()
	fi, err := os.Stat(path)
	if os.IsNotExist(err) {
		return 0
	}
	if err != nil {
		t.Fatal(err)
	}
	return fi.Size()
}

func readAudit(t *testing.T, path string) []string {
	t.Helper()
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return strings.Split(strings.TrimRight(string(b), "\n"), "\n")
}

// --- MCP relay E2E ------------------------------------------------------

// Fixture JSON-RPC traffic: request lines and the exact response bytes
// the fixture server returns (used for the byte-identity assertions).
var (
	mcpRequests = strings.Join([]string{
		`{"jsonrpc":"2.0","id":1,"method":"initialize","params":{}}`,
		`{"jsonrpc":"2.0","id":2,"method":"tools/call","params":{"name":"eval_sandbox","arguments":{"command":"rm -rf out"}}}`,
		`{"jsonrpc":"2.0","id":3,"method":"tools/call","params":{"name":"list_files","arguments":{"path":"docs"}}}`,
		`{"jsonrpc":"2.0","id":4,"method":"tools/call","params":{"name":"read_env","arguments":{"file_path":"/home/u/.aws/credentials"}}}`,
	}, "\n") + "\n"

	mcpResponses = strings.Join([]string{
		`{"jsonrpc":"2.0","id":1,"result":{"protocolVersion":"x"}}`,
		`{"jsonrpc":"2.0","id":2,"result":{"content":[{"type":"text","text":"done"}]}}`,
		`{"jsonrpc":"2.0","id":3,"result":{"content":[{"type":"text","text":"done"}]}}`,
		`{"jsonrpc":"2.0","id":4,"result":{"content":[{"type":"text","text":"done"}]}}`,
	}, "\n") + "\n"
)

// TestMCPRelayEndToEnd runs the full relay against the fixture server
// and proves: (1) byte identity at both ends vs a direct server run,
// (2) audit lines for each tools/call, (3) would_block annotations for
// dangerous calls WITHOUT any change to the response bytes.
func TestMCPRelayEndToEnd(t *testing.T) {
	for _, a := range os.Args {
		if a == "--ac-mcp-fixture" {
			mcpFixtureServer()
			os.Exit(0) // before the test framework prints its PASS line
		}
	}
	if os.Getenv("AC_MCP_ROLE") == "proxy" {
		cfg := config{
			out:    os.Getenv("AC_MCP_OUT"),
			server: os.Args[0],
		}
		err := runMCPSub(cfg, []string{"-test.run=^TestMCPRelayEndToEnd$", "--", "--ac-mcp-fixture"})
		if err != nil {
			os.Exit(1)
		}
		os.Exit(0)
	}
	dir := t.TempDir()
	out := filepath.Join(dir, "audit.jsonl")

	// Control run: talk to the fixture server DIRECTLY (no relay).
	direct := exec.Command(os.Args[0], "-test.run=^TestMCPRelayEndToEnd$", "--", "--ac-mcp-fixture")
	direct.Stdin = strings.NewReader(mcpRequests)
	directB, err := direct.Output()
	if err != nil {
		t.Fatalf("fixture control run: %v", err)
	}
	if string(directB) != mcpResponses {
		t.Fatalf("fixture control bytes unexpected:\n%q", directB)
	}

	// Through the relay.
	proxy := exec.Command(os.Args[0], "-test.run=^TestMCPRelayEndToEnd$")
	proxy.Env = append(os.Environ(), "AC_MCP_ROLE=proxy", "AC_MCP_OUT="+out)
	proxy.Stdin = strings.NewReader(mcpRequests)
	relayedB, err := proxy.Output()
	if err != nil {
		t.Fatalf("proxy run: %v", err)
	}
	if string(relayedB) != string(directB) {
		t.Fatalf("relay altered client-visible bytes:\nvia=%q\ndirect=%q", relayedB, directB)
	}

	lines := readAudit(t, out)
	var calls, decWB, decAllow int
	for _, ln := range lines {
		var e schema.Event
		if err := json.Unmarshal([]byte(ln), &e); err != nil {
			t.Fatalf("undecodable: %v\n%s", err, ln)
		}
		if err := e.Validate(); err != nil {
			t.Fatalf("invalid audit line: %v", err)
		}
		switch {
		case e.Type == schema.TypeToolCall:
			calls++
			for _, k := range []string{"server", "tool", "args_h", "args_x", "lat_ms", "req_b", "resp_b"} {
				if e.Attrs[k] == "" {
					t.Errorf("tool.call missing attr %s: %v", k, e.Attrs)
				}
			}
			if !strings.Contains(e.Attrs["args_x"], "rm -rf") && e.Attrs["tool"] == "eval_sandbox" {
				// excerpt keeps the danger shape for forensics (redaction
				// strips credentials, not command text)
				t.Errorf("danger shape must survive the redacted excerpt: %q", e.Attrs["args_x"])
			}
		case e.Type == schema.TypePolicyDecision && e.Decision == schema.DecisionWouldBlock:
			decWB++
			if e.Attrs["rule"] == "" {
				t.Errorf("would_block without rule id: %v", e.Attrs)
			}
		case e.Type == schema.TypePolicyDecision && e.Decision == schema.DecisionAllow:
			decAllow++
		}
	}
	// 3 calls audited; 2 are dangerous-shaped (eval rule + rm cmdline),
	// list_files passes -> exactly 2 would_block, 1 allow.
	if calls != 3 || decWB != 2 || decAllow != 1 {
		t.Fatalf("audit mix: calls=%d wb=%d allow=%d\n%s", calls, decWB, decAllow, strings.Join(lines, "\n"))
	}
	// The decisive Phase 0 proof already ran above: response bytes for
	// the would_block calls (2 and 4) are byte-identical to the direct
	// server run. Annotation never touched transport.
}

func mcpFixtureServer() {
	// Deterministic canned responder: parse id + method, echo the exact
	// response table. Any deviation would be a fixture bug, not relay.
	scan := bufio.NewScanner(os.Stdin)
	scan.Buffer(make([]byte, 1<<20), 1<<20)
	byID := map[string]string{}
	for _, ln := range strings.Split(strings.TrimRight(mcpResponses, "\n"), "\n") {
		var m map[string]any
		if err := json.Unmarshal([]byte(ln), &m); err != nil {
			os.Exit(3)
		}
		idb, _ := json.Marshal(m["id"])
		byID[string(idb)] = ln
	}
	for scan.Scan() {
		line := scan.Bytes()
		var req struct {
			ID     json.RawMessage `json:"id"`
			Method string          `json:"method"`
		}
		if err := json.Unmarshal(line, &req); err != nil {
			continue // notifications etc: no response
		}
		if req.Method == "" {
			continue
		}
		if resp, ok := byID[string(req.ID)]; ok {
			os.Stdout.WriteString(resp + "\n")
		}
	}
}

// --- integrate (config generator) ---------------------------------------

func TestIntegrateThreeTargets(t *testing.T) {
	bin := "/opt/rt/agent-collector"

	// claude-code: creates settings.json, idempotent rerun.
	d1 := t.TempDir()
	if err := runIntegrateSub(config{target: "claude-code", dir: d1, bin: bin}); err != nil {
		t.Fatal(err)
	}
	first, err := os.ReadFile(filepath.Join(d1, "settings.json"))
	if err != nil {
		t.Fatal(err)
	}
	if err := runIntegrateSub(config{target: "claude-code", dir: d1, bin: bin}); err != nil {
		t.Fatal(err)
	}
	second, _ := os.ReadFile(filepath.Join(d1, "settings.json"))
	if string(first) != string(second) {
		t.Error("rerun mutated the config (idempotence broken)")
	}
	// Generated artifact carries zero decision vocabulary.
	for _, w := range []string{"deny", "permissionDecision", "\"ask\"", "\"block\""} {
		if strings.Contains(string(first), w) {
			t.Errorf("generated settings carry %q", w)
		}
	}

	// codex: NOTHING is written; the gap note is the output.
	d2 := t.TempDir()
	cmd := exec.Command(os.Args[0], "-test.run=^TestIntegrateThreeTargets$")
	if os.Getenv("AC_INT_ROLE") == "codex" {
		_ = runIntegrateSub(config{target: "codex", dir: os.Getenv("AC_DIR"), bin: bin})
		os.Exit(0)
	}
	cmd.Env = append(os.Environ(), "AC_INT_ROLE=codex", "AC_DIR="+d2)
	outb, err := cmd.Output()
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(outb), "No configuration written") {
		t.Errorf("codex note missing: %q", outb)
	}
	if fi, _ := os.ReadDir(d2); len(fi) != 0 {
		t.Error("codex target wrote files")
	}

	// openclaw: merged block, existing keys kept.
	d3 := t.TempDir()
	if err := os.WriteFile(filepath.Join(d3, "openclaw.json"), []byte(`{"keep":"me"}`), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := runIntegrateSub(config{target: "openclaw", dir: d3, bin: bin}); err != nil {
		t.Fatal(err)
	}
	raw, err := os.ReadFile(filepath.Join(d3, "openclaw.json"))
	if err != nil {
		t.Fatal(err)
	}
	var doc map[string]any
	if err := json.Unmarshal(raw, &doc); err != nil {
		t.Fatal(err)
	}
	if doc["keep"] != "me" || doc["agentRuntime"] == nil {
		t.Errorf("openclaw merge: %s", raw)
	}

	// unknown target fails without touching anything.
	if err := runIntegrateSub(config{target: "nope", dir: t.TempDir(), bin: bin}); err == nil {
		t.Error("unknown target must error")
	}
	// missing dir must error (never creates surprise trees).
	if err := runIntegrateSub(config{target: "claude-code", dir: filepath.Join(t.TempDir(), "gone"), bin: bin}); err == nil {
		t.Error("missing --dir must error")
	}
}
