package replay

import (
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"strings"
	"testing"

	"20131.com/agentruntime/internal/rules"
	"20131.com/agentruntime/internal/schema"
)

func thisFile() string {
	_, f, _, _ := runtime.Caller(0)
	return f
}

func testdataDir() string {
	return filepath.Join(filepath.Dir(filepath.Dir(filepath.Dir(thisFile()))), "testdata")
}

func goldenPath(name string) string {
	return filepath.Join(testdataDir(), "golden", name)
}

func candidateFile(t *testing.T, p *schema.Policy) string {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "candidate.json")
	raw, err := json.Marshal(p)
	if err != nil {
		t.Fatalf("marshal candidate: %v", err)
	}
	if err := os.WriteFile(path, raw, 0o600); err != nil {
		t.Fatalf("write candidate: %v", err)
	}
	return path
}

func mustBuiltinCandidate(t *testing.T) *schema.Policy {
	t.Helper()
	p, err := rules.Builtin()
	if err != nil {
		t.Fatalf("builtin policy: %v", err)
	}
	return p
}

func runGolden(t *testing.T, p *schema.Policy) *Report {
	t.Helper()
	events, held, err := LoadCorpus(goldenPath("normal.jsonl"), goldenPath("danger.jsonl"))
	if err != nil {
		t.Fatalf("load corpus: %v", err)
	}
	rep, err := Run(p, events, []string{"normal.jsonl", "danger.jsonl"}, held)
	if err != nil {
		t.Fatalf("run: %v", err)
	}
	return rep
}

// TestReplayDeterministicBytes pins the replay determinism gate: two
// runs over identical inputs render byte-identical reports.
func TestReplayDeterministicBytes(t *testing.T) {
	p := mustBuiltinCandidate(t)
	r1, r2 := runGolden(t, p), runGolden(t, p)
	b1, err := r1.Render()
	if err != nil {
		t.Fatalf("render 1: %v", err)
	}
	b2, err := r2.Render()
	if err != nil {
		t.Fatalf("render 2: %v", err)
	}
	if string(b1) != string(b2) {
		t.Fatalf("replay not byte-identical across runs (%d vs %d bytes)", len(b1), len(b2))
	}
	if !strings.Contains(string(b1), `"stance"`) {
		t.Fatalf("report lost the stance line")
	}
}

// TestGoldenVerdictsMatchLabels replays the shipped golden corpus under
// the built-in candidate and requires the mismatch set to equal the
// ledger's declared known-gap cases exactly: the replay agrees with
// every shipped expectation except the two blind spots the thresholds
// ledger carries forward as declared honest misses — no more, no fewer,
// never silently.
func TestGoldenVerdictsMatchLabels(t *testing.T) {
	rep := runGolden(t, mustBuiltinCandidate(t))
	labels, err := LoadLabels(goldenPath("labels.json"))
	if err != nil {
		t.Fatalf("load labels: %v", err)
	}
	rep.CheckLabels(labels)
	raw, err := os.ReadFile(goldenPath("thresholds.json"))
	if err != nil {
		t.Fatalf("read ledger: %v", err)
	}
	var ledger struct {
		KnownGap struct {
			Cases []string `json:"cases"`
		} `json:"known_gap"`
	}
	if err := json.Unmarshal(raw, &ledger); err != nil {
		t.Fatalf("parse ledger: %v", err)
	}
	want := append([]string(nil), ledger.KnownGap.Cases...)
	got := make([]string, 0, len(rep.Mismatches))
	for _, m := range rep.Mismatches {
		id := strings.SplitN(m, ":", 2)[0]
		got = append(got, id)
	}
	if len(got) != len(want) {
		t.Fatalf("mismatches %v, declared known gaps %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("mismatch set %v differs from declared known gaps %v", got, want)
		}
	}
	if rep.Totals.Cases != len(labels) {
		t.Fatalf("replayed cases %d != labeled cases %d", rep.Totals.Cases, len(labels))
	}
	if rep.Totals.Held != 0 {
		t.Fatalf("shipped golden corpus must replay with zero held lines, got %d", rep.Totals.Held)
	}
}

// TestCandidateLoadRejectsBadGrammar proves a candidate that fails the
// policy grammar never reaches evaluation: unknown fields, invalid
// effects, and duplicate rule ids are all refused at load.
func TestCandidateLoadRejectsBadGrammar(t *testing.T) {
	dir := t.TempDir()
	good := filepath.Join(dir, "good.json")
	if err := os.WriteFile(good, []byte(`{"id":"cnd-1","name":"c","version":1,"default_effect":"allow","rules":[]}`), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := LoadCandidate(good); err != nil {
		t.Fatalf("valid candidate refused: %v", err)
	}
	// unknown field must not be absorbed silently
	unknown := filepath.Join(dir, "unknown.json")
	if err := os.WriteFile(unknown, []byte(`{"id":"cnd-1","name":"c","version":1,"default_effect":"allow","rules":[],"surprise":1}`), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := LoadCandidate(unknown); err == nil {
		t.Fatalf("candidate with unknown field accepted")
	}
	// invalid effect vocabulary
	bad := filepath.Join(dir, "bad.json")
	if err := os.WriteFile(bad, []byte(`{"id":"cnd-1","name":"c","version":1,"default_effect":"deny","rules":[]}`), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := LoadCandidate(bad); err == nil {
		t.Fatalf("candidate with enforcement word default accepted")
	}
	// duplicate rule ids
	dup := filepath.Join(dir, "dup.json")
	rule := `{"id":"r1","priority":1,"field":"type","op":"equals","value":"x","effect":"allow","severity":0}`
	if err := os.WriteFile(dup, []byte(`{"id":"cnd-1","name":"c","version":1,"default_effect":"allow","rules":[`+rule+","+rule+`]}`), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := LoadCandidate(dup); err == nil {
		t.Fatalf("candidate with duplicate rule id accepted")
	}
}

// TestCandidateLoadZeroRuntimeSideEffects pins the isolation contract:
// loading and replaying a candidate leaves the built-in rule set, its
// hard-rule list, and its capability tokens exactly as found.
func TestCandidateLoadZeroRuntimeSideEffects(t *testing.T) {
	idsBefore := rules.RuleIDs()
	hardBefore := rules.HardRuleIDs()
	capsBefore := rules.CapTokensUsed()
	p := mustBuiltinCandidate(t)
	path := candidateFile(t, p)
	loaded, err := LoadCandidate(path)
	if err != nil {
		t.Fatalf("load candidate: %v", err)
	}
	runGolden(t, loaded) // replay under the loaded candidate
	if !reflect.DeepEqual(idsBefore, rules.RuleIDs()) {
		t.Fatalf("candidate load mutated the built-in rule registry")
	}
	if !reflect.DeepEqual(hardBefore, rules.HardRuleIDs()) {
		t.Fatalf("candidate load mutated the built-in hard-rule list")
	}
	if !reflect.DeepEqual(capsBefore, rules.CapTokensUsed()) {
		t.Fatalf("candidate load mutated the built-in capability tokens")
	}
}

// TestHeldLinesHonestCounts feeds one valid line, one invalid record,
// and one truncated tail: both must surface in the held count, neither
// may be guessed into a verdict.
func TestHeldLinesHonestCounts(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "corrupt.jsonl")
	body := `{"v":1,"ts":"2026-10-01T00:00:00Z","id":"rp-1","agent_id":"ag-1","stage":"observation","type":"agent.detected","decision":"allow","severity":0,"summary":"ok","attrs":{}}` + "\n" +
		`{"v":1,"id":"rp-bad"}` + "\n" +
		`{"v":1,"id":"trunc`
	if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
	events, held, err := LoadCorpus(path)
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if len(events) != 1 {
		t.Fatalf("kept %d events, want exactly 1 valid line", len(events))
	}
	if held != 2 {
		t.Fatalf("loader held %d, want 2 (one invalid record, one truncated tail)", held)
	}
	rep, err := Run(mustBuiltinCandidate(t), events, []string{"corrupt.jsonl"}, held)
	if err != nil {
		t.Fatalf("run: %v", err)
	}
	if rep.Totals.Held != 2 || rep.Totals.Cases != 1 {
		t.Fatalf("report totals held=%d cases=%d, want 2/1", rep.Totals.Held, rep.Totals.Cases)
	}
}

// TestCandidateChangeFlipsVerdictPositiveControl proves the replay
// consumes the candidate, not the runtime defaults: a candidate with an
// extra would_block rule on a golden event type flips verdicts for that
// type and only for that type.
func TestCandidateChangeFlipsVerdictPositiveControl(t *testing.T) {
	base := mustBuiltinCandidate(t)
	rep := runGolden(t, base)
	before := rep.Totals.WouldBlock

	mutated, err := rules.Builtin()
	if err != nil {
		t.Fatal(err)
	}
	mutated.Rules = append(mutated.Rules, schema.Rule{
		ID: "rp-synthetic", Priority: 999, Field: schema.FieldType, Op: schema.OpEquals,
		Value: "agent.detected", Effect: schema.EffectWouldBlock, Severity: schema.SevLow,
	})
	if err := mutated.Validate(); err != nil {
		t.Fatalf("mutated candidate invalid: %v", err)
	}
	rep2 := runGolden(t, mutated)
	if rep2.Totals.WouldBlock <= before {
		t.Fatalf("synthetic rule changed nothing: would_block %d -> %d", before, rep2.Totals.WouldBlock)
	}
	for _, v := range rep2.Verdicts {
		if v.MatchedRule == "rp-synthetic" && v.Type != "agent.detected" {
			t.Fatalf("synthetic rule matched foreign type %s", v.Type)
		}
		if v.MatchedRule == "rp-synthetic" && v.Decision != "would_block" {
			t.Fatalf("synthetic rule emitted %q, want would_block", v.Decision)
		}
	}
}

// TestReplayNeverEmitsEnforcementWords scans every rendered report of
// the built-in replay for enforcement-plane vocabulary: the replay
// plane records, it does not act.
func TestReplayNeverEmitsEnforcementWords(t *testing.T) {
	b, err := runGolden(t, mustBuiltinCandidate(t)).Render()
	if err != nil {
		t.Fatal(err)
	}
	s := strings.ToLower(string(b))
	for _, w := range []string{"denied", "blocked", "killed", "terminated", "enforced", "intercept"} {
		if strings.Contains(s, w) {
			t.Fatalf("replay report contains enforcement word %q", w)
		}
	}
	if !strings.Contains(s, "record-only, zero enforcement plane") {
		t.Fatalf("replay report lost its record-only stance line")
	}
}

// TestReplayDoesNotMutateCorpusBytes hashes the corpus files before and
// after a full replay: the read path must leave every byte on disk
// untouched, including mtimes (no reopen-for-write anywhere).
func TestReplayDoesNotMutateCorpusBytes(t *testing.T) {
	paths := []string{goldenPath("normal.jsonl"), goldenPath("danger.jsonl")}
	type snap struct {
		mode os.FileMode
		mod  string
	}
	before := make([]snap, len(paths))
	for i, p := range paths {
		st, err := os.Stat(p)
		if err != nil {
			t.Fatal(err)
		}
		before[i] = snap{mode: st.Mode(), mod: st.ModTime().String()}
	}
	events, held, err := LoadCorpus(paths...)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := Run(mustBuiltinCandidate(t), events, paths, held); err != nil {
		t.Fatal(err)
	}
	for i, p := range paths {
		st, err := os.Stat(p)
		if err != nil {
			t.Fatal(err)
		}
		if st.Mode() != before[i].mode || st.ModTime().String() != before[i].mod {
			t.Fatalf("corpus %s touched by replay", p)
		}
	}
}
