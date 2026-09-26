package schema

import (
	"strings"
	"testing"
	"time"
)

func validEvent() *Event {
	return &Event{
		V:        SchemaVersion,
		TS:       time.Date(2026, 9, 27, 1, 30, 0, 123456789, time.UTC),
		ID:       "ev-0001",
		AgentID:  "agent-01",
		Stage:    StageProposed,
		Type:     TypeCommandProposed,
		Decision: DecisionAllow,
		Severity: SevLow,
		Summary:  "proposal",
		Attrs:    map[string]string{"tool": "echo"},
	}
}

func TestEventValid(t *testing.T) {
	if err := validEvent().Validate(); err != nil {
		t.Fatalf("expected valid, got %v", err)
	}
	e := validEvent()
	e.Attrs = nil
	if err := e.Validate(); err != nil {
		t.Fatalf("nil attrs must be valid, got %v", err)
	}
}

func TestEventInvalid(t *testing.T) {
	cases := []struct {
		name string
		mut  func(*Event)
	}{
		{"wrong version", func(e *Event) { e.V = 0 }},
		{"future version", func(e *Event) { e.V = SchemaVersion + 1 }},
		{"empty id", func(e *Event) { e.ID = "" }},
		{"bad agent ref", func(e *Event) { e.AgentID = "bad id!" }},
		{"zero ts", func(e *Event) { e.TS = time.Time{} }},
		{"ts year too old", func(e *Event) { e.TS = time.Date(1999, 1, 1, 0, 0, 0, 0, time.UTC) }},
		{"ts year too new", func(e *Event) { e.TS = time.Date(2999, 1, 1, 0, 0, 0, 0, time.UTC) }},
		{"unknown stage", func(e *Event) { e.Stage = Stage("teleport") }},
		{"unknown type", func(e *Event) { e.Type = EventType("exploit.fire") }},
		{"unknown decision", func(e *Event) { e.Decision = Decision("blocked") }},
		{"severity negative", func(e *Event) { e.Severity = Severity(-1) }},
		{"severity too high", func(e *Event) { e.Severity = SevCritical + 1 }},
		{"empty summary", func(e *Event) { e.Summary = "" }},
		{"summary too long", func(e *Event) { e.Summary = strings.Repeat("x", 513) }},
		{"attr empty key", func(e *Event) { e.Attrs = map[string]string{"": "v"} }},
		{"attr key too long", func(e *Event) { e.Attrs = map[string]string{strings.Repeat("k", 65): "v"} }},
		{"attr value too long", func(e *Event) { e.Attrs = map[string]string{"k": strings.Repeat("v", 1025)} }},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			e := validEvent()
			tc.mut(e)
			if err := e.Validate(); err == nil {
				t.Fatalf("expected error for %s", tc.name)
			}
		})
	}
	// boundary: exactly at limits is valid
	e := validEvent()
	e.Summary = strings.Repeat("x", 512)
	e.Attrs = map[string]string{strings.Repeat("k", 64): strings.Repeat("v", 1024)}
	if err := e.Validate(); err != nil {
		t.Fatalf("boundary sizes must pass, got %v", err)
	}
}

func TestStageCoverageOfPipeline(t *testing.T) {
	// The four pipeline steps from the product model must all exist as stages.
	for _, s := range []Stage{StageProposed, StageEvaluated, StageEnforcement, StageAction} {
		if !s.Valid() {
			t.Fatalf("pipeline stage %s missing or invalid", s)
		}
	}
}

func TestDecisionFor(t *testing.T) {
	for ef, want := range map[Effect]Decision{
		EffectAllow:      DecisionAllow,
		EffectAsk:        DecisionAsk,
		EffectWouldBlock: DecisionWouldBlock,
	} {
		got, err := DecisionFor(ef)
		if err != nil || got != want {
			t.Fatalf("DecisionFor(%s) = %s, %v; want %s", ef, got, err, want)
		}
	}
	if _, err := DecisionFor(Effect("zap")); err == nil {
		t.Fatal("unknown effect must error")
	}
}
