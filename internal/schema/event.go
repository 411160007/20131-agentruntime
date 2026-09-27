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
	// Additive v1 vocabulary (platform adapter slice, compatibility note in
	// docs/api-v0.md): hook session lifecycle events observed through the
	// agent-facing adapter surface.
	TypeSessionStart EventType = "session.start"
	TypeTurnStop     EventType = "turn.stop"
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
		string(TypeSessionStart), string(TypeTurnStop),
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
		TypeAgentDetected, TypeAgentScan, TypeSessionStart, TypeTurnStop:
		return true
	}
	return false
}

// Tier orders events through the bus pipeline (spec vNext section 34).
// It is an ADDITIVE v1 field: a record with no "tier" key is valid and
// means L1, so every line written before tiers existed still validates
// and still routes identically.
//
//	L0  dropped at the bus; only counted, never persisted
//	L1  persisted directly (the pre-tier default path)
//	L2  persisted, and additionally delivered to rule-engine consumers
//	L3  persisted, delivered to rule-engine consumers, highest emphasis
//
// Rule-engine consumption lands with the security judgement slice (D4);
// until a consumer is attached, L2/L3 delivery is a no-op beyond the
// counters.
type Tier string

const (
	TierL0 Tier = "L0"
	TierL1 Tier = "L1"
	TierL2 Tier = "L2"
	TierL3 Tier = "L3"
)

// AllTiers lists every Tier value in declaration order. L0 is listed even
// though the bus never persists L0 records: the validator must recognize
// the vocabulary to reject wild values instead of silently accepting them.
func AllTiers() []string {
	return []string{string(TierL0), string(TierL1), string(TierL2), string(TierL3)}
}

// Valid reports whether t is a legal tier. The empty string is the
// absent-key legacy default (see EffectiveTier).
func (t Tier) Valid() bool {
	switch t {
	case "", TierL0, TierL1, TierL2, TierL3:
		return true
	}
	return false
}

// EffectiveTier resolves the legacy default: absent means L1.
func (t Tier) EffectiveTier() Tier {
	if t == "" {
		return TierL1
	}
	return t
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

// AllDecisions lists every Decision value in declaration order. The
// second-source validators (the Node JSONL validator, scripts/apicontract
// checks, and docs/api-v0.md) mirror this list; the sync tests fail if
// any source diverges.
func AllDecisions() []string {
	return []string{string(DecisionAllow), string(DecisionAsk), string(DecisionWouldBlock)}
}

func (d Decision) Valid() bool {
	switch d {
	case DecisionAllow, DecisionAsk, DecisionWouldBlock:
		return true
	}
	return false
}

// Phase0RuntimeDecisions is the non-obvious constraint binding every
// Phase 0 emitter: product decision values must stay inside
// {allow, would_block} (observation without enforcement). "ask" exists
// only as reserved contract vocabulary for a later phase. Emission sites
// call MustPhase0Decision; the audit writer never sees an out-of-set
// value, and gate-d3 greps for the ask literal on emission paths.
func Phase0RuntimeDecisions() []Decision {
	return []Decision{DecisionAllow, DecisionWouldBlock}
}

// MustPhase0Decision returns an error for any decision outside the
// Phase 0 runtime set (see Phase0RuntimeDecisions).
func MustPhase0Decision(d Decision) error {
	for _, ok := range Phase0RuntimeDecisions() {
		if d == ok {
			return nil
		}
	}
	return fmt.Errorf("schema: decision %q outside the phase-0 runtime vocabulary (allow|would_block only)", string(d))
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
// Field contract (v1, additively extended by the core completion tier:
// tier, cap/res_class attr vocabulary):
//   - V: schema version, currently 1.
//   - TS: RFC3339 with nanoseconds, UTC.
//   - ID: unique within a collector run.
//   - AgentID: references Agent.ID of the emitting/observed agent.
//   - Stage/Type/Decision/Severity: enums above.
//   - Tier: optional routing tier (L0-L3); absent = L1.
//   - Summary: short human-readable text, no secrets.
//   - Attrs: optional flat string map for details (paths, tool names...);
//     the reserved keys "cap" and "res_class", when present, must be
//     members of the capability and resource-sensitivity vocabularies.
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
	Tier     Tier              `json:"tier,omitempty"`
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
	if !e.Tier.Valid() {
		return fmt.Errorf("schema: event %s: unknown tier %q", e.ID, string(e.Tier))
	}
	if v, ok := e.Attrs["cap"]; ok && !Capability(v).Valid() {
		return fmt.Errorf("schema: event %s: attr cap %q not in vocabulary", e.ID, v)
	}
	if v, ok := e.Attrs["res_class"]; ok && !ResClass(v).Valid() {
		return fmt.Errorf("schema: event %s: attr res_class %q not in vocabulary", e.ID, v)
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
