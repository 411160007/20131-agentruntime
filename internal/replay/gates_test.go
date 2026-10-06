package replay

import (
	"encoding/json"
	"os"
	"regexp"
	"strings"
	"testing"

	"20131.com/agentruntime/internal/schema"
)

// gatesFixture builds a small synthetic report directly (the gate
// builder only reads a Report, a candidate, and a label table), so
// each section gets its own fixture without touching golden state.
func gatesFixture() (*Report, *schema.Policy, map[string]LabelExpect) {
	cand := &schema.Policy{
		ID: "cand-gates", Name: "gates fixture candidate", Version: 1,
		DefaultEffect: schema.EffectAllow,
		Rules: []schema.Rule{
			{ID: "gf-block-1", Priority: 50, Field: schema.FieldType, Op: schema.OpEquals,
				Value: "agent.detected", Effect: schema.EffectWouldBlock, Severity: schema.SevLow},
		},
	}
	rep := &Report{
		CandidateID: cand.ID, CandidateVersion: cand.Version,
		DefaultEffect: "allow",
		Sources:       []string{"fixture-a.jsonl", "fixture-b.jsonl"},
		Stance:        stanceLine,
		Totals: Totals{
			Cases: 6, Held: 2, Allow: 3, WouldBlock: 3,
			ByRule: []RuleHit{{Rule: "gf-block-1", Count: 3}},
		},
		Verdicts: []Verdict{
			{Seq: 1, ID: "fa-1", Type: "process_action", AgentID: "codex", Decision: "allow", Severity: 0, DefaultApplied: true},
			{Seq: 2, ID: "fa-2", Type: "process_action", AgentID: "codex", Decision: "would_block", Severity: 2, MatchedRule: "gf-block-1"},
			{Seq: 3, ID: "fa-3", Type: "file_open", AgentID: "codex", Decision: "allow", Severity: 0, DefaultApplied: true},
			{Seq: 4, ID: "fb-1", Type: "process_action", AgentID: "other-agent", Decision: "would_block", Severity: 2, MatchedRule: "gf-block-1"},
			{Seq: 5, ID: "fb-2", Type: "process_action", AgentID: "other-agent", Decision: "would_block", Severity: 3, MatchedRule: "gf-block-1"},
			{Seq: 6, ID: "fb-3", Type: "file_open", AgentID: "other-agent", Decision: "allow", Severity: 0, DefaultApplied: true},
		},
	}
	labels := map[string]LabelExpect{
		"fa-1": {Expect: "allow"},
		"fa-2": {Expect: "allow"}, // false positive: blocked but expected allow
		"fa-3": {Expect: "allow"},
		"fb-1": {Expect: "would_block"},
		"fb-2": {Expect: "allow"},       // false positive
		"fb-3": {Expect: "would_block"}, // missed block
		"zz-9": {Expect: "allow"},       // label with no replayed event
	}
	return rep, cand, labels
}

// TestGatesByteDeterministic pins the determinism contract on the
// gate surface too: two builds over identical inputs render the same
// bytes.
func TestGatesByteDeterministic(t *testing.T) {
	rep, cand, labels := gatesFixture()
	g1, err := BuildGates(rep, cand, labels)
	if err != nil {
		t.Fatalf("build 1: %v", err)
	}
	g2, err := BuildGates(rep, cand, labels)
	if err != nil {
		t.Fatalf("build 2: %v", err)
	}
	b1, err := g1.RenderGates()
	if err != nil {
		t.Fatalf("render 1: %v", err)
	}
	b2, err := g2.RenderGates()
	if err != nil {
		t.Fatalf("render 2: %v", err)
	}
	if string(b1) != string(b2) {
		t.Fatalf("gate report not byte-identical")
	}
}

// TestGatesSecuritySectionFixture pins the security evaluation face
// against a hand-declared expectation of the fixture above.
func TestGatesSecuritySectionFixture(t *testing.T) {
	rep, cand, labels := gatesFixture()
	g, err := BuildGates(rep, cand, labels)
	if err != nil {
		t.Fatalf("build: %v", err)
	}
	s := g.Security
	if s.Cases != 6 || s.Held != 2 || s.Allow != 3 || s.WouldBlock != 3 {
		t.Fatalf("security totals drifted: %+v", s)
	}
	if len(s.Severity) != 2 || s.Severity[0].Severity != 2 || s.Severity[0].WouldBlock != 2 ||
		s.Severity[1].Severity != 3 || s.Severity[1].WouldBlock != 1 {
		t.Fatalf("severity banding wrong (must be ascending, would_block only): %+v", s.Severity)
	}
	if len(s.ByRule) != 1 || s.ByRule[0].Rule != "gf-block-1" || s.ByRule[0].Count != 3 {
		t.Fatalf("rule tally drifted: %+v", s.ByRule)
	}
}

// TestGatesFalsePositiveSectionFixture pins the labelled comparison
// arithmetic, including the explicit rate definitions.
func TestGatesFalsePositiveSectionFixture(t *testing.T) {
	rep, cand, labels := gatesFixture()
	g, err := BuildGates(rep, cand, labels)
	if err != nil {
		t.Fatalf("build: %v", err)
	}
	if g.FalsePositive.Status != "measured" || g.FalsePositive.Metrics == nil {
		t.Fatalf("fp section must be measured with labels: %+v", g.FalsePositive)
	}
	m := g.FalsePositive.Metrics
	if m.LabelledCases != 6 || m.LabelsWithoutEvent != 1 || m.UnlabelledVerdicts != 0 {
		t.Fatalf("label census wrong: %+v", m)
	}
	if m.ExpectAllowTotal != 4 || m.ExpectBlockTotal != 2 {
		t.Fatalf("expectation census wrong: %+v", m)
	}
	if m.FalsePositives != 2 || m.MissedBlocks != 1 || m.MatchedExpectations != 3 {
		t.Fatalf("comparison counts wrong: %+v", m)
	}
	if m.FalsePositiveRate != 0.5 || m.MissedBlockRate != 0.5 {
		t.Fatalf("rates wrong: fp=%v missed=%v", m.FalsePositiveRate, m.MissedBlockRate)
	}
}

// TestGatesFalsePositiveOracleRecountOnGolden replays the shipped
// golden corpus under the built-in candidate, builds the gate report
// from the shipped label table, and re-counts the false-positive
// arithmetic with an independent regex oracle straight against the
// raw label and corpus files: the shipped numbers must equal the
// recomputed ones, no hand-copied constants anywhere.
func TestGatesFalsePositiveOracleRecountOnGolden(t *testing.T) {
	cand := mustBuiltinCandidate(t)
	rep := runGolden(t, cand)
	labels, err := LoadLabels(goldenPath("labels.json"))
	if err != nil {
		t.Fatalf("labels: %v", err)
	}
	g, err := BuildGates(rep, cand, labels)
	if err != nil {
		t.Fatalf("build: %v", err)
	}
	if g.FalsePositive.Status != "measured" {
		t.Fatalf("golden has a shipped label table; fp must be measured, got %+v", g.FalsePositive)
	}
	m := g.FalsePositive.Metrics

	raw, err := os.ReadFile(goldenPath("labels.json"))
	if err != nil {
		t.Fatalf("raw labels: %v", err)
	}
	expectAllow := len(regexp.MustCompile(`"expect":\s*"allow"`).FindAllIndex(raw, -1))
	expectBlock := len(regexp.MustCompile(`"expect":\s*"would_block"`).FindAllIndex(raw, -1))
	if m.ExpectAllowTotal+0 != expectAllow || m.ExpectBlockTotal != expectBlock {
		t.Fatalf("oracle recount mismatch on expectation census: go=%d+%d regex=%d+%d",
			m.ExpectAllowTotal, m.ExpectBlockTotal, expectAllow, expectBlock)
	}
	// Independent oracle over the verdict bytes: decode the rendered
	// report through generic containers (no shared structs) and recount
	// decisions per labelled id straight from those decoded values.
	rendered, err := rep.Render()
	if err != nil {
		t.Fatalf("render: %v", err)
	}
	var doc map[string]any
	if err := json.Unmarshal(rendered, &doc); err != nil {
		t.Fatalf("oracle decode: %v", err)
	}
	oracle := map[string]string{}
	if vs, ok := doc["verdicts"].([]any); ok {
		for _, item := range vs {
			v, ok := item.(map[string]any)
			if !ok {
				continue
			}
			id, _ := v["id"].(string)
			dec, _ := v["decision"].(string)
			oracle[id] = dec
		}
	}
	var oFP, oMissed, oMatch int
	for id, lab := range labels {
		got, ok := oracle[id]
		if !ok {
			continue
		}
		switch {
		case lab.Expect == "allow" && got != "allow":
			oFP++
		case lab.Expect == "would_block" && got != "would_block":
			oMissed++
		default:
			oMatch++
		}
	}
	if m.FalsePositives != oFP || m.MissedBlocks != oMissed || m.MatchedExpectations != oMatch {
		t.Fatalf("oracle recount mismatch: go fp=%v missed=%v match=%v vs oracle fp=%d missed=%d match=%d",
			m.FalsePositives, m.MissedBlocks, m.MatchedExpectations, oFP, oMissed, oMatch)
	}
	// The declared known-gap blind spots must show up exactly as the
	// missed-block count: honest misses, not silent passes.
	if oMissed == 0 {
		t.Fatalf("golden replay lost the declared blind spots entirely")
	}
}

// TestGatesFalsePositiveKnownGapWithoutLabels pins the honest
// absence: without a label table no metric keys are emitted at all.
func TestGatesFalsePositiveKnownGapWithoutLabels(t *testing.T) {
	rep, cand, _ := gatesFixture()
	g, err := BuildGates(rep, cand, nil)
	if err != nil {
		t.Fatalf("build: %v", err)
	}
	b, err := g.RenderGates()
	if err != nil {
		t.Fatalf("render: %v", err)
	}
	if g.FalsePositive.Status != "known_gap" || g.FalsePositive.Reason == "" {
		t.Fatalf("missing label table must state a known gap: %+v", g.FalsePositive)
	}
	for _, banned := range []string{`"false_positives"`, `"missed_blocks"`, `"false_positive_rate"`, `"labelled_cases"`} {
		if strings.Contains(string(b), banned) {
			t.Fatalf("known-gap section leaked metric key %s", banned)
		}
	}
}

// TestGatesProductivitySectionFixture pins the per-agent exposure
// arithmetic and the stable ordering.
func TestGatesProductivitySectionFixture(t *testing.T) {
	rep, cand, labels := gatesFixture()
	g, err := BuildGates(rep, cand, labels)
	if err != nil {
		t.Fatalf("build: %v", err)
	}
	if g.Productivity.Status != "measured" || g.Productivity.Total != 2 {
		t.Fatalf("productivity census wrong: %+v", g.Productivity)
	}
	a := g.Productivity.Agents
	if len(a) != 2 || a[0].AgentID != "codex" || a[1].AgentID != "other-agent" {
		t.Fatalf("agent order must be by id: %+v", a)
	}
	if a[0].Events != 3 || a[0].WouldBlock != 1 || a[0].BlockedShare != 1.0/3.0 {
		t.Fatalf("codex exposure wrong: %+v", a[0])
	}
	if a[1].Events != 3 || a[1].WouldBlock != 2 || a[1].BlockedShare != 2.0/3.0 {
		t.Fatalf("second agent exposure wrong: %+v", a[1])
	}
}

// TestGatesPerformanceStructuralOnly pins the honest performance
// face: structural counts plus the two declared absence stances, and
// no unstable timing value on the machine surface.
func TestGatesPerformanceStructuralOnly(t *testing.T) {
	rep, cand, labels := gatesFixture()
	g, err := BuildGates(rep, cand, labels)
	if err != nil {
		t.Fatalf("build: %v", err)
	}
	b, err := g.RenderGates()
	if err != nil {
		t.Fatalf("render: %v", err)
	}
	p := g.Performance
	if p.Status != "measured" || p.EvaluatedEvents != 6 || p.HeldLines != 2 || p.CandidateRules != len(cand.Rules) {
		t.Fatalf("performance structural counts wrong: %+v", p)
	}
	if p.TimingStance == "" || p.PerSourceStance == "" {
		t.Fatalf("performance face must state both absence stances")
	}
	for _, banned := range []string{"elapsed", "duration_ms", "latency", "nanoseconds", "µs"} {
		if strings.Contains(string(b), banned) {
			t.Fatalf("unstable timing vocabulary leaked into the gate surface: %s", banned)
		}
	}
}

// TestGatesNeverEmitsEnforcementWords pins the Phase 0 emission
// discipline on the gate surface, same contract as the replay report.
func TestGatesNeverEmitsEnforcementWords(t *testing.T) {
	rep, cand, labels := gatesFixture()
	g, err := BuildGates(rep, cand, labels)
	if err != nil {
		t.Fatalf("build: %v", err)
	}
	b, err := g.RenderGates()
	if err != nil {
		t.Fatalf("render: %v", err)
	}
	s := string(b)
	for _, banned := range []string{`"ask"`, `"deny"`, `"enforce"`, `"quarantine"`, `"rollback"`} {
		if strings.Contains(s, banned) {
			t.Fatalf("reserved enforcement vocabulary present in gate report: %s", banned)
		}
	}
	if !strings.Contains(s, GatesStance) {
		t.Fatalf("gate stance line missing")
	}
}

// TestGatesEmptyVerdictsKnownGap pins the vacuous-input honesty:
// with nothing replayed, the label and productivity faces state a
// known gap instead of presenting empty success.
func TestGatesEmptyVerdictsKnownGap(t *testing.T) {
	cand := &schema.Policy{ID: "cand-empty", Name: "empty fixture", Version: 1, DefaultEffect: schema.EffectAllow}
	rep := &Report{CandidateID: cand.ID, DefaultEffect: "allow", Stance: stanceLine}
	labels := map[string]LabelExpect{"x-1": {Expect: "allow"}}
	g, err := BuildGates(rep, cand, labels)
	if err != nil {
		t.Fatalf("build: %v", err)
	}
	if g.FalsePositive.Status != "known_gap" || g.FalsePositive.Reason != reasonNoVerdict {
		t.Fatalf("empty replay must state the no-verdict gap: %+v", g.FalsePositive)
	}
	if g.Productivity.Status != "known_gap" || len(g.Productivity.Agents) != 0 {
		t.Fatalf("empty replay must state the gap and leak no productivity rows: %+v", g.Productivity)
	}
}

// TestGatesInputsNotMutated pins the read-only contract: building the
// gate report must not touch the replay report, the candidate, or the
// label table.
func TestGatesInputsNotMutated(t *testing.T) {
	rep, cand, labels := gatesFixture()
	repBefore, err := rep.Render()
	if err != nil {
		t.Fatalf("render before: %v", err)
	}
	candBefore, err := json.Marshal(cand)
	if err != nil {
		t.Fatalf("marshal before: %v", err)
	}
	labelsBefore := len(labels)
	if _, err := BuildGates(rep, cand, labels); err != nil {
		t.Fatalf("build: %v", err)
	}
	repAfter, err := rep.Render()
	if err != nil {
		t.Fatalf("render after: %v", err)
	}
	candAfter, err := json.Marshal(cand)
	if err != nil {
		t.Fatalf("marshal after: %v", err)
	}
	if string(repBefore) != string(repAfter) {
		t.Fatalf("gate build mutated the replay report")
	}
	if string(candBefore) != string(candAfter) {
		t.Fatalf("gate build mutated the candidate")
	}
	if len(labels) != labelsBefore {
		t.Fatalf("gate build mutated the label table")
	}
}
