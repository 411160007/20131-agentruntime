package schema

import (
	"fmt"
	"time"
)

// Stage is one step of the security pipeline:
// Agent Proposed -> Policy Evaluated -> Native Enforcement -> Action.
type Stage string

const (
	StageProposed    Stage = "proposed"
	StageEvaluated   Stage = "evaluated"
	StageEnforcement Stage = "enforcement"
	StageAction      Stage = "action"
	StageObservation Stage = "observation" // collector lifecycle (start/stop/heartbeat)
)

func (s Stage) Valid() bool {
	switch s {
	case StageProposed, StageEvaluated, StageEnforcement, StageAction, StageObservation:
		return true
	}
	return false
}

// EventType classifies what an event describes.
type EventType string

const (
	TypeCommandProposed EventType = "command.proposed"
	TypeToolCall        EventType = "tool.call"
	TypeFileAccess      EventType = "file.access"
	TypeNetworkIntent   EventType = "network.intent"
	TypePolicyDecision  EventType = "policy.decision"
	TypeEnforceAction   EventType = "enforce.action"
	TypeCollectorStart  EventType = "collector.start"
	TypeCollectorStop   EventType = "collector.stop"
	TypeAgentDetected   EventType = "agent.detected"
	TypeAgentScan       EventType = "agent.scan"
)

// AllEventTypes lists every valid EventType in declaration order.
// scripts/validate-jsonl.mjs mirrors this list as the independent second
// source of truth; TestEnumsStayInSyncWithValidator fails if they diverge.
func AllEventTypes() []string {
	return []string{
		string(TypeCommandProposed), string(TypeToolCall), string(TypeFileAccess),
		string(TypeNetworkIntent), string(TypePolicyDecision), string(TypeEnforceAction),
		string(TypeCollectorStart), string(TypeCollectorStop),
		string(TypeAgentDetected), string(TypeAgentScan),
	}
}

// AllStages lists every valid Stage in declaration order.
func AllStages() []string {
	return []string{
		string(StageProposed), string(StageEvaluated), string(StageEnforcement),
		string(StageAction), string(StageObservation),
	}
}

func (t EventType) Valid() bool {
	switch t {
	case TypeCommandProposed, TypeToolCall, TypeFileAccess, TypeNetworkIntent,
		TypePolicyDecision, TypeEnforceAction, TypeCollectorStart, TypeCollectorStop,
		TypeAgentDetected, TypeAgentScan:
		return true
	}
	return false
}

// Decision is the outcome of policy evaluation for an event.
// In Phase 0 the runtime observes and records only; decisions are computed
// but never enforced, and "would_block" is recorded as such.
type Decision string

const (
	DecisionAllow      Decision = "allow"
	DecisionAsk        Decision = "ask"
	DecisionWouldBlock Decision = "would_block" // Phase 0: recorded, not enforced
)

func (d Decision) Valid() bool {
	switch d {
	case DecisionAllow, DecisionAsk, DecisionWouldBlock:
		return true
	}
	return false
}

// DecisionFor maps a policy Effect to the Event Decision vocabulary.
func DecisionFor(ef Effect) (Decision, error) {
	switch ef {
	case EffectAllow:
		return DecisionAllow, nil
	case EffectAsk:
		return DecisionAsk, nil
	case EffectWouldBlock:
		return DecisionWouldBlock, nil
	}
	return "", fmt.Errorf("schema: no decision for effect %q", string(ef))
}

// Severity grades the risk weight of an event for later ranking.
type Severity int

const (
	SevInfo     Severity = 0
	SevLow      Severity = 1
	SevMedium   Severity = 2
	SevHigh     Severity = 3
	SevCritical Severity = 4
)

func (s Severity) Valid() bool { return s >= SevInfo && s <= SevCritical }

// Event is the single JSONL audit record shape.
//
// Field contract (v1):
//   - V: schema version, currently 1.
//   - TS: RFC3339 with nanoseconds, UTC.
//   - ID: unique within a collector run.
//   - AgentID: references Agent.ID of the emitting/observed agent.
//   - Stage/Type/Decision/Severity: enums above.
//   - Summary: short human-readable text, no secrets.
//   - Attrs: optional flat string map for details (paths, tool names...).
type Event struct {
	V        int               `json:"v"`
	TS       time.Time         `json:"ts"`
	ID       string            `json:"id"`
	AgentID  string            `json:"agent_id"`
	Stage    Stage             `json:"stage"`
	Type     EventType         `json:"type"`
	Decision Decision          `json:"decision"`
	Severity Severity          `json:"severity"`
	Summary  string            `json:"summary"`
	Attrs    map[string]string `json:"attrs,omitempty"`
}

// SchemaVersion is the current Event schema version.
const SchemaVersion = 1

// Validate enforces the Event field contract.
func (e *Event) Validate() error {
	if e.V != SchemaVersion {
		return fmt.Errorf("schema: event %s: unsupported version %d (want %d)", e.ID, e.V, SchemaVersion)
	}
	if !validID(e.ID) {
		return fmt.Errorf("schema: event id %q invalid", e.ID)
	}
	if !validID(e.AgentID) {
		return fmt.Errorf("schema: event %s: agent_id %q invalid", e.ID, e.AgentID)
	}
	if e.TS.IsZero() {
		return fmt.Errorf("schema: event %s: ts is zero", e.ID)
	}
	if e.TS.Year() < 2020 || e.TS.Year() > 2100 {
		return fmt.Errorf("schema: event %s: ts year %d out of sane range", e.ID, e.TS.Year())
	}
	if !e.Stage.Valid() {
		return fmt.Errorf("schema: event %s: unknown stage %q", e.ID, string(e.Stage))
	}
	if !e.Type.Valid() {
		return fmt.Errorf("schema: event %s: unknown type %q", e.ID, string(e.Type))
	}
	if !e.Decision.Valid() {
		return fmt.Errorf("schema: event %s: unknown decision %q", e.ID, string(e.Decision))
	}
	if !e.Severity.Valid() {
		return fmt.Errorf("schema: event %s: severity %d out of range", e.ID, int(e.Severity))
	}
	if e.Summary == "" || len(e.Summary) > 512 {
		return fmt.Errorf("schema: event %s: summary empty or >512 bytes", e.ID)
	}
	for k, v := range e.Attrs {
		if k == "" || len(k) > 64 {
			return fmt.Errorf("schema: event %s: attr key %q invalid", e.ID, k)
		}
		if len(v) > 1024 {
			return fmt.Errorf("schema: event %s: attr %s value >1024 bytes", e.ID, k)
		}
	}
	return nil
}
