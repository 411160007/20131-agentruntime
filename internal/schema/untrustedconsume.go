package schema

// UNTRUSTED-mark consumption point for the intent-contract wave
// (slice W3.3). The owner specification's intent-contract section ends
// with one normative sentence: intent modifications coming from
// untrusted content must be treated as UNTRUSTED and must never
// auto-raise user authority. Slice W2.2 contracted the record shape
// for that rule (the sticky AuthorityChain mark, carried but not
// consumed). This slice is the RECORD-FORM CONSUMER: a pure function
// that folds one proposed intent modification into one intent record,
// applying the mark, preservation, forgery-rejection, and stickiness
// rules at record time.
//
// Like every other contract in this package this is an observation-
// phase shape only. Nothing in the evaluator, the rule engine, the
// bus, the audit writer, or the command tree calls
// ApplyIntentModification; wiring it into a runtime path (including
// the fail-open versus fail-closed posture question for enforcement)
// is a later-phase decision, never a silent one. docs/schema-v2.md
// section 13 is the human-facing contract; the sync test in this
// package pins the two sources verbatim, and the Node structural
// checker mirrors the five contract keys from the other side.

import (
	"fmt"
)

// The five recorded rule strings of the consumption contract. Each
// constant here, its docs key line, and the Node checker predicate
// must agree verbatim; drift is a sync-test failure, not a comment.
const (
	// UntrustedConsumeRule is the consumption reading of the spec
	// sentence: an untrusted-source modification is applied as a
	// record and carries the UNTRUSTED mark; it never stands as an
	// authority escalation on its own.
	UntrustedConsumeRule = "untrusted-source-modifications-are-recorded-marked-never-auto-escalate"

	// AuthorityPreservationRule: the links already present on the
	// base chain are copied through byte-identically (same order,
	// same origins, same trusted bits). The consumption path can
	// only append a marked link; it can never rewrite, reorder, or
	// drop an existing grant record.
	AuthorityPreservationRule = "original-chain-links-preserved-identical"

	// OriginForgeRule: untrusted content may never name itself as
	// the user. A modification that arrives marked untrusted while
	// claiming the user_direct origin is rejected before any field
	// value is applied - the forge shape raises nothing.
	OriginForgeRule = "untrusted-source-never-claims-user_direct"

	// StickyClearanceRule: consumption never clears an existing
	// chain-level UNTRUSTED mark. A later trusted modification does
	// not launder an earlier untrusted one; clearing the mark is a
	// human-side (user) action outside this path, matching the
	// W2.2 propagation direction that only escalation-silence is
	// structurally forbidden here.
	StickyClearanceRule = "consumption-never-clears-an-existing-untrusted-mark"

	// ConsumeEnforcementPlane names the (currently empty) plane
	// that would act on the mark at decision time. In the
	// observation phase the mark is recorded and propagated, never
	// consumed by any decision path; the open posture questions
	// (fail-open versus fail-closed at enforcement time) stay
	// deliberately undecided, exactly as section W2.2 left them.
	ConsumeEnforcementPlane = "none-in-observation-phase"
)

// IntentModification is one proposed field change to an intent
// record, carrying the trust classification and the grant-origin
// label of the content that produced it. Field must be one of the
// ten closed intent wire tokens; the authority token is rejected by
// the consumption path itself (see ApplyIntentModification), so a
// nine-writable plus one-forbidden split is the whole vocabulary.
// SourceTrusted false means the modification came from untrusted
// content (hook payloads, tool output, model interpretation, or any
// other low layer); it is a property of the channel, recorded at
// consumption time, never inferred from the payload's own claims.
type IntentModification struct {
	Field         string      `json:"field"`
	NewValue      string      `json:"new_value"`
	SourceOrigin  GrantOrigin `json:"source_origin"`
	SourceTrusted bool        `json:"source_trusted"`
}

// isIntentField reports whether f is one of the ten closed intent
// wire tokens, reusing the single normative list - no second
// vocabulary may exist.
func isIntentField(f string) bool {
	for _, t := range intentFieldWireNames {
		if t == f {
			return true
		}
	}
	return false
}

// Validate rejects a wild field token and a wild origin token before
// anything downstream can look at the value.
func (m IntentModification) Validate() error {
	if !isIntentField(m.Field) {
		return fmt.Errorf("intent modification field %q is outside the closed ten-token set", m.Field)
	}
	if !m.SourceOrigin.Valid() {
		return fmt.Errorf("intent modification origin %q is outside the closed set", string(m.SourceOrigin))
	}
	return nil
}

// ApplyIntentModification is the consumption point. Contract (all
// four rules above, machine-tested):
//
//   - The base record is never mutated; the result is a fresh record.
//   - A modification naming the authority field is rejected outright:
//     grants travel as chain links appended in time order, never as
//     a field overwrite, so the original authority value is
//     preserved by construction (no overwrite path exists to audit).
//   - An untrusted-source modification claiming user_direct origin
//     is rejected before any value is applied (OriginForgeRule).
//   - Otherwise the named field is updated (scalar fields replaced,
//     list fields appended with a fresh backing copy) and a link
//     {origin, untrusted} is appended to a byte-identical copy of
//     the existing chain. The chain-level mark is set for an
//     untrusted source and is never cleared for a trusted one.
//   - Any rejection returns a nil record: nothing downstream of a
//     red shape ever holds a usable value, matching the parse and
//     encode rejection forms of W2.2.
func ApplyIntentModification(rec *IntentRecord, mod IntentModification) (*IntentRecord, error) {
	if rec == nil {
		return nil, fmt.Errorf("nil intent record")
	}
	if rec.Authority != nil {
		if err := rec.Authority.Validate(); err != nil {
			return nil, fmt.Errorf("base record fails its own contract: %w", err)
		}
	}
	if err := mod.Validate(); err != nil {
		return nil, err
	}
	if mod.Field == "authority" {
		return nil, fmt.Errorf("authority field is not reachable through the consumption path: %s", AuthorityPreservationRule)
	}
	if !mod.SourceTrusted && mod.SourceOrigin == OriginUserDirect {
		return nil, fmt.Errorf("%s: untrusted content claiming user origin is rejected before any value is applied", OriginForgeRule)
	}

	out := *rec
	var oldLinks []AuthorityLink
	if rec.Authority != nil {
		oldLinks = rec.Authority.Links
	}
	links := make([]AuthorityLink, 0, len(oldLinks)+1)
	links = append(links, oldLinks...)
	links = append(links, AuthorityLink{Origin: mod.SourceOrigin, Untrusted: !mod.SourceTrusted})
	chain := &AuthorityChain{Links: links}
	if rec.Authority != nil && rec.Authority.Untrusted {
		chain.Untrusted = true // sticky: an existing mark is carried, never laundered
	}
	if !mod.SourceTrusted {
		chain.Untrusted = true // fresh untrusted source marks the chain (W2.2 propagation direction)
	}
	out.Authority = chain

	if err := setIntentField(&out, mod.Field, mod.NewValue); err != nil {
		return nil, err
	}
	if err := chain.Validate(); err != nil {
		return nil, err
	}
	return &out, nil
}

// setIntentField is the single writer for the nine writable intent
// fields (the tenth, authority, is rejected upstream). Scalar fields
// replace; list fields append behind a fresh allocation so the base
// record's backing arrays are never shared or touched.
func setIntentField(rec *IntentRecord, field, value string) error {
	switch field {
	case "goal":
		rec.Goal = value
	case "scope":
		rec.Scope = value
	case "expected_outcome":
		rec.ExpectedOutcome = value
	case "forbidden_scope":
		rec.ForbiddenScope = value
	case "duration":
		rec.Duration = value
	case "expected_actions":
		rec.ExpectedActions = append(append([]string{}, rec.ExpectedActions...), value)
	case "allowed_resources":
		rec.AllowedResources = append(append([]string{}, rec.AllowedResources...), value)
	case "sensitive_resources":
		rec.SensitiveResources = append(append([]string{}, rec.SensitiveResources...), value)
	case "constraints":
		rec.Constraints = append(append([]string{}, rec.Constraints...), value)
	default:
		return fmt.Errorf("intent field %q has no consumption writer", field)
	}
	return nil
}
