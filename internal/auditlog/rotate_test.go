package auditlog

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"20131.com/agentruntime/internal/schema"
)

func evAt(id string, ts time.Time) *schema.Event {
	return &schema.Event{
		V:        schema.SchemaVersion,
		TS:       ts,
		ID:       id,
		AgentID:  "agent-01",
		Stage:    schema.StageObservation,
		Type:     schema.TypeAgentScan,
		Decision: schema.DecisionAllow,
		Severity: schema.SevInfo,
		Summary:  "scan line for rotation tests",
	}
}

func countLines(t *testing.T, path string) int {
	t.Helper()
	raw, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return 0
		}
		t.Fatalf("read %s: %v", path, err)
	}
	if len(raw) == 0 {
		return 0
	}
	return strings.Count(strings.TrimRight(string(raw), "\n"), "\n") + 1
}

func TestNilRotationMatchesWriter(t *testing.T) {
	path := filepath.Join(t.TempDir(), "a.jsonl")
	l, err := OpenLog(path, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer l.Close()
	for i := 0; i < 3; i++ {
		if err := l.WriteEvent(evAt("e1", time.Now().UTC())); err != nil {
			t.Fatal(err)
		}
	}
	if n := countLines(t, path); n != 3 {
		t.Fatalf("nil rotation: %d lines, want 3", n)
	}
	if len(l.Segments()) != 0 {
		t.Fatalf("nil rotation must not create segments, got %v", l.Segments())
	}
}

func TestSizeRotation(t *testing.T) {
	path := filepath.Join(t.TempDir(), "size.jsonl")
	l, err := OpenLog(path, &Rotation{MaxBytes: 120})
	if err != nil {
		t.Fatal(err)
	}
	defer l.Close()
	for i := range [6]struct{}{} {
		id := string(rune('a' + i))
		if err := l.WriteEvent(evAt("sz-"+id, time.Now().UTC())); err != nil {
			t.Fatal(err)
		}
	}
	segs := l.Segments()
	if len(segs) < 2 {
		t.Fatalf("expected >=2 rotated segments with MaxBytes=120 over 6 lines, got %d: %v", len(segs), segs)
	}
	total := countLines(t, path)
	for _, s := range segs {
		total += countLines(t, s)
		st, err := os.Stat(s)
		if err != nil {
			t.Fatal(err)
		}
		if st.Mode().Perm() != 0o600 && st.Size() > 0 {
			t.Errorf("segment %s mode %o, want 600", s, st.Mode().Perm())
		}
	}
	if total != 6 {
		t.Fatalf("lines lost across rotation: %d, want 6", total)
	}
	if l.Count() != 6 {
		t.Fatalf("Count=%d want 6", l.Count())
	}
	if l.Bytes() > 120 {
		t.Fatalf("live segment should stay within MaxBytes, got %d", l.Bytes())
	}
}

func TestSizeRotationSegmentPerms(t *testing.T) {
	path := filepath.Join(t.TempDir(), "perm.jsonl")
	l, err := OpenLog(path, &Rotation{MaxBytes: 10})
	if err != nil {
		t.Fatal(err)
	}
	defer l.Close()
	if err := l.WriteEvent(evAt("pp", time.Now().UTC())); err != nil {
		t.Fatal(err)
	}
	// first line exceeded 10 bytes -> rotated; rotated segment keeps 0600
	// (rename preserves mode), live file recreated at 0600 by Open.
	st, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if st.Mode().Perm() != 0o600 {
		t.Fatalf("live segment mode %o, want 600", st.Mode().Perm())
	}
	for _, s := range l.Segments() {
		sst, err := os.Stat(s)
		if err != nil {
			t.Fatal(err)
		}
		if sst.Mode().Perm() != 0o600 {
			t.Fatalf("rotated segment %s mode %o, want 600", s, sst.Mode().Perm())
		}
	}
}

func TestDailyCut(t *testing.T) {
	path := filepath.Join(t.TempDir(), "daily.jsonl")
	l, err := OpenLog(path, &Rotation{Daily: true})
	if err != nil {
		t.Fatal(err)
	}
	defer l.Close()
	day1 := time.Date(2026, 9, 26, 23, 59, 0, 0, time.UTC)
	day2 := time.Date(2026, 9, 27, 0, 5, 0, 0, time.UTC)
	if err := l.WriteEvent(evAt("d1", day1)); err != nil {
		t.Fatal(err)
	}
	if err := l.WriteEvent(evAt("d2", day2)); err != nil {
		t.Fatal(err)
	}
	if err := l.WriteEvent(evAt("d3", day2.Add(time.Hour))); err != nil {
		t.Fatal(err)
	}
	segs := l.Segments()
	if len(segs) != 1 {
		t.Fatalf("expected exactly 1 rotated segment at day boundary, got %v", segs)
	}
	if n := countLines(t, path); n != 2 {
		t.Fatalf("live segment %d lines, want 2 (both on day2)", n)
	}
	if n := countLines(t, segs[0]); n != 1 {
		t.Fatalf("rotated segment %d lines, want 1 (day1 event)", n)
	}
}

func TestInvalidEventNoRotationSideEffects(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "inval.jsonl")
	l, err := OpenLog(path, &Rotation{MaxBytes: 1, Daily: true})
	if err != nil {
		t.Fatal(err)
	}
	defer l.Close()
	bad := evAt("bad", time.Now().UTC())
	bad.Stage = schema.Stage("wild")
	before := l.Bytes()
	if err := l.WriteEvent(bad); err == nil {
		t.Fatal("invalid event must be rejected")
	}
	if l.Count() != 0 {
		t.Fatal("rejected event counted")
	}
	if l.Bytes() != before {
		t.Fatalf("bytes moved on rejection: %d -> %d", before, l.Bytes())
	}
	if len(l.Segments()) != 0 {
		t.Fatalf("rejection triggered rotation: %v", l.Segments())
	}
}

func TestKeepHistoryPrunesOldestOnly(t *testing.T) {
	path := filepath.Join(t.TempDir(), "keep.jsonl")
	l, err := OpenLog(path, &Rotation{MaxBytes: 10, KeepHistory: 2})
	if err != nil {
		t.Fatal(err)
	}
	defer l.Close()
	// seed a foreign file that shares the base prefix: prune must never touch it
	foreign := path + ".notes"
	if err := os.WriteFile(foreign, []byte("keep me"), 0o600); err != nil {
		t.Fatal(err)
	}
	for n := 0; n < 5; n++ {
		if err := l.WriteEvent(evAt(fmt.Sprintf("kh-%d", n), time.Now().UTC())); err != nil {
			t.Fatal(err)
		}
		time.Sleep(1100 * time.Millisecond) // distinct segment stamps
	}
	segs := l.Segments()
	if len(segs) > 2 {
		t.Fatalf("KeepHistory=2 but %d segments: %v", len(segs), segs)
	}
	if _, err := os.Stat(foreign); err != nil {
		t.Fatalf("prune deleted a non-segment file: %v", err)
	}
}

func TestSegmentsSortedOldestFirst(t *testing.T) {
	dir := t.TempDir()
	base := filepath.Join(dir, "s.jsonl")
	for _, suffix := range []string{".20260101T000000Z", ".20260103T000000Z", ".20260102T000001Z.1", ".backup"} {
		if err := os.WriteFile(base+suffix, nil, 0o600); err != nil {
			t.Fatal(err)
		}
	}
	l := &Log{path: base}
	got := l.Segments()
	want := []string{base + ".20260101T000000Z", base + ".20260102T000001Z.1", base + ".20260103T000000Z"}
	if len(got) != len(want) {
		t.Fatalf("segments = %v, want %v", got, want)
	}
	for i := range got {
		if got[i] != want[i] {
			t.Fatalf("segments[%d] = %s, want %s", i, got[i], want[i])
		}
	}
}
