package schema

import (
	"encoding/json"
	"strings"
	"testing"
)

// sourceClassEvent builds a minimal good record; tests mutate one aspect.
func sourceClassEvent() *Event {
	e := tierCapEvent()
	e.ID = "ev-srcclass-1"
	return e
}

// TestSourceClassValidationMatrix is the table-driven machine judgement
// for the additive source_class field: absent stays valid (unclassified
// legacy), every vocabulary member validates, and wild values are
// rejected on both sides of the enum boundary (case, near-miss spellings,
// self-claim vocabulary, digits, whitespace).
func TestSourceClassValidationMatrix(t *testing.T) {
	cases := []struct {
		name  string
		class SourceClass
		ok    bool
	}{
		{"absent-legacy", "", true},
		{"native_os", SrcNativeOS, true},
		{"runtime", SrcRuntime, true},
		{"tool_mcp", SrcToolMCP, true},
		{"agent_meta", SrcAgentMeta, true},
		{"agent_self", SrcAgentSelf, true},
		{"llm_interpretation", SrcLLMInterpretation, true},
		// wild values (must all reject)
		{"wild-uppercase", "Native_OS", false},
		{"wild-nearmiss-native", "native", false},
		{"wild-nearmiss-runtime", "runtime_event", false},
		{"wild-selfclaim-hightrust", "system", false},
		{"wild-selfclaim-maximum", "agent_self_description", false},
		{"wild-llm-short", "llm", false},
		{"wild-mcp-only", "mcp", false},
		{"wild-bare-digit", "1", false},
		{"wild-empty-ish", SourceClass(" "), false},
		{"wild-trailing-space", "native_os ", false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			e := sourceClassEvent()
			e.SourceClass = tc.class
			err := e.Validate()
			if tc.ok && err != nil {
				t.Fatalf("source_class %q rejected: %v", string(tc.class), err)
			}
			if !tc.ok && err == nil {
				t.Fatalf("wild source_class %q accepted", string(tc.class))
			}
		})
	}
}

// TestSourceClassVocabularyShape pins the closed set: exactly six values
// in trust order (native_os highest first, llm_interpretation lowest),
// each distinct, and the Go wire values match the six provenance classes
// of the trust order contract.
func TestSourceClassVocabularyShape(t *testing.T) {
	all := AllSourceClasses()
	if len(all) != 6 {
		t.Fatalf("source class vocabulary must be six values, got %v", all)
	}
	if all[0] != string(SrcNativeOS) || all[5] != string(SrcLLMInterpretation) {
		t.Fatalf("declaration order must be trust order (native_os first, llm_interpretation last), got %v", all)
	}
	seen := map[string]bool{}
	for _, v := range all {
		if seen[v] {
			t.Errorf("duplicate source class %q", v)
		}
		seen[v] = true
		if !SourceClass(v).Valid() {
			t.Errorf("listed value %q fails its own Valid()", v)
		}
	}
	// the empty legacy meaning must NOT be a listed vocabulary member
	for _, v := range all {
		if v == "" {
			t.Fatal("unclassified must stay absent-key, not a listed value")
		}
	}
}

// TestLegacyLineWithoutSourceClassStillValid pins the additive
// non-breaking property at schema level: unmarshalling a pre-source_class
// JSONL line yields a valid event with the class left unclassified.
// There is deliberately NO default promotion: absent does not become
// native_os or any other class.
func TestLegacyLineWithoutSourceClassStillValid(t *testing.T) {
	legacy := `{"v":1,"ts":"2026-09-27T15:52:32.103006806Z","id":"ev-aaa111","agent_id":"agent-hello-collector","stage":"observation","type":"collector.start","decision":"allow","severity":0,"summary":"legacy line predating the provenance field","attrs":{"version":"x"}}`
	var e Event
	if err := json.Unmarshal([]byte(legacy), &e); err != nil {
		t.Fatal(err)
	}
	if err := e.Validate(); err != nil {
		t.Fatalf("legacy line must validate: %v", err)
	}
	if e.SourceClass != "" {
		t.Fatalf("absent source_class must stay unclassified, got %q", string(e.SourceClass))
	}
	// marshalling the unclassified event omits the key entirely (additive
	// wire shape: old consumers see exactly the old line shape)
	out, err := json.Marshal(&e)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(out), "source_class") {
		t.Fatalf("unclassified event must not emit a source_class key: %s", out)
	}
}

// TestSourceClassIsObservationOnlyType closes the vocabulary hole: the
// six classes describe evidence provenance and must never overlap the
// decision vocabulary (no class string equals a Decision value, so no
// wire value can be mistaken across the two enums by a sloppy cast).
func TestSourceClassIsObservationOnlyType(t *testing.T) {
	for _, sc := range AllSourceClasses() {
		for _, d := range AllDecisions() {
			if sc == d {
				t.Fatalf("source class %q collides with decision vocabulary %q", sc, d)
			}
		}
	}
}
