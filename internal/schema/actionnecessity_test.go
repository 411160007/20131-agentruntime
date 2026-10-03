package schema

import (
	"os"
	"path/filepath"
	"reflect"
	"regexp"
	"strings"
	"testing"
)

// TestNecessityVocabularyAndRulesPin pins the three-question and
// three-answer closed vocabularies in normative order and the four
// rule constant values against the shipped contract.
func TestNecessityVocabularyAndRulesPin(t *testing.T) {
	wantQ := []string{"reasonable_step", "necessary_step", "substitutable_step"}
	gotQ := AllNecessityQuestions()
	if len(gotQ) != 3 || strings.Join(gotQ, ",") != strings.Join(wantQ, ",") {
		t.Fatalf("necessity question vocabulary drifted: %v", gotQ)
	}
	wantA := []string{"supports", "refutes", "no_material"}
	gotA := AllNecessityAnswers()
	if len(gotA) != 3 || strings.Join(gotA, ",") != strings.Join(wantA, ",") {
		t.Fatalf("necessity answer vocabulary drifted: %v", gotA)
	}
	if ActionNecessityDangerOnlyRule != "danger-judgement-alone-never-suffices" {
		t.Errorf("danger-only rule drifted: %q", ActionNecessityDangerOnlyRule)
	}
	if ActionNecessityEvidenceRule != "every-recorded-answer-requires-nonempty-evidence" {
		t.Errorf("evidence rule drifted: %q", ActionNecessityEvidenceRule)
	}
	if ActionNecessityAbsentDefault != "absent-means-unassessed-never-inferred" {
		t.Errorf("absent default drifted: %q", ActionNecessityAbsentDefault)
	}
	if ActionNecessityEnforcementPlane != "none-in-observation-phase" {
		t.Errorf("enforcement plane drifted: %q", ActionNecessityEnforcementPlane)
	}
	// The necessity questions are a separate closed table from the
	// nine data actions and the eight packaging words; sharing token
	// text across tables would silently merge their meanings.
	if len(AllNecessityQuestions()) == len(AllDataActions()) {
		t.Error("question vocabulary must stay distinguishable from the nine (3 vs 9)")
	}
	if len(AllNecessityAnswers()) == len(AllPackagingWords()) {
		t.Error("answer vocabulary must stay distinguishable from the eight (3 vs 8)")
	}
}

// TestNecessityQuestionAndAnswerValidity accepts each closed member
// and rejects the wild shapes: empty string, respellings, near
// misses, and tokens borrowed from the other vocabularies.
func TestNecessityQuestionAndAnswerValidity(t *testing.T) {
	for _, q := range AllNecessityQuestions() {
		if !NecessityQuestion(q).Valid() {
			t.Errorf("closed question rejected: %q", q)
		}
	}
	for _, a := range AllNecessityAnswers() {
		if !NecessityAnswer(a).Valid() {
			t.Errorf("closed answer rejected: %q", a)
		}
	}
	wildQ := []string{"", "Reasonable_Step", "reasonable", "necessary_steps", "substituable_step", " read", "read "}
	for _, q := range wildQ {
		if NecessityQuestion(q).Valid() {
			t.Errorf("wild question accepted: %q", q)
		}
	}
	wildA := []string{"", "SUPPORTS", "support", "refuted", "no-material", "maybe", "likely", "share"}
	for _, a := range wildA {
		if NecessityAnswer(a).Valid() {
			t.Errorf("wild answer accepted: %q", a)
		}
	}
}

// TestBuildActionNecessityGoldenDeployChain is the section 233
// positive side: a deploy chain answering a "deploy the site"
// intent records supports across the three questions with non-empty
// evidence, and nothing in the row is flagged - the zero false
// positive requirement means a normal task chain must produce no
// refutation projection and no borrowed decision of any shape.
func TestBuildActionNecessityGoldenDeployChain(t *testing.T) {
	row, err := BuildActionNecessity("deploy the marketing site", DataActionExecute, []NecessityAnswerEntry{
		{NecessityQuestionReasonable, NecessityAnswerSupports, "git, npm and docker steps are listed in the task brief"},
		{NecessityQuestionNecessary, NecessityAnswerSupports, "the publish runbook requires a container build before rollout"},
		{NecessityQuestionSubstitutable, NecessityAnswerSupports, "no lighter step achieves the same rollout in this environment"},
	})
	if err != nil {
		t.Fatalf("deploy chain row: %v", err)
	}
	if !row.Assessed {
		t.Error("three recorded answers must surface as assessed")
	}
	if row.RefutedPresent {
		t.Error("normal task chain must not carry a refutation projection (zero false positives)")
	}
	for i, e := range row.Answers {
		if strings.TrimSpace(e.Evidence) == "" {
			t.Errorf("answer %d lost its evidence text", i)
		}
	}
	if row.EnforcementPlane != ActionNecessityEnforcementPlane {
		t.Errorf("row must surface the none-plane constant, got %q", row.EnforcementPlane)
	}
	if row.Action != DataActionExecute || row.TaskIntent != "deploy the marketing site" {
		t.Errorf("classification anchors drifted: %+v", row)
	}
}

// TestBuildActionNecessityGoldenUnrelatedKeyRead is the section 233
// negative side: reading an unrelated SSH key while the stated task
// is a site deploy records refutes with non-empty evidence, the row
// surfaces the refutation projection, and still carries no decision
// - low necessity is material for the later judgement, never a
// judgement made here.
func TestBuildActionNecessityGoldenUnrelatedKeyRead(t *testing.T) {
	row, err := BuildActionNecessity("deploy the marketing site", DataActionRead, []NecessityAnswerEntry{
		{NecessityQuestionReasonable, NecessityAnswerRefutes, "the deploy brief names no credential outside the project"},
		{NecessityQuestionNecessary, NecessityAnswerRefutes, "every documented step proceeds without this file"},
		{NecessityQuestionSubstitutable, NecessityAnswerNoMaterial, "no recorder material shows what this read would replace"},
	})
	if err != nil {
		t.Fatalf("unrelated key read row: %v", err)
	}
	if !row.Assessed || !row.RefutedPresent {
		t.Errorf("unnecessary action must record assessed + refutation projection: %+v", row)
	}
	if len(row.Answers) != 3 {
		t.Fatalf("all three answers must be kept: %+v", row.Answers)
	}
	if row.Answers[2].Answer != NecessityAnswerNoMaterial {
		t.Errorf("no_material must survive as an explicit honest answer: %+v", row.Answers[2])
	}
}

// TestBuildActionNecessityAbsentMeansUnassessed pins the empty
// answer list: a row recorded with no answers is legal, surfaces
// Assessed=false and RefutedPresent=false, and no answer is
// synthesised for any question - absence of the refutation flag on
// an unassessed row is never read as a necessity finding.
func TestBuildActionNecessityAbsentMeansUnassessed(t *testing.T) {
	row, err := BuildActionNecessity("some task", DataActionRead, nil)
	if err != nil {
		t.Fatalf("unassessed row: %v", err)
	}
	if row.Assessed || row.RefutedPresent {
		t.Errorf("empty answers must project unassessed, never inferred: %+v", row)
	}
	if len(row.Answers) != 0 {
		t.Errorf("answers must stay empty: %v", row.Answers)
	}
}

// TestNecessityRejectionsLeaveNoRecord pins the before-value
// rejection shape: every red input returns the zero struct plus an
// error - empty or whitespace intent, absent or wild action class,
// wild or repeated question, wild answer, and any entry whose
// evidence is empty (the verbatim section 233 obligation enforced
// at the door) - no partial row escapes the constructor.
func TestNecessityRejectionsLeaveNoRecord(t *testing.T) {
	good := NecessityAnswerEntry{NecessityQuestionReasonable, NecessityAnswerSupports, "evidence text"}
	cases := []struct {
		name    string
		intent  string
		action  DataAction
		entries []NecessityAnswerEntry
	}{
		{"empty intent", "", DataActionRead, nil},
		{"whitespace intent", "   ", DataActionRead, nil},
		{"absent class", "t", "", nil},
		{"wild class", "t", DataAction("transmit"), nil},
		{"wild question", "t", DataActionRead, []NecessityAnswerEntry{{NecessityQuestion("plausible_step"), NecessityAnswerSupports, "e"}}},
		{"empty question", "t", DataActionRead, []NecessityAnswerEntry{{"", NecessityAnswerSupports, "e"}}},
		{"repeated question", "t", DataActionRead, []NecessityAnswerEntry{good, good}},
		{"wild answer", "t", DataActionRead, []NecessityAnswerEntry{{NecessityQuestionReasonable, NecessityAnswer("probably"), "e"}}},
		{"empty answer", "t", DataActionRead, []NecessityAnswerEntry{{NecessityQuestionReasonable, "", "e"}}},
		{"empty evidence", "t", DataActionRead, []NecessityAnswerEntry{{NecessityQuestionReasonable, NecessityAnswerSupports, ""}}},
		{"whitespace evidence", "t", DataActionRead, []NecessityAnswerEntry{{NecessityQuestionReasonable, NecessityAnswerSupports, "   "}}},
		{"late bad entry keeps early good out", "t", DataActionRead, []NecessityAnswerEntry{good, {NecessityQuestionNecessary, NecessityAnswer("maybe"), "e"}}},
	}
	for _, c := range cases {
		row, err := BuildActionNecessity(c.intent, c.action, c.entries)
		if err == nil {
			t.Errorf("%s: rejection missing", c.name)
		}
		if row.TaskIntent != "" || row.Action != "" || len(row.Answers) != 0 ||
			row.Assessed || row.RefutedPresent || row.EnforcementPlane != "" {
			t.Errorf("%s: partial record leaked past rejection: %+v", c.name, row)
		}
	}
	// Order is kept verbatim and nothing is deduplicated by
	// reordering: input sequence is the recorder's sequence.
	row, err := BuildActionNecessity("t", DataActionExport, []NecessityAnswerEntry{
		{NecessityQuestionSubstitutable, NecessityAnswerSupports, "e1"},
		{NecessityQuestionReasonable, NecessityAnswerRefutes, "e2"},
	})
	if err != nil || len(row.Answers) != 2 ||
		row.Answers[0].Question != NecessityQuestionSubstitutable ||
		row.Answers[1].Question != NecessityQuestionReasonable {
		t.Errorf("input order pin broken: err=%v row=%+v", err, row)
	}
}

// TestActionNecessityFieldSetCarriesNoDecision reflects over the
// record so no later edit can bolt an outcome onto the necessity
// seed: exactly six fields, and no field name anywhere resembling a
// decision.
func TestActionNecessityFieldSetCarriesNoDecision(t *testing.T) {
	tp := reflect.TypeOf(ActionNecessityRecord{})
	if tp.NumField() != 6 {
		t.Fatalf("field set drifted (want 6): %d", tp.NumField())
	}
	red := regexp.MustCompile(`(?i)decision|allow|block|verdict|outcome`)
	for i := 0; i < tp.NumField(); i++ {
		if red.MatchString(tp.Field(i).Name) {
			t.Errorf("field %q resembles a decision - forbidden while the plane reads none", tp.Field(i).Name)
		}
	}
}

// TestActionNecessitySymbolsStayOffTheDecisionPlane walks the four
// decision-plane directories and the command tree for this slice's
// symbols while the enforcement plane reads none-in-observation-phase,
// with a planted-shape positive control so the walk proves its own
// teeth before any green is believed.
func TestActionNecessitySymbolsStayOffTheDecisionPlane(t *testing.T) {
	needle := "NecessityQuestion|NecessityAnswer|AllNecessityQuestions|AllNecessityAnswers|ActionNecessityRecord|BuildActionNecessity|ActionNecessityDangerOnlyRule|ActionNecessityEvidenceRule|ActionNecessityAbsentDefault|ActionNecessityEnforcementPlane"
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
				t.Errorf("decision-plane file %s references action necessity symbol %q", filepath.Join(d, e.Name()), loc)
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
				t.Errorf("command tree file %s references action necessity symbol %q", p, loc)
			}
		}
	}
	walkCmd(filepath.Join("..", "..", "cmd"))
	if cmdHits == 0 {
		// Positive control: the same regexp must find the symbols in
		// this slice's own shipped source before any green from the
		// walk is believed.
		self, err := os.ReadFile("actionnecessity.go")
		if err != nil {
			t.Fatalf("read self: %v", err)
		}
		if !re.MatchString(string(self)) {
			t.Error("positive control broken: needle matches no shipped action necessity source")
		}
	}
}

// TestActionNecessityDocsSync pins the six section 18 keys verbatim
// against the Go constants and vocabularies, completing the
// docs<->Go<->Node-checker three-way mirror.
func TestActionNecessityDocsSync(t *testing.T) {
	raw, err := os.ReadFile(filepath.Join("..", "..", "docs", "schema-v2.md"))
	if err != nil {
		t.Fatalf("read docs/schema-v2.md: %v", err)
	}
	doc := string(raw)
	if !strings.Contains(doc, "## 18. Action necessity record contract") {
		t.Fatal("section 18 header missing from docs/schema-v2.md")
	}
	pins := [][2]string{
		{"action_necessity_question_vocabulary", strings.Join(AllNecessityQuestions(), ",")},
		{"action_necessity_answer_vocabulary", strings.Join(AllNecessityAnswers(), ",")},
		{"action_necessity_danger_only_rule", ActionNecessityDangerOnlyRule},
		{"action_necessity_evidence_rule", ActionNecessityEvidenceRule},
		{"action_necessity_absent_default", ActionNecessityAbsentDefault},
		{"action_necessity_enforcement_plane", ActionNecessityEnforcementPlane},
	}
	for _, p := range pins {
		line := p[0] + ": " + p[1]
		if !strings.Contains(doc, line) {
			t.Errorf("docs section 18 drifted, want line %q", line)
		}
	}
}
