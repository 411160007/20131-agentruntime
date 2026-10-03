package schema

import (
	"fmt"
	"strings"
)

// Agency guard observation records (spec vNext sections 234 and
// 261, slice W6.1): Least Agency is the spec's demand that an agent
// be bounded not only in what it may touch but in how much it may
// do - how long it runs, how many tools it calls, how many things
// it runs in parallel, how many children it spawns, and what it
// spends. Section 261 names ten hard-bound fields per agent and
// per task and shows the escalation ladder LIMIT, HOLD,
// CANCEL/RECOVER that Phase 1 will walk when a bound trips.
//
// This slice lands the observation half only, exactly as the
// taskbook's W6.1 consumption note requires: three pure counters
// (event rate, step count, parallelism) over per-agent or per-task
// streams, and a limit-record shape for what a ceiling comparison
// shows. What the records carry:
//
//   - the closed ten-word section 261 field vocabulary with a
//     10/10 coverage registration: five fields are counted by this
//     slice (steps and parallelism directly; tool calls, network
//     requests, and child-agent creations through the event-rate
//     counter, which counts every recorded event of whatever class
//     the recorder states), three are anchored to named later
//     slices (W6.2 cost and runtime proxies, W8.2 storage quota,
//     the same collection source this counters' dimensions share),
//     and two stand honestly pending (CPU and memory have no
//     collector owner yet - a stated gap, never a silent hole);
//   - a count-conservation rule the aggregate constructors enforce
//     before any value is applied: the event sequence must be the
//     gap-free run 1..N, the event count equals the number of
//     accepted events, and nothing is lost, double-counted, or
//     invented;
//   - a limit record whose hit state is computed (below, at, or
//     above the stated ceiling) and whose only response value is
//     the constructor-pinned record-only-no-action. The ladder
//     tokens LIMIT, HOLD, CANCEL, and RECOVER are Phase 1 action
//     values; no field of any Phase 0 record may ever carry one,
//     and the action-enum gate rejects them before they could be
//     smuggled in as a stated value.
//
// What the records never carry: a kill, a halt, an error on
// hitting a ceiling, a decision, a score, or a severity. Hitting a
// limit in this slice changes which record is appended, and
// nothing else: the aggregate built over a stream that trips a
// ceiling is byte-identical to the aggregate built over the same
// stream with no ceiling declared, because the ceiling never
// enters the counter. The high-water rule for parallelism is a
// pure maximum over stated counts, and event rate is a count over
// a recorder-stated window - the window text is restated, never
// divided into (no time arithmetic, same stance as the W5.3 TTL).
//
// docs/schema-v2.md section 21 is the human-facing contract; the
// Node checker mirrors its seven keys, the three closed
// vocabularies, the four limit states, the ten anchor bullets, and
// the 10/10 coverage table against this file; no decision-plane
// (policy, rules, bus, auditlog) or command-tree file may
// reference the symbols below while agency_enforcement_plane reads
// none-in-observation-phase (the planeLeak needle set scans for
// exactly that).

// AgencyGuardField names one of the ten section 261 bound fields.
// Declaration order is the spec's listing order and is normative:
// the section 21 anchor bullets, the coverage table, and the Node
// checker are all pinned row by row against it.
type AgencyGuardField string

// The ten section 261 fields in spec order.
const (
	GuardMaxSteps       AgencyGuardField = "max_steps"
	GuardMaxRuntime     AgencyGuardField = "max_runtime"
	GuardMaxToolCalls   AgencyGuardField = "max_tool_calls"
	GuardMaxNetworkReqs AgencyGuardField = "max_network_requests"
	GuardMaxParallelism AgencyGuardField = "max_parallelism"
	GuardMaxCPU         AgencyGuardField = "max_cpu"
	GuardMaxMemory      AgencyGuardField = "max_memory"
	GuardMaxStorage     AgencyGuardField = "max_storage"
	GuardMaxAPICost     AgencyGuardField = "max_api_cost"
	GuardMaxChildAgents AgencyGuardField = "max_child_agents"
)

// agencyGuardFieldWireNames lists the ten bound-field tokens in
// normative (spec) order. AllAgencyGuardFields mirrors it; nobody
// recounts them by hand, and the docs bullets derive to the same
// list or the build goes red in both directions.
var agencyGuardFieldWireNames = []string{
	"max_steps", "max_runtime", "max_tool_calls", "max_network_requests",
	"max_parallelism", "max_cpu", "max_memory", "max_storage",
	"max_api_cost", "max_child_agents",
}

// AllAgencyGuardFields returns the ten bound-field tokens in spec
// order.
func AllAgencyGuardFields() []string {
	out := make([]string, len(agencyGuardFieldWireNames))
	copy(out, agencyGuardFieldWireNames)
	return out
}

// Valid reports whether f is a member of the closed ten-field
// vocabulary. The empty string is not a field.
func (f AgencyGuardField) Valid() bool {
	if f == "" {
		return false
	}
	for _, w := range agencyGuardFieldWireNames {
		if string(f) == w {
			return true
		}
	}
	return false
}

// AgencyGuardPhase names how one bound field stands in the 10/10
// coverage registration: counted by this slice's three counters,
// deferred to a named later slice, or honestly pending with no
// owner yet. Every row carries exactly one phase; a row with no
// phase is a dangling hole, and holes are red builds here.
type AgencyGuardPhase string

// The three coverage phases.
const (
	GuardPhaseCounted  AgencyGuardPhase = "counted_v0"
	GuardPhaseAnchored AgencyGuardPhase = "anchored"
	GuardPhasePending  AgencyGuardPhase = "pending"
)

// Valid reports whether p is one of the three coverage phases.
func (p AgencyGuardPhase) Valid() bool {
	switch p {
	case GuardPhaseCounted, GuardPhaseAnchored, GuardPhasePending:
		return true
	}
	return false
}

// AgencyGuardCoverageRow registers one bound field's standing: its
// phase and, for anchored rows, the later slice that will cover it
// (W6.2 cost and runtime proxies, W8.2 storage quota and the
// shared collection source; enforcement stays out of scope for
// every row while the plane reads none).
type AgencyGuardCoverageRow struct {
	Field  AgencyGuardField
	Phase  AgencyGuardPhase
	Anchor string
}

// agencyGuardCoverage is the normative 10/10 registration in spec
// order. The five counted rows are this slice's counters; the
// anchored rows name their real owners; the two pending rows say
// out loud that CPU and memory have no collector owner yet.
// docs/schema-v2.md section 21 mirrors these ten rows verbatim,
// row order included, and the Node checker pins them pair by pair.
func agencyGuardCoverage() []AgencyGuardCoverageRow {
	return []AgencyGuardCoverageRow{
		{GuardMaxSteps, GuardPhaseCounted, ""},
		{GuardMaxRuntime, GuardPhaseAnchored, "W6.2"},
		{GuardMaxToolCalls, GuardPhaseCounted, ""},
		{GuardMaxNetworkReqs, GuardPhaseCounted, ""},
		{GuardMaxParallelism, GuardPhaseCounted, ""},
		{GuardMaxCPU, GuardPhasePending, ""},
		{GuardMaxMemory, GuardPhasePending, ""},
		{GuardMaxStorage, GuardPhaseAnchored, "W8.2"},
		{GuardMaxAPICost, GuardPhaseAnchored, "W6.2"},
		{GuardMaxChildAgents, GuardPhaseCounted, ""},
	}
}

// AllAgencyGuardCoverage returns a copy of the ten registration
// rows in normative order.
func AllAgencyGuardCoverage() []AgencyGuardCoverageRow {
	rows := agencyGuardCoverage()
	out := make([]AgencyGuardCoverageRow, len(rows))
	copy(out, rows)
	return out
}

// AgencyCounterKind names one of the three observation counters of
// slice W6.1. The order is the taskbook's (event rate, step count,
// parallelism) and is normative.
type AgencyCounterKind string

// The three counters in normative order.
const (
	CounterEventRate   AgencyCounterKind = "event_rate"
	CounterStepCount   AgencyCounterKind = "step_count"
	CounterParallelism AgencyCounterKind = "parallelism"
)

// agencyCounterWireNames lists the three counter tokens in
// normative order.
var agencyCounterWireNames = []string{
	"event_rate", "step_count", "parallelism",
}

// AllAgencyCounterKinds returns the three counter tokens in
// normative order.
func AllAgencyCounterKinds() []string {
	out := make([]string, len(agencyCounterWireNames))
	copy(out, agencyCounterWireNames)
	return out
}

// Valid reports whether k is one of the three counters. The empty
// string is not a counter.
func (k AgencyCounterKind) Valid() bool {
	if k == "" {
		return false
	}
	for _, w := range agencyCounterWireNames {
		if string(k) == w {
			return true
		}
	}
	return false
}

// AgencyCounterScope is the closed pair the taskbook names for
// every counter: per-agent or per-task. Section 261 bounds each
// agent and each task, and section 234 refuses to let either
// dimension be averaged away into the other.
type AgencyCounterScope string

// The two scopes in normative order.
const (
	ScopeAgent AgencyCounterScope = "agent"
	ScopeTask  AgencyCounterScope = "task"
)

// AllAgencyCounterScopes lists the two scope tokens in normative
// order.
func AllAgencyCounterScopes() []string {
	return []string{string(ScopeAgent), string(ScopeTask)}
}

// Valid reports whether s is one of the two scopes. The empty
// string is not a scope: a counter whose dimension was never
// stated fails construction, it does not quietly become global.
func (s AgencyCounterScope) Valid() bool {
	if s == "" {
		return false
	}
	for _, w := range AllAgencyCounterScopes() {
		if string(s) == w {
			return true
		}
	}
	return false
}

// AgencyLimitState is the closed tri-state of one ceiling
// comparison. Every state is computed by the constructor from the
// stated ceiling and observed value - the recorder states numbers,
// never a verdict - and hitting a ceiling yields a record, never
// an action.
type AgencyLimitState string

// The three limit states in normative order.
const (
	LimitBelow AgencyLimitState = "below_ceiling"
	LimitAt    AgencyLimitState = "at_ceiling"
	LimitAbove AgencyLimitState = "above_ceiling"
)

// AllAgencyLimitStates lists the three states in normative order.
func AllAgencyLimitStates() []string {
	return []string{string(LimitBelow), string(LimitAt), string(LimitAbove)}
}

// Valid reports whether s is one of the three states.
func (s AgencyLimitState) Valid() bool {
	if s == "" {
		return false
	}
	for _, w := range AllAgencyLimitStates() {
		if string(s) == w {
			return true
		}
	}
	return false
}

// Rule constants pinned verbatim against the seven-key machine
// block in docs/schema-v2.md section 21.
const (
	// AgencyCountConservationRule pins the count obligation: an
	// aggregate's event sequence is the gap-free run 1..N and its
	// event count equals the number of accepted events. Nothing is
	// lost, nothing is double-counted, nothing is invented.
	AgencyCountConservationRule = "sequences-one-to-N-events-never-lost-or-duplicated"
	// AgencyActionEnumGate pins the Phase 0 closed set over the
	// section 261 ladder: LIMIT, HOLD, CANCEL, and RECOVER are
	// Phase 1 action values and never appear as a recorded
	// response, verdict, or state. The limit record's only
	// response value is AgencyGuardResponseRule below.
	AgencyActionEnumGate = "limit-hold-cancel-recover-never-recorded-as-response"
	// AgencyGuardResponseRule pins the record-only stance every
	// limit record carries, constructor-held and not caller
	// editable: the observation records, the kill and the halt
	// wait for Phase 1.
	AgencyGuardResponseRule = "record-only-no-action"
	// AgencyLadderRegistry lists the section 261 escalation ladder
	// in spec order as a registry line, not as live checks:
	// walking it is enforcement and sits in Phase 1 (pending
	// decision); pre-borrowing a rung here would itself violate
	// the enum gate the record exists to restate.
	AgencyLadderRegistry = "limit,hold,cancel-or-recover"
	// AgencyEnforcementPlane pins the Phase 0 stance: counters and
	// limit records are observations; no plane borrows them to
	// kill, halt, throttle, or decide anything yet.
	AgencyEnforcementPlane = "none-in-observation-phase"
)

// agencyForbiddenActionTokens lists the ladder tokens that may
// never be recorded as a value: the spec's four uppercase action
// words, their lower-case wire forms, and the joined cancel shape.
// The list is the closed set the reflective test pins every record
// field against; the registry line above is deliberately a whole
// list, never one of these bare tokens.
var agencyForbiddenActionTokens = []string{
	"LIMIT", "HOLD", "CANCEL", "RECOVER",
	"limit", "hold", "cancel", "recover", "cancel/recover",
}

// AgencyForbiddenActionTokens returns a copy of the closed
// never-recordable action set pinned by the enum gate.
func AgencyForbiddenActionTokens() []string {
	out := make([]string, len(agencyForbiddenActionTokens))
	copy(out, agencyForbiddenActionTokens)
	return out
}

// AgencyCounterEvent is one event of one counted stream: its
// 1-based position and the value the recorder states for it. For
// step_count and event_rate every event contributes exactly one
// count, so the stated value must be 1 (stating anything else is
// a recorder error caught before construction, not an arithmetic
// fudge applied later). For parallelism the stated value is the
// concurrency observed at that moment and may be any count
// including zero - silence about activity is stated as zero,
// never inferred.
type AgencyCounterEvent struct {
	Sequence    uint64 `json:"sequence"`
	StatedValue uint64 `json:"stated_value"`
}

// AgencyCounterAggregateInput is the recorder's side of one
// counted stream: the scope pair, the counter, the stated event
// class (free text on purpose - matching a class against any live
// vocabulary would pre-borrow enforcement), and the events. The
// struct carries no decision, score, severity, or ceiling field
// and the reflective test pins that.
type AgencyCounterAggregateInput struct {
	Scope      AgencyCounterScope   `json:"scope"`
	ScopeID    string               `json:"scope_id"`
	Counter    AgencyCounterKind    `json:"counter"`
	EventClass string               `json:"event_class"`
	Events     []AgencyCounterEvent `json:"events"`
}

// AgencyCounterAggregate is the row BuildAgencyCounterAggregate
// produces: the stated identity lines restated verbatim, the
// conserved count, the counter-specific observed value, and the
// two constructor-pinned restatements. It is a record of one
// counted stream, not a judgement about it, and it never carries
// a ceiling: a ceiling that was never declared cannot silently
// shape the count.
type AgencyCounterAggregate struct {
	Scope            AgencyCounterScope `json:"scope"`
	ScopeID          string             `json:"scope_id"`
	Counter          AgencyCounterKind  `json:"counter"`
	EventClass       string             `json:"event_class"`
	EventCount       uint64             `json:"event_count"`
	ObservedValue    uint64             `json:"observed_value"`
	Conservation     string             `json:"count_conservation"`
	EnforcementPlane string             `json:"enforcement_plane"`
}

// requireAgencyText rejects the empty and whitespace-only shape of
// a must-state identity line, mirroring the W5.3 stance: a field
// that cannot be stated aborts the whole record.
func requireAgencyText(field, value string) error {
	if strings.TrimSpace(value) == "" {
		return fmt.Errorf("agency guard: field %q must state text (section 261 per-agent/per-task identity)", field)
	}
	for _, tok := range agencyForbiddenActionTokens {
		if value == tok {
			return fmt.Errorf("agency guard: field %q states the Phase 1 action token %q; the enum gate admits no action value into a record (section 261 ladder is registry-only in Phase 0)", field, tok)
		}
	}
	return nil
}

// BuildAgencyCounterAggregate validates one counted stream and
// projects the aggregate. Every rejection fires before any value
// is applied: a wild scope or counter, an unstated scope id or
// event class, an empty stream, a gap, a duplicate, an
// out-of-order, or a zero sequence, or a step_count/event_rate
// event stating anything but exactly one count leaves no record
// behind. The constructors never judge and never halt: there is
// no input value that produces a kill, a stop, or an error on
// hitting anything, because no ceiling exists in this shape.
func BuildAgencyCounterAggregate(in AgencyCounterAggregateInput) (AgencyCounterAggregate, error) {
	if !in.Scope.Valid() {
		return AgencyCounterAggregate{}, fmt.Errorf("agency guard: scope %q is outside the closed pair (per-agent or per-task, stated)", string(in.Scope))
	}
	if !in.Counter.Valid() {
		return AgencyCounterAggregate{}, fmt.Errorf("agency guard: counter %q is outside the closed three", string(in.Counter))
	}
	if err := requireAgencyText("scope_id", in.ScopeID); err != nil {
		return AgencyCounterAggregate{}, err
	}
	if err := requireAgencyText("event_class", in.EventClass); err != nil {
		return AgencyCounterAggregate{}, err
	}
	if len(in.Events) == 0 {
		return AgencyCounterAggregate{}, fmt.Errorf("agency guard: an empty stream records nothing (absence is stated by not building, never counted in)")
	}
	observed := uint64(0)
	switch in.Counter {
	case CounterParallelism:
		for i, ev := range in.Events {
			if ev.Sequence != uint64(i+1) {
				return AgencyCounterAggregate{}, fmt.Errorf("agency guard: sequence %d at event %d breaks the gap-free run 1..N (conservation rule)", ev.Sequence, i+1)
			}
			if ev.StatedValue > observed {
				observed = ev.StatedValue
			}
		}
	default:
		for i, ev := range in.Events {
			if ev.Sequence != uint64(i+1) {
				return AgencyCounterAggregate{}, fmt.Errorf("agency guard: sequence %d at event %d breaks the gap-free run 1..N (conservation rule)", ev.Sequence, i+1)
			}
			if ev.StatedValue != 1 {
				return AgencyCounterAggregate{}, fmt.Errorf("agency guard: counter %q counts each event as exactly one; stated value %d at event %d is a recorder error", string(in.Counter), ev.StatedValue, i+1)
			}
		}
		observed = uint64(len(in.Events))
	}
	return AgencyCounterAggregate{
		Scope:            in.Scope,
		ScopeID:          in.ScopeID,
		Counter:          in.Counter,
		EventClass:       in.EventClass,
		EventCount:       uint64(len(in.Events)),
		ObservedValue:    observed,
		Conservation:     AgencyCountConservationRule,
		EnforcementPlane: AgencyEnforcementPlane,
	}, nil
}

// AgencyLimitRecordInput is the recorder's side of one ceiling
// comparison: which section 261 field is bounded, on which scope,
// which counter produced the observed value, and the two numbers.
// The hit state is computed, never stated: a recorder that tried
// to state a verdict would be trying to smuggle a judgement past
// the closed vocabularies, so the shape offers no verdict field at
// all. No caller-editable response exists either.
type AgencyLimitRecordInput struct {
	GuardField AgencyGuardField   `json:"guard_field"`
	Counter    AgencyCounterKind  `json:"counter"`
	Scope      AgencyCounterScope `json:"scope"`
	ScopeID    string             `json:"scope_id"`
	Ceiling    uint64             `json:"ceiling"`
	Observed   uint64             `json:"observed"`
}

// AgencyLimitRecord is the row BuildAgencyLimitRecord produces:
// the six stated lines, the computed state, the computed trigger
// flag, and the constructor-pinned response, ladder registry, and
// plane restatements. Hitting a ceiling returns this record with
// error nil - the trigger is data, not a control signal, and the
// only admissible response value is record-only-no-action.
type AgencyLimitRecord struct {
	GuardField       AgencyGuardField   `json:"guard_field"`
	Counter          AgencyCounterKind  `json:"counter"`
	Scope            AgencyCounterScope `json:"scope"`
	ScopeID          string             `json:"scope_id"`
	Ceiling          uint64             `json:"ceiling"`
	Observed         uint64             `json:"observed"`
	State            AgencyLimitState   `json:"limit_state"`
	Triggered        bool               `json:"limit_triggered"`
	Response         string             `json:"response"`
	LadderRegistry   string             `json:"escalation_ladder_registry"`
	EnforcementPlane string             `json:"enforcement_plane"`
}

// BuildAgencyLimitRecord validates one ceiling comparison and
// projects the record. Every rejection fires before any value is
// applied: a guard field outside the closed ten, a counter outside
// the closed three, a scope outside the closed pair, an unstated
// scope id, or any stated value equal to a Phase 1 ladder action
// token leaves no record behind. The comparison itself never
// fails: at and above are states, not errors, and the record for
// a tripped ceiling is built exactly like the record for one
// that was never approached, field for field, error channel nil
// both times.
func BuildAgencyLimitRecord(in AgencyLimitRecordInput) (AgencyLimitRecord, error) {
	if !in.GuardField.Valid() {
		return AgencyLimitRecord{}, fmt.Errorf("agency guard: bound field %q is outside the closed ten", string(in.GuardField))
	}
	if !in.Counter.Valid() {
		return AgencyLimitRecord{}, fmt.Errorf("agency guard: counter %q is outside the closed three", string(in.Counter))
	}
	if !in.Scope.Valid() {
		return AgencyLimitRecord{}, fmt.Errorf("agency guard: scope %q is outside the closed pair (per-agent or per-task, stated)", string(in.Scope))
	}
	if err := requireAgencyText("scope_id", in.ScopeID); err != nil {
		return AgencyLimitRecord{}, err
	}
	state := LimitBelow
	switch {
	case in.Observed > in.Ceiling:
		state = LimitAbove
	case in.Observed == in.Ceiling:
		state = LimitAt
	}
	return AgencyLimitRecord{
		GuardField:       in.GuardField,
		Counter:          in.Counter,
		Scope:            in.Scope,
		ScopeID:          in.ScopeID,
		Ceiling:          in.Ceiling,
		Observed:         in.Observed,
		State:            state,
		Triggered:        state != LimitBelow,
		Response:         AgencyGuardResponseRule,
		LadderRegistry:   AgencyLadderRegistry,
		EnforcementPlane: AgencyEnforcementPlane,
	}, nil
}
