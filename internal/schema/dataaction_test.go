package schema

// W4.1 slice tests: the nine-word data action vocabulary (spec vNext
// section 237) as a record-only additive Event field. Each test names
// the contract line it pins; the wild fixture is the rejection battery
// and the docs sync tests are the third mirror source.

import (
	"encoding/json"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

func readDataActionFixture(t *testing.T, rel string) []string {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join("..", "..", rel))
	if err != nil {
		t.Fatalf("read fixture %s: %v", rel, err)
	}
	var out []string
	for _, line := range strings.Split(strings.TrimRight(string(raw), "\n"), "\n") {
		if strings.TrimSpace(line) != "" {
			out = append(out, line)
		}
	}
	return out
}

func decodeEventLine(t *testing.T, line string) (*Event, error) {
	t.Helper()
	var e Event
	if err := json.Unmarshal([]byte(line), &e); err != nil {
		return nil, err
	}
	return &e, nil
}

func TestDataActionVocabularyAndRulesPin(t *testing.T) {
	want := []string{
		"discover", "read", "write", "modify", "delete", "execute",
		"export", "share", "persist",
	}
	got := AllDataActions()
	if len(got) != len(want) {
		t.Fatalf("vocabulary size drifted: got %d want %d", len(got), len(want))
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("order drifted at %d: got %q want %q", i, got[i], want[i])
		}
	}
	if DataActionNonequivalenceRule != "read-not-export-read-not-share-write-not-execute" {
		t.Error("nonequivalence rule constant drifted")
	}
	if DataActionExportRejudgementRule != "export-requires-independent-rejudgement" {
		t.Error("export rejudgement rule constant drifted")
	}
	if DataActionAbsentDefault != "absent-means-unclassified-legacy-never-inferred" {
		t.Error("absent default constant drifted")
	}
	if DataActionEnforcementPlane != "none-in-observation-phase" {
		t.Error("enforcement plane constant drifted")
	}
	// Membership: every listed word validates; the empty value is the
	// honest absence shape and also validates.
	for _, w := range want {
		if !DataAction(w).Valid() {
			t.Errorf("vocabulary member %q rejected by Valid()", w)
		}
	}
	if !DataAction("").Valid() {
		t.Error("absent (empty) data action must be valid legacy")
	}
}

func TestDataActionGoodFixturesRoundTrip(t *testing.T) {
	lines := readDataActionFixture(t, filepath.Join("testdata", "dataaction", "good_dataaction.jsonl"))
	if len(lines) != 10 {
		t.Fatalf("good fixture count drifted: %d", len(lines))
	}
	seen := map[string]int{}
	for i, line := range lines {
		e, err := decodeEventLine(t, line)
		if err != nil {
			t.Fatalf("good line %d undecodable: %v", i+1, err)
		}
		if err := e.Validate(); err != nil {
			t.Fatalf("good line %d rejected: %v", i+1, err)
		}
		if e.DataAction == "" {
			if i != 9 {
				t.Errorf("line %d unexpectedly carries no data action", i+1)
			}
			seen["<absent>"]++
			continue
		}
		seen[string(e.DataAction)]++
		// Re-encoding keeps the key present and identical (additive
		// round trip: no silent drop, no vocabulary coercion).
		out, err := json.Marshal(e)
		if err != nil {
			t.Fatalf("line %d re-encode: %v", i+1, err)
		}
		if !strings.Contains(string(out), `"data_action":"`+string(e.DataAction)+`"`) {
			t.Errorf("line %d lost its data_action on re-encode", i+1)
		}
	}
	if seen["<absent>"] != 1 {
		t.Errorf("absent-key legacy line missing from good fixture")
	}
	for _, w := range AllDataActions() {
		if seen[w] == 0 {
			t.Errorf("good fixture never exercises vocabulary member %q", w)
		}
	}
}

func TestDataActionWildFixturesRejected(t *testing.T) {
	lines := readDataActionFixture(t, filepath.Join("testdata", "dataaction", "wild_dataaction.jsonl"))
	if len(lines) != 8 {
		t.Fatalf("wild fixture count drifted: %d", len(lines))
	}
	rejected := 0
	for i, line := range lines {
		e, err := decodeEventLine(t, line)
		if err != nil {
			// Wrong JSON type for the key is itself a rejection shape.
			rejected++
			continue
		}
		if err := e.Validate(); err == nil {
			t.Errorf("wild line %d accepted: %q", i+1, string(e.DataAction))
		} else if !strings.Contains(err.Error(), "data_action") {
			t.Errorf("wild line %d rejected for the wrong reason: %v", i+1, err)
		} else {
			rejected++
		}
	}
	if rejected != 8 {
		t.Errorf("wild battery incomplete: %d/8 rejected", rejected)
	}
}

func TestDataActionAbsentMeansUnclassified(t *testing.T) {
	// A legacy line without the key stays valid and reads as unclassified
	// - never coerced into a default class, never reported as any action.
	var e Event
	if err := json.Unmarshal([]byte(`{"v":1,"ts":"2026-10-02T12:02:00Z","id":"evt-dal-001","agent_id":"agent-1","stage":"action","type":"network.intent","decision":"allow","severity":1,"summary":"legacy net.outbound era line","attrs":{"cap":"net.outbound","domain":"example.org"}}`), &e); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if err := e.Validate(); err != nil {
		t.Fatalf("legacy line rejected: %v", err)
	}
	if e.DataAction != "" {
		t.Errorf("absent key inferred as %q", string(e.DataAction))
	}
	// And it must round-trip without inventing the key.
	out, err := json.Marshal(&e)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	if strings.Contains(string(out), "data_action") {
		t.Error("re-encode fabricated a data_action key on a legacy line")
	}
}

func TestDataActionCapabilityVocabularyUntouched(t *testing.T) {
	// net.outbound zero-breakage clause: the eleven capability tokens
	// stay byte-identical in order, so the data action wave is purely
	// additive over the existing contract.
	want := []string{
		"file.read", "file.write", "file.delete", "net.outbound", "net.listen",
		"proc.spawn", "shell.exec", "mcp.tool", "credential.access", "env.read",
		"process.inspect",
	}
	got := AllCapabilities()
	if len(got) != len(want) {
		t.Fatalf("capability vocabulary size drifted: %d", len(got))
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("capability %d drifted: got %q want %q", i, got[i], want[i])
		}
	}
	// The nine data action words are NOT capability tokens and never
	// become valid attrs cap values through this slice.
	for _, w := range AllDataActions() {
		if Capability(w).Valid() {
			t.Errorf("data action %q leaked into the capability vocabulary", w)
		}
	}
}

func TestDataActionSymbolsStayOffTheDecisionPlane(t *testing.T) {
	needle := "DataAction|AllDataActions|DataActionNonequivalenceRule|DataActionExportRejudgementRule|DataActionAbsentDefault|DataActionEnforcementPlane"
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
				t.Errorf("decision-plane file %s references data action symbol %q", filepath.Join(d, e.Name()), loc)
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
				t.Errorf("command tree file %s references data action symbol %q", p, loc)
			}
		}
	}
	walkCmd(filepath.Join("..", "..", "cmd"))
	if cmdHits == 0 {
		// Positive control for the walk: the same regexp must find the
		// symbols in this slice's own shipped source before any green
		// from the walk is believed.
		self, err := os.ReadFile("dataaction.go")
		if err != nil {
			t.Fatalf("read self: %v", err)
		}
		if !re.MatchString(string(self)) {
			t.Error("positive control broken: needle matches no shipped data action source")
		}
	}
}

func TestDataActionDocsSync(t *testing.T) {
	raw, err := os.ReadFile(filepath.Join("..", "..", "docs", "schema-v2.md"))
	if err != nil {
		t.Fatalf("read docs/schema-v2.md: %v", err)
	}
	doc := string(raw)
	if !strings.Contains(doc, "## 15. Data action vocabulary contract") {
		t.Fatal("section 15 header missing from docs/schema-v2.md")
	}
	pins := [][2]string{
		{"data_action_vocabulary", "discover,read,write,modify,delete,execute,export,share,persist"},
		{"data_action_nonequivalence_rule", DataActionNonequivalenceRule},
		{"data_action_export_rejudgement_rule", DataActionExportRejudgementRule},
		{"data_action_absent_default", DataActionAbsentDefault},
		{"data_action_enforcement_plane", DataActionEnforcementPlane},
	}
	for _, p := range pins {
		line := p[0] + ": " + p[1]
		if !strings.Contains(doc, line) {
			t.Errorf("docs section 15 drifted, want line %q", line)
		}
	}
	// Third mirror: docs/api-v0.md data_actions contract line matches
	// the Go vocabulary in order.
	apiRaw, err := os.ReadFile(filepath.Join("..", "..", "docs", "api-v0.md"))
	if err != nil {
		t.Fatalf("read docs/api-v0.md: %v", err)
	}
	m := regexp.MustCompile("(?m)^data_actions: (.+)$").FindStringSubmatch(string(apiRaw))
	if m == nil {
		t.Fatal("no data_actions contract line in api-v0.md")
	}
	got := []string{}
	for _, part := range strings.Split(m[1], ",") {
		got = append(got, strings.TrimSpace(part))
	}
	if diff := symDiff(got, AllDataActions()); len(diff) > 0 {
		t.Errorf("docs/api-v0.md data_actions diverges from Go enum: %v", diff)
	}
}
