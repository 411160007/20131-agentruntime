//go:build linux

package main

import (
	"path/filepath"
	"testing"
)

// TestRunOnceRealProcScan exercises the full binary path against the
// live /proc of the test machine: collector.start, one agent.scan (plus
// any detections), collector.stop — every line schema-valid.
func TestRunOnceRealProcScan(t *testing.T) {
	out := filepath.Join(t.TempDir(), "live.jsonl")
	if err := run(config{out: out, once: true, rotateBytes: 1 << 20, rotateDaily: true}); err != nil {
		t.Fatalf("run --once: %v", err)
	}
	events := readEvents(t, out)
	if len(events) < 3 {
		t.Fatalf("live scan produced %d events, want >= 3 (start/scan/stop)", len(events))
	}
	if events[0].Type == "" {
		t.Fatal("empty first event")
	}
	var start, scan, stop bool
	for _, e := range events {
		switch e.Type {
		case "collector.start":
			start = true
		case "agent.scan":
			scan = true
		case "collector.stop":
			stop = true
		}
		if e.Attrs["mode"] == "block" || string(e.Stage) == "enforcement" {
			t.Fatalf("Phase 0 violation: blocking-flavored event %+v", e)
		}
	}
	if !start || !scan || !stop {
		t.Fatalf("missing lifecycle events: start=%v scan=%v stop=%v", start, scan, stop)
	}
}
