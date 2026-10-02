package schema

import (
	"bufio"
	"encoding/json"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

// Intent inflow contract sync tests (intent wave, slice W3.1). They
// pin docs/schema-v2.md section 14 against the Go declarations in
// intentinflow.go: the three-token channel vocabulary is compared as
// a joined string in normative order, the four rule constants are
// back-checked verbatim from the docs anchor lines (recomputed on
// every run, never by hand), and the fixture set drives both
// channels: every good inflow round-trips to the expected source and
// record shape, every wild shape - smuggled wire names, forged
// origins, broken propagation, type violations, unknown channels -
// is rejected before any usable value exists. The grep gates mirror
// the Node checker's red-shape assertions from this side: no inflow
// symbol may touch the decision planes or the command tree while the
// enforcement plane is contracted as none. Pure record and
// documentation evolution: nothing here changes runtime decision
// semantics.

func inflowFixtureLine(t *testing.T, name string) (channel string, payload []byte) {
	t.Helper()
	var line struct {
		Channel string          `json:"channel"`
		Payload json.RawMessage `json:"payload"`
	}
	if err := json.Unmarshal([]byte(name), &line); err != nil {
		t.Fatalf("fixture line not in channel/payload shape: %v", err)
	}
	return line.Channel, line.Payload
}

func runInflowChannel(channel string, payload []byte) (IntentInflow, error) {
	switch channel {
	case string(InflowSourceCLIFile):
		return InflowFromFile(payload)
	case string(InflowSourceHookTask):
		return InflowFromHookPayload(payload)
	default:
		return IntentInflow{}, &inflowUnknownChannel{}
	}
}

type inflowUnknownChannel struct{}

func (e *inflowUnknownChannel) Error() string { return "unknown inflow channel" }

func readInflowFixture(t *testing.T, rel string) []string {
	t.Helper()
	path := filepath.Join("..", "..", rel)
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", rel, err)
	}
	var lines []string
	sc := bufio.NewScanner(strings.NewReader(string(raw)))
	for sc.Scan() {
		if s := strings.TrimSpace(sc.Text()); s != "" {
			lines = append(lines, s)
		}
	}
	return lines
}

func TestInflowVocabularyAndRulesPin(t *testing.T) {
	want := "cli_file,hook_task,absent_not_reported"
	if got := strings.Join(AllIntentInflowSources(), ","); got != want {
		t.Errorf("channel vocabulary drifted: %q", got)
	}
	for _, s := range AllIntentInflowSources() {
		if !IntentInflowSource(s).Valid() {
			t.Errorf("source %q fails Valid()", s)
		}
	}
	if IntentInflowSource("cli_File").Valid() || IntentInflowSource("hook").Valid() {
		t.Error("closed set must reject near-miss tokens")
	}
	rules := map[string]string{
		"hook_task_mapping_rule:":   HookTaskMappingRule,
		"inflow_absent_default:":    InflowAbsentDefault,
		"inflow_forge_rule:":        InflowForgeRule,
		"inflow_enforcement_plane:": InflowEnforcementPlane,
	}
	for key, val := range rules {
		if val == "" {
			t.Errorf("rule constant for %s is empty", key)
		}
	}
}

func TestInflowGoodFixturesRoundTrip(t *testing.T) {
	lines := readInflowFixture(t, filepath.Join("testdata", "intentinflow", "good_inflow.jsonl"))
	if len(lines) != 7 {
		t.Fatalf("good fixture count drifted: %d", len(lines))
	}
	for i, line := range lines {
		channel, payload := inflowFixtureLine(t, line)
		inflow, err := runInflowChannel(channel, payload)
		if err != nil {
			t.Fatalf("good line %d rejected: %v", i+1, err)
		}
		if inflow.Record == nil {
			t.Fatalf("good line %d produced no record", i+1)
		}
		if out, err := EncodeChecked(inflow.Record); err != nil || out == nil {
			t.Fatalf("good line %d fails checked encode: %v", i+1, err)
		}
		// Hook-channel records carry exactly the goal mapping.
		if channel == string(InflowSourceHookTask) {
			if inflow.Source == InflowSourceHookTask {
				r := inflow.Record
				if r.Authority != nil || r.Scope != "" || len(r.ExpectedActions) > 0 || r.ForbiddenScope != "" {
					t.Errorf("hook line %d wrote beyond the goal mapping", i+1)
				}
			}
			if inflow.Source == InflowSourceAbsent && (!inflow.KnownGap || inflow.Stated()) {
				t.Errorf("absent hook line %d must be known-gap and unstated", i+1)
			}
		}
	}
}

func TestInflowWildFixturesRejected(t *testing.T) {
	lines := readInflowFixture(t, filepath.Join("testdata", "intentinflow", "wild_inflow.jsonl"))
	if len(lines) != 10 {
		t.Fatalf("wild fixture count drifted: %d", len(lines))
	}
	for i, line := range lines {
		channel, payload := inflowFixtureLine(t, line)
		inflow, err := runInflowChannel(channel, payload)
		if err == nil {
			t.Errorf("wild line %d accepted: %v", i+1, inflow)
		}
		if inflow.Record != nil {
			t.Errorf("wild line %d left a usable record behind", i+1)
		}
	}
}

func TestInflowHookChannelCannotReachAuthority(t *testing.T) {
	// Yin: a hook payload smuggling the authority wire name is the
	// escalation shape - rejected before any value is applied.
	if got, err := InflowFromHookPayload([]byte(`{"task":"x","authority":{"links":[{"origin":"user_direct"}]}}`)); err == nil || got.Record != nil {
		t.Errorf("smuggled authority payload must reject with no record (got %v err %v)", got, err)
	}
	// Yang: the clean payload maps task to goal and nothing else.
	got, err := InflowFromHookPayload([]byte(`{"task":"do the thing"}`))
	if err != nil {
		t.Fatalf("clean hook payload rejected: %v", err)
	}
	if got.Source != InflowSourceHookTask || got.Record.Goal != "do the thing" || got.Record.Authority != nil {
		t.Errorf("clean hook payload did not map task->goal only: %+v", got)
	}
}

func TestInflowAbsentMeansKnownGap(t *testing.T) {
	got, err := InflowFromHookPayload([]byte(`{}`))
	if err != nil {
		t.Fatalf("absent hook payload must not error: %v", err)
	}
	if got.Source != InflowSourceAbsent || !got.KnownGap || got.Stated() {
		t.Errorf("absent inflow shape wrong: %+v", got)
	}
	out, err := EncodeChecked(got.Record)
	if err != nil || string(out) != "{}" {
		t.Errorf("absent record must encode to the empty object, got %q (err %v)", out, err)
	}
	if AbsentIntentRecord().Goal != "" || AbsentIntentRecord().Authority != nil {
		t.Error("absent record must fabricate nothing")
	}
}

func TestInflowSymbolsStayOffTheDecisionPlane(t *testing.T) {
	needle := "IntentInflowSource|AllIntentInflowSources|InflowFromFile|InflowFromHookPayload|AbsentIntentRecord|HookTaskMappingRule|InflowAbsentDefault|InflowForgeRule|InflowEnforcementPlane"
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
				t.Fatalf("read %s: %v", e.Name(), err)
			}
			if loc := re.FindString(string(data)); loc != "" {
				t.Errorf("decision-plane file %s references inflow record symbol %q", filepath.Join(d, e.Name()), loc)
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
				t.Errorf("command tree file %s references inflow record symbol %q", p, loc)
			}
		}
	}
	walkCmd(filepath.Join("..", "..", "cmd"))
	if cmdHits == 0 {
		// positive control for the walk itself: a planted reference in
		// this package's own file is found by the same regexp.
		self, err := os.ReadFile("intentinflow.go")
		if err != nil {
			t.Fatalf("read self: %v", err)
		}
		if !re.MatchString(string(self)) {
			t.Error("positive control broken: needle matches no shipped inflow source")
		}
	}
}

func TestInflowDocsSync(t *testing.T) {
	data, err := os.ReadFile(filepath.Join("..", "..", "docs", "schema-v2.md"))
	if err != nil {
		t.Fatalf("read docs: %v", err)
	}
	doc := string(data)
	if !strings.Contains(doc, "## 14. Intent inflow contract (slice W3.1)") {
		t.Error("section 14 header missing")
	}
	pins := [][2]string{
		{"inflow_channel_vocabulary: cli_file,hook_task,absent_not_reported", strings.Join(AllIntentInflowSources(), ",")},
		{"hook_task_mapping_rule: task-field-maps-to-goal-and-nothing-else", HookTaskMappingRule},
		{"inflow_absent_default: not-reported-is-known-gap-never-fabricated", InflowAbsentDefault},
		{"inflow_forge_rule: inflow-channel-never-escalates-authority", InflowForgeRule},
		{"inflow_enforcement_plane: none-in-observation-phase", InflowEnforcementPlane},
	}
	for _, p := range pins {
		if !strings.Contains(doc, p[0]) {
			t.Errorf("docs anchor line missing or drifted: %s", p[0])
		}
		key := strings.SplitN(p[0], ": ", 2)[0]
		if strings.Count(doc, key+"\n") > 1 || strings.Count(doc, key+":") > 1 {
			t.Errorf("docs key duplicated: %s", key)
		}
	}
}
