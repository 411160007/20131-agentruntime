package schema

import (
	"fmt"
	"strings"
)

// Decision cache observation shape (spec v2 section 267, slice W9.4
// record surface): section 267 constrains any future decision cache
// to a context-bound key and forbids one grant being reused
// indefinitely across unrelated scenes, requiring invalidation when
// the key context changes. Phase 0 has no cache and no decisions to
// cache - every decision is recomputed (the decision-zero-state
// continuation the taskbook pins) - so this slice lands only:
//
//   - the cache-key shape with the section 267 eight context
//     constraints in spec order (agent, task, resource, capability,
//     policy version, context, ttl, confidence) plus the
//     context_epoch invalidation slot the taskbook names as the
//     construction field (context_epoch form);
//   - an observation record reusing the section 261 counter source
//     already shipped in slice W6.1 (event rate + per-agent /
//     per-task scopes) for entry counts and hit-rate observations -
//     no second collector may start here;
//   - constructor-pinned restatements no caller can edit: the
//     record-only stance carrying the pre-borrowed section 267
//     rules (finite TTL mandatory, context change invalidates,
//     Phase 0 hits identically zero) and the enforcement-plane
//     token, pinned to the single shipped spelling
//     none-in-observation-phase every wave record already uses.
//
// What the shape never does: consult a store, return a verdict,
// short-circuit a computation, or hold runtime state. The
// invalidation helpers are pure comparisons over stated values. The
// hit count is a recorded observation, and in Phase 0 a nonzero hit
// is not recordable at all: a hit would assert that some decision
// was answered from memory instead of recomputed, which this phase
// declares structurally impossible (positive-control pin - hits
// present, construction red). The hit-rate line follows the shipped
// honest-absence discipline: with no consultations the rate renders
// the explicit absent token, never a fabricated zero.
//
// docs/schema-v2.md section 31 is the human-facing contract. The
// record is not an EventType and enters no decision-plane file
// (policy, rules, bus, auditlog) while the cache plane reads
// none-in-observation-phase; the three-gate and gate fences of the
// wave tail count the ```cache word family with them.

// DecisionCacheKind is the fixed kind marker of one observation.
const DecisionCacheKind = "decision.cache"

// DecisionCache key bounds. TTL is mandatory and finite: section 267
// forbids indefinite reuse, so a non-positive TTL (the "once allowed,
// forever valid" shape) fails construction, and the outer bound
// states the recording-side ceiling - no recorded context binding is
// admissible past it. The bound is a record grammar limit, not a
// policy value: nothing consumes it as a decision input.
const (
	DecisionCacheMaxTTLSeconds = 2592000 // thirty days, record grammar bound
	DecisionCacheMaxConfidence = 100     // percent scale, closed upper end
)

// DecisionCacheRateAbsent is the explicit token for "no hit rate can
// be stated because nothing was consulted": absence is stated, never
// smuggled in as a number.
const DecisionCacheRateAbsent = "absent"

// The two constructor-pinned restatements. They are values, not
// fields a caller supplies.
const (
	DecisionCacheRecordStance = "decision_cache_observation: record-only, zero enforcement plane; in Phase 0 every decision is recomputed and the cache is never consulted, a nonzero hit count is not recordable; no key survives a changed context binding or a stale epoch, and indefinite reuse is not recordable (pre-borrowed section 267 discipline, not wired in this phase)"
	DecisionCacheEnforcement  = "none-in-observation-phase"
)

// decisionCacheKeyFieldNames lists the section 267 eight constraints
// in spec order. The declaration order of DecisionCacheKey, the
// section 31 anchor bullets, and any later checker mirror derive the
// wire tokens from it and fail on drift in either direction. The
// epoch slot is named separately because section 267 lists eight
// constraints while the taskbook construction adds the ninth
// invalidation field; conflating them would misquote the spec.
var decisionCacheKeyFieldNames = []string{
	"agent", "task", "resource", "capability",
	"policy_version", "context", "ttl_seconds", "confidence_percent",
}

// AllDecisionCacheKeyFields returns the eight constraint tokens in
// normative order.
func AllDecisionCacheKeyFields() []string {
	out := make([]string, len(decisionCacheKeyFieldNames))
	copy(out, decisionCacheKeyFieldNames)
	return out
}

// DecisionCacheKey is one section 267 context-bound cache key shape.
// Field order is the wire order and is part of the contract: the
// first eight fields are the spec's constraints verbatim in spec
// order, and context_epoch is the taskbook's named invalidation slot
// (context_epoch form) carried beside them.
type DecisionCacheKey struct {
	Agent         string `json:"agent"`
	Task          string `json:"task"`
	Resource      string `json:"resource"`
	Capability    string `json:"capability"`
	PolicyVersion int    `json:"policy_version"`
	Context       string `json:"context"`
	TTLSeconds    int    `json:"ttl_seconds"`
	Confidence    int    `json:"confidence_percent"`
	ContextEpoch  uint64 `json:"context_epoch"`
}

// Validate checks the key grammar: every identifier shaped slot
// holds a valid id, the version and epoch are at least one, the
// context statement is non-blank text, the TTL is finite and inside
// the record bound, and confidence is a stated percent. A key that
// does not validate is a shape error, never a partial key.
func (k DecisionCacheKey) Validate() error {
	for _, req := range []struct {
		name  string
		value string
	}{
		{"agent", k.Agent},
		{"task", k.Task},
		{"resource", k.Resource},
		{"capability", k.Capability},
	} {
		if !validID(req.value) {
			return fmt.Errorf("decision cache key: %q %q is not a valid identifier", req.name, req.value)
		}
	}
	if strings.TrimSpace(k.Context) == "" {
		return fmt.Errorf("decision cache key: context must state text (an unbounded context is exactly the reuse shape section 267 forbids)")
	}
	if k.PolicyVersion < 1 {
		return fmt.Errorf("decision cache key: policy_version must be >= 1, got %d", k.PolicyVersion)
	}
	if k.ContextEpoch < 1 {
		return fmt.Errorf("decision cache key: context_epoch must be >= 1 (the epoch slot starts at one; zero is the unset shape), got %d", k.ContextEpoch)
	}
	if k.TTLSeconds <= 0 {
		return fmt.Errorf("decision cache key: ttl_seconds must be finite and positive - indefinite reuse is the forbidden shape")
	}
	if k.TTLSeconds > DecisionCacheMaxTTLSeconds {
		return fmt.Errorf("decision cache key: ttl_seconds %d exceeds the record bound %d", k.TTLSeconds, DecisionCacheMaxTTLSeconds)
	}
	if k.Confidence < 0 || k.Confidence > DecisionCacheMaxConfidence {
		return fmt.Errorf("decision cache key: confidence_percent must be within 0..%d, got %d", DecisionCacheMaxConfidence, k.Confidence)
	}
	return nil
}

// contextBinding returns the tuple section 267 calls the key
// context: the identity dimensions plus the epoch slot. TTL and
// confidence are properties of the recorded entry, not of the
// context; two keys with the same binding differ only in entry
// properties, and any change to the binding is a context change.
func (k DecisionCacheKey) contextBinding() [7]string {
	return [7]string{k.Agent, k.Task, k.Resource, k.Capability, k.Context,
		fmt.Sprintf("pv%d", k.PolicyVersion), fmt.Sprintf("e%d", k.ContextEpoch)}
}

// SameContextBinding reports whether two keys bind the same
// context. It is a pure comparison over stated values - it consults
// no store and caches nothing.
func (k DecisionCacheKey) SameContextBinding(other DecisionCacheKey) bool {
	a, b := k.contextBinding(), other.contextBinding()
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

// StaleAtEpoch reports whether a key recorded at its own epoch is
// stale against the epoch now stated by the caller's lineage.
// Equality is the only fresh answer: a later epoch means the
// context moved on (invalidate), an earlier epoch is a rewind the
// monotonic lineage does not admit (stale as well - the record
// refuses to bless it as fresh). Pure comparison over stated
// values; nothing here holds state.
func (k DecisionCacheKey) StaleAtEpoch(current uint64) bool {
	return current != k.ContextEpoch
}

// DecisionCacheObservation is one record-side sighting of the cache
// surface for a subject under the shipped section 261 counter
// source. It counts what a replay or trace shows; it never drives
// anything.
type DecisionCacheObservation struct {
	Kind             string             `json:"kind"`
	CounterKind      AgencyCounterKind  `json:"counter_kind"`
	Scope            AgencyCounterScope `json:"scope"`
	SubjectID        string             `json:"subject_id"`
	Key              DecisionCacheKey   `json:"key"`
	EntriesObserved  int                `json:"entries_observed"`
	Consultations    int                `json:"consultations_observed"`
	HitsRecorded     int                `json:"hits_recorded"`
	Invalidated      int                `json:"invalidated_by_context_change"`
	HitRatePercent   string             `json:"hit_rate_percent"`
	Stance           string             `json:"stance"`
	EnforcementPlane string             `json:"decision_cache_enforcement_plane"`
}

// DecisionCacheObservationInput is what a caller supplies; the kind
// marker, the pinned restatements, and the derived rate line are not
// among it.
type DecisionCacheObservationInput struct {
	CounterKind   AgencyCounterKind
	Scope         AgencyCounterScope
	SubjectID     string
	Key           DecisionCacheKey
	Entries       int
	Consultations int
	Hits          int
	Invalidated   int
}

// decisionCacheHitRate derives the honest rate line: no
// consultations render the explicit absent token; otherwise the
// integer percent of hits over consultations is computed from the
// stated counts (record-side arithmetic only - it is a reading, not
// a verdict).
func decisionCacheHitRate(consultations, hits int) string {
	if consultations == 0 {
		return DecisionCacheRateAbsent
	}
	return fmt.Sprintf("%d", hits*100/consultations)
}

// Validate checks the record grammar on a hand-built record too, so
// Build only pins values and Validate does all the judging. The
// Phase 0 pins: the counter source must be the shipped W6.1 kind
// under a shipped scope (a new counter name is exactly the second
// collector section 261 warned against), the key validates as a
// shape, counts are non-negative, hits are identically zero, and the
// rate line equals what the counts derive.
func (o *DecisionCacheObservation) Validate() error {
	if o.Kind != DecisionCacheKind {
		return fmt.Errorf("decision cache record: kind %q invalid", o.Kind)
	}
	if o.CounterKind != CounterEventRate {
		return fmt.Errorf("decision cache record: counter_kind must reuse the shipped event-rate source, got %q", string(o.CounterKind))
	}
	if !o.Scope.Valid() {
		return fmt.Errorf("decision cache record: scope %q is outside the shipped per-agent / per-task pair", string(o.Scope))
	}
	if !validID(o.SubjectID) {
		return fmt.Errorf("decision cache record: subject_id %q is not a valid identifier", o.SubjectID)
	}
	if err := o.Key.Validate(); err != nil {
		return err
	}
	for _, c := range []struct {
		name  string
		value int
	}{
		{"entries_observed", o.EntriesObserved},
		{"consultations_observed", o.Consultations},
		{"hits_recorded", o.HitsRecorded},
		{"invalidated_by_context_change", o.Invalidated},
	} {
		if c.value < 0 {
			return fmt.Errorf("decision cache record: %s must be non-negative, got %d", c.name, c.value)
		}
	}
	if o.HitsRecorded != 0 {
		return fmt.Errorf("decision cache record: hits_recorded must be zero in Phase 0 - every decision is recomputed and a nonzero sighting belongs to a later phase's re-examination, not to this record")
	}
	if o.Consultations > 0 && o.HitsRecorded > o.Consultations {
		return fmt.Errorf("decision cache record: hits cannot exceed consultations")
	}
	if want := decisionCacheHitRate(o.Consultations, o.HitsRecorded); o.HitRatePercent != want {
		return fmt.Errorf("decision cache record: hit_rate_percent %q does not match the derived line %q (absence is stated, never invented)", o.HitRatePercent, want)
	}
	if o.Stance != DecisionCacheRecordStance {
		return fmt.Errorf("decision cache record: stance must be the pinned record-only line, got %q", o.Stance)
	}
	if o.EnforcementPlane != DecisionCacheEnforcement {
		return fmt.Errorf("decision cache record: decision_cache_enforcement_plane must be the shipped none-in-observation-phase token, got %q", o.EnforcementPlane)
	}
	return nil
}

// BuildDecisionCacheKey produces one validated key or the zero
// value: every rejection returns no key, never a half-filled shape.
func BuildDecisionCacheKey(in DecisionCacheKey) (DecisionCacheKey, error) {
	if err := in.Validate(); err != nil {
		return DecisionCacheKey{}, err
	}
	return in, nil
}

// BuildDecisionCacheObservation produces one validated record or no
// record at all. Counters that were never observed are absent by
// construction being impossible (an input must state them); nothing
// here invents a sighting, and no caller can state a hit.
func BuildDecisionCacheObservation(in DecisionCacheObservationInput) (DecisionCacheObservation, error) {
	rec := DecisionCacheObservation{
		Kind:             DecisionCacheKind,
		CounterKind:      in.CounterKind,
		Scope:            in.Scope,
		SubjectID:        in.SubjectID,
		Key:              in.Key,
		EntriesObserved:  in.Entries,
		Consultations:    in.Consultations,
		HitsRecorded:     in.Hits,
		Invalidated:      in.Invalidated,
		HitRatePercent:   decisionCacheHitRate(in.Consultations, in.Hits),
		Stance:           DecisionCacheRecordStance,
		EnforcementPlane: DecisionCacheEnforcement,
	}
	if err := rec.Validate(); err != nil {
		return DecisionCacheObservation{}, err
	}
	return rec, nil
}
