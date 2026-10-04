package schema

import (
	"fmt"
	"strings"
)

// Cost guard observation records (owner spec section 261, slice
// W6.2): every bound on an agent or a task is eventually asked in
// one of two shapes - a count (requests, calls, children) or a
// duration (wall time, cpu time, memory held over time). The one
// section 261 bound that is neither is Max API Cost: its true
// value lives on an external billing plane this system does not
// speak to, so it is structurally absent, and the honest record
// shape states that absence instead of inventing a number.
//
// This slice lands the proxy half only, exactly as the taskbook's
// W6.2 consumption note requires:
//
//   - a closed five-word proxy-field vocabulary in count/duration
//     shapes (runtime duration and cpu time are duration form,
//     billable requests and LLM invocations are count form,
//     memory held over time is duration form), each field's shape
//     fixed by one normative coverage table shared with docs and
//     the Node checker;
//   - a three-state value line - stated_observed, known_gap,
//     gate_evidenced_zero - whose rule constant pins the
//     no-fabrication stance: a record with no source carries NO
//     value at all (the wire shape omits the value key entirely),
//     it never carries a fabricated zero. An explicitly stated
//     zero with its source and an honest absence are two
//     different, both-admissible lines (the W5.1 two-shapes
//     stance restated for consumption);
//   - the honest known_gap line for the cost truth itself: the
//     truth record shape structurally has no value entrance at
//     all, so no caller can smuggle a fabricated cost number into
//     it any more than a sourceless zero into a proxy record;
//   - the zero-LLM fact line: this runtime's decision path calls
//     no LLM at all, so llm_invocation_count is zero by
//     construction, and that zero is admissible only through the
//     named gate-evidence constructor - the decision-path gate
//     (stdlib-only surface plus the outbound-closure and planeLeak
//     scans this same PR extends) is what converts the statement
//     "zero" from a fabrication into an evidenced fact.
//
// W6.1's coverage table stays byte-stable behind this slice: its
// two anchored rows (max_runtime and max_api_cost to W6.2) are
// fulfilled here by the runtime and billable-request proxies, and
// its two pending rows (max_cpu, max_memory) stay pending for a
// collector owner - this slice gives those bounds a field shape
// whose only honest record today is the known_gap line, which is
// exactly what "says out loud that there is no collector yet"
// means in record form. Enforcement, ceilings, and the section
// 261 ladder stay Phase 1: the response line is again the
// constructor-pinned record-only-no-action, and no new symbol
// below may appear in any decision-plane file while
// cost_enforcement_plane reads none-in-observation-phase.
//
// docs/schema-v2.md section 22 is the human-facing contract; the
// Node checker mirrors its eight keys, the four closed
// vocabularies, the derived field-shape mapping, the five anchor
// bullets of the proxy sources, and the 5/3 coverage table
// against this file; drift in either direction is a red build.

// CostProxyField names one of the five consumption-proxy fields.
// Declaration order is normative: docs section 22, the coverage
// table, and the Node checker are pinned row by row against it.
type CostProxyField string

// The five proxy fields in normative order.
const (
	CostProxyRuntimeDuration CostProxyField = "runtime_duration_millis"
	CostProxyBillableRequest CostProxyField = "billable_request_count"
	CostProxyCPUTime         CostProxyField = "cpu_time_millis"
	CostProxyMemoryHeld      CostProxyField = "memory_mib_millis"
	CostProxyLLMInvocations  CostProxyField = "llm_invocation_count"
)

// costProxyFieldWireNames lists the five proxy-field tokens in
// normative order. AllCostProxyFields mirrors it; nobody recounts
// them by hand, and the docs lines derive to the same list or the
// build goes red in both directions.
var costProxyFieldWireNames = []string{
	"runtime_duration_millis", "billable_request_count",
	"cpu_time_millis", "memory_mib_millis", "llm_invocation_count",
}

// AllCostProxyFields returns the five proxy-field tokens in
// normative order.
func AllCostProxyFields() []string {
	out := make([]string, len(costProxyFieldWireNames))
	copy(out, costProxyFieldWireNames)
	return out
}

// Valid reports whether f is a member of the closed five-word
// vocabulary. The empty string is not a field.
func (f CostProxyField) Valid() bool {
	if f == "" {
		return false
	}
	for _, w := range costProxyFieldWireNames {
		if string(f) == w {
			return true
		}
	}
	return false
}

// CostProxyShape names the two consumption shapes section 261
// bounds are stated in: a count of things, or a duration of
// occupancy. Every field's shape is fixed by the single coverage
// table below; a recorder that states a shape the table does not
// assign is caught before any value is applied.
type CostProxyShape string

// The two shapes in normative order.
const (
	CostShapeCount    CostProxyShape = "count"
	CostShapeDuration CostProxyShape = "duration"
)

// AllCostProxyShapes returns the two shape tokens in normative
// order.
func AllCostProxyShapes() []string {
	return []string{string(CostShapeCount), string(CostShapeDuration)}
}

// Valid reports whether s is one of the two shapes.
func (s CostProxyShape) Valid() bool {
	if s == "" {
		return false
	}
	for _, w := range AllCostProxyShapes() {
		if string(s) == w {
			return true
		}
	}
	return false
}

// CostValueState is the closed tri-state of one consumption line:
// a value stated with its source, an honest known_gap (no source
// exists - the value key is then absent from the wire shape,
// never a fabricated zero), or the gate-evidenced zero the
// named LLM constructor alone may produce once the decision-path
// gate proves the call site does not exist.
type CostValueState string

// The three value states in normative order.
const (
	CostStateStated   CostValueState = "stated_observed"
	CostStateKnownGap CostValueState = "known_gap"
	CostStateGateZero CostValueState = "gate_evidenced_zero"
)

// AllCostValueStates returns the three state tokens in normative
// order.
func AllCostValueStates() []string {
	return []string{string(CostStateStated), string(CostStateKnownGap), string(CostStateGateZero)}
}

// Valid reports whether s is one of the three states.
func (s CostValueState) Valid() bool {
	if s == "" {
		return false
	}
	for _, w := range AllCostValueStates() {
		if string(s) == w {
			return true
		}
	}
	return false
}

// CostCollectorStanding names how one proxy field stands for
// collection in the 5/3 registration: stated by a recorder today,
// awaiting a collector owner (the W6.1 pending rows' stance,
// honest as a known_gap line), or proven by the decision-path
// gate itself.
type CostCollectorStanding string

// The three collector standings in normative order.
const (
	CostCollectorStated CostCollectorStanding = "stated_by_recorder"
	CostCollectorGap    CostCollectorStanding = "no_collector_known_gap"
	CostCollectorGatePv CostCollectorStanding = "gate_evidenced_zero_line"
)

// AllCostCollectorStandings returns the three standing tokens in
// normative order.
func AllCostCollectorStandings() []string {
	return []string{string(CostCollectorStated), string(CostCollectorGap), string(CostCollectorGatePv)}
}

// Valid reports whether c is one of the three standings.
func (c CostCollectorStanding) Valid() bool {
	if c == "" {
		return false
	}
	for _, w := range AllCostCollectorStandings() {
		if string(c) == w {
			return true
		}
	}
	return false
}

// CostProxyCoverageRow registers one proxy field: its fixed shape
// and its collector standing. The table is the single source both
// the field-shape mapping and the docs coverage rows derive from.
type CostProxyCoverageRow struct {
	Field     CostProxyField
	Shape     CostProxyShape
	Collector CostCollectorStanding
}

// costProxyCoverage is the normative 5/3 registration in field
// order. docs/schema-v2.md section 22 mirrors these five rows
// verbatim, row order included, and the Node checker pins them
// pair by pair. The runtime and billable-request rows fulfil
// W6.1's anchored pair; the cpu and memory rows carry W6.1's
// pending stance forward as an explicit no-collector known_gap
// (field shape today, number only once a collector owner exists);
// the LLM row is the zero the gate proves, not a zero recorded
// from a stream.
func costProxyCoverage() []CostProxyCoverageRow {
	return []CostProxyCoverageRow{
		{CostProxyRuntimeDuration, CostShapeDuration, CostCollectorStated},
		{CostProxyBillableRequest, CostShapeCount, CostCollectorStated},
		{CostProxyCPUTime, CostShapeDuration, CostCollectorGap},
		{CostProxyMemoryHeld, CostShapeDuration, CostCollectorGap},
		{CostProxyLLMInvocations, CostShapeCount, CostCollectorGatePv},
	}
}

// AllCostProxyCoverage returns a copy of the five registration
// rows in normative order.
func AllCostProxyCoverage() []CostProxyCoverageRow {
	rows := costProxyCoverage()
	out := make([]CostProxyCoverageRow, len(rows))
	copy(out, rows)
	return out
}

// costShapeOf returns the one shape the coverage table assigns to
// a proxy field, or the empty shape for a field outside the
// closed five.
func costShapeOf(f CostProxyField) CostProxyShape {
	for _, row := range costProxyCoverage() {
		if row.Field == f {
			return row.Shape
		}
	}
	return CostProxyShape("")
}

// Rule constants pinned verbatim against the eight-key machine
// block in docs/schema-v2.md section 22.
const (
	// CostAbsenceRule pins the slice's core assertion line: a
	// consumption proxy without a stated source is recorded as a
	// known_gap whose value key is absent from the wire shape -
	// absence, never a fabricated zero (no-inventory doctrine).
	CostAbsenceRule = "no-source-is-known-gap-never-fabricated-zero"
	// CostTruthStance pins where the true cost number lives: an
	// external billing plane this system does not speak to. The
	// stance is why the truth record has no value entrance at
	// all - a structurally absent source is recorded as absent,
	// not as any number its absence could be confused with.
	CostTruthStance = "cost-truth-lives-on-external-billing-plane-structurally-absent"
	// CostGuardEnforcementPlane pins the Phase 0 stance: proxies
	// and truth lines are observations; no plane borrows them to
	// halt, throttle, bill, or decide anything yet.
	CostGuardEnforcementPlane = "none-in-observation-phase"
	// CostGuardResponseRule pins the record-only stance every
	// cost record carries, constructor-held and not caller
	// editable (the W6.1 response line restated so neither slice
	// can be told apart by a smuggled action value).
	CostGuardResponseRule = "record-only-no-action"
	// CostZeroLLMStatement is the evidence source named by the
	// gate-evidenced zero: the decision path of this runtime
	// calls no LLM at all, and the decision-path gate (stdlib
	// import surface, outbound-closure and planeLeak scans) is
	// what makes "zero" a fact statement rather than a
	// fabrication.
	CostZeroLLMStatement = "zero-llm-calls-in-decision-path-proven-by-gate"
)

// CostProxyRecordInput is the recorder's side of one consumption
// proxy line: which proxy field, the shape the table assigns it,
// the W6.1 closed scope pair and its stated id, and the two
// coherent value lines - a source with a stated value, or neither
// (the honest known_gap). The shape is derived by the table, not
// chosen by the recorder: a stated shape the assignment rejects
// is a recorder error caught before construction. The struct
// carries no state, verdict, decision, score, or severity field
// and the reflective test pins that.
type CostProxyRecordInput struct {
	Field   CostProxyField     `json:"field"`
	Shape   CostProxyShape     `json:"shape"`
	Scope   AgencyCounterScope `json:"scope"`
	ScopeID string             `json:"scope_id"`
	Source  string             `json:"source"`
	Stated  *uint64            `json:"stated_value"`
}

// CostProxyRecord is the row BuildCostProxyRecord and
// BuildCostLLMZeroRecord produce. A known_gap record omits both
// the source and the value keys entirely on the wire - absence is
// literally the absence, not a zero in disguise - while a stated
// record restates both, and the gate-evidenced zero restates the
// pinned evidence line with a value of exactly zero.
type CostProxyRecord struct {
	Field       CostProxyField     `json:"field"`
	Shape       CostProxyShape     `json:"shape"`
	Scope       AgencyCounterScope `json:"scope"`
	ScopeID     string             `json:"scope_id"`
	Source      string             `json:"source,omitempty"`
	Value       *uint64            `json:"value,omitempty"`
	State       CostValueState     `json:"value_state"`
	Absence     string             `json:"absence_rule"`
	Response    string             `json:"response"`
	Enforcement string             `json:"enforcement_plane"`
}

// requireCostText mirrors requireAgencyText for this slice's free
// text lines: empty or whitespace-only identity lines abort the
// whole record, and a line equal to a Phase 1 ladder action token
// is rejected by the same enum gate (the section 261 ladder stays
// registry-only; LIMIT, HOLD, CANCEL, RECOVER keep zero writes
// into any product path).
func requireCostText(field, value string) error {
	if strings.TrimSpace(value) == "" {
		return fmt.Errorf("cost guard: field %q must state text (section 261 per-agent/per-task identity)", field)
	}
	for _, tok := range agencyForbiddenActionTokens {
		if value == tok {
			return fmt.Errorf("cost guard: field %q states the Phase 1 action token %q; the enum gate admits no action value into a record (section 261 ladder is registry-only in Phase 0)", field, tok)
		}
	}
	return nil
}

// BuildCostProxyRecord validates one consumption proxy line and
// projects the record. Every rejection fires before any value is
// applied: a field outside the closed five, a shape the coverage
// table does not assign, a scope outside the W6.1 closed pair, an
// unstated scope id, a source stating a ladder action token, or
// the two half shapes (a value with no source, a source with no
// value) leave no record behind. Absence is single-meaning and
// never numeric: the known_gap line requires both the source and
// the stated value to be absent, and its wire shape carries no
// value key at all - the absence is structurally distinguishable
// from a stated zero, which stays admissible only with its source.
func BuildCostProxyRecord(in CostProxyRecordInput) (CostProxyRecord, error) {
	if !in.Field.Valid() {
		return CostProxyRecord{}, fmt.Errorf("cost guard: proxy field %q is outside the closed five", string(in.Field))
	}
	if !in.Shape.Valid() {
		return CostProxyRecord{}, fmt.Errorf("cost guard: shape %q is outside the closed pair (count or duration)", string(in.Shape))
	}
	if want := costShapeOf(in.Field); in.Shape != want {
		return CostProxyRecord{}, fmt.Errorf("cost guard: field %q is %s form by the coverage table, stated shape %q is a recorder error", string(in.Field), string(want), string(in.Shape))
	}
	if !in.Scope.Valid() {
		return CostProxyRecord{}, fmt.Errorf("cost guard: scope %q is outside the closed pair (per-agent or per-task, stated)", string(in.Scope))
	}
	if err := requireCostText("scope_id", in.ScopeID); err != nil {
		return CostProxyRecord{}, err
	}
	hasSource := strings.TrimSpace(in.Source) != ""
	if hasSource && in.Stated == nil {
		return CostProxyRecord{}, fmt.Errorf("cost guard: source stated without a value is a half record; state both or neither (absence is one line, not a trailing blank)")
	}
	if !hasSource && in.Stated != nil {
		return CostProxyRecord{}, fmt.Errorf("cost guard: value %d stated with no source would fabricate a reading (the absence rule pins this to the known_gap line, never a number)", *in.Stated)
	}
	if hasSource {
		if err := requireCostText("source", in.Source); err != nil {
			return CostProxyRecord{}, err
		}
		v := *in.Stated
		return CostProxyRecord{
			Field:       in.Field,
			Shape:       in.Shape,
			Scope:       in.Scope,
			ScopeID:     in.ScopeID,
			Source:      in.Source,
			Value:       &v,
			State:       CostStateStated,
			Absence:     CostAbsenceRule,
			Response:    CostGuardResponseRule,
			Enforcement: CostGuardEnforcementPlane,
		}, nil
	}
	return CostProxyRecord{
		Field:       in.Field,
		Shape:       in.Shape,
		Scope:       in.Scope,
		ScopeID:     in.ScopeID,
		State:       CostStateKnownGap,
		Absence:     CostAbsenceRule,
		Response:    CostGuardResponseRule,
		Enforcement: CostGuardEnforcementPlane,
	}, nil
}

// BuildCostLLMZeroRecord projects the one zero this slice may
// state without a stream: llm_invocation_count is zero because
// the decision path has no LLM call site at all, and the named
// evidence line is the decision-path gate itself. The field and
// state are constructor-pinned and not caller editable - no
// other proxy field can borrow the gate-evidenced shape, and the
// value line is the literal zero the gate proves, restated with
// its evidence source. Like every record here it carries no
// ceiling, no verdict, and no action.
func BuildCostLLMZeroRecord(scope AgencyCounterScope, scopeID string) (CostProxyRecord, error) {
	if !scope.Valid() {
		return CostProxyRecord{}, fmt.Errorf("cost guard: scope %q is outside the closed pair (per-agent or per-task, stated)", string(scope))
	}
	if err := requireCostText("scope_id", scopeID); err != nil {
		return CostProxyRecord{}, err
	}
	zero := uint64(0)
	return CostProxyRecord{
		Field:       CostProxyLLMInvocations,
		Shape:       costShapeOf(CostProxyLLMInvocations),
		Scope:       scope,
		ScopeID:     scopeID,
		Source:      CostZeroLLMStatement,
		Value:       &zero,
		State:       CostStateGateZero,
		Absence:     CostAbsenceRule,
		Response:    CostGuardResponseRule,
		Enforcement: CostGuardEnforcementPlane,
	}, nil
}

// CostTruthRecord is the honest known_gap line for the section
// 261 Max API Cost truth itself. The struct has no value field -
// not "value omitted when unset" but no entrance at all - because
// the plane that holds the number is external and structurally
// absent here; the shape is how "no source is absence, not zero"
// becomes mechanical. The state, stance, response, and plane
// lines are constructor-pinned and not caller editable.
type CostTruthRecord struct {
	Scope       AgencyCounterScope `json:"scope"`
	ScopeID     string             `json:"scope_id"`
	State       CostValueState     `json:"value_state"`
	Stance      string             `json:"cost_truth_stance"`
	Absence     string             `json:"absence_rule"`
	Response    string             `json:"response"`
	Enforcement string             `json:"enforcement_plane"`
}

// BuildCostTruthRecord projects the one admissible truth line:
// scope and identity are validated like everywhere else, and what
// comes back states known_gap with the pinned stance. There is
// no input shape that can put a number into it.
func BuildCostTruthRecord(scope AgencyCounterScope, scopeID string) (CostTruthRecord, error) {
	if !scope.Valid() {
		return CostTruthRecord{}, fmt.Errorf("cost guard: scope %q is outside the closed pair (per-agent or per-task, stated)", string(scope))
	}
	if err := requireCostText("scope_id", scopeID); err != nil {
		return CostTruthRecord{}, err
	}
	return CostTruthRecord{
		Scope:       scope,
		ScopeID:     scopeID,
		State:       CostStateKnownGap,
		Stance:      CostTruthStance,
		Absence:     CostAbsenceRule,
		Response:    CostGuardResponseRule,
		Enforcement: CostGuardEnforcementPlane,
	}, nil
}
