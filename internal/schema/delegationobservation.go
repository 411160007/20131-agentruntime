package schema

import (
	"fmt"
	"strings"
)

// Delegation observation record (spec vNext sections 283 and 236,
// slice W5.3): when one agent delegates to another subject the spec
// refuses to let the child ride on the parent's authority. Section
// 236 states the inequality (Agent A Capability is not Agent B
// Capability, A's authority never transfers automatically to B) and
// lists seven objects that must be re-evaluated at the moment of
// delegation (identity, authority, capability, scope, data, risk,
// TTL); it extends the discipline beyond agents to skills, MCP
// servers, tools, child processes, and plugins. Section 283 names
// the eight fields every delegation must record and requires that
// any child agent revalidate its own permissions.
//
// This slice lands the record shape only, exactly as the taskbook's
// consumption note requires. What the record carries:
//
//   - the eight section 283 fields (the taskbook's seven-field
//     shorthand parent/child/reason/delegated-cap/scope/TTL/outcome
//     plus the spec's own eighth line, Authority Source, which is
//     what section 236's Authority re-evaluation object records);
//   - the subject of the delegation as a member of the closed
//     section 236 five (skill, mcp, tool, child_process, plugin) or
//     an agent-to-agent delegation, where the subject vocabulary
//     still names the receiving shape;
//   - the child revalidation state as a member of the closed three:
//     recorded-revalidated, recorded-not-revalidated, or explicitly
//     unrecorded - silence is only admissible when it is itself
//     recorded, never as a default;
//   - two constructor-pinned restatements (no-auto-inheritance and
//     the enforcement-plane stance) that no caller can edit.
//
// What it never carries: a decision, a score, a severity, or an
// enforcement effect. Permission checking against live capability
// tables is enforcement and sits in Phase 1 (pending decision);
// pre-borrowing it here would itself violate the no-inheritance
// rule the record exists to restate. The delegated capability and
// data scope are recorded as free text precisely so this slice
// claims no matching power. Absence is single-meaning throughout:
// an unstated revalidation is not "fine", an empty field never
// builds, and a rejected input leaves no half record behind.
//
// docs/schema-v2.md section 20 is the human-facing contract; the
// Node checker mirrors its six key lines and both spec anchor bullet
// blocks against this file; no decision-plane (policy, rules, bus,
// auditlog) or command-tree file may reference the symbols below
// while delegation_enforcement_plane reads none-in-observation-phase
// (the planeLeak needle set scans for exactly that).

// DelegationSubject names one receiving shape of the section 236
// transfer discipline. Declaration order is the spec's listing
// order (Skill, MCP, Tool, Child Process, Plugin) and is normative:
// the section 20 anchor bullets and the Node checker derive the
// wire tokens from it and fail on any drift in either direction.
type DelegationSubject string

// The five section 236 subjects.
const (
	SubjectSkill        DelegationSubject = "skill"
	SubjectMCP          DelegationSubject = "mcp"
	SubjectTool         DelegationSubject = "tool"
	SubjectChildProcess DelegationSubject = "child_process"
	SubjectPlugin       DelegationSubject = "plugin"
)

// delegationSubjectWireNames lists the five subject tokens in
// normative (spec) order. AllDelegationSubjects mirrors it; nobody
// recounts them by hand.
var delegationSubjectWireNames = []string{
	"skill", "mcp", "tool", "child_process", "plugin",
}

// AllDelegationSubjects returns the five subject tokens in spec
// order. The count and the order are machine-asserted on every run.
func AllDelegationSubjects() []string {
	out := make([]string, len(delegationSubjectWireNames))
	copy(out, delegationSubjectWireNames)
	return out
}

// Valid reports whether s is a member of the closed five-subject
// vocabulary. The empty string is not a subject: a delegation whose
// receiving shape was never recorded fails construction, it does
// not quietly become a wildcard member.
func (s DelegationSubject) Valid() bool {
	if s == "" {
		return false
	}
	for _, w := range delegationSubjectWireNames {
		if string(s) == w {
			return true
		}
	}
	return false
}

// DelegationRevalidationState is the closed tri-state of section
// 283's child-revalidation obligation over one recorded delegation.
// revalidated and not_revalidated are findings over what was
// recorded; unrecorded is the honest explicit shape when the
// obligation's fulfilment was never observed - "nothing recorded"
// is restated as itself, never laundered into either a finding or a
// clean bill of health.
type DelegationRevalidationState string

// The three revalidation states in normative order.
const (
	RevalidationDone       DelegationRevalidationState = "revalidated"
	RevalidationSkipped    DelegationRevalidationState = "not_revalidated"
	RevalidationUnrecorded DelegationRevalidationState = "unrecorded"
)

// AllDelegationRevalidationStates lists the three states in
// normative order.
func AllDelegationRevalidationStates() []string {
	return []string{
		string(RevalidationDone), string(RevalidationSkipped),
		string(RevalidationUnrecorded),
	}
}

// Valid reports whether st is one of the three states. The empty
// string is not a state: callers must state the silence explicitly
// as unrecorded.
func (st DelegationRevalidationState) Valid() bool {
	if st == "" {
		return false
	}
	for _, w := range AllDelegationRevalidationStates() {
		if string(st) == w {
			return true
		}
	}
	return false
}

// Rule constants pinned verbatim against the six-key machine block
// in docs/schema-v2.md section 20.
const (
	// DelegationNoAutoInheritanceRule pins section 236's headline
	// inequality: whatever the parent held, the child holds nothing
	// of it by the fact of delegation alone. The record restates
	// the rule; it cannot enforce it from inside Phase 0.
	DelegationNoAutoInheritanceRule = "parent-authority-never-auto-transfers-to-child"
	// DelegationReevaluationRegistry lists, in section 236 bullet
	// order, the seven objects a delegation forces back onto the
	// table. It is recorded as a closed registry line, not as live
	// checks: running those checks is enforcement (Phase 1).
	DelegationReevaluationRegistry = "identity,authority,capability,scope,data,risk,ttl"
	// DelegationRevalidationObligationRule pins section 283's
	// closing sentence: any child agent must revalidate its own
	// permissions. The record's revalidation field restates who
	// says that happened, when, and how, is Phase 1's shape.
	DelegationRevalidationObligationRule = "every-child-must-revalidate-own-permissions"
	// DelegationEnforcementPlane pins the Phase 0 stance: this is
	// an observation record; no plane borrows it to decide,
	// block, or grant anything yet.
	DelegationEnforcementPlane = "none-in-observation-phase"
)

// DelegationObservationInput is the recorder's side of one
// delegation: the eight section 283 fields plus the receiving
// subject and the stated child-revalidation outcome. Every field is
// required and free text is deliberately not validated against any
// live vocabulary - matching delegated capability or scope against
// real tables would pre-borrow Phase 1 enforcement. The struct
// carries no decision, score, or severity field and the reflective
// test pins that.
type DelegationObservationInput struct {
	ParentAgent         string                      `json:"parent_agent"`
	ChildAgent          string                      `json:"child_agent"`
	DelegationReason    string                      `json:"delegation_reason"`
	DelegatedCapability string                      `json:"delegated_capability"`
	DataScope           string                      `json:"data_scope"`
	AuthoritySource     string                      `json:"authority_source"`
	TTL                 string                      `json:"ttl"`
	Outcome             string                      `json:"outcome"`
	Subject             DelegationSubject           `json:"subject"`
	Revalidation        DelegationRevalidationState `json:"child_revalidation"`
}

// DelegationRecordFieldWireNames is the normative field list of the
// full record (the ten recorder-supplied lines plus the two
// constructor-pinned restatements), in this order. The section 20
// field table and the Node checker pin it row by row.
var DelegationRecordFieldWireNames = []string{
	"parent_agent", "child_agent", "delegation_reason", "delegated_capability",
	"data_scope", "authority_source", "ttl", "outcome",
	"subject", "child_revalidation", "authority_not_inherited", "enforcement_plane",
}

// DelegationObservation is the row BuildDelegationObservation
// produces: the input restated verbatim plus two constants no
// caller can edit. It is a record of one delegation event, not a
// judgement about it.
type DelegationObservation struct {
	ParentAgent           string                      `json:"parent_agent"`
	ChildAgent            string                      `json:"child_agent"`
	DelegationReason      string                      `json:"delegation_reason"`
	DelegatedCapability   string                      `json:"delegated_capability"`
	DataScope             string                      `json:"data_scope"`
	AuthoritySource       string                      `json:"authority_source"`
	TTL                   string                      `json:"ttl"`
	Outcome               string                      `json:"outcome"`
	Subject               DelegationSubject           `json:"subject"`
	ChildRevalidation     DelegationRevalidationState `json:"child_revalidation"`
	AuthorityNotInherited string                      `json:"authority_not_inherited"`
	EnforcementPlane      string                      `json:"enforcement_plane"`
}

// requireText rejects the empty and whitespace-only shapes of a
// section 283 must-record field. "Must record" leaves no third
// option: a field that cannot be stated aborts the whole record.
func requireText(field, value string) error {
	if strings.TrimSpace(value) == "" {
		return fmt.Errorf("delegation observation: field %q must record text (section 283 must-record list)", field)
	}
	return nil
}

// BuildDelegationObservation validates one delegation input and
// projects the record. Every rejection fires before any value is
// applied: a missing or whitespace-only must-record field, a
// subject outside the closed five, or a revalidation state outside
// the closed three leaves no record behind. The constructor never
// judges: it restates what the recorder said and appends the two
// pinned rules verbatim.
func BuildDelegationObservation(in DelegationObservationInput) (DelegationObservation, error) {
	for _, req := range []struct {
		name  string
		value string
	}{
		{"parent_agent", in.ParentAgent},
		{"child_agent", in.ChildAgent},
		{"delegation_reason", in.DelegationReason},
		{"delegated_capability", in.DelegatedCapability},
		{"data_scope", in.DataScope},
		{"authority_source", in.AuthoritySource},
		{"ttl", in.TTL},
		{"outcome", in.Outcome},
	} {
		if err := requireText(req.name, req.value); err != nil {
			return DelegationObservation{}, err
		}
	}
	if !in.Subject.Valid() {
		return DelegationObservation{}, fmt.Errorf("delegation observation: subject %q is outside the closed five", string(in.Subject))
	}
	if !in.Revalidation.Valid() {
		return DelegationObservation{}, fmt.Errorf("delegation observation: child revalidation state %q is outside the closed three (state the silence explicitly as unrecorded)", string(in.Revalidation))
	}
	return DelegationObservation{
		ParentAgent:           in.ParentAgent,
		ChildAgent:            in.ChildAgent,
		DelegationReason:      in.DelegationReason,
		DelegatedCapability:   in.DelegatedCapability,
		DataScope:             in.DataScope,
		AuthoritySource:       in.AuthoritySource,
		TTL:                   in.TTL,
		Outcome:               in.Outcome,
		Subject:               in.Subject,
		ChildRevalidation:     in.Revalidation,
		AuthorityNotInherited: DelegationNoAutoInheritanceRule,
		EnforcementPlane:      DelegationEnforcementPlane,
	}, nil
}
