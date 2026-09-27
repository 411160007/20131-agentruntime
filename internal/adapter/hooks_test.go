package adapter

import (
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"

	"20131.com/agentruntime/internal/schema"
)

const testMachine = "test-machine-fp"

func hookRaw(t *testing.T, v map[string]any) []byte {
	t.Helper()
	b, err := json.Marshal(v)
	if err != nil {
		t.Fatal(err)
	}
	return b
}

func TestParseHookInputShapes(t *testing.T) {
	ok := hookRaw(t, map[string]any{
		"hook_event_name": "PreToolUse",
		"session_id":      "sess-42",
		"tool_name":       "Bash",
		"tool_input":      map[string]any{"command": "echo hi"},
	})
	h, err := ParseHookInput(ok)
	if err != nil || h.SessionID != "sess-42" {
		t.Fatalf("valid payload rejected: %v %+v", err, h)
	}
	if _, err := ParseHookInput(nil); err == nil {
		t.Error("empty payload must fail parse")
	}
	if _, err := ParseHookInput([]byte("{not json")); err == nil {
		t.Error("garbage payload must fail parse")
	}
	if _, err := ParseHookInput(hookRaw(t, map[string]any{"tool_name": "Bash"})); err == nil {
		t.Error("missing hook_event_name must fail parse")
	}
	// Unknown event NAMES parse fine (forward-compatible reading); the
	// refusal happens at event build (ErrUnknownHook) — asserted in
	// TestHookEventMapping.
	if _, err := ParseHookInput(hookRaw(t, map[string]any{"hook_event_name": "Nope"})); err != nil {
		t.Errorf("parse must accept unknown names for the mapper to judge: %v", err)
	}
}

func TestHookEventMapping(t *testing.T) {
	now := time.Date(2026, 9, 28, 4, 0, 0, 0, time.UTC)
	cases := []struct {
		hook  string
		stype schema.EventType
		stage schema.Stage
	}{
		{"PreToolUse", schema.TypeToolCall, schema.StageProposed},
		{"PostToolUse", schema.TypeToolCall, schema.StageAction},
		{"SessionStart", schema.TypeSessionStart, schema.StageObservation},
		{"Stop", schema.TypeTurnStop, schema.StageObservation},
	}
	for _, c := range cases {
		h := HookInput{HookEventName: c.hook, SessionID: "s1"}
		ev, err := HookEvent(h, testMachine, now, "hk-1")
		if err != nil {
			t.Fatalf("%s: %v", c.hook, err)
		}
		if ev.Type != c.stype || ev.Stage != c.stage {
			t.Errorf("%s mapped to %s/%s, want %s/%s", c.hook, ev.Stage, ev.Type, c.stage, c.stype)
		}
		if err := ev.Validate(); err != nil {
			t.Errorf("%s: event invalid: %v", c.hook, err)
		}
		if ev.Decision != schema.DecisionAllow || ev.Tier != schema.TierL2 {
			t.Errorf("%s: decision/tier wrong: %s %s", c.hook, ev.Decision, ev.Tier)
		}
	}
	if _, err := HookEvent(HookInput{HookEventName: "SubagentStop", SessionID: "s"}, testMachine, now, "x"); !errors.Is(err, ErrUnknownHook) {
		t.Errorf("unknown event must surface ErrUnknownHook, got %v", err)
	}
}

func TestHookEventRedactionAndAttrs(t *testing.T) {
	now := time.Now()
	raw := hookRaw(t, map[string]any{
		"hook_event_name": "PreToolUse",
		"session_id":      "sess-7",
		"tool_name":       "Bash",
		"tool_input":      map[string]any{"command": "deploy --token supersecret123 && rm -rf build"},
	})
	h, err := ParseHookInput(raw)
	if err != nil {
		t.Fatal(err)
	}
	ev, err := HookEvent(h, testMachine, now, "hk-r1")
	if err != nil {
		t.Fatal(err)
	}
	cmd := ev.Attrs["cmdline"]
	if strings.Contains(cmd, "supersecret123") {
		t.Errorf("secret survived redaction into audit: %q", cmd)
	}
	if !strings.Contains(cmd, "rm -rf") {
		t.Errorf("dangerous shape must survive redaction for the rules engine: %q", cmd)
	}
	if !strings.Contains(cmd, "[redacted]") {
		t.Errorf("redaction marker missing: %q", cmd)
	}
	if ev.Attrs["hook"] != "PreToolUse" || ev.Attrs["kind"] != HookSourceLabel {
		t.Errorf("attrs shape: %v", ev.Attrs)
	}
	if err := ev.Validate(); err != nil {
		t.Fatal(err)
	}
	// Same session, same machine => same stable agent id (continuity).
	ev2, _ := HookEvent(HookInput{HookEventName: "Stop", SessionID: "sess-7"}, testMachine, now, "hk-r2")
	if ev2.AgentID != ev.AgentID {
		t.Errorf("session agent id unstable: %s vs %s", ev.AgentID, ev2.AgentID)
	}
	if !strings.HasPrefix(ev.AgentID, "agi-") {
		t.Errorf("agent id shape: %s", ev.AgentID)
	}
}

func TestToolAttrsExtraction(t *testing.T) {
	a := ToolAttrs(json.RawMessage(`{"file_path":"/etc/sudoers","url":"https://example.invalid/x"}`))
	if a["path"] != "/etc/sudoers" || a["domain"] == "" {
		t.Errorf("extraction: %v", a)
	}
	if _, ok := a["cmdline"]; ok {
		t.Error("absent command must not be invented")
	}
	if len(ToolAttrs(nil)) != 0 || len(ToolAttrs(json.RawMessage(`[]`))) != 0 {
		t.Error("non-object input must yield no attrs")
	}
}

func TestMCPEventShape(t *testing.T) {
	args := json.RawMessage(`{"command":"run --token topsecret99","file_path":"/home/u/.ssh/id_ed25519"}`)
	ev, err := MCPEvent("srv-bin", "eval_sandbox", args, 125*time.Millisecond, 210, 480, testMachine, time.Now(), "mc-1")
	if err != nil {
		t.Fatal(err)
	}
	if err := ev.Validate(); err != nil {
		t.Fatal(err)
	}
	if ev.Type != schema.TypeToolCall || ev.Tier != schema.TierL2 {
		t.Errorf("mcp audit shape: %s/%s", ev.Type, ev.Tier)
	}
	if ev.Attrs["tool"] != "eval_sandbox" || ev.Attrs["server"] != "srv-bin" {
		t.Errorf("attrs: %v", ev.Attrs)
	}
	if len(ev.Attrs["args_h"]) != 16 {
		t.Errorf("args hash: %q", ev.Attrs["args_h"])
	}
	if strings.Contains(ev.Attrs["args_x"], "topsecret99") || strings.Contains(ev.Attrs["cmdline"], "topsecret99") {
		t.Error("raw secret leaked through mcp audit attrs")
	}
	if ev.Attrs["path"] == "" || ev.Attrs["cmdline"] == "" {
		t.Errorf("rule-matchable enrichment missing: %v", ev.Attrs)
	}
	if ev.Attrs["lat_ms"] != "125" || ev.Attrs["req_b"] != "210" || ev.Attrs["resp_b"] != "480" {
		t.Errorf("transport metrics: %v", ev.Attrs)
	}
	// Deterministic hash commitment over identical args.
	ev2, _ := MCPEvent("srv-bin", "eval_sandbox", args, time.Millisecond, 1, 1, testMachine, time.Now(), "mc-2")
	if ev2.Attrs["args_h"] != ev.Attrs["args_h"] {
		t.Error("args hash not deterministic")
	}
}

func TestSessionHandleOneWay(t *testing.T) {
	a := SessionHandle("sess-7")
	b := SessionHandle("sess-7")
	if a != b || len(a) != 12 || strings.Contains(a, "sess-7") {
		t.Errorf("session handle: %s %s", a, b)
	}
	if SessionHandle("other") == a {
		t.Error("distinct sessions collided")
	}
}

func TestNoBlockingVocabularyOnShippedPaths(t *testing.T) {
	// Structural Phase 0 guard for this slice's surface: neither the
	// generated configuration nor the receiver's notes may carry a
	// decision word that could instruct an agent.
	for _, s := range []string{CodexGap} {
		low := strings.ToLower(s)
		for _, w := range []string{"deny", "block", "permissiondecision"} {
			if strings.Contains(low, w) {
				t.Errorf("vocabulary leak %q in %q", w, s)
			}
		}
	}
}
