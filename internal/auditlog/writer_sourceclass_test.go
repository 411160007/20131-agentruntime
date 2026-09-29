package auditlog

import (
	"os"
	"path/filepath"
	"testing"

	"20131.com/agentruntime/internal/schema"
)

// TestWildSourceClassRejectedWithoutWrite reuses the zero-byte rejection
// gate shape (the writer validates before the first byte moves): an
// event carrying a source_class outside the six-class vocabulary is
// rejected and the audit file keeps the exact byte count it had before
// the attempt. Legacy events with the field absent keep writing
// unchanged (the additive proof on the write side).
func TestWildSourceClassRejectedWithoutWrite(t *testing.T) {
	path := filepath.Join(t.TempDir(), "srcclass.jsonl")
	w, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer w.Close()

	// one good legacy line first: absent field must still write
	if err := w.WriteEvent(ev("ev-legacy-1")); err != nil {
		t.Fatalf("legacy event (absent source_class) rejected: %v", err)
	}
	before, _ := os.Stat(path)

	for _, wild := range []schema.SourceClass{
		"Native_OS", "native", "agent_self_description", "system", "llm", "1", " ",
	} {
		e := ev("ev-wild-1")
		e.SourceClass = wild
		if err := w.WriteEvent(e); err == nil {
			t.Fatalf("wild source_class %q accepted at write gate", string(wild))
		}
	}
	if w.Count() != 1 {
		t.Fatalf("rejected wild events must not be counted, count=%d", w.Count())
	}
	after, _ := os.Stat(path)
	if after.Size() != before.Size() {
		t.Fatalf("rejected events must not touch file bytes: %d -> %d", before.Size(), after.Size())
	}

	// every vocabulary class writes cleanly (positive control: the gate
	// above is class-vocabulary strictness, not a blanket field ban)
	for i, c := range schema.AllSourceClasses() {
		e := ev("ev-class-" + string(rune('a'+i)))
		e.SourceClass = schema.SourceClass(c)
		if err := w.WriteEvent(e); err != nil {
			t.Fatalf("vocabulary class %q rejected: %v", c, err)
		}
	}
	if w.Count() != 7 {
		t.Fatalf("want 7 written (1 legacy + 6 classes), got %d", w.Count())
	}
}
