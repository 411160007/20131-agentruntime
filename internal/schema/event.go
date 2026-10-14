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
	// Additive v1 vocabulary (intent recorder slice, contract note in
	// docs/api-v0.md): a stated-intent hint recorded beside a session.
	// Carries the optional Goal field, which is required for this type;
	// observation-only, never a decision of its own.
	TypeIntentRecord EventType = "intent.record"
	// Additive v1 vocabulary (task record-form slice, contract note in
	// docs/api-v0.md): the task lifecycle object materialized as a
	// record shape. Carries TaskID and TaskStatus, both required for
	// this type; observation-only, never a decision of its own. This
	// slice lands the schema side only — no shipped emission path
	// writes this line yet; see docs/task-primitive-design.md section 5.
	TypeTaskRecord EventType = "task.record"
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
		string(TypeIntentRecord), string(TypeTaskRecord),
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
		TypeAgentDetected, TypeAgentScan, TypeSessionStart, TypeTurnStop,
		TypeIntentRecord, TypeTaskRecord:
		return true
	}
	return false
}

// TaskStatus is the seven-token lifecycle closed set defined by
// docs/task-primitive-design.md (task_status_vocabulary). It is an
// ADDITIVE v1 field: a record with no "task_status" key is valid and
// means legacy-unclassified, so every line written before the task
// record form existed still validates and still replays identically.
// Absent-key posture mirrors SourceClass; wild values are rejected,
// never silently accepted. Observation-only: no judgement path here
// consumes it, and no token grants enforcement existence to any plane.
type TaskStatus string

const (
	TaskOpen      TaskStatus = "open"
	TaskActive    TaskStatus = "active"
	TaskCompleted TaskStatus = "completed"
	TaskFailed    TaskStatus = "failed"
	TaskAbandoned TaskStatus = "abandoned"
	TaskBlocked   TaskStatus = "blocked"
	TaskUnknown   TaskStatus = "unknown"
)

// AllTaskStatuses lists every TaskStatus value in declaration order.
// scripts/validate-jsonl.mjs mirrors this list (second source) and
// docs/api-v0.md restates it (third source); the sync tests fail if
// the three sources diverge.
func AllTaskStatuses() []string {
	return []string{
		string(TaskOpen), string(TaskActive), string(TaskCompleted),
		string(TaskFailed), string(TaskAbandoned), string(TaskBlocked),
		string(TaskUnknown),
	}
}

// Valid reports whether s is a legal task status. The empty string is
// the absent-key legacy default (unclassified, valid on every type
// except task.record, which requires a real member).
func (s TaskStatus) Valid() bool {
	switch s {
	case "", TaskOpen, TaskActive, TaskCompleted, TaskFailed,
		TaskAbandoned, TaskBlocked, TaskUnknown:
		return true
	}
	return false
}

// validTaskID checks the positional task identity shape of
// docs/task-primitive-design.md section 4: session_id + ":" + ordinal.
// The session part reuses validID; the ordinal is 1-9 plain digits.
// No content hash of mutable text ever lands in this shape.
func validTaskID(s string) bool {
	for i := 0; i < len(s); i++ {
		if s[i] != ':' {
			continue
		}
		if i == 0 || !validID(s[:i]) {
			return false
		}
		rest := s[i+1:]
		if len(rest) == 0 || len(rest) > 9 {
			return false
		}
		for j := 0; j < len(rest); j++ {
			if rest[j] < '0' || rest[j] > '9' {
				return false
			}
		}
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

// SourceClass records WHERE an event's evidence was collected, using the
// six evidence-provenance classes of the trust order, listed in
// descending trust (highest first). It is an ADDITIVE v1 field, carried
// on the same version-evolution mechanism as Tier: a record with no
// "source_class" key is valid and means unclassified legacy, so every
// line written before this field existed still validates and nothing
// about it is reinterpreted.
//
// The class is a property of the collection mount, never of the payload:
// a payload describing itself as high-trust does not raise its own
// class (mount wiring lands with the collection slice; until a mount
// stamps the field, emitters leave it absent). In this phase the field
// is OBSERVATION-ONLY: no judgement, rule, or decision path consumes
// it, and gate-d7 machine-asserts that zero-consumption structurally.
type SourceClass string

const (
	SrcNativeOS          SourceClass = "native_os"          // native OS evidence (direct kernel/proc/FS observation)
	SrcRuntime           SourceClass = "runtime"            // runtime event (in-process collector observation)
	SrcToolMCP           SourceClass = "tool_mcp"           // tool/MCP metadata (relay-observed round trips)
	SrcAgentMeta         SourceClass = "agent_meta"         // agent metadata (adapter/hook lifecycle facts)
	SrcAgentSelf         SourceClass = "agent_self"         // agent self-description (payload claims)
	SrcLLMInterpretation SourceClass = "llm_interpretation" // external LLM interpretation (derived reading)
)

// AllSourceClasses lists every SourceClass in declaration order, which
// is the trust order: native_os is the highest class and
// llm_interpretation the lowest. scripts/validate-jsonl.mjs mirrors
// this list as the independent second source of truth and
// docs/api-v0.md carries it as the frozen external contract; the sync
// tests fail if any of the three sources diverges.
func AllSourceClasses() []string {
	return []string{
		string(SrcNativeOS), string(SrcRuntime), string(SrcToolMCP),
		string(SrcAgentMeta), string(SrcAgentSelf), string(SrcLLMInterpretation),
	}
}

// Valid reports whether s is a legal source class. The empty string is
// the absent-key legacy meaning (unclassified), matching the additive
// contract: legacy lines validate unchanged.
func (s SourceClass) Valid() bool {
	switch s {
	case "", SrcNativeOS, SrcRuntime, SrcToolMCP, SrcAgentMeta, SrcAgentSelf, SrcLLMInterpretation:
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
//   - SourceClass: optional evidence-provenance class (see SourceClass);
//     absent = unclassified legacy; observation-only, never decides.
//   - DataAction: optional nine-class data action annotation (see
//     DataAction); absent = unclassified legacy; record-only, never
//     decides.
//   - Summary: short human-readable text, no secrets.
//   - Goal: optional stated-intent text (see intent_field_vocabulary,
//     docs/schema-v2.md section 14); absent on every other type; the
//     intent.record type requires it. Observation-only, never decides.
//   - TaskID/TaskStatus: optional task record-form fields (see
//     task_status_vocabulary, docs/task-primitive-design.md section 3);
//     absent = legacy-unclassified on every other type; the task.record
//     type requires both. Observation-only, never decides.
//   - Attrs: optional flat string map for details (paths, tool names...);
//     the reserved keys "cap" and "res_class", when present, must be
//     members of the capability and resource-sensitivity vocabularies.
type Event struct {
	V           int                    `json:"v"`
	TS          time.Time              `json:"ts"`
	ID          string                 `json:"id"`
	AgentID     string                 `json:"agent_id"`
	Stage       Stage                  `json:"stage"`
	Type        EventType              `json:"type"`
	Decision    Decision               `json:"decision"`
	Severity    Severity               `json:"severity"`
	Summary     string                 `json:"summary"`
	Tier        Tier                   `json:"tier,omitempty"`
	SourceClass SourceClass            `json:"source_class,omitempty"`
	DataAction  DataAction             `json:"data_action,omitempty"`
	Memory      *MemoryProvenanceInput `json:"memory_provenance,omitempty"`
	Goal        string                 `json:"goal,omitempty"`
	TaskID      string                 `json:"task_id,omitempty"`
	TaskStatus  TaskStatus             `json:"task_status,omitempty"`
	Attrs       map[string]string      `json:"attrs,omitempty"`
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
	if e.Goal != "" && len(e.Goal) > 512 {
		return fmt.Errorf("schema: event %s: goal >512 bytes", e.ID)
	}
	if e.Type == TypeIntentRecord && e.Goal == "" {
		return fmt.Errorf("schema: event %s: intent.record without a goal statement", e.ID)
	}
	if !e.TaskStatus.Valid() {
		return fmt.Errorf("schema: event %s: unknown task_status %q", e.ID, string(e.TaskStatus))
	}
	if e.TaskID != "" && !validTaskID(e.TaskID) {
		return fmt.Errorf("schema: event %s: task_id %q not in positional session:ordinal shape", e.ID, e.TaskID)
	}
	if e.Type == TypeTaskRecord {
		if e.TaskStatus == "" {
			return fmt.Errorf("schema: event %s: task.record without a task_status", e.ID)
		}
		if !validTaskID(e.TaskID) {
			return fmt.Errorf("schema: event %s: task.record without a positional task_id", e.ID)
		}
	}
	if !e.Tier.Valid() {
		return fmt.Errorf("schema: event %s: unknown tier %q", e.ID, string(e.Tier))
	}
	if !e.SourceClass.Valid() {
		return fmt.Errorf("schema: event %s: unknown source_class %q", e.ID, string(e.SourceClass))
	}
	if !e.DataAction.Valid() {
		return fmt.Errorf("schema: event %s: unknown data_action %q", e.ID, string(e.DataAction))
	}
	if e.Memory != nil {
		// Section 262 typed memory surface riding the observation event
		// (slice W12.2 follow-up): present means every recorder-stated
		// attribute must build a complete provenance record - a rejected
		// record invalidates the whole line and leaves no half
		// observation behind. Absent means no memory record was
		// attached; nothing is ever inferred from silence. No shipped
		// rule reads this field: a recorded claim is evidence of a
		// claim, never a grant (Memory is not Authorization).
		if _, err := BuildMemoryProvenance(*e.Memory); err != nil {
			return fmt.Errorf("schema: event %s: %w", e.ID, err)
		}
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
