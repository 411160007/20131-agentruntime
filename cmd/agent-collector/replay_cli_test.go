package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"20131.com/agentruntime/internal/rules"
)

func cmdTestdataDir() string {
	_, f, _, _ := runtime.Caller(0)
	return filepath.Join(filepath.Dir(filepath.Dir(filepath.Dir(f))), "testdata")
}

// TestReplayCLIGoldenRun drives the full subcommand path over the
// shipped golden corpus: the report renders to stdout with the
// record-only stance, and the CLI honestly surfaces the two declared
// known-gap label mismatches instead of exiting silent-green.
func TestReplayCLIGoldenRun(t *testing.T) {
	p, err := rules.Builtin()
	if err != nil {
		t.Fatal(err)
	}
	raw, err := json.Marshal(p)
	if err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	cand := filepath.Join(dir, "cand.json")
	if err := os.WriteFile(cand, raw, 0o600); err != nil {
		t.Fatal(err)
	}
	golden := filepath.Join(cmdTestdataDir(), "golden")

	// Swap os.Stdout for a temp file to capture the report.
	real := os.Stdout
	outf, err := os.CreateTemp(dir, "report")
	if err != nil {
		t.Fatal(err)
	}
	os.Stdout = outf
	err = runReplay(config{
		replayPolicy: cand,
		replayLabels: filepath.Join(golden, "labels.json"),
		rest:         []string{filepath.Join(golden, "normal.jsonl"), filepath.Join(golden, "danger.jsonl")},
	})
	os.Stdout = real
	outf.Close()
	if err == nil || !strings.Contains(err.Error(), "2 label mismatches") {
		t.Fatalf("want honest 2-mismatch error for the declared known gaps, got %v", err)
	}
	body, err := os.ReadFile(outf.Name())
	if err != nil {
		t.Fatal(err)
	}
	var rep struct {
		Stance string `json:"stance"`
		Totals struct {
			Cases int `json:"cases"`
			Held  int `json:"held_lines"`
		} `json:"totals"`
	}
	if err := json.Unmarshal(body, &rep); err != nil {
		t.Fatalf("report is not valid json: %v", err)
	}
	if !strings.Contains(rep.Stance, "record-only, zero enforcement plane") {
		t.Fatalf("CLI report lost the stance line: %q", rep.Stance)
	}
	if rep.Totals.Cases != 129 || rep.Totals.Held != 0 {
		t.Fatalf("CLI totals cases=%d held=%d, want 129/0", rep.Totals.Cases, rep.Totals.Held)
	}
	for _, w := range []string{"denied", "blocked", "killed", "terminated", "enforced", "intercept"} {
		if strings.Contains(strings.ToLower(string(body)), w) {
			t.Fatalf("CLI report contains enforcement word %q", w)
		}
	}
}
