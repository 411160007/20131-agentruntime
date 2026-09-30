package schema

import (
	"os"
	"reflect"
	"regexp"
	"strings"
	"testing"
	"time"
)

// Decision contract sync tests (stability wave, first slice). They pin
// docs/schema-v2.md as a THIRD machine-checked surface: the contract file
// must agree verbatim with the Go enums, the independent Node validator,
// and the frozen external contract doc. Any drift fails the build. This
// is schema and documentation evolution only: nothing here changes
// runtime decision semantics.

func schemaV2Blocks(t *testing.T, text string) []map[string]string {
	t.Helper()
	re := regexp.MustCompile("(?s)```schemav2\n(.*?)```")
	var out []map[string]string
	for _, m := range re.FindAllStringSubmatch(text, -1) {
		kv := map[string]string{}
		for _, line := range strings.Split(m[1], "\n") {
			line = strings.TrimSpace(line)
			if line == "" {
				continue
			}
			i := strings.Index(line, ":")
			if i < 0 {
				t.Fatalf("unparsable schemav2 line %q", line)
			}
			k, val := strings.TrimSpace(line[:i]), strings.TrimSpace(line[i+1:])
			if _, dup := kv[k]; dup {
				t.Fatalf("duplicate key %q in schemav2 block", k)
			}
			kv[k] = val
		}
		out = append(out, kv)
	}
	if len(out) == 0 {
		t.Fatal("no schemav2 blocks found (silent probe failure guard)")
	}
	return out
}

func schemaV2Key(t *testing.T, text, key string) string {
	t.Helper()
	blocks := schemaV2Blocks(t, text)
	var found []string
	for _, kv := range blocks {
		if v, ok := kv[key]; ok {
			found = append(found, v)
		}
	}
	if len(found) != 1 {
		t.Fatalf("key %q appears %d times, want exactly 1", key, len(found))
	}
	return found[0]
}

// TestDecisionContractSourcesAgree is the four-way verbatim check of the
// decision vocabulary: schema-v2 contract line == Go declaration order ==
// Node validator list == external api contract line.
func TestDecisionContractSourcesAgree(t *testing.T) {
	raw, err := os.ReadFile(filepathJoinDotDot("docs", "schema-v2.md"))
	if err != nil {
		t.Skipf("schema-v2 doc not visible from this working dir: %v", err)
	}
	doc := string(raw)
	const wantVocab = "allow, ask, would_block"

	if got := schemaV2Key(t, doc, "decision_vocabulary"); got != wantVocab {
		t.Errorf("schema-v2 vocabulary %q diverges from the frozen literal", got)
	}
	if got := strings.Join(AllDecisions(), ", "); got != wantVocab {
		t.Errorf("Go AllDecisions() join %q diverges from the contract literal", got)
	}
	valSrc, err := os.ReadFile(filepathJoinDotDot("scripts", "validate-jsonl.mjs"))
	if err != nil {
		t.Skipf("validator script not visible: %v", err)
	}
	if got := strings.Join(extractArray(t, string(valSrc), "DECISIONS"), ", "); got != wantVocab {
		t.Errorf("validator DECISIONS join %q diverges from the contract literal", got)
	}
	apiRaw, err := os.ReadFile(filepathJoinDotDot("docs", "api-v0.md"))
	if err != nil {
		t.Skipf("api contract doc not visible: %v", err)
	}
	m := regexp.MustCompile("(?m)^decisions: (.+)$").FindStringSubmatch(string(apiRaw))
	if m == nil || m[1] != wantVocab {
		t.Errorf("api-v0 decisions line diverges from the contract literal (got %v)", m)
	}

	// Runtime closed set: contract line must equal the Go set verbatim.
	var p0 []string
	for _, d := range Phase0RuntimeDecisions() {
		p0 = append(p0, string(d))
	}
	if got := schemaV2Key(t, doc, "decision_phase0_runtime_set"); got != strings.Join(p0, ", ") {
		t.Errorf("contract runtime set %q diverges from Phase0RuntimeDecisions() %v", got, p0)
	}
	// Teeth: reserved vocabulary membership and emission rejection.
	if !DecisionAsk.Valid() {
		t.Error("ask must remain valid contract vocabulary")
	}
	if err := MustPhase0Decision(DecisionAsk); err == nil {
		t.Error("observation-phase emission set must reject ask")
	}
	if err := MustPhase0Decision(DecisionAllow); err != nil {
		t.Errorf("allow must stay inside the emission set: %v", err)
	}
	if err := MustPhase0Decision(DecisionWouldBlock); err != nil {
		t.Errorf("would_block must stay inside the emission set: %v", err)
	}
}

// TestDecisionCarrierFieldShape pins the wire shape the contract names:
// the carrier field is the required "decision" key (no omitempty - it has
// existed on every line since generation one), contrasted against the two
// additive optional fields that do carry omitempty.
func TestDecisionCarrierFieldShape(t *testing.T) {
	ft, ok := reflect.TypeOf(Event{}).FieldByName("Decision")
	if !ok {
		t.Fatal("Event has no Decision field")
	}
	tag := ft.Tag.Get("json")
	if tag != "decision" {
		t.Errorf("Decision json tag %q, want exactly %q (no omitempty: required since v1)", tag, "decision")
	}
	for _, name := range []string{"Tier", "SourceClass"} {
		f, ok := reflect.TypeOf(Event{}).FieldByName(name)
		if !ok {
			t.Fatalf("Event missing additive field %s", name)
		}
		if !strings.Contains(f.Tag.Get("json"), "omitempty") {
			t.Errorf("additive field %s must keep omitempty (absent-key legacy contract)", name)
		}
	}

	// Wild decision values must fail validation on an otherwise legal event.
	base := func(d Decision) Event {
		return Event{
			V: SchemaVersion, TS: time.Now().UTC(), ID: "ev-1", AgentID: "ag-1",
			Stage: StageEvaluated, Type: TypePolicyDecision, Decision: d,
			Severity: SevInfo, Summary: "contract shape probe",
		}
	}
	for _, d := range []Decision{"deny", "ALLOW", "would-block", ""} {
		e := base(d)
		if err := e.Validate(); err == nil {
			t.Errorf("decision %q passed validation, want rejection", d)
		}
	}
	e := base(DecisionWouldBlock)
	if err := e.Validate(); err != nil {
		t.Errorf("would_block rejected on legal event: %v", err)
	}
}

// TestDecisionEffectMappingContract verifies the contract's mapping line
// is the verbatim identity relation and matches DecisionFor() for every
// effect in the policy vocabulary.
func TestDecisionEffectMappingContract(t *testing.T) {
	raw, err := os.ReadFile(filepathJoinDotDot("docs", "schema-v2.md"))
	if err != nil {
		t.Skipf("schema-v2 doc not visible: %v", err)
	}
	got := schemaV2Key(t, string(raw), "decision_effect_mapping")
	want := "allow->allow; ask->ask; would_block->would_block"
	if got != want {
		t.Errorf("mapping line %q, want %q", got, want)
	}
	for _, eff := range []Effect{EffectAllow, EffectAsk, EffectWouldBlock} {
		d, err := DecisionFor(eff)
		if err != nil {
			t.Errorf("DecisionFor(%s): %v", eff, err)
			continue
		}
		if string(d) != string(eff) {
			t.Errorf("effect %s maps to decision %s, want identity", eff, d)
		}
	}
	if _, err := DecisionFor(Effect("explode")); err == nil {
		t.Error("unknown effect must have no decision mapping")
	}
}

// TestSchemaV2SlotAndTemplateCensus independently re-implements the slot
// lifecycle machine-check (second implementation, same rules as the Node
// checker): seven slots, one complete, six pending with pointers, and a
// substantive four-element template census.
func TestSchemaV2SlotAndTemplateCensus(t *testing.T) {
	raw, err := os.ReadFile(filepathJoinDotDot("docs", "schema-v2.md"))
	if err != nil {
		t.Skipf("schema-v2 doc not visible: %v", err)
	}
	doc := string(raw)
	blocks := schemaV2Blocks(t, doc)

	want := map[string]struct{ status, slice string }{
		"decision":  {"complete", ""},
		"intent":    {"pending", "W2.2"},
		"authority": {"pending", "W2.2"},
		"impact":    {"pending", "W2.3"},
		"recovery":  {"pending", "W2.3"},
		"evidence":  {"pending", "W2.4"},
		"profile":   {"pending", "W2.4"},
	}
	seen := map[string]map[string]string{}
	tmpl := 0
	for _, kv := range blocks {
		if s, ok := kv["schema"]; ok {
			if _, dup := seen[s]; dup {
				t.Errorf("duplicate slot %s", s)
			}
			seen[s] = kv
		}
		if _, ok := kv["template_element"]; ok {
			tmpl++
		}
	}
	if len(seen) != 7 {
		t.Fatalf("slot count %d, want 7", len(seen))
	}
	for name, exp := range want {
		kv, ok := seen[name]
		if !ok {
			t.Errorf("slot %s missing", name)
			continue
		}
		if kv["status"] != exp.status {
			t.Errorf("slot %s status %q, want %q", name, kv["status"], exp.status)
		}
		if got := kv["planned_slice"]; (exp.slice != "" && got != exp.slice) || (exp.slice == "" && got != "") {
			t.Errorf("slot %s pointer %q, want %q", name, got, exp.slice)
		}
	}
	if tmpl != 4 {
		t.Errorf("template element census %d, want 4", tmpl)
	}
}

// TestSchemaV2ElementsSubstantive machine-judges that the four element
// sections of the decision contract carry entity lines, not slogans:
// >=3 lines, >=1 list item, >=2 inline code spans, >=120 non-space
// characters (identical thresholds to the Node checker by contract).
func TestSchemaV2ElementsSubstantive(t *testing.T) {
	raw, err := os.ReadFile(filepathJoinDotDot("docs", "schema-v2.md"))
	if err != nil {
		t.Skipf("schema-v2 doc not visible: %v", err)
	}
	doc := string(raw)
	for _, name := range []string{"Version", "Compatibility", "Migration", "Validation"} {
		head := "#### " + name + "\n"
		i := strings.Index(doc, head)
		if i < 0 {
			t.Errorf("element section %s missing", name)
			continue
		}
		rest := doc[i+len(head):]
		if m := regexp.MustCompile("(?m)^#{2,4} ").FindStringIndex(rest); m != nil {
			rest = rest[:m[0]]
		}
		body := doc[i : i+len(head)+len(rest)]
		lines := 0
		listItem := false
		for _, l := range strings.Split(body, "\n") {
			if strings.TrimSpace(l) == "" {
				continue
			}
			lines++
			if strings.HasPrefix(strings.TrimSpace(l), "- ") {
				listItem = true
			}
		}
		spans := regexp.MustCompile("`[^`\n]+`").FindAllString(body, -1)
		nonSpace := len(strings.Join(strings.Fields(body), ""))
		if lines < 3 || !listItem || len(spans) < 2 || nonSpace < 120 {
			t.Errorf("element %s not substantive: lines=%d list=%v spans=%d chars=%d",
				name, lines, listItem, len(spans), nonSpace)
		}
	}
}
