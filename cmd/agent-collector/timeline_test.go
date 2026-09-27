// Timeline snapshot + filter tests: golden rendering pinned against
// testdata/timeline-golden.txt (regenerate deliberately with
// -update + human review only), plus --agent/--since behaviour and the
// read-only corrupt-input refusal the control surface promises.
package main

import (
	"flag"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

var update = flag.Bool("update-timeline-golden", false, "rewrite the timeline golden snapshot from current output (review the diff!)")

func samplePath(t *testing.T) string {
	t.Helper()
	p := filepath.Join("..", "..", "testdata", "timeline-sample.jsonl")
	if _, err := os.Stat(p); err != nil {
		t.Fatalf("timeline fixture missing: %v", err)
	}
	return p
}

func renderFile(t *testing.T, path, agent string, since time.Time) string {
	t.Helper()
	evs, err := timelineEvents(path, agent, since)
	if err != nil {
		t.Fatal(err)
	}
	return strings.Join(renderTimeline(evs), "\n") + "\n"
}

func TestTimelineGoldenSnapshot(t *testing.T) {
	sample := samplePath(t)
	got := renderFile(t, sample, "", time.Time{})
	golden := filepath.Join("..", "..", "testdata", "timeline-golden.txt")
	if *update {
		if err := os.WriteFile(golden, []byte(got), 0o644); err != nil {
			t.Fatal(err)
		}
		t.Logf("golden rewritten (%d bytes) — review before committing", len(got))
	}
	want, err := os.ReadFile(golden)
	if err != nil {
		t.Fatalf("golden snapshot missing (run with -update-timeline-golden once and review): %v", err)
	}
	if got != string(want) {
		t.Errorf("timeline output drifted from golden snapshot:\n--- got ---\n%s--- want ---\n%s", got, want)
	}
	// shape invariants (guard the golden against silent emptiness)
	lines := strings.Split(strings.TrimRight(got, "\n"), "\n")
	if len(lines) != 7 { // header + 6 events
		t.Fatalf("golden must hold header + 6 lines, got %d:\n%s", len(lines), got)
	}
	if !strings.HasPrefix(lines[0], "timeline: 6 event(s)") {
		t.Errorf("header shape wrong: %q", lines[0])
	}
	if !strings.Contains(got, "would_block") || !strings.Contains(got, "policy.decision") {
		t.Error("golden must pin a decision row shape (would_block/policy.decision)")
	}
}

func TestTimelineFilters(t *testing.T) {
	sample := samplePath(t)
	// --agent: exact id filter, header counts what survives
	one := renderFile(t, sample, "agi-tl0002", time.Time{})
	if !strings.HasPrefix(one, "timeline: 2 event(s)") {
		t.Errorf("agent filter count wrong:\n%s", one)
	}
	for _, ln := range strings.Split(one, "\n") {
		if ln != "" && !strings.Contains(ln, "agi-tl0002") && !strings.HasPrefix(ln, "timeline:") {
			t.Errorf("agent filter leaked a row: %q", ln)
		}
	}
	// --since: lower bound on ts
	late, err := time.Parse(time.RFC3339, "2026-09-27T09:05:30Z")
	if err != nil {
		t.Fatal(err)
	}
	s := renderFile(t, sample, "", late)
	if !strings.HasPrefix(s, "timeline: 2 event(s)") {
		t.Errorf("since filter count wrong:\n%s", s)
	}
	if !strings.Contains(s, "would_block") || strings.Contains(s, "tl-04") {
		// tl-04 (09:05:00) excluded, tl-05 decision (09:06) kept
		t.Errorf("since window rows wrong:\n%s", s)
	}
	// combined: agent AND since (intersection semantics)
	comb := renderFile(t, sample, "agi-tl0001", late)
	if !strings.HasPrefix(comb, "timeline: 1 event(s)") || !strings.Contains(comb, "agent.detected") {
		t.Errorf("combined filter wrong:\n%s", comb)
	}
}

func TestTimelineRejectsCorrupt(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "bad.jsonl")
	os.WriteFile(p, []byte("not json\n"), 0o600)
	if _, err := timelineEvents(p, "", time.Time{}); err == nil {
		t.Error("timeline accepted corrupt audit input")
	}
	// empty file renders the header only (0 events)
	e := filepath.Join(dir, "empty.jsonl")
	os.WriteFile(e, []byte(""), 0o600)
	out := renderFile(t, e, "", time.Time{})
	if strings.TrimSpace(out) != "timeline: 0 event(s)" {
		t.Errorf("empty audit must render zero-row timeline, got %q", out)
	}
}
