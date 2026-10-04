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

func costU(v uint64) *uint64 { return &v }

func costStated(field CostProxyField, scope AgencyCounterScope, scopeID, source string, v *uint64) CostProxyRecordInput {
	return CostProxyRecordInput{
		Field:   field,
		Shape:   costShapeOf(field),
		Scope:   scope,
		ScopeID: scopeID,
		Source:  source,
		Stated:  v,
	}
}

func TestCostProxyFieldVocabularyPin(t *testing.T) {
	want := []string{
		"runtime_duration_millis", "billable_request_count",
		"cpu_time_millis", "memory_mib_millis", "llm_invocation_count",
	}
	got := AllCostProxyFields()
	if len(got) != 5 || strings.Join(got, ",") != strings.Join(want, ",") {
		t.Fatalf("cost proxy vocabulary drifted from the normative order: %v", got)
	}
	if CostProxyField("max_mood").Valid() || CostProxyField("").Valid() {
		t.Error("wild or empty proxy field must not validate")
	}
	if !CostProxyField("llm_invocation_count").Valid() {
		t.Error("fifth proxy field must validate")
	}
	if strings.Join(AllCostProxyShapes(), ",") != "count,duration" {
		t.Error("shape pair drifted")
	}
	if strings.Join(AllCostValueStates(), ",") != "stated_observed,known_gap,gate_evidenced_zero" {
		t.Error("value state triple drifted")
	}
	if strings.Join(AllCostCollectorStandings(), ",") != "stated_by_recorder,no_collector_known_gap,gate_evidenced_zero_line" {
		t.Error("collector standing triple drifted")
	}
	for _, bad := range []CostProxyShape{CostProxyShape("bytes"), CostProxyShape("")} {
		if bad.Valid() {
			t.Errorf("wild shape %q must not validate", string(bad))
		}
	}
}

func TestCostProxyCoverageRows(t *testing.T) {
	rows := AllCostProxyCoverage()
	if len(rows) != 5 {
		t.Fatalf("coverage rows %d, want 5", len(rows))
	}
	want := [][3]string{
		{"runtime_duration_millis", "duration", "stated_by_recorder"},
		{"billable_request_count", "count", "stated_by_recorder"},
		{"cpu_time_millis", "duration", "no_collector_known_gap"},
		{"memory_mib_millis", "duration", "no_collector_known_gap"},
		{"llm_invocation_count", "count", "gate_evidenced_zero_line"},
	}
	for i, r := range rows {
		w := want[i]
		if string(r.Field) != w[0] || string(r.Shape) != w[1] || string(r.Collector) != w[2] {
			t.Errorf("coverage row %d drifted: %v %v %v", i+1, r.Field, r.Shape, r.Collector)
		}
		if !r.Field.Valid() || !r.Shape.Valid() || !r.Collector.Valid() {
			t.Errorf("coverage row %d carries an out-of-vocabulary token", i+1)
		}
		if got := costShapeOf(r.Field); got != r.Shape {
			t.Errorf("shape derivation drifted at row %d", i+1)
		}
	}
	if costShapeOf(CostProxyField("max_cpu")) != "" {
		t.Error("a field outside the closed five must derive no shape")
	}
}

func TestCostStatedRecordGolden(t *testing.T) {
	rec, err := BuildCostProxyRecord(costStated(CostProxyRuntimeDuration, ScopeTask, "task-77", "wall-clock sampler", costU(183400)))
	if err != nil {
		t.Fatalf("stated record rejected: %v", err)
	}
	if rec.State != CostStateStated || rec.Value == nil || *rec.Value != 183400 {
		t.Errorf("stated line drifted: %+v", rec)
	}
	if rec.Shape != CostShapeDuration || rec.Source != "wall-clock sampler" {
		t.Errorf("restatement drifted: %+v", rec)
	}
	if rec.Absence != CostAbsenceRule || rec.Response != CostGuardResponseRule || rec.Enforcement != CostGuardEnforcementPlane {
		t.Error("constructor-pinned rule lines drifted")
	}
	data, err := json.Marshal(rec)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	if !strings.Contains(string(data), "\"value\":183400") || !strings.Contains(string(data), "\"source\":\"wall-clock sampler\"") {
		t.Errorf("wire shape lost the stated lines: %s", data)
	}
}

func TestCostKnownGapAbsenceNotZero(t *testing.T) {
	gap, err := BuildCostProxyRecord(costStated(CostProxyCPUTime, ScopeAgent, "agent-9", "", nil))
	if err != nil {
		t.Fatalf("known_gap record rejected: %v", err)
	}
	if gap.State != CostStateKnownGap || gap.Value != nil || gap.Source != "" {
		t.Errorf("known_gap line must carry no value and no source: %+v", gap)
	}
	data, err := json.Marshal(gap)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	// The machine-check assertion line of slice W6.2: absence is
	// literally absent on the wire - no value key to be confused
	// with a reading - while an explicit zero with its source
	// stays a present, distinguishable line (two-shapes stance).
	if strings.Contains(string(data), "\"value\":") || strings.Contains(string(data), "\"source\":") {
		t.Errorf("known_gap wire shape fabricated an entrance: %s", data)
	}
	zero, err := BuildCostProxyRecord(costStated(CostProxyBillableRequest, ScopeTask, "task-77", "audit stream tally", costU(0)))
	if err != nil {
		t.Fatalf("stated zero rejected: %v", err)
	}
	zdata, err := json.Marshal(zero)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	if !strings.Contains(string(zdata), "\"value\":0") {
		t.Errorf("an explicitly stated zero must stay stated: %s", zdata)
	}
	if string(zero.State) != "stated_observed" || gap.State == zero.State {
		t.Error("stated zero and honest absence must not collapse into one state")
	}
}

func TestCostRejectionsLeaveNoRecord(t *testing.T) {
	cases := []struct {
		name string
		in   CostProxyRecordInput
	}{
		{"wild field", CostProxyRecordInput{Field: CostProxyField("max_cpu"), Shape: CostShapeDuration, Scope: ScopeTask, ScopeID: "t", Source: "s", Stated: costU(1)}},
		{"wild shape", CostProxyRecordInput{Field: CostProxyRuntimeDuration, Shape: CostProxyShape("bytes"), Scope: ScopeTask, ScopeID: "t", Source: "s", Stated: costU(1)}},
		{"shape mismatching the table", CostProxyRecordInput{Field: CostProxyRuntimeDuration, Shape: CostShapeCount, Scope: ScopeTask, ScopeID: "t", Source: "s", Stated: costU(1)}},
		{"wild scope", CostProxyRecordInput{Field: CostProxyBillableRequest, Shape: CostShapeCount, Scope: AgencyCounterScope("global"), ScopeID: "t", Source: "s", Stated: costU(1)}},
		{"empty scope id", CostProxyRecordInput{Field: CostProxyBillableRequest, Shape: CostShapeCount, Scope: ScopeTask, ScopeID: "  ", Source: "s", Stated: costU(1)}},
		{"value without source", CostProxyRecordInput{Field: CostProxyMemoryHeld, Shape: CostShapeDuration, Scope: ScopeAgent, ScopeID: "a", Source: "", Stated: costU(4096)}},
		{"source without value", CostProxyRecordInput{Field: CostProxyMemoryHeld, Shape: CostShapeDuration, Scope: ScopeAgent, ScopeID: "a", Source: "s", Stated: nil}},
	}
	for _, c := range cases {
		rec, err := BuildCostProxyRecord(c.in)
		if err == nil {
			t.Errorf("%s: accepted", c.name)
		}
		if rec != (CostProxyRecord{}) {
			t.Errorf("%s: left a half record behind: %+v", c.name, rec)
		}
	}
}

func TestCostLadderTokensNeverStated(t *testing.T) {
	for _, tok := range AgencyForbiddenActionTokens() {
		if _, err := BuildCostProxyRecord(costStated(CostProxyRuntimeDuration, ScopeTask, tok, "sampler", costU(5))); err == nil {
			t.Errorf("scope_id smuggling the ladder token %q was accepted", tok)
		}
		if _, err := BuildCostProxyRecord(costStated(CostProxyRuntimeDuration, ScopeTask, "task-1", tok, costU(5))); err == nil {
			t.Errorf("source smuggling the ladder token %q was accepted", tok)
		}
	}
	rec, err := BuildCostProxyRecord(costStated(CostProxyRuntimeDuration, ScopeTask, "task-1", "sampler", costU(5)))
	if err != nil {
		t.Fatalf("clean record rejected: %v", err)
	}
	typ := reflect.TypeOf(rec)
	val := reflect.ValueOf(rec)
	for i := 0; i < typ.NumField(); i++ {
		if val.Field(i).Kind() != reflect.String {
			continue
		}
		s := val.Field(i).String()
		for _, tok := range AgencyForbiddenActionTokens() {
			if s == tok {
				t.Errorf("record field %s carries a bare ladder token", typ.Field(i).Name)
			}
		}
	}
	truth, err := BuildCostTruthRecord(ScopeTask, "task-1")
	if err != nil {
		t.Fatalf("truth record rejected: %v", err)
	}
	tval := reflect.ValueOf(truth)
	ttyp := reflect.TypeOf(truth)
	for i := 0; i < ttyp.NumField(); i++ {
		if tval.Field(i).Kind() == reflect.String {
			for _, tok := range AgencyForbiddenActionTokens() {
				if tval.Field(i).String() == tok {
					t.Errorf("truth field %s carries a bare ladder token", ttyp.Field(i).Name)
				}
			}
		}
	}
}

func TestCostLLMZeroGateRecord(t *testing.T) {
	rec, err := BuildCostLLMZeroRecord(ScopeTask, "task-77")
	if err != nil {
		t.Fatalf("gate zero record rejected: %v", err)
	}
	if rec.Field != CostProxyLLMInvocations || rec.State != CostStateGateZero || rec.Value == nil || *rec.Value != 0 {
		t.Errorf("gate zero line drifted: %+v", rec)
	}
	if rec.Source != CostZeroLLMStatement || rec.Shape != CostShapeCount {
		t.Errorf("pinned evidence line drifted: %+v", rec)
	}
	data, err := json.Marshal(rec)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	if !strings.Contains(string(data), "\"value\":0") || !strings.Contains(string(data), "\"value_state\":\"gate_evidenced_zero\"") {
		t.Errorf("wire shape lost the evidenced zero: %s", data)
	}
	// The gate-evidenced state has exactly one door: the named
	// constructor for the named field. The normal path cannot
	// reach it, so no other proxy can borrow a proven zero.
	typ := reflect.TypeOf(CostProxyRecordInput{})
	for _, name := range []string{"State", "Verdict"} {
		if _, ok := typ.FieldByName(name); ok {
			t.Errorf("input must not expose a %s entrance; the state is constructor-pinned", name)
		}
	}
	if _, err := BuildCostLLMZeroRecord(AgencyCounterScope("global"), "x"); err == nil {
		t.Error("gate zero accepted a wild scope")
	}
	if _, err := BuildCostLLMZeroRecord(ScopeAgent, ""); err == nil {
		t.Error("gate zero accepted an unstated scope id")
	}
}

func TestCostTruthRecordHasNoValueEntrance(t *testing.T) {
	rec, err := BuildCostTruthRecord(ScopeTask, "task-77")
	if err != nil {
		t.Fatalf("truth record rejected: %v", err)
	}
	typ := reflect.TypeOf(rec)
	for i := 0; i < typ.NumField(); i++ {
		name := strings.ToLower(typ.Field(i).Name)
		if name == "value" || strings.HasPrefix(name, "value") && name != "value_state" {
			t.Errorf("truth record carries a value entrance: %s", typ.Field(i).Name)
		}
	}
	if rec.State != CostStateKnownGap || rec.Stance != CostTruthStance {
		t.Errorf("truth stance drifted: %+v", rec)
	}
	data, err := json.Marshal(rec)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	if strings.Contains(string(data), "\"value\":") {
		t.Errorf("cost truth wire shape carries a number: %s", data)
	}
	if _, err := BuildCostTruthRecord(AgencyCounterScope("global"), "x"); err == nil {
		t.Error("truth record accepted a wild scope")
	}
}

func TestCostRecordsCarryNoDecision(t *testing.T) {
	if n := reflect.TypeOf(CostProxyRecord{}).NumField(); n != 10 {
		t.Errorf("proxy record field census %d, want 10", n)
	}
	if n := reflect.TypeOf(CostTruthRecord{}).NumField(); n != 7 {
		t.Errorf("truth record field census %d, want 7", n)
	}
	typ := reflect.TypeOf(CostProxyRecordInput{})
	if typ.NumField() != 6 {
		t.Errorf("input field census %d, want 6", typ.NumField())
	}
	for i := 0; i < typ.NumField(); i++ {
		name := strings.ToLower(typ.Field(i).Name)
		for _, bad := range []string{"decision", "score", "severity", "verdict", "kill", "halt", "action", "ceiling"} {
			if strings.Contains(name, bad) {
				t.Errorf("input carries a decision-shaped field %s", name)
			}
		}
	}
}

func TestCostSymbolsStayOffTheDecisionPlane(t *testing.T) {
	needle := `CostProxyField|CostProxyShape|CostValueState|CostCollectorStanding|CostProxyCoverageRow|CostProxyRecordInput|CostProxyRecord|CostTruthRecord|BuildCostProxyRecord|BuildCostLLMZeroRecord|BuildCostTruthRecord|AllCostProxy|AllCostValueStates|AllCostCollectorStandings|CostAbsenceRule|CostTruthStance|CostGuardResponseRule|CostGuardEnforcementPlane|CostZeroLLMStatement`
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
				t.Errorf("decision-plane file %s references cost symbol %q", filepath.Join(d, e.Name()), loc)
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
				t.Errorf("command tree file %s references cost symbol %q", p, loc)
			}
		}
	}
	walkCmd(filepath.Join("..", "..", "cmd"))
	if cmdHits == 0 {
		self, err := os.ReadFile("costguard.go")
		if err != nil {
			t.Fatalf("read self: %v", err)
		}
		if !re.MatchString(string(self)) {
			t.Error("positive control broken: needle matches no shipped cost source")
		}
	}
}

func TestCostSurfaceIsStdlibOnly(t *testing.T) {
	data, err := os.ReadFile("costguard.go")
	if err != nil {
		t.Fatalf("read self: %v", err)
	}
	block := string(data)
	start := strings.Index(block, "import (")
	if start < 0 {
		t.Fatal("import block missing")
	}
	block = block[start:]
	block = block[:strings.Index(block, ")")]
	for _, line := range strings.Split(block, "\n") {
		line = strings.TrimSpace(line)
		if line == "" || line == "import (" || line == ")" {
			continue
		}
		if line != `"fmt"` && line != `"strings"` {
			t.Errorf("cost surface imports %s; the zero-outbound closure admits fmt and strings only", line)
		}
	}
	if strings.Contains(block, "net") || strings.Contains(block, "http") {
		t.Error("cost surface must carry no network import (zero-LLM fact and outbound closure stance)")
	}
}

func TestCostGuardDocsSync(t *testing.T) {
	doc := string(mustReadDocs(t))
	if !strings.Contains(doc, "## 22. Cost guard observation record contract") {
		t.Fatal("section 22 header missing from docs/schema-v2.md")
	}
	pins := [][2]string{
		{"cost_proxy_field_vocabulary", strings.Join(AllCostProxyFields(), ",")},
		{"cost_proxy_shape_vocabulary", strings.Join(AllCostProxyShapes(), ",")},
		{"cost_value_state_vocabulary", strings.Join(AllCostValueStates(), ",")},
		{"cost_collector_standing_vocabulary", strings.Join(AllCostCollectorStandings(), ",")},
		{"cost_field_shape_mapping", "runtime_duration_millis=duration,billable_request_count=count,cpu_time_millis=duration,memory_mib_millis=duration,llm_invocation_count=count"},
		{"cost_absence_rule", CostAbsenceRule},
		{"cost_truth_stance", CostTruthStance},
		{"cost_enforcement_plane", CostGuardEnforcementPlane},
	}
	for _, p := range pins {
		line := p[0] + ": " + p[1]
		if !strings.Contains(doc, line) {
			t.Errorf("docs section 22 drifted, want line %q", line)
		}
	}
	rows := AllCostProxyCoverage()
	for _, r := range rows {
		line := "| `" + string(r.Field) + "` | " + string(r.Shape) + " | " + string(r.Collector) + " |"
		if !strings.Contains(doc, line) {
			t.Errorf("docs coverage table drifted, want row %q", line)
		}
	}
	for _, rest := range []string{"record-only-no-action", "gate_evidenced_zero", CostZeroLLMStatement} {
		if !strings.Contains(doc, rest) {
			t.Errorf("docs section 22 lost a restatement: %q", rest)
		}
	}
}
