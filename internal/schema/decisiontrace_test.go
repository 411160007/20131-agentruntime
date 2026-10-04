package schema

import (
	"encoding/json"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
	"time"
)

func traceDecisionLine() *Event {
	return &Event{
		V:        1,
		TS:       time.Date(2026, 10, 5, 1, 30, 0, 0, time.UTC),
		ID:       "dec-0f1e2d3c4b5a6978",
		AgentID:  "agent-codex-1",
		Stage:    StageEvaluated,
		Type:     TypePolicyDecision,
		Decision: DecisionWouldBlock,
		Severity: SevCritical,
		Summary:  "would block: destructive path",
		Attrs: map[string]string{
			"hard":      "true",
			"rule":      "deny-destructive",
			"cap":       "file_write",
			"res_class": "high",
		},
	}
}

func TestTraceFieldVocabularyPin(t *testing.T) {
	got := AllTraceFields()
	if len(got) != 15 || len(strings.Join(got, ",")) == 0 {
		t.Fatalf("trace field table is fifteen tokens, got %d", len(got))
	}
	seen := map[string]bool{}
	for _, w := range got {
		if seen[w] {
			t.Fatalf("duplicate trace field token %q", w)
		}
		seen[w] = true
		if !TraceField(w).Valid() {
			t.Fatalf("token %q fails its own Valid()", w)
		}
	}
	// Spec listing order (section 251): first three and last two.
	want := []string{"decision_id", "timestamp", "agent", "outcome", "recovery_state"}
	if got[0] != want[0] || got[1] != want[1] || got[2] != want[2] || got[13] != want[3] || got[14] != want[4] {
		t.Fatalf("trace field order drifted from the spec listing")
	}
	if TraceField("nope").Valid() || TraceField("").Valid() {
		t.Fatalf("wild or empty token must not validate")
	}
}

func TestTraceStanceVocabularyPin(t *testing.T) {
	got := AllTraceStances()
	if len(got) != 4 {
		t.Fatalf("stance vocabulary wants four, got %d", len(got))
	}
	for _, s := range got {
		if !TraceStance(s).Valid() {
			t.Fatalf("stance %q fails its own Valid()", s)
		}
	}
	if TraceStance("half_carried").Valid() {
		t.Fatal("wild stance accepted")
	}
}

func TestTraceCoverageRows(t *testing.T) {
	rows := AllTraceCoverage()
	if len(rows) != 15 {
		t.Fatalf("coverage table wants fifteen rows, got %d", len(rows))
	}
	counts := map[TraceStance]int{}
	fields := map[TraceField]bool{}
	for _, r := range rows {
		if !r.Field.Valid() || !r.Stance.Valid() {
			t.Fatalf("row %q/%q not both valid", r.Field, r.Stance)
		}
		if fields[r.Field] {
			t.Fatalf("field %q counted twice", r.Field)
		}
		fields[r.Field] = true
		counts[r.Stance]++
		if r.Stance == StanceKnownGap && r.Source != "" {
			t.Fatalf("known_gap row %q carries a source - a gap has none", r.Field)
		}
		if r.Stance != StanceKnownGap && r.Source == "" {
			t.Fatalf("row %q states no audit source", r.Field)
		}
	}
	if len(fields) != 15 {
		t.Fatalf("coverage rows must cover every table field exactly once")
	}
	want := map[TraceStance]int{
		StanceOnAuditLine: 6, StanceViaAttrs: 3,
		StanceAdditive: 2, StanceKnownGap: 4,
	}
	for s, n := range want {
		if counts[s] != n {
			t.Fatalf("stance census %q: want %d got %d", s, n, counts[s])
		}
	}
}

func TestBuildFromDecisionAuditRow(t *testing.T) {
	ev := traceDecisionLine()
	tr, err := BuildDecisionTrace(ev, "evt-0001", "builtin-v1")
	if err != nil {
		t.Fatalf("build: %v", err)
	}
	// Dual-source sync: every copied line field equals the audit row.
	if tr.DecisionID != ev.ID || tr.Agent != ev.AgentID || tr.Decision != string(ev.Decision) {
		t.Fatal("trace drifted from its audit line")
	}
	if tr.Timestamp != "2026-10-05T01:30:00Z" || tr.Action != string(TypePolicyDecision) {
		t.Fatalf("timestamp/action pin broke: %q %q", tr.Timestamp, tr.Action)
	}
	if tr.RiskFactors != "severity=4|hard=true" {
		t.Fatalf("risk factors rendering: %q", tr.RiskFactors)
	}
	if tr.PolicyRule != "deny-destructive" || tr.Capability != "file_write" || tr.Resource != "high" {
		t.Fatal("attrs-backed fields not copied verbatim")
	}
	if ev.Attrs["rule"] != "deny-destructive" {
		t.Fatal("source event mutated")
	}
}

func TestTraceKnownGapFieldsAbsentOnWire(t *testing.T) {
	tr, err := BuildDecisionTrace(traceDecisionLine(), "evt-0001", "builtin-v1")
	if err != nil {
		t.Fatalf("build: %v", err)
	}
	if strings.Join(tr.KnownGapFields, ",") != "task,intent,outcome,recovery_state" {
		t.Fatalf("gap list must be the four known_gap rows in table order, got %v", tr.KnownGapFields)
	}
	data, err := json.Marshal(tr)
	if err != nil {
		t.Fatal(err)
	}
	var m map[string]any
	if err := json.Unmarshal(data, &m); err != nil {
		t.Fatal(err)
	}
	for _, k := range []string{"task", "intent", "outcome", "recovery_state"} {
		if v, ok := m[k]; ok {
			t.Fatalf("known_gap field %q must be absent from the wire, got %v", k, v)
		}
	}
	if len(m) != 14 {
		t.Fatalf("trace wire key census wants 14, got %d", len(m))
	}
	for _, f := range []TraceField{FieldTask, FieldIntent, FieldOutcome, FieldRecoveryState} {
		if tr.Stated(f) {
			t.Fatalf("Stated(%q) must be false for a known_gap field", f)
		}
	}
}

func TestTraceUnrecordedAttrsNamedNotSilent(t *testing.T) {
	ev := traceDecisionLine()
	delete(ev.Attrs, "cap")
	delete(ev.Attrs, "res_class")
	tr, err := BuildDecisionTrace(ev, "evt-0001", "builtin-v1")
	if err != nil {
		t.Fatalf("build: %v", err)
	}
	if strings.Join(tr.UnrecordedFields, ",") != "resource,capability" {
		t.Fatalf("unrecorded list must follow table order, got %v", tr.UnrecordedFields)
	}
	data, _ := json.Marshal(tr)
	if strings.Contains(string(data), `"capability":""`) || strings.Contains(string(data), `"resource":""`) {
		t.Fatal("unrecorded attrs must be absent, not empty-string in disguise")
	}
	if tr.Stated(FieldCapability) || tr.Stated(FieldResource) {
		t.Fatal("Stated() must report the absence")
	}
}

func TestTraceRejectsLeaveNoRecord(t *testing.T) {
	base := traceDecisionLine()
	other := traceDecisionLine()
	other.Type = TypeToolCall
	noID := traceDecisionLine()
	noID.ID = ""
	noAgent := traceDecisionLine()
	noAgent.AgentID = ""
	noTS := traceDecisionLine()
	noTS.TS = time.Time{}
	badDec := traceDecisionLine()
	badDec.Decision = DecisionAsk
	badSev := traceDecisionLine()
	badSev.Severity = Severity(9)
	noHard := traceDecisionLine()
	delete(noHard.Attrs, "hard")
	ladder := traceDecisionLine()
	ladder.Attrs["rule"] = "LIMIT"
	cases := []struct {
		name   string
		ev     *Event
		origin string
		pver   string
	}{
		{"nil event", nil, "evt-0001", "builtin-v1"},
		{"wrong type", other, "evt-0001", "builtin-v1"},
		{"empty id", noID, "evt-0001", "builtin-v1"},
		{"empty agent", noAgent, "evt-0001", "builtin-v1"},
		{"zero ts", noTS, "evt-0001", "builtin-v1"},
		{"ask outside phase0", badDec, "evt-0001", "builtin-v1"},
		{"wild severity", badSev, "evt-0001", "builtin-v1"},
		{"missing hard annotation", noHard, "evt-0001", "builtin-v1"},
		{"ladder token collision", ladder, "evt-0001", "builtin-v1"},
		{"empty origin", base, "", "builtin-v1"},
		{"self-correlation", base, base.ID, "builtin-v1"},
		{"unstated policy version", base, "evt-0001", ""},
		{"policy version with space", base, "evt-0001", "builtin v1"},
		{"policy version non ascii", base, "evt-0001", "builtin-µ"},
	}
	for _, c := range cases {
		tr, err := BuildDecisionTrace(c.ev, c.origin, c.pver)
		if err == nil || tr != nil {
			t.Fatalf("%s: want rejection with no record, got %v/%v", c.name, tr, err)
		}
	}
}

func TestTracePolicyRuleDefaultApplied(t *testing.T) {
	ev := traceDecisionLine()
	delete(ev.Attrs, "rule")
	tr, err := BuildDecisionTrace(ev, "evt-0001", "builtin-v1")
	if err != nil {
		t.Fatalf("build: %v", err)
	}
	if tr.PolicyRule != TraceDefaultApplied {
		t.Fatalf("empty rule must restate as %q, got %q", TraceDefaultApplied, tr.PolicyRule)
	}
	if TraceDefaultAppliedRule != "empty-rule-means-default-applied" {
		t.Fatal("default-applied rule constant drifted")
	}
}

func TestTraceEnforcementModePinned(t *testing.T) {
	tr, err := BuildDecisionTrace(traceDecisionLine(), "evt-0001", "builtin-v1")
	if err != nil {
		t.Fatalf("build: %v", err)
	}
	if tr.EnforcementMode != TraceEnforcementModeValue {
		t.Fatalf("mode must be constructor-pinned, got %q", tr.EnforcementMode)
	}
	if TraceEnforcementModeValue != "record_only_phase0" || TraceEnforcementPlane != "none-in-observation-phase" {
		t.Fatal("pinned stance constants drifted")
	}
}

func TestEncodeTraceCheckedDoor(t *testing.T) {
	tr, err := BuildDecisionTrace(traceDecisionLine(), "evt-0001", "builtin-v1")
	if err != nil {
		t.Fatalf("build: %v", err)
	}
	data, err := EncodeTraceChecked(tr)
	if err != nil {
		t.Fatalf("encode: %v", err)
	}
	var back DecisionTrace
	if err := json.Unmarshal(data, &back); err != nil {
		t.Fatal(err)
	}
	if back.DecisionID != tr.DecisionID || strings.Join(back.KnownGapFields, ",") != strings.Join(tr.KnownGapFields, ",") {
		t.Fatal("round-trip broke")
	}
	hand := traceDecisionLine()
	hand2, err := BuildDecisionTrace(hand, "evt-0002", "builtin-v1")
	if err != nil {
		t.Fatal(err)
	}
	hand2.KnownGapFields = []string{"task"} // smuggled hand-edit
	if _, err := EncodeTraceChecked(hand2); err == nil {
		t.Fatal("EncodeTraceChecked must refuse drifted gap lists")
	}
}

func TestTraceSymbolsStayOffTheDecisionPlane(t *testing.T) {
	needle := `TraceField|AllTraceFields|TraceStance|AllTraceStances|TraceCoverageRow|AllTraceCoverage|TraceStanceOf|DecisionTrace|BuildDecisionTrace|TraceAbsenceRule|TraceCorrelationRule|TraceEnforcementModeValue|TraceEnforcementPlane|TraceDefaultApplied|EncodeTraceChecked|TraceDefaultAppliedRule`
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
				t.Fatalf("read %s: %v", d, e.Name())
			}
			if loc := re.FindString(string(data)); loc != "" {
				t.Errorf("decision-plane file %s references trace symbol %q", e.Name(), loc)
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
				t.Errorf("command tree file %s references trace symbol %q", p, loc)
				cmdHits++
			}
		}
	}
	walkCmd(filepath.Join("..", "..", "cmd"))
	if cmdHits == 0 {
		// Positive control: the scan must have teeth - prove the
		// needle fires on a planted reference before trusting zero.
		plant := filepath.Join(t.TempDir(), "plant.go")
		src := "package main\nvar _ = BuildDecisionTrace\n"
		if err := os.WriteFile(plant, []byte(src), 0o600); err != nil {
			t.Fatal(err)
		}
		data, _ := os.ReadFile(plant)
		if !re.Match(data) {
			t.Fatal("planeLeak needle set does not fire on the planted shape")
		}
	}
}

func TestTraceSurfaceIsStdlibOnly(t *testing.T) {
	data, err := os.ReadFile("decisiontrace.go")
	if err != nil {
		t.Fatal(err)
	}
	imports := regexp.MustCompile(`(?m)^\t"([a-z0-9/._-]+)"$`).FindAllStringSubmatch(string(data), -1)
	for _, m := range imports {
		if strings.Contains(m[1], ".") && !strings.HasPrefix(m[1], "encoding/") {
			t.Fatalf("non-stdlib import %q", m[1])
		}
	}
}

func TestTraceDocsSync(t *testing.T) {
	doc, err := os.ReadFile(filepath.Join("..", "..", "docs", "schema-v2.md"))
	if err != nil {
		t.Fatal(err)
	}
	text := string(doc)
	lines := strings.Split(text, "\n")
	for _, tok := range AllTraceFields() {
		found := false
		for _, ln := range lines {
			if strings.Contains(ln, "| `"+tok+"` ") {
				found = true
				break
			}
		}
		if !found {
			t.Fatalf("docs table row missing for trace field %q", tok)
		}
	}
	for _, k := range [][2]string{
		{"decision_trace_absence_rule", TraceAbsenceRule},
		{"decision_trace_correlation_rule", TraceCorrelationRule},
		{"decision_trace_enforcement_mode", TraceEnforcementModeValue},
		{"decision_trace_enforcement_plane", TraceEnforcementPlane},
		{"decision_trace_default_applied_rule", TraceDefaultAppliedRule},
	} {
		if !strings.Contains(text, k[0]+": "+k[1]) {
			t.Fatalf("docs key line drifted or missing: %s", k[0])
		}
	}
}
