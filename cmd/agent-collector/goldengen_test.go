// w14_goldengen_test.go - golden maintenance path: run
// REPORT_GEN_GOLDEN=1 go test -run TestGenerateReportGolden to rewrite
// testdata/report-golden.txt from the renderer. Normal test runs
// (and CI) skip this file's test entirely; drift is caught by
// TestReportGolden, never silently repaired here.
package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestGenerateReportGolden(t *testing.T) {
	if os.Getenv("REPORT_GEN_GOLDEN") != "1" {
		t.Skip("golden generation is opt-in (REPORT_GEN_GOLDEN=1)")
	}
	evs, err := timelineEvents(filepath.Join("..", "..", "testdata", "timeline-sample.jsonl"), "", time.Time{})
	if err != nil {
		t.Fatalf("load sample: %v", err)
	}
	out := strings.Join(renderReport(evs), "\n") + "\n"
	if err := os.WriteFile(filepath.Join("..", "..", "testdata", "report-golden.txt"), []byte(out), 0o644); err != nil {
		t.Fatalf("write golden: %v", err)
	}
}
