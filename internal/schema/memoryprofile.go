package schema

import (
	"fmt"
	"strings"
)

// Memory provenance observation record (owner spec section 262,
// landing slice of the memory-safety design surface
// docs/memory-learning-safety-design.md): the design face promises
// "a typed surface a poisoning observation needs" and this file is
// that typed surface - a record shape, nothing more.
//
// Section 262 closes two sets that this record mirrors and nothing
// else. The six memory surfaces (Conversation Memory, Task Memory,
// Long-term Memory, Vector Store, Scratchpad, Cached Context) are a
// closed set, not an exemplar list, and every significant memory
// record carries at minimum the seven provenance attributes (Origin,
// Author, Timestamp, Trust, Scope, TTL, Integrity). The headline
// principle is pinned in form as well as in spirit: Memory is not
// Authorization. A memory entry that asserts the user already
// allowed something is evidence of an intent claim, never a grant;
// the real authority and the current policy have to be re-verified
// at the moment of use.
//
// What the record carries:
//
//   - the seven section 262 attributes, each stated by the recorder,
//     with Trust recorded exactly as stated and never derived: no
//     trust computation, clearance arithmetic, or memory-scoring
//     ships behind this field;
//   - the memory surface as a member of the closed six;
//   - the moment-of-use re-verification state as a member of the
//     closed three: recorded-reverified, recorded-not-reverified, or
//     explicitly unrecorded - silence is admissible only when the
//     silence itself is stated, never as a default;
//   - three constructor-pinned restatements (the memory-is-not-
//     authorization rule, the re-verification obligation, and the
//     Phase 0 enforcement-plane stance) plus the closed attribute
//     registry line, none of which any caller can edit.
//
// What it never carries: a decision, a score, a severity, an effect,
// or a grant. Detecting a poisoned memory record means comparing a
// quoted claim against live authority - that comparison is
// enforcement and sits behind the Phase 1 gate, and pre-borrowing it
// here would break the very rule this record exists to restate. The
// attribute values are free text precisely so that this slice claims
// no matching power over real authority or policy state. Absence is
// single-meaning throughout: an empty attribute never builds, a
// surface outside the closed six is rejected, and a rejected input
// leaves no half record behind.
//
// docs/schema-v2.md section 41 is the human-facing contract; the Node
// checker mirrors its six key lines and both closed-set anchor bullet
// blocks against this file; no decision-plane (policy, rules, bus,
// auditlog) or command-tree file may reference the symbols below
// while memory_enforcement_plane reads none-in-observation-phase (the
// planeLeak needle set scans for exactly that).

// MemorySurface names one member of the section 262 closed set of
// memory surfaces. Declaration order is the spec's listing order and
// is normative: the section 41 anchor bullets and the Node checker
// derive the wire tokens from it and fail on drift in either
// direction. The set is closed by the spec, so a seventh surface is
// a contract change, not a constant to add.
type MemorySurface string

// The six memory surfaces.
const (
	SurfaceConversationMemory MemorySurface = "conversation_memory"
	SurfaceTaskMemory         MemorySurface = "task_memory"
	SurfaceLongTermMemory     MemorySurface = "long_term_memory"
	SurfaceVectorStore        MemorySurface = "vector_store"
	SurfaceScratchpad         MemorySurface = "scratchpad"
	SurfaceCachedContext      MemorySurface = "cached_context"
)

// memorySurfaceWireNames lists the six surface tokens in normative
// (spec) order. AllMemorySurfaces mirrors it; nobody recounts them by
// hand.
var memorySurfaceWireNames = []string{
	"conversation_memory", "task_memory", "long_term_memory",
	"vector_store", "scratchpad", "cached_context",
}

// AllMemorySurfaces returns the six surface tokens in spec order.
// The count and the order are machine-asserted on every run.
func AllMemorySurfaces() []string {
	out := make([]string, len(memorySurfaceWireNames))
	copy(out, memorySurfaceWireNames)
	return out
}

// Valid reports whether s is a member of the closed six-surface
// vocabulary. The empty string is not a surface: a record that never
// stated which memory surface it describes fails construction, it
// does not quietly become a wildcard member.
func (s MemorySurface) Valid() bool {
	if s == "" {
		return false
	}
	for _, w := range memorySurfaceWireNames {
		if string(s) == w {
			return true
		}
	}
	return false
}

// MemoryReverificationState is the closed tri-state of section 262's
// moment-of-use re-verification obligation over one recorded memory
// claim. reverified and not_reverified are findings over what was
// recorded; unrecorded is the honest explicit shape when the
// obligation's fulfilment was never observed. A memory record never
// launders "nothing recorded" into a clean bill of health, and it
// never launders it into a suspicion either: both would be a
// judgement, and judgements are not this record's plane.
type MemoryReverificationState string

// The three re-verification states in normative order.
const (
	ReverificationDone       MemoryReverificationState = "reverified"
	ReverificationSkipped    MemoryReverificationState = "not_reverified"
	ReverificationUnrecorded MemoryReverificationState = "unrecorded"
)

// AllMemoryReverificationStates lists the three states in normative
// order.
func AllMemoryReverificationStates() []string {
	return []string{
		string(ReverificationDone), string(ReverificationSkipped),
		string(ReverificationUnrecorded),
	}
}

// Valid reports whether st is one of the three states. The empty
// string is not a state: callers must state the silence explicitly as
// unrecorded.
func (st MemoryReverificationState) Valid() bool {
	if st == "" {
		return false
	}
	for _, w := range AllMemoryReverificationStates() {
		if string(st) == w {
			return true
		}
	}
	return false
}

// Rule constants pinned verbatim against the six-key machine block in
// docs/schema-v2.md section 41.
const (
	// MemoryNotAuthorizationRule pins section 262's headline rule: a
	// claim quoted from memory carries no authorization by virtue of
	// being in memory. The record restates the rule; it cannot
	// enforce it from inside Phase 0.
	MemoryNotAuthorizationRule = "memory-claim-never-carries-authorization"
	// MemoryProvenanceAttributeRegistry lists, in section 262 bullet
	// order, the seven attributes every significant memory record
	// carries at minimum. It is recorded as a closed registry line,
	// not as live checks: running those checks is enforcement.
	MemoryProvenanceAttributeRegistry = "origin,author,timestamp,trust,scope,ttl,integrity"
	// MemoryReverificationObligationRule pins section 262's operating
	// consequence: authority and policy are re-verified at the moment
	// of use. Who says that happened, when, and how is Phase 1's
	// shape; this record only states the outcome as reported.
	MemoryReverificationObligationRule = "authority-and-policy-reverified-at-moment-of-use"
	// MemoryEnforcementPlane pins the Phase 0 stance: this is an
	// observation record; no plane borrows it to decide, block,
	// grant, or distrust anything yet.
	MemoryEnforcementPlane = "none-in-observation-phase"
)

// MemoryProvenanceInput is the recorder's side of one memory record:
// the seven section 262 attributes plus the surface and the stated
// moment-of-use re-verification outcome. Every attribute is required
// and free text is deliberately not validated against any live
// vocabulary - matching a quoted memory claim against real authority
// or policy state would pre-borrow the Phase 1 posture this slice
// freezes. The struct carries no decision, score, severity, or effect
// field and the reflective test pins that.
type MemoryProvenanceInput struct {
	Origin         string                    `json:"origin"`
	Author         string                    `json:"author"`
	Timestamp      string                    `json:"timestamp"`
	Trust          string                    `json:"trust"`
	Scope          string                    `json:"scope"`
	TTL            string                    `json:"ttl"`
	Integrity      string                    `json:"integrity"`
	Surface        MemorySurface             `json:"surface"`
	Reverification MemoryReverificationState `json:"reverification"`
}

// MemoryProvenanceFieldWireNames is the normative field list of the
// full record (the nine recorder-supplied lines plus the two
// constructor-pinned restatements), in this order. The section 41
// field table and the Node checker pin it row by row: the seven
// attribute tokens are derived mechanically from the spec's own
// attribute bullets, so a rename in either direction is a red build.
var MemoryProvenanceFieldWireNames = []string{
	"origin", "author", "timestamp", "trust", "scope", "ttl", "integrity",
	"surface", "reverification", "memory_is_not_authorization", "enforcement_plane",
}

// MemoryProvenance is the row BuildMemoryProvenance produces: the
// input restated verbatim plus the constants no caller can edit. It
// is a record of one memory claim's stated provenance, not a
// judgement about whether that claim is poisoned.
type MemoryProvenance struct {
	Origin                 string                    `json:"origin"`
	Author                 string                    `json:"author"`
	Timestamp              string                    `json:"timestamp"`
	Trust                  string                    `json:"trust"`
	Scope                  string                    `json:"scope"`
	TTL                    string                    `json:"ttl"`
	Integrity              string                    `json:"integrity"`
	Surface                MemorySurface             `json:"surface"`
	Reverification         MemoryReverificationState `json:"reverification"`
	MemoryNotAuthorization string                    `json:"memory_is_not_authorization"`
	EnforcementPlane       string                    `json:"enforcement_plane"`
}

// requireText rejects the empty and whitespace-only shapes of a
// section 262 must-carry attribute. "At minimum carries" leaves no
// third option: an attribute that cannot be stated aborts the whole
// record rather than degrading it into a partial one.
func requireMemoryText(field, value string) error {
	if strings.TrimSpace(value) == "" {
		return fmt.Errorf("memory provenance: attribute %q must carry text (section 262 minimum set)", field)
	}
	return nil
}

// BuildMemoryProvenance validates one memory input and projects the
// record. Every rejection fires before any value is applied: a
// missing or whitespace-only attribute, a surface outside the closed
// six, or a re-verification state outside the closed three leaves no
// record behind. The constructor never judges: it restates what the
// recorder said and appends the pinned rules verbatim.
func BuildMemoryProvenance(in MemoryProvenanceInput) (MemoryProvenance, error) {
	for _, req := range []struct {
		name  string
		value string
	}{
		{"origin", in.Origin},
		{"author", in.Author},
		{"timestamp", in.Timestamp},
		{"trust", in.Trust},
		{"scope", in.Scope},
		{"ttl", in.TTL},
		{"integrity", in.Integrity},
	} {
		if err := requireMemoryText(req.name, req.value); err != nil {
			return MemoryProvenance{}, err
		}
	}
	if !in.Surface.Valid() {
		return MemoryProvenance{}, fmt.Errorf("memory provenance: surface %q is outside the closed six", string(in.Surface))
	}
	if !in.Reverification.Valid() {
		return MemoryProvenance{}, fmt.Errorf("memory provenance: reverification state %q is outside the closed three (state the silence explicitly as unrecorded)", string(in.Reverification))
	}
	return MemoryProvenance{
		Origin:                 in.Origin,
		Author:                 in.Author,
		Timestamp:              in.Timestamp,
		Trust:                  in.Trust,
		Scope:                  in.Scope,
		TTL:                    in.TTL,
		Integrity:              in.Integrity,
		Surface:                in.Surface,
		Reverification:         in.Reverification,
		MemoryNotAuthorization: MemoryNotAuthorizationRule,
		EnforcementPlane:       MemoryEnforcementPlane,
	}, nil
}
