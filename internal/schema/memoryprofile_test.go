package schema

import (
	"os"
	"path/filepath"
	"reflect"
	"regexp"
	"strings"
	"testing"
)

// vectorStoreInput is the machine-judgment fixture shape: a cached
// claim read back out of a vector store, carrying all section 262
// attributes plus the surface and the stated re-verification outcome.
func vectorStoreInput() MemoryProvenanceInput {
	return MemoryProvenanceInput{
		Origin:         "retrieved chunk quoted during planning",
		Author:         "unknown third-party document",
		Timestamp:      "stored at the recorded ingest time, stated not derived",
		Trust:          "as stated by the recorder, never computed",
		Scope:          "project:docs-index",
		TTL:            "stated as 30d, no expiry arithmetic in this slice",
		Integrity:      "sha256 of the stored chunk as reported",
		Surface:        SurfaceVectorStore,
		Reverification: ReverificationUnrecorded,
	}
}

// TestMemorySurfaceVocabularyPin pins the closed six-surface
// vocabulary: count, spec order, uniqueness, membership, and the
// wild-value rejection. The order literal is a third recount of the
// section 262 listing (Go, docs, Node checker must all agree).
func TestMemorySurfaceVocabularyPin(t *testing.T) {
	want := []string{
		"conversation_memory", "task_memory", "long_term_memory",
		"vector_store", "scratchpad", "cached_context",
	}
	got := AllMemorySurfaces()
	if len(got) != 6 {
		t.Fatalf("surface census drifted: %d", len(got))
	}
	if strings.Join(got, ",") != strings.Join(want, ",") {
		t.Fatalf("surface order drifted from the spec listing:\n got %v\nwant %v", got, want)
	}
	if len(AllMemoryReverificationStates()) != 3 {
		t.Fatal("reverification state census is not three")
	}
	for _, w := range want {
		if !MemorySurface(w).Valid() {
			t.Errorf("vocabulary member %q rejected by Valid", w)
		}
	}
	for _, wild := range []MemorySurface{"", "longterm_memory", "Long-term Memory", "memory", "browser_history"} {
		if wild.Valid() {
			t.Errorf("wild surface %q accepted", wild)
		}
	}
	for _, s := range AllMemoryReverificationStates() {
		if !MemoryReverificationState(s).Valid() {
			t.Errorf("reverification state %q rejected", s)
		}
	}
	for _, wild := range []MemoryReverificationState{"", "Reverified", "maybe", "trusted"} {
		if wild.Valid() {
			t.Errorf("wild reverification state %q accepted", wild)
		}
	}
}

// TestMemoryRuleConstantsAreVerbatim pins the four constructor-pinned
// lines against the section 41 machine block verbatim. Drift here is
// a contract change, not a string edit.
func TestMemoryRuleConstantsAreVerbatim(t *testing.T) {
	pins := []struct{ got, want string }{
		{MemoryNotAuthorizationRule, "memory-claim-never-carries-authorization"},
		{MemoryProvenanceAttributeRegistry, "origin,author,timestamp,trust,scope,ttl,integrity"},
		{MemoryReverificationObligationRule, "authority-and-policy-reverified-at-moment-of-use"},
		{MemoryEnforcementPlane, "none-in-observation-phase"},
	}
	for i, p := range pins {
		if p.got != p.want {
			t.Errorf("pinned rule %d drifted: %q, want %q", i, p.got, p.want)
		}
	}
	if len(strings.Split(MemoryProvenanceAttributeRegistry, ",")) != 7 {
		t.Error("attribute registry is not the closed seven")
	}
}

// TestMemoryProvenanceBuildsVerbatimShape builds one legal record and
// asserts the constructor restates every input line and appends the
// pinned constants, with no judgement field anywhere.
func TestMemoryProvenanceBuildsVerbatimShape(t *testing.T) {
	in := vectorStoreInput()
	rec, err := BuildMemoryProvenance(in)
	if err != nil {
		t.Fatalf("legal input rejected: %v", err)
	}
	if rec.Origin != in.Origin || rec.Author != in.Author || rec.Timestamp != in.Timestamp ||
		rec.Trust != in.Trust || rec.Scope != in.Scope || rec.TTL != in.TTL || rec.Integrity != in.Integrity {
		t.Error("constructor did not restate the seven attributes verbatim")
	}
	if rec.Surface != SurfaceVectorStore || rec.Reverification != ReverificationUnrecorded {
		t.Error("constructor did not restate the stated surface or reverification state")
	}
	if rec.MemoryNotAuthorization != MemoryNotAuthorizationRule {
		t.Errorf("not-authorization restatement %q is not the pinned rule", rec.MemoryNotAuthorization)
	}
	if rec.EnforcementPlane != MemoryEnforcementPlane {
		t.Errorf("enforcement plane %q is not the pinned stance", rec.EnforcementPlane)
	}
}

// TestMemoryRejectionsLeaveNoRecord walks every rejection door and
// asserts both the error and the zero record: a rejected input must
// not survive as a half-provenanced row.
func TestMemoryRejectionsLeaveNoRecord(t *testing.T) {
	cases := []struct {
		name string
		mut  func(*MemoryProvenanceInput)
	}{
		{"empty origin", func(i *MemoryProvenanceInput) { i.Origin = "" }},
		{"blank author", func(i *MemoryProvenanceInput) { i.Author = "   " }},
		{"empty timestamp", func(i *MemoryProvenanceInput) { i.Timestamp = "" }},
		{"empty trust", func(i *MemoryProvenanceInput) { i.Trust = "" }},
		{"empty scope", func(i *MemoryProvenanceInput) { i.Scope = "" }},
		{"empty ttl", func(i *MemoryProvenanceInput) { i.TTL = "" }},
		{"empty integrity", func(i *MemoryProvenanceInput) { i.Integrity = "" }},
		{"empty surface", func(i *MemoryProvenanceInput) { i.Surface = "" }},
		{"wild surface", func(i *MemoryProvenanceInput) { i.Surface = "cookies" }},
		{"empty reverification", func(i *MemoryProvenanceInput) { i.Reverification = "" }},
		{"wild reverification", func(i *MemoryProvenanceInput) { i.Reverification = "assumed_ok" }},
	}
	for _, c := range cases {
		in := vectorStoreInput()
		c.mut(&in)
		rec, err := BuildMemoryProvenance(in)
		if err == nil {
			t.Errorf("%s: rejection door did not fire", c.name)
		}
		if rec != (MemoryProvenance{}) {
			t.Errorf("%s: rejection left a half record behind", c.name)
		}
	}
}

// TestMemoryRecordCarriesNoDecision pins the reflective shape: the
// record and the input carry exactly their declared field sets and
// none of them is named like a decision, score, severity, verdict, or
// grant. Phase 0's closed pair stays elsewhere and this slice borrows
// none of it - in particular Trust is recorded text, never a derived
// clearance.
func TestMemoryRecordCarriesNoDecision(t *testing.T) {
	rt := reflect.TypeOf(MemoryProvenance{})
	if rt.NumField() != len(MemoryProvenanceFieldWireNames) {
		t.Fatalf("record field count %d, want %d", rt.NumField(), len(MemoryProvenanceFieldWireNames))
	}
	for i := 0; i < rt.NumField(); i++ {
		tag := strings.Split(rt.Field(i).Tag.Get("json"), ",")[0]
		if tag != MemoryProvenanceFieldWireNames[i] {
			t.Errorf("field %d wire name %q, want %q (order is normative)", i, tag, MemoryProvenanceFieldWireNames[i])
		}
		low := strings.ToLower(tag)
		for _, banned := range []string{"decision", "allow", "block", "deny", "score", "severity", "risk_level", "verdict", "grant"} {
			if strings.Contains(low, banned) {
				t.Errorf("record field %q carries decision-flavoured name %q", tag, banned)
			}
		}
	}
	it := reflect.TypeOf(MemoryProvenanceInput{})
	if it.NumField() != 9 {
		t.Fatalf("input field count %d, want 9", it.NumField())
	}
}

// TestMemorySymbolsStayOffTheDecisionPlane greps the four
// decision-plane directories and the command tree for this slice's
// symbols and carries its own teeth control: the needle must match
// this slice's own source, proving the scan could catch a leak.
func TestMemorySymbolsStayOffTheDecisionPlane(t *testing.T) {
	needle := "MemorySurface|AllMemorySurfaces|MemoryReverificationState|AllMemoryReverificationStates|MemoryProvenanceInput|MemoryProvenance|BuildMemoryProvenance|MemoryProvenanceFieldWireNames|MemoryNotAuthorizationRule|MemoryProvenanceAttributeRegistry|MemoryReverificationObligationRule|MemoryEnforcementPlane"
	re := regexp.MustCompile(needle)
	dirs := []string{
		filepath.Join("..", "policy"), filepath.Join("..", "rules"),
		filepath.Join("..", "bus"), filepath.Join("..", "auditlog"),
	}
	for _, d := range dirs {
		entries, err := os.ReadDir(d)
		if err != nil {
			t.Fatalf("read dir %s: %v", d, err)
		}
		for _, e := range entries {
			if e.IsDir() || !strings.HasSuffix(e.Name(), ".go") || strings.HasSuffix(e.Name(), "_test.go") {
				continue
			}
			data, err := os.ReadFile(filepath.Join(d, e.Name()))
			if err != nil {
				t.Fatalf("read %s: %v", d, err)
			}
			if loc := re.FindString(string(data)); loc != "" {
				t.Errorf("decision-plane file %s references memory symbol %q", filepath.Join(d, e.Name()), loc)
			}
		}
	}
	cmdHits := 0
	var walkCmd func(dir string)
	walkCmd = func(dir string) {
		entries, err := os.ReadDir(dir)
		if err != nil {
			t.Fatalf("read cmd dir: %v", err)
		}
		for _, e := range entries {
			p := filepath.Join(dir, e.Name())
			if e.IsDir() {
				walkCmd(p)
				continue
			}
			if !strings.HasSuffix(e.Name(), ".go") || strings.HasSuffix(e.Name(), "_test.go") {
				continue
			}
			data, err := os.ReadFile(p)
			if err != nil {
				t.Fatalf("read %s: %v", p, err)
			}
			if loc := re.FindString(string(data)); loc != "" {
				cmdHits++
				t.Errorf("command tree file %s references memory symbol %q", p, loc)
			}
		}
	}
	walkCmd(filepath.Join("..", "..", "cmd"))
	if cmdHits == 0 {
		self, err := os.ReadFile("memoryprofile.go")
		if err != nil {
			t.Fatalf("teeth control unreadable: %v", err)
		}
		if re.FindString(string(self)) == "" {
			t.Error("teeth control failed: needle does not match its own source")
		}
	}
}
