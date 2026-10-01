package schema

// Intent alignment record shapes for the intent-alignment wave
// (slice W3.2, owner spec section 232). Section 232 asks for a
// continuous comparison of User Intent against Agent Plan against
// Actual Action, with every action classified into a closed four-token
// set. Like the impact, recovery, evidence, and profile records these
// are observation-phase RECORD contracts: nothing in the runtime emits
// them, nothing in the evaluator, the rule engine, the bus, or the
// audit writer consumes them, and the classification carries zero
// decision effect. Raising a control level on the strength of a class
// is an enforcement-plane action; section 232 states that escalation
// precondition ("unrelated or uncertain plus high impact, high
// sensitivity, or irreversibility must raise the control level") as a
// recorded line only in this slice, never wired behavior.
// docs/schema-v2.md section 12 is the human-facing contract; the sync
// tests in this package pin the two sources verbatim, and the Node
// structural checker mirrors them from the other side.

import (
	"bytes"
	"encoding/json"
	"fmt"
)

// ActionClass is one member of the closed four-class alignment
// vocabulary of section 232. Declaration order is normative: the
// contract file, this list, and the Node checker compare joined
// strings verbatim, in order. The spec spells these tokens in upper
// case; the wire tokens below are derived mechanically (lower case)
// and the docs anchor block pins the mapping, exactly like the
// recovery scale did. MALICIOUS is deliberately NOT a class: the
// spec states two honesty equations - UNCERTAIN is not MALICIOUS and
// UNRELATED is not MALICIOUS - and the closed set enforces them by
// construction: any attempt to record a suspicion token as a class
// is rejected before any bytes or usable values exist.
type ActionClass string

const (
	ActionDirect    ActionClass = "direct"    // action matches an intent or plan step directly
	ActionInferred  ActionClass = "inferred"  // match established by inference from context, not a stated step
	ActionUncertain ActionClass = "uncertain" // match cannot be established either way from recorded data
	ActionUnrelated ActionClass = "unrelated" // no correspondence to intent or plan found
)

// AlignmentMaliciousRule is the recorded promise lifted from the
// specification's special note: neither suspicion-flavoured class is
// a malice verdict. The string lives beside the vocabulary so the two
// honesty equations travel with every consumer of these records.
const AlignmentMaliciousRule = "uncertain-is-not-malicious-unrelated-is-not-malicious"

// AlignmentEscalationPrecondition records section 232's closing
// sentence as a string only. In the observation phase the precondition
// is carried, never enforced: no code path reads a class and changes a
// control level, an enforcement ruling, or a decision value.
const AlignmentEscalationPrecondition = "recorded-not-enforced"

// AlignmentEnforcementPlane mirrors the plane discipline of the
// earlier record contracts: nothing consumes alignment fields in the
// observation phase.
const AlignmentEnforcementPlane = "none-in-observation-phase"

// AlignmentAbsentSemantics is the single honest meaning of a missing
// alignment record: the action was never compared, a known gap. It
// must never be read as an implicit direct classification, and no
// default-fill path may manufacture a record to close the gap.
const AlignmentAbsentSemantics = "no-record-means-never-compared-never-implied-direct"

// alignmentRecordFieldWireNames lists the five wire tokens of the
// alignment record in normative order, mechanically derived from the
// struct tags. The docs field table pins the same census; the count
// and the mapping are machine-asserted on every run.
var alignmentRecordFieldWireNames = []string{
	"class", "action_ref", "intent_ref", "plan_ref", "basis",
}

// AllAlignmentRecordFields returns the five record field tokens in
// normative order.
func AllAlignmentRecordFields() []string {
	out := make([]string, len(alignmentRecordFieldWireNames))
	copy(out, alignmentRecordFieldWireNames)
	return out
}

// alignmentClassWireNames lists the four wire tokens in normative
// declaration order. scripts/schema-v2-check.mjs and
// docs/schema-v2.md mirror this list verbatim; the sync tests fail on
// any drift.
var alignmentClassWireNames = []string{
	"direct", "inferred", "uncertain", "unrelated",
}

// AllActionClasses lists every ActionClass in normative order.
func AllActionClasses() []string {
	out := make([]string, len(alignmentClassWireNames))
	copy(out, alignmentClassWireNames)
	return out
}

// Valid reports whether c is one of the closed set of four classes.
// The spec display spellings in upper case, any malice-flavoured
// coinage, and every other shape are outside the set by construction.
func (c ActionClass) Valid() bool {
	switch c {
	case ActionDirect, ActionInferred, ActionUncertain, ActionUnrelated:
		return true
	}
	return false
}

// AlignmentRecord is the per-action classification shape: the class
// itself plus optional correlation references into the three streams
// section 232 compares (user intent, agent plan, actual action) and an
// optional human-readable basis line. Class is required: an alignment
// record without a class is malformed, not "unknown" - honest absence
// lives at the record level (no record written means the action was
// never compared, a known gap), never at the class level (an implicit
// direct default would silently upgrade every unexamined action). The
// reference fields are record bits: they point at other records by
// identifier and no code path acts on any field of this record in the
// observation phase.
type AlignmentRecord struct {
	Class     ActionClass `json:"class"`
	ActionRef string      `json:"action_ref,omitempty"`
	IntentRef string      `json:"intent_ref,omitempty"`
	PlanRef   string      `json:"plan_ref,omitempty"`
	Basis     string      `json:"basis,omitempty"`
}

// Validate checks the closed-set member of the record: the class must
// be one of the four alignment tokens.
func (r *AlignmentRecord) Validate() error {
	if r == nil {
		return fmt.Errorf("nil alignment record")
	}
	if !r.Class.Valid() {
		return fmt.Errorf("alignment class %q is outside the closed four-class set", string(r.Class))
	}
	return nil
}

// ParseAlignmentRecord decodes one alignment record, rejecting any
// field name outside the closed vocabulary before trusting a value
// and validating the closed-set member. On any error it returns a nil
// record: nothing downstream of a rejection ever holds a usable value,
// matching the zero-byte rejection shape the other record contracts
// use.
func ParseAlignmentRecord(data []byte) (*AlignmentRecord, error) {
	dec := json.NewDecoder(bytes.NewReader(data))
	dec.DisallowUnknownFields()
	var rec AlignmentRecord
	if err := dec.Decode(&rec); err != nil {
		return nil, err
	}
	if err := rec.Validate(); err != nil {
		return nil, err
	}
	return &rec, nil
}

// EncodeAlignmentChecked validates the record before serializing and
// returns nil bytes on any rejection: a record that fails the
// contract produces no output at all.
func EncodeAlignmentChecked(rec *AlignmentRecord) ([]byte, error) {
	if err := rec.Validate(); err != nil {
		return nil, err
	}
	return json.Marshal(rec)
}
