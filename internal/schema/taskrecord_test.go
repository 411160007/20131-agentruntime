package schema

import (
	"testing"
	"time"
)

// baseTaskRecordEvent builds a minimal valid task.record line; tests
// mutate one face at a time and assert Validate's verdict.
func baseTaskRecordEvent() Event {
	return Event{
		V:        SchemaVersion,
		TS:       time.Date(2026, 10, 11, 14, 55, 0, 0, time.UTC),
		ID:       "trk-t01",
		AgentID:  "agi-test",
		Stage:    StageObservation,
		Type:     TypeTaskRecord,
		Decision: DecisionAllow,
		Severity: SevInfo,
		Summary:  "task record-form fixture line",
		TaskID:   "sess-test:1",
	}
}

func TestTaskRecordValidate(t *testing.T) {
	good := baseTaskRecordEvent()
	good.TaskStatus = TaskCompleted
	if err := good.Validate(); err != nil {
		t.Fatalf("valid task.record rejected: %v", err)
	}
	// A legacy line carrying neither field validates as before: the
	// record form is additive and zero-breakage for old JSONL.
	legacy := baseTaskRecordEvent()
	legacy.Type = TypeTurnStop
	legacy.TaskID = ""
	if err := legacy.Validate(); err != nil {
		t.Fatalf("legacy-shaped line without additive fields rejected: %v", err)
	}
	// An additive pair beside any other type is accepted as a view
	// annotation (record-only, never decides).
	annotated := baseTaskRecordEvent()
	annotated.Type = TypeTurnStop
	annotated.TaskStatus = TaskActive
	if err := annotated.Validate(); err != nil {
		t.Fatalf("additive-annotated line rejected: %v", err)
	}
	bad := []struct {
		name string
		mut  func(*Event)
	}{
		{"missing status", func(e *Event) { e.TaskStatus = "" }},
		{"wild status", func(e *Event) { e.TaskStatus = TaskStatus("succeeded") }},
		{"missing id", func(e *Event) { e.TaskID = "" }},
		{"non-positional id", func(e *Event) { e.TaskID = "sess:test:1" }},
		{"id without ordinal", func(e *Event) { e.TaskID = "sess-test:" }},
		{"ordinal not numeric", func(e *Event) { e.TaskID = "sess-test:one" }},
		{"empty session part", func(e *Event) { e.TaskID = ":1" }},
	}
	for _, tc := range bad {
		e := baseTaskRecordEvent()
		e.TaskStatus = TaskCompleted
		tc.mut(&e)
		if err := e.Validate(); err == nil {
			t.Errorf("task.record defect %q accepted", tc.name)
		}
	}
	// Wild status on a non-task type is rejected too: the validator
	// recognizes the vocabulary instead of silently accepting extras.
	w := baseTaskRecordEvent()
	w.Type = TypeToolCall
	w.TaskStatus = TaskStatus("done")
	if err := w.Validate(); err == nil {
		t.Error("wild task_status on another type accepted")
	}
}

func TestTaskStatusVocabularyCensus(t *testing.T) {
	want := []string{"open", "active", "completed", "failed", "abandoned", "blocked", "unknown"}
	got := AllTaskStatuses()
	if len(got) != len(want) {
		t.Fatalf("task status census is %d, want %d", len(got), len(want))
	}
	for i, tok := range want {
		if got[i] != tok {
			t.Errorf("task status %d is %q, want %q", i, got[i], tok)
		}
	}
	if !TaskStatus("").Valid() {
		t.Error("absent-key legacy default rejected by TaskStatus.Valid")
	}
	if TaskStatus("closed").Valid() {
		t.Error("synonym outside the closed set validated")
	}
}
