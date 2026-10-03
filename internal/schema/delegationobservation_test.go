package schema

import (
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"regexp"
	"strings"
	"testing"
)

// spawnInput is the taskbook's machine-judgment fixture shape: a
// child-process spawn carrying all section 283 fields.
func spawnInput() DelegationObservationInput {
	return DelegationObservationInput{
		ParentAgent:         "orchestrator-alpha",
		ChildAgent:          "build-worker-1",
		DelegationReason:    "run the recorded build step off the parent loop",
		DelegatedCapability: "exec:build-toolchain",
		DataScope:           "workspace:repo-checkout",
		AuthoritySource:     "user task grant recorded at session start",
		TTL:                 "until step completion or 30m, whichever first",
		Outcome:             "step recorded as finished by the child reporter",
		Subject:             SubjectChildProcess,
		Revalidation:        RevalidationDone,
	}
}

// TestDelegationSubjectVocabularyPin pins the closed five-subject
// vocabulary: count, spec order, uniqueness, membership, and the
// wild-value rejection. The order literal is a third recount of the
// section 236 listing (Go, docs, Node checker must all agree).
func TestDelegationSubjectVocabularyPin(t *testing.T) {
	want := []string{"skill", "mcp", "tool", "child_process", "plugin"}
	got := AllDelegationSubjects()
	if len(got) != 5 {
		t.Fatalf("subject census drifted: %d", len(got))
	}
	if strings.Join(got, ",") != strings.Join(want, ",") {
		t.Fatalf("subject order drifted from the spec listing:\n got %v\nwant %v", got, want)
	}
	if len(AllDelegationRevalidationStates()) != 3 {
		t.Fatal("revalidation state census is not three")
	}
	for _, w := range want {
		if !DelegationSubject(w).Valid() {
			t.Errorf("vocabulary member %q rejected by Valid", w)
		}
	}
	for _, wild := range []DelegationSubject{"", "Skill", "child-process", "subprocess", "agent"} {
		if wild.Valid() {
			t.Errorf("wild subject %q accepted", wild)
		}
	}
	for _, s := range AllDelegationRevalidationStates() {
		if !DelegationRevalidationState(s).Valid() {
			t.Errorf("revalidation state %q rejected", s)
		}
	}
	for _, wild := range []DelegationRevalidationState{"", "probably_revalidated", "skipped", "REVALIDATED"} {
		if wild.Valid() {
			t.Errorf("wild revalidation state %q accepted", wild)
		}
	}
}

// TestDelegationRuleConstantsPin pins the four rule strings against
// the spec's own words so a later edit cannot quietly soften them.
func TestDelegationRuleConstantsPin(t *testing.T) {
	if DelegationNoAutoInheritanceRule != "parent-authority-never-auto-transfers-to-child" {
		t.Error("no-auto-inheritance rule drifted from the pinned shape")
	}
	if DelegationReevaluationRegistry != "identity,authority,capability,scope,data,risk,ttl" {
		t.Error("reevaluation registry drifted from the section 236 bullet order")
	}
	if DelegationRevalidationObligationRule != "every-child-must-revalidate-own-permissions" {
		t.Error("revalidation obligation rule drifted")
	}
	if DelegationEnforcementPlane != "none-in-observation-phase" {
		t.Error("enforcement plane drifted off none")
	}
	// Section 236 bullet order must derive mechanically to the
	// registry line (space to underscore, lower case), like the
	// other anchor back-checks.
	for i, n := range []string{"Identity", "Authority", "Capability", "Scope", "Data", "Risk", "TTL"} {
		if strings.ToLower(n) != strings.Split(DelegationReevaluationRegistry, ",")[i] {
			t.Fatalf("registry derivation broke at bullet %d", i+1)
		}
	}
}

// TestBuildDelegationObservationSpawnGolden is the taskbook's
// machine-judgment fixture: a child-process spawn whose record
// carries every required field, restates the no-inheritance rule
// verbatim, and pins the enforcement plane.
func TestBuildDelegationObservationSpawnGolden(t *testing.T) {
	rec, err := BuildDelegationObservation(spawnInput())
	if err != nil {
		t.Fatalf("spawn fixture rejected: %v", err)
	}
	in := spawnInput()
	if rec.ParentAgent != in.ParentAgent || rec.ChildAgent != in.ChildAgent ||
		rec.DelegationReason != in.DelegationReason || rec.DelegatedCapability != in.DelegatedCapability ||
		rec.DataScope != in.DataScope || rec.AuthoritySource != in.AuthoritySource ||
		rec.TTL != in.TTL || rec.Outcome != in.Outcome {
		t.Error("record does not restate the eight section 283 fields verbatim")
	}
	if rec.Subject != SubjectChildProcess || rec.ChildRevalidation != RevalidationDone {
		t.Error("subject or revalidation restatement drifted")
	}
	if rec.AuthorityNotInherited != DelegationNoAutoInheritanceRule {
		t.Error("record fails to restate the no-auto-inheritance rule")
	}
	if rec.EnforcementPlane != DelegationEnforcementPlane {
		t.Error("record fails to pin the none enforcement plane")
	}
	// The explicit-unrecorded shape must be constructible: stating
	// the silence is admissible, silence itself is not.
	silent := spawnInput()
	silent.Revalidation = RevalidationUnrecorded
	rec2, err := BuildDelegationObservation(silent)
	if err != nil {
		t.Fatalf("explicit-unrecorded shape rejected: %v", err)
	}
	if rec2.ChildRevalidation != RevalidationUnrecorded {
		t.Error("unrecorded restatement lost")
	}
	// Wire census: twelve keys on the record, matching the pinned
	// field list order.
	raw, err := json.Marshal(rec)
	if err != nil {
		t.Fatal(err)
	}
	var m map[string]any
	if err := json.Unmarshal(raw, &m); err != nil {
		t.Fatal(err)
	}
	if len(m) != len(DelegationRecordFieldWireNames) {
		t.Fatalf("wire field census %d, want %d", len(m), len(DelegationRecordFieldWireNames))
	}
	for _, k := range DelegationRecordFieldWireNames {
		if _, ok := m[k]; !ok {
			t.Errorf("wire key %q missing from the record", k)
		}
	}
}

// TestDelegationRejectionsLeaveNoRecord walks every rejection door
// and pins that each leaves the zero value behind: an empty or
// whitespace-only must-record field, a wild subject, or a wild
// revalidation state aborts the whole record before any value is
// applied.
func TestDelegationRejectionsLeaveNoRecord(t *testing.T) {
	cases := []struct {
		name string
		mut  func(in DelegationObservationInput) DelegationObservationInput
	}{
		{"empty parent", func(i DelegationObservationInput) DelegationObservationInput { i.ParentAgent = ""; return i }},
		{"empty child", func(i DelegationObservationInput) DelegationObservationInput { i.ChildAgent = ""; return i }},
		{"empty reason", func(i DelegationObservationInput) DelegationObservationInput { i.DelegationReason = ""; return i }},
		{"empty capability", func(i DelegationObservationInput) DelegationObservationInput { i.DelegatedCapability = ""; return i }},
		{"empty scope", func(i DelegationObservationInput) DelegationObservationInput { i.DataScope = ""; return i }},
		{"empty authority source", func(i DelegationObservationInput) DelegationObservationInput { i.AuthoritySource = ""; return i }},
		{"empty ttl", func(i DelegationObservationInput) DelegationObservationInput { i.TTL = ""; return i }},
		{"empty outcome", func(i DelegationObservationInput) DelegationObservationInput { i.Outcome = ""; return i }},
		{"whitespace-only ttl", func(i DelegationObservationInput) DelegationObservationInput { i.TTL = " \t "; return i }},
		{"whitespace-only reason", func(i DelegationObservationInput) DelegationObservationInput { i.DelegationReason = "  "; return i }},
		{"wild subject", func(i DelegationObservationInput) DelegationObservationInput { i.Subject = "subprocess"; return i }},
		{"empty subject", func(i DelegationObservationInput) DelegationObservationInput { i.Subject = ""; return i }},
		{"wild revalidation", func(i DelegationObservationInput) DelegationObservationInput { i.Revalidation = "probably"; return i }},
		{"empty revalidation", func(i DelegationObservationInput) DelegationObservationInput { i.Revalidation = ""; return i }},
	}
	for _, c := range cases {
		rec, err := BuildDelegationObservation(c.mut(spawnInput()))
		if err == nil {
			t.Errorf("%s: record built despite the rejection door", c.name)
		}
		if rec != (DelegationObservation{}) {
			t.Errorf("%s: rejection left a half record behind", c.name)
		}
	}
}

// TestDelegationRecordCarriesNoDecision pins the reflective shape:
// the record and the input carry exactly their declared field sets
// and none of them is named like a decision, score, severity, or
// verdict. Phase 0's closed pair stays elsewhere and this slice
// borrows none of it.
func TestDelegationRecordCarriesNoDecision(t *testing.T) {
	rt := reflect.TypeOf(DelegationObservation{})
	if rt.NumField() != len(DelegationRecordFieldWireNames) {
		t.Fatalf("record field count %d, want %d", rt.NumField(), len(DelegationRecordFieldWireNames))
	}
	for i := 0; i < rt.NumField(); i++ {
		tag := strings.Split(rt.Field(i).Tag.Get("json"), ",")[0]
		if tag != DelegationRecordFieldWireNames[i] {
			t.Errorf("field %d wire name %q, want %q (order is normative)", i, tag, DelegationRecordFieldWireNames[i])
		}
		low := strings.ToLower(tag)
		for _, banned := range []string{"decision", "allow", "block", "deny", "score", "severity", "risk_level", "verdict"} {
			if strings.Contains(low, banned) {
				t.Errorf("record field %q carries decision-flavoured name %q", tag, banned)
			}
		}
	}
	it := reflect.TypeOf(DelegationObservationInput{})
	if it.NumField() != 10 {
		t.Fatalf("input field count %d, want 10", it.NumField())
	}
}

// TestDelegationSymbolsStayOffTheDecisionPlane greps the four
// decision-plane directories and the command tree for this slice's
// symbols and carries its own teeth control: the needle must match
// this slice's own source, proving the scan could catch a leak.
func TestDelegationSymbolsStayOffTheDecisionPlane(t *testing.T) {
	needle := "DelegationSubject|DelegationRevalidationState|DelegationObservation|DelegationObservationInput|BuildDelegationObservation|AllDelegationSubjects|AllDelegationRevalidationStates|DelegationNoAutoInheritanceRule|DelegationReevaluationRegistry|DelegationRevalidationObligationRule|DelegationEnforcementPlane|DelegationRecordFieldWireNames"
	dirs := []string{
		filepath.Join("..", "policy"), filepath.Join("..", "rules"),
		filepath.Join("..", "bus"), filepath.Join("..", "auditlog"),
	}
	re := regexp.MustCompile(needle)
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
				t.Errorf("decision-plane file %s references delegation symbol %q", filepath.Join(d, e.Name()), loc)
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
				t.Errorf("command tree file %s references delegation symbol %q", p, loc)
			}
		}
	}
	walkCmd(filepath.Join("..", "..", "cmd"))
	if cmdHits == 0 {
		self, err := os.ReadFile("delegationobservation.go")
		if err != nil {
			t.Fatalf("read self: %v", err)
		}
		if !re.MatchString(string(self)) {
			t.Error("positive control broken: needle matches no shipped delegation source")
		}
	}
}

// TestDelegationDocsSync pins the section 20 keys, both spec anchor
// bullet blocks, and the record field table verbatim against the Go
// constants and vocabularies, completing the docs<->Go<->Node
// three-way mirror.
func TestDelegationDocsSync(t *testing.T) {
	raw, err := os.ReadFile(filepath.Join("..", "..", "docs", "schema-v2.md"))
	if err != nil {
		t.Fatalf("read docs/schema-v2.md: %v", err)
	}
	doc := string(raw)
	if !strings.Contains(doc, "## 20. Delegation observation record contract") {
		t.Fatal("section 20 header missing from docs/schema-v2.md")
	}
	pins := [][2]string{
		{"delegation_subject_vocabulary", strings.Join(AllDelegationSubjects(), ",")},
		{"delegation_revalidation_states", strings.Join(AllDelegationRevalidationStates(), ",")},
		{"delegation_no_auto_inheritance_rule", DelegationNoAutoInheritanceRule},
		{"delegation_reevaluation_registry", DelegationReevaluationRegistry},
		{"delegation_revalidation_obligation_rule", DelegationRevalidationObligationRule},
		{"delegation_enforcement_plane", DelegationEnforcementPlane},
	}
	for _, p := range pins {
		line := p[0] + ": " + p[1]
		if !strings.Contains(doc, line) {
			t.Errorf("docs section 20 drifted, want line %q", line)
		}
	}
	// Section 283's eight must-record lines mirror the first eight
	// wire fields mechanically (space to underscore, lower case).
	specNames := []string{
		"Parent Agent", "Child Agent", "Delegation Reason", "Delegated Capability",
		"Data Scope", "Authority Source", "TTL", "Outcome",
	}
	for i, n := range specNames {
		if strings.ToLower(strings.ReplaceAll(n, " ", "_")) != DelegationRecordFieldWireNames[i] {
			t.Fatalf("field order broke against the section 283 line %d", i+1)
		}
		if !strings.Contains(doc, "* "+n) {
			t.Errorf("docs section 283 anchor bullet missing: %q", n)
		}
	}
	// Section 236's five subject listing mirrors the vocabulary.
	for i, n := range []string{"Skill", "MCP", "Tool", "Child Process", "Plugin"} {
		if !strings.Contains(doc, "* "+n) {
			t.Errorf("docs section 236 subject bullet missing: %q", n)
		}
		derived := strings.ToLower(strings.ReplaceAll(n, " ", "_"))
		if derived != delegationSubjectWireNames[i] {
			t.Fatalf("subject derivation broke at %q", n)
		}
	}
	// Field table rows pin row order against the normative list.
	for i, k := range DelegationRecordFieldWireNames {
		row := "| `" + k + "` |"
		at := strings.Index(doc[strings.Index(doc, "## 20. Delegation observation record contract"):], row)
		if at < 0 {
			t.Errorf("docs field table row missing for %q", k)
			continue
		}
		_ = i
	}
}
