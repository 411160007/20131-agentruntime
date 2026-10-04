package schema

import (
	"encoding/json"
	"fmt"
	"strings"
)

// Decision trace field table (spec vNext section 251, slice W7.1):
// every important security decision must keep a machine-readable
// Decision Trace. Section 251 lists fifteen field names. The audit
// line that rules.DecisionEvent already writes carries most of them;
// a few arrive here as additive fields, and the rest stay honestly
// absent - named in the record's gap list, never fabricated into a
// value.
//
// This slice is a REVERSE-LOOKUP, not a re-literal: nothing upstream
// of the audit line changes. The event struct, its wire keys, and
// every emitter stay byte-stable; the trace is a derived read-side
// record shape, built from an already-written policy.decision line,
// and no plane consumes it yet. What lands here:
//
//   - the fifteen section 251 field tokens in spec listing order
//     (normative: the docs table rows and the Node checker derive
//     the wire tokens from this order and fail on drift);
//   - a closed four-stance coverage vocabulary that classifies how
//     each field stands against today's audit line: carried on the
//     line itself, carried via the line's attrs (rule/cap/res_class
//     written by DecisionEvent), delivered additively by this slice
//     (policy_version, enforcement_mode, the correlation family),
//     or structurally absent at Phase 0 (task, intent, outcome,
//     recovery state);
//   - the trace record itself: known_gap fields are omitted from
//     the wire shape and named in known_gap_fields, unrecorded
//     attr-backed fields are named in unrecorded_fields - absence
//     is literally absence, never a zero or an empty string in
//     disguise (the no-inventory doctrine restated from the cost
//     guard slice, which is why neither list is hand-typed: both
//     are derived from the coverage table);
//   - the correlation family: every trace pins the origin event id
//     beside the decision id, refusing a trace that correlates to
//     itself;
//   - a constructor-pinned enforcement-mode line no caller can
//     edit, restating the Phase 0 stance (record, do not enforce).
//
// What it never carries: a new decision, a score, a severity of its
// own, or any enforcement effect. Values are COPIED from the audit
// line, never re-derived or re-judged. Absence is single-meaning
// throughout, every rejection happens before construction leaves a
// record behind, and the Phase 1 ladder action tokens keep zero
// writes into any copied value (the shared section 261 enum gate).
//
// docs/schema-v2.md section 23 is the human-facing contract; the
// Node checker mirrors its key lines and the fifteen-row table
// against this file; no decision-plane (policy, rules, bus,
// auditlog) or command-tree file may reference the symbols below
// while decision_trace_enforcement_plane reads
// none-in-observation-phase (the planeLeak needle set scans for
// exactly that).

// TraceField names one row of the section 251 Decision Trace field
// table. Declaration order is the spec's listing order and is
// normative: the docs table rows and the Node checker derive the
// wire tokens from it and fail on any drift in either direction.
type TraceField string

// The fifteen section 251 fields.
const (
	FieldDecisionID      TraceField = "decision_id"
	FieldTimestamp       TraceField = "timestamp"
	FieldAgent           TraceField = "agent"
	FieldTask            TraceField = "task"
	FieldIntent          TraceField = "intent"
	FieldAction          TraceField = "action"
	FieldResource        TraceField = "resource"
	FieldCapability      TraceField = "capability"
	FieldRiskFactors     TraceField = "risk_factors"
	FieldPolicyVersion   TraceField = "policy_version"
	FieldPolicyRule      TraceField = "policy_rule"
	FieldDecision        TraceField = "decision"
	FieldEnforcementMode TraceField = "enforcement_mode"
	FieldOutcome         TraceField = "outcome"
	FieldRecoveryState   TraceField = "recovery_state"
)

// traceFieldWireNames lists the fifteen tokens in normative (spec)
// order. AllTraceFields mirrors it; nobody recounts them by hand.
var traceFieldWireNames = []string{
	"decision_id", "timestamp", "agent", "task", "intent",
	"action", "resource", "capability", "risk_factors",
	"policy_version", "policy_rule", "decision", "enforcement_mode",
	"outcome", "recovery_state",
}

// AllTraceFields returns the fifteen field tokens in spec order.
// The count and the order are machine-asserted on every run.
func AllTraceFields() []string {
	out := make([]string, len(traceFieldWireNames))
	copy(out, traceFieldWireNames)
	return out
}

// Valid reports whether f is a member of the closed fifteen-field
// table. The empty string is not a field.
func (f TraceField) Valid() bool {
	if f == "" {
		return false
	}
	for _, w := range traceFieldWireNames {
		if string(f) == w {
			return true
		}
	}
	return false
}

// TraceStance is the closed classification of how one section 251
// field stands against the audit line the product writes today.
// The four stances are exhaustive and mutually exclusive; every
// table row carries exactly one, so the reverse-lookup cannot leave
// a field silently unaccounted.
type TraceStance string

// The four coverage stances in normative order.
const (
	StanceOnAuditLine TraceStance = "carried_on_audit_line"
	StanceViaAttrs    TraceStance = "carried_via_attrs"
	StanceAdditive    TraceStance = "additive_field"
	StanceKnownGap    TraceStance = "known_gap_absent"
)

// AllTraceStances lists the four stances in normative order.
func AllTraceStances() []string {
	return []string{
		string(StanceOnAuditLine), string(StanceViaAttrs),
		string(StanceAdditive), string(StanceKnownGap),
	}
}

func (s TraceStance) Valid() bool {
	switch s {
	case StanceOnAuditLine, StanceViaAttrs, StanceAdditive, StanceKnownGap:
		return true
	}
	return false
}

// TraceCoverageRow is one row of the fifteen-row reverse-lookup
// table: field, stance, and the audit-side source the value is
// copied from (empty for known_gap rows - a gap has no source,
// which is exactly what makes it honest).
type TraceCoverageRow struct {
	Field  TraceField
	Stance TraceStance
	Source string
}

// traceCoverageTable is the reverse-lookup itself: today's audit
// line (Event + DecisionEvent attrs) read against the section 251
// table. Six fields ride the line itself; three ride its attrs
// (rule, cap, res_class - the literals DecisionEvent writes); two
// arrive additively with this slice (policy_version stated by the
// recorder, enforcement_mode pinned by the constructor, beside the
// correlation family that always accompanies the record); four are
// structurally absent at Phase 0 and stay named, not filled: task,
// intent, outcome, and recovery_state (the recovery record slice
// exists in this package, but no audit line carries its verdict
// yet, and borrowing one would be a fabrication).
var traceCoverageTable = []TraceCoverageRow{
	{FieldDecisionID, StanceOnAuditLine, "id"},
	{FieldTimestamp, StanceOnAuditLine, "ts"},
	{FieldAgent, StanceOnAuditLine, "agent_id"},
	{FieldTask, StanceKnownGap, ""},
	{FieldIntent, StanceKnownGap, ""},
	{FieldAction, StanceOnAuditLine, "type"},
	{FieldResource, StanceViaAttrs, "res_class"},
	{FieldCapability, StanceViaAttrs, "cap"},
	{FieldRiskFactors, StanceOnAuditLine, "severity"},
	{FieldPolicyVersion, StanceAdditive, "policy_version"},
	{FieldPolicyRule, StanceViaAttrs, "rule"},
	{FieldDecision, StanceOnAuditLine, "decision"},
	{FieldEnforcementMode, StanceAdditive, "enforcement_mode"},
	{FieldOutcome, StanceKnownGap, ""},
	{FieldRecoveryState, StanceKnownGap, ""},
}

// AllTraceCoverage returns a copy of the fifteen-row table.
func AllTraceCoverage() []TraceCoverageRow {
	out := make([]TraceCoverageRow, len(traceCoverageTable))
	copy(out, traceCoverageTable)
	return out
}

// TraceStanceOf reports the stance the table assigns a field.
func TraceStanceOf(f TraceField) TraceStance {
	for _, row := range traceCoverageTable {
		if row.Field == f {
			return row.Stance
		}
	}
	return TraceStance("")
}

// Rule constants pinned verbatim against the key-line block in
// docs/schema-v2.md section 23.
const (
	// TraceAbsenceRule pins the slice's core honesty line: a
	// section 251 field with no source value is omitted from the
	// wire shape and named in the gap list - absence, never a
	// zero, an empty string, or a borrowed number in disguise.
	TraceAbsenceRule = "field-without-value-is-absent-not-zero"
	// TraceCorrelationRule pins the correlation family: every
	// trace names the origin event id beside the decision id,
	// and the origin may never equal the decision itself (a line
	// correlating to itself would launder a missing join).
	TraceCorrelationRule = "trace-pins-origin-event-id-not-equal-decision-id"
	// TraceEnforcementModeValue is the constructor-pinned
	// enforcement-mode line: Phase 0 records and does not
	// enforce, so the value is pinned in code and has no caller
	// entrance at all.
	TraceEnforcementModeValue = "record_only_phase0"
	// TraceEnforcementPlane pins the Phase 0 stance the planeLeak
	// set and the command-tree grep scan for: the trace is a
	// read-side record shape; no plane consumes it yet.
	TraceEnforcementPlane = "none-in-observation-phase"
	// TraceDefaultAppliedRule pins what an empty matched-rule on
	// the audit line means in trace form: the documented
	// DecisionEvent literal - empty rule = default applied -
	// restated as the closed token below, never dropped into a
	// silence that reads as "no rule existed".
	TraceDefaultAppliedRule = "empty-rule-means-default-applied"
)

// TraceDefaultApplied is the restated empty-rule literal.
const TraceDefaultApplied = "default_applied"

// DecisionTrace is the section 251 machine-readable trace derived
// from one already-written policy.decision audit line. The wire
// shape omits known_gap and unrecorded fields entirely (both gap
// lists are derived from the coverage table and the source line,
// never hand-typed) and pins the correlation family and the
// enforcement-mode stance. The struct carries no verdict field of
// its own beyond the copied decision value, no score, and no
// severity: the reflective test pins the exact key census.
type DecisionTrace struct {
	DecisionID       string   `json:"decision_id"`
	Timestamp        string   `json:"timestamp"`
	Agent            string   `json:"agent"`
	Action           string   `json:"action"`
	RiskFactors      string   `json:"risk_factors"`
	PolicyRule       string   `json:"policy_rule"`
	Decision         string   `json:"decision"`
	PolicyVersion    string   `json:"policy_version"`
	EnforcementMode  string   `json:"enforcement_mode"`
	OriginEventID    string   `json:"origin_event_id"`
	Capability       string   `json:"capability,omitempty"`
	Resource         string   `json:"resource,omitempty"`
	KnownGapFields   []string `json:"known_gap_fields"`
	UnrecordedFields []string `json:"unrecorded_fields"`
}

// BuildDecisionTrace derives the trace for one audit line. The
// source event is never mutated. Every rejection happens before
// construction: nil or wrong-typed lines (only policy.decision
// lines trace), decisions outside the Phase 0 runtime vocabulary,
// empty ids/agent/zero timestamp, a wild policy_version (the value
// must be a stated, printable, single-token form - an un-stated
// version is a recorder error, not a gap, because the field's
// stance here is additive and the recorder is expected to state
// it), an origin id equal to the decision id (self-correlation),
// and any copied attr value colliding with a Phase 1 ladder action
// token (the shared section 261 enum gate: LIMIT, HOLD, CANCEL,
// RECOVER keep zero writes into product-path record values).
func BuildDecisionTrace(ev *Event, originEventID, policyVersion string) (*DecisionTrace, error) {
	if ev == nil {
		return nil, fmt.Errorf("schema: nil source event")
	}
	if ev.Type != TypePolicyDecision {
		return nil, fmt.Errorf("schema: decision trace needs a policy.decision line, got %q", string(ev.Type))
	}
	if err := MustPhase0Decision(ev.Decision); err != nil {
		return nil, err
	}
	if !ev.Severity.Valid() {
		return nil, fmt.Errorf("schema: decision trace needs a valid severity on the source line, got %d", int(ev.Severity))
	}
	if ev.ID == "" || ev.AgentID == "" || ev.TS.IsZero() {
		return nil, fmt.Errorf("schema: decision trace needs id, agent, and timestamp on the source line")
	}
	if originEventID == "" {
		return nil, fmt.Errorf("schema: decision trace needs a non-empty origin event id")
	}
	if originEventID == ev.ID {
		return nil, fmt.Errorf("schema: decision trace origin may not correlate to itself (%q)", originEventID)
	}
	if err := validTraceStatedToken("policy_version", policyVersion); err != nil {
		return nil, err
	}
	rule := ev.Attrs["rule"]
	if rule == "" {
		rule = TraceDefaultApplied
	}
	hard := ev.Attrs["hard"]
	if hard == "" {
		return nil, fmt.Errorf(`schema: decision trace needs the "hard" annotation on the source line`)
	}
	capTok := ev.Attrs["cap"]
	resTok := ev.Attrs["res_class"]
	for name, v := range map[string]string{"policy_rule": rule, "cap": capTok, "res_class": resTok} {
		if v == "" {
			continue
		}
		for _, bad := range AgencyForbiddenActionTokens() {
			if v == bad {
				return nil, fmt.Errorf("schema: decision trace field %s collides with ladder token %q", name, bad)
			}
		}
	}
	t := &DecisionTrace{
		DecisionID:      ev.ID,
		Timestamp:       ev.TS.UTC().Format("2006-01-02T15:04:05Z"),
		Agent:           ev.AgentID,
		Action:          string(ev.Type),
		RiskFactors:     fmt.Sprintf("severity=%d|hard=%s", int(ev.Severity), hard),
		PolicyRule:      rule,
		Decision:        string(ev.Decision),
		PolicyVersion:   policyVersion,
		EnforcementMode: TraceEnforcementModeValue,
		OriginEventID:   originEventID,
	}
	for _, row := range traceCoverageTable {
		switch row.Stance {
		case StanceKnownGap:
			t.KnownGapFields = append(t.KnownGapFields, string(row.Field))
		case StanceViaAttrs:
			switch row.Field {
			case FieldCapability:
				if capTok == "" {
					t.UnrecordedFields = append(t.UnrecordedFields, string(row.Field))
				} else {
					t.Capability = capTok
				}
			case FieldResource:
				if resTok == "" {
					t.UnrecordedFields = append(t.UnrecordedFields, string(row.Field))
				} else {
					t.Resource = resTok
				}
			}
		}
	}
	return t, nil
}

// validTraceStatedToken pins the recorder-stated form: non-empty,
// printable ASCII, no spaces or control bytes.
func validTraceStatedToken(name, v string) error {
	if v == "" {
		return fmt.Errorf("schema: decision trace %s must be stated", name)
	}
	for i := 0; i < len(v); i++ {
		c := v[i]
		if c < 0x21 || c > 0x7e {
			return fmt.Errorf("schema: decision trace %s carries an unprintable or non-ASCII byte at %d", name, i)
		}
	}
	return nil
}

// Stated reports whether one section 251 field carries a value on
// this trace record. The answer is derived by consulting the
// coverage table and the wire shape together: a known_gap field is
// never stated, an attrs field is stated only when present, and a
// line/additive field is always stated (construction rejected
// otherwise).
func (t *DecisionTrace) Stated(f TraceField) bool {
	switch f {
	case FieldDecisionID, FieldTimestamp, FieldAgent, FieldAction,
		FieldRiskFactors, FieldPolicyVersion, FieldPolicyRule,
		FieldDecision, FieldEnforcementMode:
		return TraceStanceOf(f) != StanceKnownGap
	case FieldCapability:
		return t.Capability != ""
	case FieldResource:
		return t.Resource != ""
	default:
		return false
	}
}

// EncodeTraceChecked serializes one trace and refuses to emit a
// record whose derived gap lists drifted from the coverage table
// (a hand-edited struct is caught at the door, mirroring the
// EncodeImpactChecked shape).
func EncodeTraceChecked(t *DecisionTrace) ([]byte, error) {
	if t == nil {
		return nil, fmt.Errorf("schema: nil trace record")
	}
	var want []string
	for _, row := range traceCoverageTable {
		if row.Stance == StanceKnownGap {
			want = append(want, string(row.Field))
		}
	}
	if strings.Join(t.KnownGapFields, ",") != strings.Join(want, ",") {
		return nil, fmt.Errorf("schema: trace known_gap list drifted from the coverage table")
	}
	if t.EnforcementMode != TraceEnforcementModeValue || t.DecisionID == "" || t.OriginEventID == "" || t.DecisionID == t.OriginEventID {
		return nil, fmt.Errorf("schema: trace correlation or pinned mode drifted")
	}
	return json.Marshal(t)
}
