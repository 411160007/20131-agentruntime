// goldengen_test.go - the report golden is a first-class artifact:
// this test pins its invariants directly (non-empty, report header,
// one census line per pinned vocabulary family) so regeneration is a
// deliberate edit of testdata/report-golden.txt after a reviewed
// renderer change. There is deliberately no t.Skip path here: the
// inherited evals gate counts any SKIP as red, and an env-gated
// generator would be a live skip in every serial run.
package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestReportGoldenInvariants(t *testing.T) {
	data, err := os.ReadFile(filepath.Join("..", "..", "testdata", "report-golden.txt"))
	if err != nil {
		t.Fatalf("read golden: %v", err)
	}
	lines := strings.Split(strings.TrimSuffix(string(data), "\n"), "\n")
	if len(lines) < 10 {
		t.Fatalf("golden collapsed to %d lines", len(lines))
	}
	if !strings.HasPrefix(lines[0], "report: ") {
		t.Fatalf("golden header drifted: %q", lines[0])
	}
	need := []string{"decisions:", "event types:", "severity census:", "agents: "}
	for _, n := range need {
		found := false
		for _, ln := range lines {
			if strings.TrimSuffix(ln, " ") == n || strings.HasPrefix(ln, n) {
				found = true
				break
			}
		}
		if !found {
			t.Fatalf("golden census family missing: %q", n)
		}
	}
}
