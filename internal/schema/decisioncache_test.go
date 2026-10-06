package schema

import (
	"encoding/json"
	"strings"
	"testing"
)

func cacheKey() DecisionCacheKey {
	return DecisionCacheKey{
		Agent:         "agent.main",
		Task:          "task.42",
		Resource:      "file.readme",
		Capability:    "fs.read",
		PolicyVersion: 3,
		Context:       "workdir=/srv;stage=build",
		TTLSeconds:    3600,
		Confidence:    87,
		ContextEpoch:  5,
	}
}

func cacheObs() DecisionCacheObservationInput {
	return DecisionCacheObservationInput{
		CounterKind:   CounterEventRate,
		Scope:         ScopeAgent,
		SubjectID:     "agent.main",
		Key:           cacheKey(),
		Entries:       0,
		Consultations: 0,
		Hits:          0,
		Invalidated:   0,
	}
}

// TestDecisionCacheKeyEightConstraints pins the wire order against
// the section 267 list: the first eight fields are the spec's
// constraints verbatim in spec order, the epoch slot is the ninth.
func TestDecisionCacheKeyEightConstraints(t *testing.T) {
	raw, err := json.Marshal(cacheKey())
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	s := string(raw)
	prev := -1
	names := append(AllDecisionCacheKeyFields(), "context_epoch")
	for _, n := range names {
		needle := `"` + n + `":`
		i := strings.Index(s, needle)
		if i < 0 {
			t.Fatalf("field %q missing from wire shape", n)
		}
		if i <= prev {
			t.Fatalf("field %q out of normative order", n)
		}
		prev = i
	}
	if len(AllDecisionCacheKeyFields()) != 8 {
		t.Fatalf("section 267 lists eight constraints, registry says %d", len(AllDecisionCacheKeyFields()))
	}
	fresh := AllDecisionCacheKeyFields()
	fresh[0] = "tampered"
	if AllDecisionCacheKeyFields()[0] != "agent" {
		t.Fatal("the field registry must return a copy")
	}
}

// TestDecisionCacheKeyDeterministicBytes guards the golden shape:
// the same key encodes byte-identically across runs, so a later
// replay census can pin cache keys without a hand-copied snapshot.
func TestDecisionCacheKeyDeterministicBytes(t *testing.T) {
	a, err := json.Marshal(cacheKey())
	if err != nil {
		t.Fatalf("marshal a: %v", err)
	}
	b, err := json.Marshal(cacheKey())
	if err != nil {
		t.Fatalf("marshal b: %v", err)
	}
	if string(a) != string(b) {
		t.Fatal("key encoding drifted between runs")
	}
	if !strings.Contains(string(a), `"ttl_seconds":3600`) {
		t.Fatalf("ttl wire shape drifted: %s", a)
	}
}

func TestDecisionCacheKeyRejectsIndefiniteReuse(t *testing.T) {
	for _, ttl := range []int{0, -1, DecisionCacheMaxTTLSeconds + 1} {
		in := cacheKey()
		in.TTLSeconds = ttl
		if _, err := BuildDecisionCacheKey(in); err == nil {
			t.Fatalf("ttl %d must fail construction (section 267 forbids indefinite reuse)", ttl)
		}
	}
}

func TestDecisionCacheKeyRejectionsLeaveNoKey(t *testing.T) {
	cases := []struct {
		name string
		mut  func(*DecisionCacheKey)
	}{
		{"empty agent", func(k *DecisionCacheKey) { k.Agent = "" }},
		{"blank context", func(k *DecisionCacheKey) { k.Context = "   " }},
		{"zero policy version", func(k *DecisionCacheKey) { k.PolicyVersion = 0 }},
		{"unset epoch", func(k *DecisionCacheKey) { k.ContextEpoch = 0 }},
		{"confidence over hundred", func(k *DecisionCacheKey) { k.Confidence = 101 }},
		{"negative confidence", func(k *DecisionCacheKey) { k.Confidence = -1 }},
		{"punctuation id", func(k *DecisionCacheKey) { k.Task = "task/../etc" }},
	}
	for _, c := range cases {
		in := cacheKey()
		c.mut(&in)
		got, err := BuildDecisionCacheKey(in)
		if err == nil {
			t.Fatalf("%s: construction must reject", c.name)
		}
		if got != (DecisionCacheKey{}) {
			t.Fatalf("%s: rejection must return the zero value, never a half key", c.name)
		}
	}
}

// TestDecisionCacheContextInvalidation pins both invalidation
// shapes: a changed context dimension breaks the binding, and any
// epoch move (forward = context changed, backward = admissible
// nothing) reads stale. Equality is the only fresh answer.
func TestDecisionCacheContextInvalidation(t *testing.T) {
	base := cacheKey()
	if _, err := BuildDecisionCacheKey(base); err != nil {
		t.Fatalf("base: %v", err)
	}
	if !base.SameContextBinding(base) {
		t.Fatal("a key must bind its own context")
	}
	if base.StaleAtEpoch(base.ContextEpoch) {
		t.Fatal("equal epochs are not stale")
	}
	if !base.StaleAtEpoch(base.ContextEpoch + 1) {
		t.Fatal("a later epoch must read stale (context moved on)")
	}
	if !base.StaleAtEpoch(base.ContextEpoch - 1) {
		t.Fatal("a rewind is not fresh either (monotonic lineage)")
	}
	mutations := []func(*DecisionCacheKey){
		func(k *DecisionCacheKey) { k.Agent = "agent.other" },
		func(k *DecisionCacheKey) { k.Task = "task.43" },
		func(k *DecisionCacheKey) { k.Resource = "file.other" },
		func(k *DecisionCacheKey) { k.Capability = "fs.write" },
		func(k *DecisionCacheKey) { k.Context = "workdir=/tmp" },
		func(k *DecisionCacheKey) { k.PolicyVersion = 4 },
		func(k *DecisionCacheKey) { k.ContextEpoch = 6 },
	}
	for i, mut := range mutations {
		other := cacheKey()
		mut(&other)
		if base.SameContextBinding(other) {
			t.Fatalf("mutation %d must break the context binding", i)
		}
	}
	// TTL and confidence are entry properties, not context: two
	// keys differing only there still bind the same context.
	other := cacheKey()
	other.TTLSeconds = 60
	other.Confidence = 12
	if !base.SameContextBinding(other) {
		t.Fatal("ttl/confidence differences are not context changes")
	}
}

// TestDecisionCacheZeroHits is the Phase 0 positive-control pin: a
// nonzero hit sighting is not recordable, construction goes red.
func TestDecisionCacheZeroHits(t *testing.T) {
	in := cacheObs()
	in.Hits = 1
	if _, err := BuildDecisionCacheObservation(in); err == nil {
		t.Fatal("a nonzero hit count must fail Phase 0 construction")
	}
	in.Hits = 0
	in.Consultations = 4
	rec, err := BuildDecisionCacheObservation(in)
	if err != nil {
		t.Fatalf("zero-hit sighting with consultations must record: %v", err)
	}
	if rec.HitsRecorded != 0 || rec.HitRatePercent != "0" {
		t.Fatalf("derived rate wrong: %+v", rec)
	}
}

func TestDecisionCacheObservationRejections(t *testing.T) {
	cases := []struct {
		name string
		mut  func(*DecisionCacheObservationInput)
	}{
		{"second collector", func(in *DecisionCacheObservationInput) { in.CounterKind = AgencyCounterKind("cache_hits") }},
		{"unshipped scope", func(in *DecisionCacheObservationInput) { in.Scope = AgencyCounterScope("global") }},
		{"negative entries", func(in *DecisionCacheObservationInput) { in.Entries = -1 }},
		{"hits over consultations", func(in *DecisionCacheObservationInput) { in.Consultations = 0; in.Hits = 1 }},
		{"bad subject id", func(in *DecisionCacheObservationInput) { in.SubjectID = "" }},
		{"half key inside record", func(in *DecisionCacheObservationInput) { k := in.Key; k.TTLSeconds = 0; in.Key = k }},
	}
	for _, c := range cases {
		in := cacheObs()
		c.mut(&in)
		got, err := BuildDecisionCacheObservation(in)
		if err == nil {
			t.Fatalf("%s: construction must reject", c.name)
		}
		if got != (DecisionCacheObservation{}) {
			t.Fatalf("%s: rejection must return no record at all", c.name)
		}
	}
}

// TestDecisionCacheRateHonestAbsent pins the absence discipline:
// with no consultations the rate renders the explicit absent token,
// never a fabricated zero.
func TestDecisionCacheRateHonestAbsent(t *testing.T) {
	rec, err := BuildDecisionCacheObservation(cacheObs())
	if err != nil {
		t.Fatalf("build: %v", err)
	}
	if rec.HitRatePercent != DecisionCacheRateAbsent {
		t.Fatalf("no-consultation rate must be the absent token, got %q", rec.HitRatePercent)
	}
	// A hand-set fabricated "0" with zero consultations fails the
	// derived-line check.
	rec.HitRatePercent = "0"
	if err := rec.Validate(); err == nil {
		t.Fatal("a fabricated rate must fail validation")
	}
}

func TestDecisionCachePinsNotEditable(t *testing.T) {
	rec, err := BuildDecisionCacheObservation(cacheObs())
	if err != nil {
		t.Fatalf("build: %v", err)
	}
	rec.Stance = "decision_cache_observation: whatever"
	if err := rec.Validate(); err == nil {
		t.Fatal("a hand-mutated stance must fail validation")
	}
	rec, _ = BuildDecisionCacheObservation(cacheObs())
	rec.EnforcementPlane = "none-in-cache-phase"
	if err := rec.Validate(); err == nil {
		t.Fatal("a hand-mutated enforcement restatement must fail validation")
	}
}

// TestDecisionCacheZeroEnforcementVocabulary guards the record
// against reserved decision-plane words in its emitted bytes. The
// pinned stance spells its own discipline in prose; the check runs
// on wire values only (quoted tokens), never prose.
func TestDecisionCacheZeroEnforcementVocabulary(t *testing.T) {
	in := cacheObs()
	in.Consultations = 3
	in.Entries = 2
	rec, err := BuildDecisionCacheObservation(in)
	if err != nil {
		t.Fatalf("build: %v", err)
	}
	raw, err := json.Marshal(rec)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	s := string(raw)
	for _, banned := range []string{`"ask"`, `"deny"`, `"enforce"`, `"quarantine"`, `"rollback"`, `"effect"`, `"severity"`, `"grant"`, `"allow"`, `"block"`} {
		if strings.Contains(s, banned) {
			t.Fatalf("reserved or decision-plane vocabulary present in cache record: %s", banned)
		}
	}
	if !strings.Contains(s, DecisionCacheEnforcement) {
		t.Fatal("record must carry the shipped plane token")
	}
}
