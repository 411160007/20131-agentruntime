// Package adapter is the platform-adapter surface: it receives hook
// callbacks from agent CLIs on stdin, maps them onto the frozen audit
// Event vocabulary, reuses the discovery redaction layer, and generates
// the per-agent configuration that routes future callbacks here.
//
// Phase 0 boundary (observation without enforcement): the hook receiver
// NEVER writes anything to stdout and always exits successfully for the
// calling agent, whatever the judgement layer computes. A matched rule
// records a policy.decision line (would_block); it cannot and does not
// change what the agent does. The generator's emitted configuration
// carries no decision vocabulary at all — it only registers the
// command that records.
package adapter

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"20131.com/agentruntime/internal/discovery"
	"20131.com/agentruntime/internal/identity"
	"20131.com/agentruntime/internal/schema"
)

// HookSourceLabel is the stable locator prefix for hook-derived agent
// ids and the src audit attribute for this surface.
const HookSourceLabel = "claude-code"

// Known hook event names on the supported event face.
const (
	EventPreToolUse   = "PreToolUse"
	EventPostToolUse  = "PostToolUse"
	EventSessionStart = "SessionStart"
	EventStop         = "Stop"
)

// ErrUnknownHook signals a hook_event_name outside the supported face.
// Callers must fail OPEN on it (record nothing, change nothing): an
// adapter that cannot interpret an input must never disturb the agent.
var ErrUnknownHook = errors.New("adapter: unsupported hook event name")

// MaxHookBytes bounds one hook stdin payload (generous; larger input is
// treated as garbage and fail-opens).
const MaxHookBytes = 1 << 20

// HookInput is the JSON object an agent CLI delivers on the receiver's
// stdin. Unknown members are ignored (forward-compatible reading); the
// receiver never echoes any of it back to the agent.
type HookInput struct {
	HookEventName  string          `json:"hook_event_name"`
	SessionID      string          `json:"session_id"`
	ToolName       string          `json:"tool_name"`
	ToolInput      json.RawMessage `json:"tool_input"`
	Cwd            string          `json:"cwd"`
	TranscriptPath string          `json:"transcript_path"`
}

// ParseHookInput decodes one payload. Empty, oversized, or unparseable
// input returns an error; the caller fail-opens.
func ParseHookInput(raw []byte) (HookInput, error) {
	var h HookInput
	if len(raw) == 0 || len(raw) > MaxHookBytes {
		return h, fmt.Errorf("adapter: hook payload size %d invalid", len(raw))
	}
	dec := json.NewDecoder(strings.NewReader(string(raw)))
	if err := dec.Decode(&h); err != nil {
		return h, fmt.Errorf("adapter: hook payload json: %w", err)
	}
	if h.HookEventName == "" {
		return h, errors.New("adapter: hook payload missing hook_event_name")
	}
	return h, nil
}

// SessionHandle is the stable, non-reversible id used to correlate the
// hook lines of one agent session across events and runs.
func SessionHandle(sessionID string) string {
	h := sha256.Sum256([]byte("hook-session|" + sessionID))
	return hex.EncodeToString(h[:6])
}

// AgentIDForSession derives the stable audit agent id for one hook
// session: same session id on same machine => same id across events
// and across receiver processes (the cross-agent continuity face).
func AgentIDForSession(machine string, h HookInput) string {
	key := HookSourceLabel + "|session|" + h.SessionID
	return identity.AgentID(machine, key)
}

// ToolAttrs extracts the rule-matchable attribute keys (cmdline, path,
// domain) from a tool_input payload. Values pass through the shared
// redaction layer BEFORE truncation. Absent or non-string members are
// skipped; nothing is invented. Shared with the MCP proxy surface so
// both adapter faces feed the same judgement fields.
func ToolAttrs(raw json.RawMessage) map[string]string {
	out := map[string]string{}
	if len(raw) == 0 {
		return out
	}
	var m map[string]any
	if err := json.Unmarshal(raw, &m); err != nil {
		return out
	}
	pick := func(dst, key string, max int) {
		v, ok := m[key].(string)
		if !ok || v == "" {
			return
		}
		out[dst] = discovery.TruncateRedacted(v, max)
	}
	pick("cmdline", "command", 320)
	pick("path", "file_path", 320)
	if _, ok := out["path"]; !ok {
		pick("path", "path", 320)
	}
	pick("domain", "url", 320)
	return out
}

// HookEvent maps one parsed hook payload onto an audit Event. The
// returned event carries tier L2 so the rule engine annotates it
// (persist + judge, still observation-only). The summary never holds
// credential-shaped content: free text passes through the redactor.
//
// Mount class (W1.2): the evidence-class stamp is decided by WHERE the
// event was collected and by the event's position on the hook face —
// never by what the payload claims. Lifecycle notifications (session
// start / turn stop) are adapter-observed facts about the agent, so
// they carry agent_meta; tool-use notifications relay the agent's own
// description of its action (tool_name, tool_input), so they carry
// agent_self. A payload forging a higher class cannot raise its own
// class: the receiver's parser reads only the known fields below, and
// the stamp is assigned from the mount, not from the input.
func HookEvent(h HookInput, machine string, now time.Time, id string) (*schema.Event, error) {
	var (
		stype   schema.EventType
		stage   schema.Stage
		summary string
		class   schema.SourceClass
	)
	switch h.HookEventName {
	case EventPreToolUse:
		stype, stage, class = schema.TypeToolCall, schema.StageProposed, schema.SrcAgentSelf
		summary = "hook " + EventPreToolUse + " tool=" + discovery.TruncateRedacted(h.ToolName, 96)
	case EventPostToolUse:
		stype, stage, class = schema.TypeToolCall, schema.StageAction, schema.SrcAgentSelf
		summary = "hook " + EventPostToolUse + " tool=" + discovery.TruncateRedacted(h.ToolName, 96)
	case EventSessionStart:
		stype, stage, class = schema.TypeSessionStart, schema.StageObservation, schema.SrcAgentMeta
		summary = "hook " + EventSessionStart
	case EventStop:
		stype, stage, class = schema.TypeTurnStop, schema.StageObservation, schema.SrcAgentMeta
		summary = "hook " + EventStop
	default:
		return nil, fmt.Errorf("%w: %q", ErrUnknownHook, h.HookEventName)
	}
	attrs := map[string]string{
		"hook": h.HookEventName,
		"kind": HookSourceLabel,
		"name": discovery.TruncateRedacted(h.ToolName, 96),
	}
	if h.SessionID != "" {
		attrs["session"] = SessionHandle(h.SessionID)
	}
	for k, v := range ToolAttrs(h.ToolInput) {
		attrs[k] = v
	}
	e := &schema.Event{
		V:        schema.SchemaVersion,
		TS:       now.UTC(),
		ID:       id,
		AgentID:  AgentIDForSession(machine, h),
		Stage:    stage,
		Type:     stype,
		Decision: schema.DecisionAllow, // source line records; judgement rides on the decision line
		Severity: schema.SevInfo,
		Summary:  summary,
		Tier:     schema.TierL2,
		Attrs:    attrs,
	}
	e.SourceClass = class
	return e, nil
}

// MCPEvent builds the audit line for one observed JSON-RPC tool call
// round trip through the proxy (fields supplied by the caller so this
// package keeps no dependency on the relay). Args never travel verbatim
// into the audit: only a truncated redacted excerpt plus a commitment
// hash of the raw bytes survive, alongside transport metrics. The event
// is L2 so the built-in rules annotate it; the transport decision is
// made by the relay, which never consults this event. The mount class is
// tool_mcp: the relay observed the round trip itself — nothing the args
// claim about provenance can move that class.
func MCPEvent(serverExe string, tool string, argsRaw []byte, latency time.Duration, reqBytes, respBytes int, machine string, now time.Time, id string) (*schema.Event, error) {
	tool = discovery.TruncateRedacted(tool, 96)
	h := sha256.Sum256(argsRaw)
	attrs := map[string]string{
		"kind":   "mcp-proxy",
		"server": discovery.TruncateRedacted(serverExe, 96),
		"tool":   tool,
		"args_h": hex.EncodeToString(h[:8]),
		"args_x": discovery.TruncateRedacted(string(argsRaw), 240),
		"lat_ms": fmt.Sprintf("%d", latency.Milliseconds()),
		"req_b":  fmt.Sprintf("%d", reqBytes),
		"resp_b": fmt.Sprintf("%d", respBytes),
	}
	// Rule-matchable enrichment from common argument shapes.
	for k, v := range ToolAttrs(argsRaw) {
		attrs[k] = v
	}
	summary := "mcp tools/call " + discovery.TruncateRedacted(serverExe, 48) + "/" + tool
	return &schema.Event{
		V:           schema.SchemaVersion,
		TS:          now.UTC(),
		ID:          id,
		AgentID:     identity.AgentID(machine, "mcp|"+strings.ToLower(serverExe)),
		Stage:       schema.StageAction,
		Type:        schema.TypeToolCall,
		Decision:    schema.DecisionAllow,
		Severity:    schema.SevInfo,
		Summary:     summary,
		Tier:        schema.TierL2,
		Attrs:       attrs,
		SourceClass: schema.SrcToolMCP,
	}, nil
}
