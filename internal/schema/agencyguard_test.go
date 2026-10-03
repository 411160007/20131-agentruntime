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

func agencyEvents(n int, stated uint64) []AgencyCounterEvent {
	ev := make([]AgencyCounterEvent, n)
	for i := range ev {
		ev[i] = AgencyCounterEvent{Sequence: uint64(i + 1), StatedValue: stated}
	}
	return ev
}

func TestAgencyGuardFieldVocabularyPin(t *testing.T) {
	want := []string{
		"max_steps", "max_runtime", "max_tool_calls", "max_network_requests",
		"max_parallelism", "max_cpu", "max_memory", "max_storage",
		"max_api_cost", "max_child_agents",
	}
	got := AllAgencyGuardFields()
	if len(got) != 10 || strings.Join(got, ",") != strings.Join(want, ",") {
		t.Fatalf("guard field vocabulary drifted from the section 261 order: %v", got)
	}
	if AgencyGuardField("max_mood").Valid() || AgencyGuardField("").Valid() {
		t.Error("wild or empty guard field must not validate")
	}
	if !AgencyGuardField("max_child_agents").Valid() {
		t.Error("tenth guard field must validate")
	}
	if strings.Join(AllAgencyCounterKinds(), ",") != "event_rate,step_count,parallelism" {
		t.Error("counter vocabulary drifted from the taskbook trio")
	}
	if strings.Join(AllAgencyCounterScopes(), ",") != "agent,task" {
		t.Error("scope vocabulary drifted from the closed pair")
	}
	if strings.Join(AllAgencyLimitStates(), ",") != "below_ceiling,at_ceiling,above_ceiling" {
		t.Error("limit state vocabulary drifted from the closed three")
	}
	if AgencyCounterKind("").Valid() || AgencyCounterScope("").Valid() || AgencyLimitState("").Valid() {
		t.Error("empty strings are never vocabulary members")
	}
	pins := [][2]string{
		{"agency_count_conservation_rule", AgencyCountConservationRule},
		{"agency_action_enum_gate", AgencyActionEnumGate},
		{"agency_enforcement_plane", AgencyEnforcementPlane},
	}
	for _, p := range pins {
		if !strings.Contains(string(mustReadDocs(t)), p[0]+": "+p[1]) {
			t.Errorf("rule constant %s not pinned verbatim in docs", p[0])
		}
	}
}

func mustReadDocs(t *testing.T) []byte {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join("..", "..", "docs", "schema-v2.md"))
	if err != nil {
		t.Fatalf("read docs/schema-v2.md: %v", err)
	}
	return raw
}

func TestAgencyGuardCoverageRows(t *testing.T) {
	rows := AllAgencyGuardCoverage()
	if len(rows) != 10 {
		t.Fatalf("coverage registration %d rows, want 10/10 zero dangling", len(rows))
	}
	fields := AllAgencyGuardFields()
	wantPhase := map[string]string{
		"max_steps": "counted_v0", "max_runtime": "anchored", "max_tool_calls": "counted_v0",
		"max_network_requests": "counted_v0", "max_parallelism": "counted_v0",
		"max_cpu": "pending", "max_memory": "pending", "max_storage": "anchored",
		"max_api_cost": "anchored", "max_child_agents": "counted_v0",
	}
	wantAnchor := map[string]string{
		"max_runtime": "W6.2", "max_storage": "W8.2", "max_api_cost": "W6.2",
	}
	for i, r := range rows {
		if string(r.Field) != fields[i] {
			t.Fatalf("coverage row %d is not in section 261 order: %s", i+1, r.Field)
		}
		if !r.Phase.Valid() {
			t.Fatalf("coverage row %d carries a wild phase", i+1)
		}
		if wantPhase[string(r.Field)] != string(r.Phase) {
			t.Errorf("coverage phase drifted at %s: %s", r.Field, r.Phase)
		}
		if wantAnchor[string(r.Field)] != r.Anchor {
			t.Errorf("coverage anchor drifted at %s: %q", r.Field, r.Anchor)
		}
		if r.Phase != GuardPhaseAnchored && r.Anchor != "" {
			t.Errorf("non-anchored row %s carries an anchor", r.Field)
		}
	}
	doc := string(mustReadDocs(t))
	for i, r := range rows {
		cell := r.Anchor
		if cell == "" {
			cell = "-"
		}
		row := "| `" + string(r.Field) + "` | " + string(r.Phase) + " | " + cell + " |"
		if !strings.Contains(doc, row) {
			t.Errorf("docs coverage table missing row %d: %s", i+1, row)
		}
	}
}

func TestAgencyCounterConservationGolden(t *testing.T) {
	// Machine judgement item one: N events, count conserved exactly.
	in := AgencyCounterAggregateInput{
		Scope: ScopeTask, ScopeID: "task-6-1", Counter: CounterStepCount,
		EventClass: "step", Events: agencyEvents(40, 1),
	}
	agg, err := BuildAgencyCounterAggregate(in)
	if err != nil {
		t.Fatalf("golden step stream rejected: %v", err)
	}
	if agg.EventCount != 40 || agg.ObservedValue != 40 {
		t.Fatalf("step count not conserved: count=%d observed=%d", agg.EventCount, agg.ObservedValue)
	}
	rate, err := BuildAgencyCounterAggregate(AgencyCounterAggregateInput{
		Scope: ScopeAgent, ScopeID: "agent-a", Counter: CounterEventRate,
		EventClass: "tool_call", Events: agencyEvents(7, 1),
	})
	if err != nil || rate.EventCount != 7 || rate.ObservedValue != 7 {
		t.Fatalf("event-rate conservation broke: %+v err=%v", rate, err)
	}
	par, err := BuildAgencyCounterAggregate(AgencyCounterAggregateInput{
		Scope: ScopeAgent, ScopeID: "agent-a", Counter: CounterParallelism,
		EventClass: "active-set", Events: []AgencyCounterEvent{{1, 2}, {2, 5}, {3, 3}, {4, 0}},
	})
	if err != nil || par.ObservedValue != 5 || par.EventCount != 4 {
		t.Fatalf("parallelism high-water broke: %+v err=%v", par, err)
	}
	// Sequence holes, duplicates, and zero-start are all refused.
	for name, ev := range map[string][]AgencyCounterEvent{
		"gap":      {{1, 1}, {3, 1}},
		"dup":      {{1, 1}, {2, 1}, {2, 1}},
		"disorder": {{2, 1}, {1, 1}},
		"zero-seq": {{0, 1}, {1, 1}},
		"empty":    {},
		"stated-2": {{1, 2}},
	} {
		_, err := BuildAgencyCounterAggregate(AgencyCounterAggregateInput{
			Scope: ScopeTask, ScopeID: "t", Counter: CounterStepCount,
			EventClass: "step", Events: ev,
		})
		if err == nil {
			t.Errorf("conservation gate accepted the %s stream", name)
		}
	}
}

func TestAgencyLimitRecordGoldenPair(t *testing.T) {
	// Machine judgement item two: a tripped ceiling is a record,
	// never a behaviour change. Same stream, with and without a
	// declared ceiling, must produce byte-identical aggregates.
	events := agencyEvents(5, 1)
	base, err := BuildAgencyCounterAggregate(AgencyCounterAggregateInput{
		Scope: ScopeAgent, ScopeID: "agent-run", Counter: CounterStepCount,
		EventClass: "step", Events: events,
	})
	if err != nil {
		t.Fatal(err)
	}
	b1, err := json.Marshal(base)
	if err != nil {
		t.Fatal(err)
	}
	hit, err := BuildAgencyLimitRecord(AgencyLimitRecordInput{
		GuardField: GuardMaxSteps, Counter: CounterStepCount, Scope: ScopeAgent,
		ScopeID: "agent-run", Ceiling: 4, Observed: base.ObservedValue,
	})
	if err != nil || !hit.Triggered || hit.State != LimitAbove {
		t.Fatalf("a tripped ceiling must build, not fail or mis-state: %+v err=%v", hit, err)
	}
	after, err := BuildAgencyCounterAggregate(AgencyCounterAggregateInput{
		Scope: ScopeAgent, ScopeID: "agent-run", Counter: CounterStepCount,
		EventClass: "step", Events: events,
	})
	if err != nil {
		t.Fatal(err)
	}
	b2, _ := json.Marshal(after)
	if string(b1) != string(b2) {
		t.Fatalf("limit record changed the counter bytes: %s vs %s", b1, b2)
	}
	if !json.Valid(append([]byte{}, b1...)) {
		t.Fatal("aggregate not JSON valid")
	}
	clean, err := BuildAgencyLimitRecord(AgencyLimitRecordInput{
		GuardField: GuardMaxParallelism, Counter: CounterParallelism, Scope: ScopeTask,
		ScopeID: "task-c", Ceiling: 4, Observed: 3,
	})
	if err != nil || clean.State != LimitBelow || clean.Triggered {
		t.Fatalf("below-ceiling golden broke: %+v err=%v", clean, err)
	}
	at, err := BuildAgencyLimitRecord(AgencyLimitRecordInput{
		GuardField: GuardMaxSteps, Counter: CounterStepCount, Scope: ScopeTask,
		ScopeID: "task-c", Ceiling: 5, Observed: 5,
	})
	if err != nil || at.State != LimitAt || !at.Triggered {
		t.Fatalf("at-ceiling golden broke: %+v err=%v", at, err)
	}
	for name, in := range map[string]AgencyLimitRecordInput{
		"wild-field": {GuardField: "max_mood", Counter: CounterStepCount, Scope: ScopeTask, ScopeID: "t"},
		"wild-count": {GuardField: GuardMaxSteps, Counter: "quota", Scope: ScopeTask, ScopeID: "t"},
		"wild-scope": {GuardField: GuardMaxSteps, Counter: CounterStepCount, Scope: "", ScopeID: "t"},
		"empty-id":   {GuardField: GuardMaxSteps, Counter: CounterStepCount, Scope: ScopeTask, ScopeID: "  "},
	} {
		if _, err := BuildAgencyLimitRecord(in); err == nil {
			t.Errorf("limit gate accepted the %s input", name)
		}
	}
}

func TestAgencyActionTokensNeverRecorded(t *testing.T) {
	// Machine judgement item three: the Phase 1 ladder enum has
	// zero writes into any product-path record value.
	forbid := map[string]bool{}
	for _, tok := range AgencyForbiddenActionTokens() {
		forbid[tok] = true
	}
	rec, err := BuildAgencyLimitRecord(AgencyLimitRecordInput{
		GuardField: GuardMaxSteps, Counter: CounterStepCount, Scope: ScopeTask,
		ScopeID: "task-x", Ceiling: 0, Observed: 9,
	})
	if err != nil {
		t.Fatal(err)
	}
	agg, err := BuildAgencyCounterAggregate(AgencyCounterAggregateInput{
		Scope: ScopeTask, ScopeID: "task-x", Counter: CounterEventRate,
		EventClass: "child_agent_spawn", Events: agencyEvents(3, 1),
	})
	if err != nil {
		t.Fatal(err)
	}
	for _, target := range []any{rec, agg} {
		v := reflect.ValueOf(target)
		for i := 0; i < v.NumField(); i++ {
			f := v.Field(i)
			if f.Kind() == reflect.String && forbid[f.String()] {
				t.Errorf("record field %s carries the Phase 1 action token %q", v.Type().Field(i).Name, f.String())
			}
		}
	}
	if rec.Response != AgencyGuardResponseRule {
		t.Errorf("response line drifted from the pinned record-only rule: %q", rec.Response)
	}
	// The gate is mechanical on inputs too: smuggling a bare token
	// as a stated identity or class fails before construction.
	for _, in := range []AgencyCounterAggregateInput{
		{Scope: ScopeTask, ScopeID: "HOLD", Counter: CounterStepCount, EventClass: "step", Events: agencyEvents(1, 1)},
		{Scope: ScopeTask, ScopeID: "t", Counter: CounterStepCount, EventClass: "limit", Events: agencyEvents(1, 1)},
	} {
		if _, err := BuildAgencyCounterAggregate(in); err == nil {
			t.Errorf("enum gate accepted the smuggling input %+v", in)
		}
	}
	// The registry line is a whole list, never a bare token.
	for _, tok := range AgencyForbiddenActionTokens() {
		if AgencyLadderRegistry == tok {
			t.Error("ladder registry must not equal a bare action token")
		}
	}
}

func TestAgencyRejectionsLeaveNoRecord(t *testing.T) {
	zero, err := BuildAgencyCounterAggregate(AgencyCounterAggregateInput{
		Scope: ScopeAgent, ScopeID: "a", Counter: CounterStepCount,
		EventClass: "step", Events: append(agencyEvents(3, 1), AgencyCounterEvent{Sequence: 9, StatedValue: 1}),
	})
	if err == nil {
		t.Fatal("broken tail sequence accepted")
	}
	if zero != (AgencyCounterAggregate{}) {
		t.Error("rejected build left a half record behind")
	}
	zeroRec, err := BuildAgencyLimitRecord(AgencyLimitRecordInput{
		GuardField: GuardMaxSteps, Counter: CounterStepCount, Scope: ScopeAgent, ScopeID: " ", Ceiling: 1,
	})
	if err == nil || zeroRec != (AgencyLimitRecord{}) {
		t.Error("rejected limit build left a record behind")
	}
}

func TestAgencyRecordsCarryNoDecision(t *testing.T) {
	aggFields := reflect.TypeOf(AgencyCounterAggregate{}).NumField()
	recFields := reflect.TypeOf(AgencyLimitRecord{}).NumField()
	if aggFields != 8 || recFields != 11 {
		t.Fatalf("agency record field sets drifted: aggregate %d (want 8), limit %d (want 11)", aggFields, recFields)
	}
	for _, typ := range []reflect.Type{reflect.TypeOf(AgencyCounterAggregateInput{}), reflect.TypeOf(AgencyLimitRecordInput{})} {
		for i := 0; i < typ.NumField(); i++ {
			name := strings.ToLower(typ.Field(i).Name)
			for _, bad := range []string{"decision", "score", "severity", "verdict", "kill", "halt", "action"} {
				if strings.Contains(name, bad) {
					t.Errorf("input %s carries a decision-shaped field %s", typ.Name(), name)
				}
			}
		}
	}
}

func TestAgencySymbolsStayOffTheDecisionPlane(t *testing.T) {
	needle := `AgencyGuardField|AgencyGuardPhase|AgencyGuardCoverageRow|AgencyCounterKind|AgencyCounterScope|AgencyLimitState|AgencyCounterEvent|AgencyCounterAggregate|AgencyLimitRecord|BuildAgencyCounterAggregate|BuildAgencyLimitRecord|AllAgencyGuard|AllAgencyCounter|AllAgencyLimit|AgencyForbiddenActionTokens|AgencyCountConservationRule|AgencyActionEnumGate|AgencyGuardResponseRule|AgencyLadderRegistry|AgencyEnforcementPlane`
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
				t.Errorf("decision-plane file %s references agency symbol %q", filepath.Join(d, e.Name()), loc)
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
				t.Errorf("command tree file %s references agency symbol %q", p, loc)
			}
		}
	}
	walkCmd(filepath.Join("..", "..", "cmd"))
	if cmdHits == 0 {
		self, err := os.ReadFile("agencyguard.go")
		if err != nil {
			t.Fatalf("read self: %v", err)
		}
		if !re.MatchString(string(self)) {
			t.Error("positive control broken: needle matches no shipped agency source")
		}
	}
}

func TestAgencyGuardDocsSync(t *testing.T) {
	doc := string(mustReadDocs(t))
	if !strings.Contains(doc, "## 21. Agency guard observation record contract") {
		t.Fatal("section 21 header missing from docs/schema-v2.md")
	}
	pins := [][2]string{
		{"agency_guard_field_vocabulary", strings.Join(AllAgencyGuardFields(), ",")},
		{"agency_counter_vocabulary", strings.Join(AllAgencyCounterKinds(), ",")},
		{"agency_counter_scope_vocabulary", strings.Join(AllAgencyCounterScopes(), ",")},
		{"agency_limit_state_vocabulary", strings.Join(AllAgencyLimitStates(), ",")},
		{"agency_count_conservation_rule", AgencyCountConservationRule},
		{"agency_action_enum_gate", AgencyActionEnumGate},
		{"agency_enforcement_plane", AgencyEnforcementPlane},
	}
	for _, p := range pins {
		line := p[0] + ": " + p[1]
		if !strings.Contains(doc, line) {
			t.Errorf("docs section 21 drifted, want line %q", line)
		}
	}
	// The section 261 ten bound lines mirror the field vocabulary
	// mechanically (space to underscore, lower case).
	specNames := []string{
		"Max Steps", "Max Runtime", "Max Tool Calls", "Max Network Requests",
		"Max Parallelism", "Max CPU", "Max Memory", "Max Storage",
		"Max API Cost", "Max Child Agents",
	}
	for i, n := range specNames {
		if strings.ToLower(strings.ReplaceAll(n, " ", "_")) != AllAgencyGuardFields()[i] {
			t.Fatalf("guard field derivation broke at the section 261 line %d", i+1)
		}
		if !strings.Contains(doc, "* "+n) {
			t.Errorf("docs section 261 anchor bullet missing: %q", n)
		}
	}
}
