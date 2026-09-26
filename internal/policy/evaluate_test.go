package policy

import (
	"testing"
	"time"

	"20131.com/agentruntime/internal/schema"
)

func testPolicy() *schema.Policy {
	return &schema.Policy{
		ID:            "p1",
		Name:          "test",
		Version:       1,
		DefaultEffect: schema.EffectAllow,
		Rules: []schema.Rule{
			{ID: "etc-write", Priority: 100, Field: schema.FieldPath, Op: schema.OpPrefix, Value: "/etc/", Effect: schema.EffectWouldBlock, Severity: schema.SevHigh},
			{ID: "shadow-suffix", Priority: 90, Field: schema.FieldPath, Op: schema.OpSuffix, Value: "shadow", Effect: schema.EffectWouldBlock, Severity: schema.SevCritical},
			{ID: "tool-calls-ask", Priority: 50, Field: schema.FieldType, Op: schema.OpEquals, Value: string(schema.TypeToolCall), Effect: schema.EffectAsk, Severity: schema.SevMedium},
			{ID: "agent-mcp", Priority: 50, Field: schema.FieldAgentID, Op: schema.OpPrefix, Value: "agent-mcp", Effect: schema.EffectAsk, Severity: schema.SevLow},
		},
	}
}

func testEvent(typ schema.EventType, attrs map[string]string) *schema.Event {
	return &schema.Event{
		V:        schema.SchemaVersion,
		TS:       time.Date(2026, 9, 27, 1, 0, 0, 0, time.UTC),
		ID:       "ev-t1",
		AgentID:  "agent-01",
		Stage:    schema.StageProposed,
		Type:     typ,
		Decision: schema.DecisionAllow,
		Severity: schema.SevInfo,
		Summary:  "test event",
		Attrs:    attrs,
	}
}

func mustEval(t *testing.T) *Evaluator {
	t.Helper()
	ev, err := New(testPolicy())
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	return ev
}

func TestNewRejectsInvalidPolicy(t *testing.T) {
	bad := testPolicy()
	bad.Version = 0
	if _, err := New(bad); err == nil {
		t.Fatal("invalid policy must be rejected at construction")
	}
	if _, err := New(nil); err == nil {
		t.Fatal("nil policy must be rejected")
	}
}

func TestEvaluateMatches(t *testing.T) {
	ev := mustEval(t)
	cases := []struct {
		name     string
		e        *schema.Event
		effect   schema.Effect
		rule     string
		severity schema.Severity
	}{
		{"etc prefix", testEvent(schema.TypeFileAccess, map[string]string{"path": "/etc/passwd"}), schema.EffectWouldBlock, "etc-write", schema.SevHigh},
		{"shadow suffix", testEvent(schema.TypeFileAccess, map[string]string{"path": "/var/lib/hy/shadow"}), schema.EffectWouldBlock, "shadow-suffix", schema.SevCritical},
		{"by type", testEvent(schema.TypeToolCall, nil), schema.EffectAsk, "tool-calls-ask", schema.SevMedium},
		{"no match default", testEvent(schema.TypeCollectorStart, map[string]string{"path": "/tmp/ok.txt"}), schema.EffectAllow, "", schema.SevInfo},
		{"missing attr skips rule", testEvent(schema.TypeFileAccess, nil), schema.EffectAllow, "", schema.SevInfo},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			out, err := ev.Evaluate(tc.e)
			if err != nil {
				t.Fatalf("Evaluate: %v", err)
			}
			if out.Effect != tc.effect || out.MatchedRule != tc.rule || out.Severity != tc.severity {
				t.Fatalf("got %+v; want effect=%s rule=%q sev=%d", out, tc.effect, tc.rule, tc.severity)
			}
		})
	}
}

func TestEvaluatePriorityOrdering(t *testing.T) {
	// Both etc-write (100) and shadow-suffix (90) match /etc/shadow;
	// the higher-priority rule must win deterministically.
	ev := mustEval(t)
	out, err := ev.Evaluate(testEvent(schema.TypeFileAccess, map[string]string{"path": "/etc/shadow"}))
	if err != nil {
		t.Fatal(err)
	}
	if out.MatchedRule != "etc-write" {
		t.Fatalf("priority order broken: matched %q", out.MatchedRule)
	}
}

func TestEvaluateStableTieBreak(t *testing.T) {
	// Equal priority (50): agent-mcp declared after tool-calls-ask must NOT
	// overtake it for an event that matches both — declaration order wins ties.
	ev := mustEval(t)
	e := testEvent(schema.TypeToolCall, map[string]string{"tool": "x"})
	e.AgentID = "agent-mcp-1"
	out, err := ev.Evaluate(e)
	if err != nil {
		t.Fatal(err)
	}
	if out.MatchedRule != "tool-calls-ask" {
		t.Fatalf("stable tie-break broken: matched %q", out.MatchedRule)
	}
}

func TestEvaluateRejectsInvalidEvent(t *testing.T) {
	ev := mustEval(t)
	bad := testEvent(schema.TypeToolCall, nil)
	bad.Stage = schema.Stage("nope")
	if _, err := ev.Evaluate(bad); err == nil {
		t.Fatal("invalid event must not be evaluated")
	}
	if _, err := ev.Evaluate(nil); err == nil {
		t.Fatal("nil event must not be evaluated")
	}
}

func TestEvaluateDoesNotMutate(t *testing.T) {
	ev := mustEval(t)
	e := testEvent(schema.TypeFileAccess, map[string]string{"path": "/etc/x"})
	before := *e
	if _, err := ev.Evaluate(e); err != nil {
		t.Fatal(err)
	}
	if before.Decision != e.Decision || before.Severity != e.Severity || before.Stage != e.Stage {
		t.Fatal("Evaluate must not mutate the event")
	}
}

func TestEqualsIsCaseSensitive(t *testing.T) {
	ev := mustEval(t)
	// "/ETC/" must NOT match the "/etc/" prefix rule (byte-exact matching).
	out, err := ev.Evaluate(testEvent(schema.TypeFileAccess, map[string]string{"path": "/ETC/passwd"}))
	if err != nil {
		t.Fatal(err)
	}
	if out.MatchedRule != "" {
		t.Fatalf("case-insensitive leak: matched %q", out.MatchedRule)
	}
}
