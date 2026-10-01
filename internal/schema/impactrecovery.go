package schema

// Impact and recovery record shapes for the stability-wave schema
// contracts (slice W2.3). Like the intent and authority records these
// are observation-phase RECORD contracts: nothing in the runtime emits
// them, nothing in the evaluator, the rule engine, the bus, or the audit
// writer consumes them, and no recovery or rollback execution plane
// exists behind them. Recovery execution (undoing side effects) is a
// later-phase surface; this slice records honest classification only.
// docs/schema-v2.md is the human-facing contract; the sync tests in this
// package pin the two sources verbatim, and the Node structural checker
// mirrors them from the other side.

import (
	"bytes"
	"encoding/json"
	"fmt"
)

// RecoveryClass is one step of the closed four-level truthfulness scale
// the owner specification defines for recovery capability. Declaration
// order is normative: the contract file, this list, and the Node
// checker compare joined strings verbatim, in order. The spec spells
// these tokens in upper case with spaces (and one hyphen); the wire
// tokens below are derived mechanically (lower case, spaces and hyphens
// to single underscores) and the docs anchor block pins the mapping.
type RecoveryClass string

const (
	RecoveryLocalReversible      RecoveryClass = "local_reversible"      // undone locally by this product alone
	RecoveryLocalPartial         RecoveryClass = "local_partial"         // undone locally only in part
	RecoveryExternalCompensation RecoveryClass = "external_compensation" // needs a compensating external action
	RecoveryNonReversible        RecoveryClass = "non_reversible"        // cannot be undone (network sends, third-party state changes, payments)
)

// RecoveryExecutionPlane names the (currently empty) plane that would
// act on a recovery class to undo or compensate anything. In the
// observation phase no execution plane exists; enabling one is a
// deliberate later-phase change, never a silent one.
const RecoveryExecutionPlane = "none-in-observation-phase"

// RecoveryTruthfulnessRule is the recorded promise lifted from the
// specification: the product must never advertise that all actions can
// be undone. The four-class scale exists precisely because external
// side effects may be non-reversible or need external compensation.
const RecoveryTruthfulnessRule = "never-claim-fully-reversible"

// AllRecoveryClasses lists every RecoveryClass in normative declaration
// order. scripts/schema-v2-check.mjs and docs/schema-v2.md mirror this
// list verbatim; the sync tests fail on any drift.
func AllRecoveryClasses() []string {
	return []string{
		string(RecoveryLocalReversible), string(RecoveryLocalPartial),
		string(RecoveryExternalCompensation), string(RecoveryNonReversible),
	}
}

// Valid reports whether c is one of the closed set of four classes. The
// spec display spellings ("LOCAL REVERSIBLE", "NON-REVERSIBLE") and any
// execution-verb-flavoured coinage are outside the set by construction.
func (c RecoveryClass) Valid() bool {
	switch c {
	case RecoveryLocalReversible, RecoveryLocalPartial,
		RecoveryExternalCompensation, RecoveryNonReversible:
		return true
	}
	return false
}

// BlastScope names one of the eight potential-blast-radius dimensions
// the specification lists for per-agent/per-task estimation. The spec
// bullet lines are Chinese prose, so these wire tokens are in-repo
// naming assigned by this slice; the docs scope table pins the mapping
// by row order and count against the specification's eight bullets
// (honest degradation from a verbatim anchor, noted in the contract).
type BlastScope string

const (
	ScopeFile       BlastScope = "file_scope"       // file-level reach
	ScopeProject    BlastScope = "project_scope"    // whole-project reach
	ScopeNetwork    BlastScope = "network_scope"    // network reach
	ScopeProcess    BlastScope = "process_scope"    // process reach
	ScopeDatabase   BlastScope = "database_scope"   // database reach
	ScopeCredential BlastScope = "credential_scope" // credential reach
	ScopeDevice     BlastScope = "device_scope"     // device reach
	ScopeSubAgent   BlastScope = "subagent_scope"   // sub-agent reach
)

// blastScopeWireNames lists the eight scope tokens in normative order,
// pinned table-row-order against the specification's eight bullets.
var blastScopeWireNames = []string{
	"file_scope", "project_scope", "network_scope", "process_scope",
	"database_scope", "credential_scope", "device_scope", "subagent_scope",
}

// AllBlastScopes returns the eight scope tokens in normative order.
func AllBlastScopes() []string {
	out := make([]string, len(blastScopeWireNames))
	copy(out, blastScopeWireNames)
	return out
}

// Valid reports whether s is one of the closed set of eight scopes.
func (s BlastScope) Valid() bool {
	for _, w := range blastScopeWireNames {
		if string(s) == w {
			return true
		}
	}
	return false
}

// impactFieldWireNames lists the seven wire tokens of the impact
// contract in normative order, derived mechanically (lower case,
// spaces to single underscores) from the verbatim anchor block in
// docs/schema-v2.md. The count and the mapping are machine-asserted on
// every run; nobody recounts them by hand.
var impactFieldWireNames = []string{
	"direct_impact", "indirect_impact", "propagation_impact",
	"blast_radius", "reversibility", "dependency_impact",
	"production_impact",
}

// AllImpactFields returns the seven-field vocabulary in normative order.
func AllImpactFields() []string {
	out := make([]string, len(impactFieldWireNames))
	copy(out, impactFieldWireNames)
	return out
}

// ImpactEnforcementPlane mirrors the authority-plane discipline for the
// impact record: nothing consumes impact fields in the observation
// phase. Keeping the string separate from the authority constant is
// deliberate - each record evolves its plane line on its own.
const ImpactEnforcementPlane = "none-in-observation-phase"

// BlastRadiusEstimate records the potential blast radius of one action
// as a subset of the closed eight-scope vocabulary. Every part is
// optional at the record level: absence means not estimated, never
// estimated-zero.
type BlastRadiusEstimate struct {
	Scopes []BlastScope `json:"scopes,omitempty"`
	Extent string       `json:"extent,omitempty"`
}

// Validate rejects any scope token outside the closed eight.
func (b *BlastRadiusEstimate) Validate() error {
	if b == nil {
		return nil
	}
	for _, s := range b.Scopes {
		if !s.Valid() {
			return fmt.Errorf("blast scope %q is outside the closed set", string(s))
		}
	}
	return nil
}

// ImpactRecord is the per-action estimation surface lifted from the
// specification's impact-analysis section: for every important action
// the system estimates the seven dimensions as best it can. Like the
// intent record every field is optional with exactly one absent
// meaning: not estimated (a known gap, never a fabricated zero). The
// Reversibility field reuses the closed four-class vocabulary so the
// later chain-reduction and harm-definition slices consume one set of
// tokens, not two; in this slice nothing consumes it, and a value that
// is not one of the four classes (for example an execution-verb
// coinage like "auto_rollback") is rejected before any bytes exist.
type ImpactRecord struct {
	DirectImpact      string               `json:"direct_impact,omitempty"`
	IndirectImpact    string               `json:"indirect_impact,omitempty"`
	PropagationImpact string               `json:"propagation_impact,omitempty"`
	BlastRadius       *BlastRadiusEstimate `json:"blast_radius,omitempty"`
	Reversibility     RecoveryClass        `json:"reversibility,omitempty"`
	DependencyImpact  string               `json:"dependency_impact,omitempty"`
	ProductionImpact  string               `json:"production_impact,omitempty"`
}

// Validate checks the closed-set members of the record: the reversibility
// observation (when present) must be one of the four recovery classes,
// and every blast scope must be one of the eight scope tokens.
func (r *ImpactRecord) Validate() error {
	if r == nil {
		return fmt.Errorf("nil impact record")
	}
	if r.Reversibility != "" && !r.Reversibility.Valid() {
		return fmt.Errorf("reversibility %q is outside the closed four-class set", string(r.Reversibility))
	}
	return r.BlastRadius.Validate()
}

// ParseImpactRecord decodes one impact record, rejecting any field name
// outside the closed seven-token vocabulary before trusting a value and
// validating the closed-set members. On any error it returns a nil
// record: nothing downstream of a rejection ever holds a usable value,
// matching the zero-byte rejection shape the audit writer uses.
func ParseImpactRecord(data []byte) (*ImpactRecord, error) {
	dec := json.NewDecoder(bytes.NewReader(data))
	dec.DisallowUnknownFields()
	var rec ImpactRecord
	if err := dec.Decode(&rec); err != nil {
		return nil, err
	}
	if err := rec.Validate(); err != nil {
		return nil, err
	}
	return &rec, nil
}

// EncodeImpactChecked validates the record before serializing and
// returns nil bytes on any rejection: a record that fails the contract
// produces no output at all.
func EncodeImpactChecked(rec *ImpactRecord) ([]byte, error) {
	if err := rec.Validate(); err != nil {
		return nil, err
	}
	return json.Marshal(rec)
}

// RecoveryRecord is the honest classification of how recoverable one
// recorded change (or one transaction of changes) is, using the closed
// four-class scale. Class is required: a recovery record without a
// class is malformed, not "unknown" - honest absence lives at the
// record level (no record written means not classified, a known gap),
// never at the class level (an implicit reversible default). The
// Transaction field correlates the record to the task-level group of
// changes it describes; it is a record bit only, and no code path acts
// on any field of this record in the observation phase.
type RecoveryRecord struct {
	Class       RecoveryClass `json:"class"`
	Transaction string        `json:"transaction,omitempty"`
}

// Validate rejects wild class tokens, including the empty value: the
// class is the reason the record exists.
func (r RecoveryRecord) Validate() error {
	if !r.Class.Valid() {
		return fmt.Errorf("recovery class %q is outside the closed four-class set", string(r.Class))
	}
	return nil
}

// ParseRecoveryRecord decodes one recovery record, rejecting any field
// name outside the closed vocabulary and requiring a classed record. On
// any error it returns a nil record.
func ParseRecoveryRecord(data []byte) (*RecoveryRecord, error) {
	dec := json.NewDecoder(bytes.NewReader(data))
	dec.DisallowUnknownFields()
	var rec RecoveryRecord
	if err := dec.Decode(&rec); err != nil {
		return nil, err
	}
	if err := rec.Validate(); err != nil {
		return nil, err
	}
	return &rec, nil
}

// EncodeRecoveryChecked validates before serializing; rejected records
// produce no bytes.
func EncodeRecoveryChecked(rec *RecoveryRecord) ([]byte, error) {
	if rec == nil {
		return nil, fmt.Errorf("nil recovery record")
	}
	if err := rec.Validate(); err != nil {
		return nil, err
	}
	return json.Marshal(rec)
}
