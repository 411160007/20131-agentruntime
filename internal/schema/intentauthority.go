package schema

// Intent and authority record shapes for the stability-wave schema
// contracts (slice W2.2). These are observation-phase RECORD contracts:
// nothing in the runtime emits them yet, and nothing in the evaluator,
// the rule engine, the bus, or the audit writer consumes them. Wiring
// any of this into a decision path would be a deliberate later-phase
// change, never a silent one. docs/schema-v2.md is the human-facing
// contract; the sync tests in this package pin the two sources verbatim
// against each other, and the Node structural checker mirrors them from
// the other side.

import (
	"bytes"
	"encoding/json"
	"fmt"
)

// GrantOrigin names how an authority grant (or an intent modification)
// entered the record. The five tokens mirror the five provider forms in
// the owner specification's intent-contract section. Declaration order
// is normative: the contract file, this list, and the Node checker
// compare joined strings verbatim, in order.
type GrantOrigin string

const (
	OriginUserDirect       GrantOrigin = "user_direct"                // provided directly by the user
	OriginAgentProvided    GrantOrigin = "agent_provided"             // agent-provided task description
	OriginUIGenerated      GrantOrigin = "ui_generated"               // generated through UI interaction
	OriginModelInterpreted GrantOrigin = "external_model_interpreted" // external model-assisted interpretation
	OriginRuntimeInferred  GrantOrigin = "runtime_inferred"           // inferred from runtime behavior
)

// AuthorityPropagationRule is the recorded rule string for the authority
// chain: an untrusted mark is sticky and never auto-escalates user
// authority. In the observation phase the rule is recorded, not
// enforced; no consumer acts on it yet.
const AuthorityPropagationRule = "untrusted-sticky-never-auto-escalate"

// AuthorityEnforcementPlane names the (currently empty) plane that acts
// on untrusted marks. No enforcement plane exists in the observation
// phase; enabling one is a later-phase decision, including the open
// posture questions that stay deliberately undecided here.
const AuthorityEnforcementPlane = "none-in-observation-phase"

// AllGrantOrigins lists every GrantOrigin in normative declaration
// order. scripts/schema-v2-check.mjs and docs/schema-v2.md mirror this
// list verbatim; the sync tests fail on any drift.
func AllGrantOrigins() []string {
	return []string{
		string(OriginUserDirect), string(OriginAgentProvided),
		string(OriginUIGenerated), string(OriginModelInterpreted),
		string(OriginRuntimeInferred),
	}
}

// Valid reports whether g is one of the closed set of grant origins.
func (g GrantOrigin) Valid() bool {
	switch g {
	case OriginUserDirect, OriginAgentProvided,
		OriginUIGenerated, OriginModelInterpreted,
		OriginRuntimeInferred:
		return true
	}
	return false
}

// AuthorityLink is one step in the recorded chain of how authority (or
// an intent modification) entered the system. Origin is required: a
// link without a named source is a malformed record, not an absent
// default. Untrusted carries the sticky propagation mark for this step.
type AuthorityLink struct {
	Origin    GrantOrigin `json:"origin"`
	Untrusted bool        `json:"untrusted,omitempty"`
}

// Validate rejects wild origin tokens and asserts the sticky property in
// the record shape: a chain that contains an untrusted link carries the
// chain-level untrusted mark itself (propagation is recorded eagerly;
// nothing consumes it yet).
func (l AuthorityLink) Validate() error {
	if !l.Origin.Valid() {
		return fmt.Errorf("authority link origin %q is outside the closed set", string(l.Origin))
	}
	return nil
}

// AuthorityChain records the grant path for one intent record.
type AuthorityChain struct {
	Links     []AuthorityLink `json:"links,omitempty"`
	Untrusted bool            `json:"untrusted,omitempty"`
}

// Validate walks the chain: every link passes the closed set, and an
// untrusted link forces the chain-level mark (sticky propagation in
// record form). A chain that claims untrusted while every link is
// trusted is accepted: the mark may also come from context a later
// slice records; the direction that is forbidden is the silent one
// (untrusted link with the chain mark cleared).
func (c AuthorityChain) Validate() error {
	for _, l := range c.Links {
		if err := l.Validate(); err != nil {
			return err
		}
		if l.Untrusted && !c.Untrusted {
			return fmt.Errorf("untrusted link %q does not propagate to the chain mark", string(l.Origin))
		}
	}
	return nil
}

// intentFieldWireNames lists the ten wire tokens of the intent contract
// in normative order, derived mechanically (lower-case, spaces to single
// underscores) from the verbatim anchor block in docs/schema-v2.md. The
// count and the mapping are machine-asserted on every run; nobody
// recounts them by hand.
var intentFieldWireNames = []string{
	"goal", "scope", "expected_outcome", "expected_actions",
	"allowed_resources", "sensitive_resources", "forbidden_scope",
	"authority", "duration", "constraints",
}

// AllIntentFields returns the ten-field vocabulary in normative order.
func AllIntentFields() []string {
	out := make([]string, len(intentFieldWireNames))
	copy(out, intentFieldWireNames)
	return out
}

// IntentRecord is the structured statement of what a user task is meant
// to achieve. Every field is optional with an absent-key default that
// means exactly one thing: not reported. An absent field is a known
// gap; later slices may derive an inferred shape from observed behavior,
// but no code path may fabricate a value for a field the agent did not
// report.
type IntentRecord struct {
	Goal               string          `json:"goal,omitempty"`
	Scope              string          `json:"scope,omitempty"`
	ExpectedOutcome    string          `json:"expected_outcome,omitempty"`
	ExpectedActions    []string        `json:"expected_actions,omitempty"`
	AllowedResources   []string        `json:"allowed_resources,omitempty"`
	SensitiveResources []string        `json:"sensitive_resources,omitempty"`
	ForbiddenScope     string          `json:"forbidden_scope,omitempty"`
	Authority          *AuthorityChain `json:"authority,omitempty"`
	Duration           string          `json:"duration,omitempty"`
	Constraints        []string        `json:"constraints,omitempty"`
}

// ParseIntentRecord decodes one intent record, rejecting any field name
// outside the closed ten-token vocabulary (wild-shape rejection before
// any value is trusted) and validating the authority chain when
// present. On any error it returns a nil record: nothing downstream of
// a rejection ever holds a usable value, matching the zero-byte
// rejection shape the audit writer already uses for invalid events.
func ParseIntentRecord(data []byte) (*IntentRecord, error) {
	dec := json.NewDecoder(bytes.NewReader(data))
	dec.DisallowUnknownFields()
	var rec IntentRecord
	if err := dec.Decode(&rec); err != nil {
		return nil, err
	}
	if rec.Authority != nil {
		if err := rec.Authority.Validate(); err != nil {
			return nil, err
		}
	}
	return &rec, nil
}

// EncodeChecked validates the embedded authority chain before
// serializing and returns nil bytes on any rejection: a record that
// fails the contract produces no output at all.
func EncodeChecked(rec *IntentRecord) ([]byte, error) {
	if rec == nil {
		return nil, fmt.Errorf("nil intent record")
	}
	if rec.Authority != nil {
		if err := rec.Authority.Validate(); err != nil {
			return nil, err
		}
	}
	return json.Marshal(rec)
}
