package auditlog

import (
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"20131.com/agentruntime/internal/schema"
)

func ev(id string) *schema.Event {
	return &schema.Event{
		V:        schema.SchemaVersion,
		TS:       time.Date(2026, 9, 27, 1, 0, 0, 0, time.UTC),
		ID:       id,
		AgentID:  "agent-01",
		Stage:    schema.StageProposed,
		Type:     schema.TypeCommandProposed,
		Decision: schema.DecisionAllow,
		Severity: schema.SevInfo,
		Summary:  "hello",
	}
}

func TestWriteAndReadBack(t *testing.T) {
	path := filepath.Join(t.TempDir(), "sub", "events.jsonl")
	w, err := Open(path)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	for i, id := range []string{"ev-a", "ev-b"} {
		if err := w.WriteEvent(ev(id)); err != nil {
			t.Fatalf("WriteEvent %d: %v", i, err)
		}
	}
	if w.Count() != 2 {
		t.Fatalf("Count = %d, want 2", w.Count())
	}
	if w.Bytes() == 0 {
		t.Fatal("Bytes should track file size")
	}
	if err := w.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}

	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	lines := strings.Split(strings.TrimRight(string(raw), "\n"), "\n")
	if len(lines) != 2 {
		t.Fatalf("want 2 JSONL lines, got %d", len(lines))
	}
	for _, ln := range lines {
		var got schema.Event
		if err := json.Unmarshal([]byte(ln), &got); err != nil {
			t.Fatalf("line not valid JSON: %v", err)
		}
		if got.V != schema.SchemaVersion {
			t.Fatalf("version not persisted: %+v", got)
		}
	}

	// append semantics: reopening adds to the same file
	w2, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := w2.WriteEvent(ev("ev-c")); err != nil {
		t.Fatal(err)
	}
	w2.Close()
	raw, _ = os.ReadFile(path)
	if got := strings.Count(strings.TrimRight(string(raw), "\n"), "\n"); got != 2 {
		t.Fatalf("append broken: %d separators, want 2", got)
	}
}

func TestFilePermissions(t *testing.T) {
	if runtime.GOOS == "windows" {
		// Go reports 0666 for regular files on NTFS: Unix mode bits are
		// not an access-control mechanism there (ACLs are). Owner-only
		// privacy on windows rides on the per-user profile ACL of the
		// output location; a native ACL hardening pass is Phase 1 scope.
		t.Skip("permission-bit assertion is unix-only; windows privacy is ACL-based (see docs/architecture.md)")
	}
	path := filepath.Join(t.TempDir(), "perm.jsonl")
	w, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	w.WriteEvent(ev("ev-p"))
	w.Close()
	st, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if perm := st.Mode().Perm(); perm != 0o600 {
		t.Fatalf("audit file mode = %o, want 600 (local secrets stay private)", perm)
	}
}

func TestInvalidEventRejectedWithoutWrite(t *testing.T) {
	path := filepath.Join(t.TempDir(), "rej.jsonl")
	w, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	bad := ev("ev-x")
	bad.Stage = schema.Stage("wild")
	if err := w.WriteEvent(bad); err == nil {
		t.Fatal("invalid event must be rejected")
	}
	if w.Count() != 0 {
		t.Fatal("rejected event must not be counted")
	}
	if st, _ := os.Stat(path); st.Size() != 0 {
		t.Fatal("rejected event must not touch file bytes")
	}
	w.Close()
}

func TestOpenErrors(t *testing.T) {
	if _, err := Open(""); err == nil {
		t.Fatal("empty path must fail")
	}
	// path whose parent is an existing regular file: mkdir must fail
	dir := t.TempDir()
	f := filepath.Join(dir, "file")
	os.WriteFile(f, []byte("x"), 0o600)
	if _, err := Open(filepath.Join(f, "nested.jsonl")); err == nil {
		t.Fatal("open under a non-directory must fail")
	}
}

func TestDoubleCloseSafe(t *testing.T) {
	w, err := Open(filepath.Join(t.TempDir(), "dc.jsonl"))
	if err != nil {
		t.Fatal(err)
	}
	if err := w.Close(); err != nil {
		t.Fatal(err)
	}
	if err := w.Close(); err != nil {
		t.Fatalf("second Close must be a no-op, got %v", err)
	}
}
