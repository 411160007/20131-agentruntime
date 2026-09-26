package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"20131.com/agentruntime/internal/policy"
	"20131.com/agentruntime/internal/schema"
)

func TestDemoPolicyAndAgentValid(t *testing.T) {
	if err := DemoPolicy().Validate(); err != nil {
		t.Fatalf("DemoPolicy invalid: %v", err)
	}
	a := DemoAgent("unit")
	if err := a.Validate(); err != nil {
		t.Fatalf("DemoAgent invalid: %v", err)
	}
	if !a.Platform.Valid() {
		t.Fatalf("platform %q must be a supported value on every build target", a.Platform)
	}
}

func TestPipelineCoversAllStages(t *testing.T) {
	ev, err := policy.New(DemoPolicy())
	if err != nil {
		t.Fatal(err)
	}
	events, err := PipelineEvents(DemoAgent("unit"), ev)
	if err != nil {
		t.Fatal(err)
	}
	if len(events) < 4 {
		t.Fatalf("expected a full pipeline of >=4 events, got %d", len(events))
	}
	seen := map[schema.Stage]bool{}
	for _, e := range events {
		seen[e.Stage] = true
		if err := e.Validate(); err != nil {
			t.Fatalf("pipeline produced invalid event: %v", err)
		}
	}
	for _, s := range []schema.Stage{schema.StageProposed, schema.StageEvaluated, schema.StageEnforcement, schema.StageAction} {
		if !seen[s] {
			t.Fatalf("pipeline missing stage %s (spec §1 contract)", s)
		}
	}
}

func TestPipelineEvaluatedDecisionMatchesPolicy(t *testing.T) {
	ev, _ := policy.New(DemoPolicy())
	events, err := PipelineEvents(DemoAgent("unit"), ev)
	if err != nil {
		t.Fatal(err)
	}
	var evaluated, enforcement *schema.Event
	for _, e := range events {
		switch e.Stage {
		case schema.StageEvaluated:
			evaluated = e
		case schema.StageEnforcement:
			enforcement = e
		}
	}
	if evaluated == nil || enforcement == nil {
		t.Fatal("missing evaluated/enforcement records")
	}
	// demo proposal targets ./hello.txt with tool echo -> allow-echo rule
	if evaluated.Decision != schema.DecisionAllow || evaluated.Attrs["matched_rule"] != "allow-echo" {
		t.Fatalf("unexpected evaluation: %+v", evaluated)
	}
	if enforcement.Decision != evaluated.Decision {
		t.Fatal("enforcement record diverges from evaluation")
	}
}

func TestPipelineRejectsBadAgent(t *testing.T) {
	ev, _ := policy.New(DemoPolicy())
	if _, err := PipelineEvents(&schema.Agent{ID: "bad id!"}, ev); err == nil {
		t.Fatal("invalid agent must abort pipeline")
	}
}

func TestBinaryEndToEnd(t *testing.T) {
	// Build the real binary with `go test`'s toolchain and run it once:
	// --version exits 0, default run writes >=1 valid JSONL events.
	t.Parallel()
	dir := t.TempDir()
	bin := filepath.Join(dir, "hello-collector")
	out := filepath.Join(dir, "events.jsonl")

	// go build
	if os.Getenv("HELLO_SKIP_BUILD") != "" {
		t.Skip("build skipped by env")
	}
	goTool := "go" // relies on the Go toolchain being on PATH in the test env
	if err := run(goTool, "build", "-o", bin, "."); err != nil {
		t.Fatalf("build binary: %v", err)
	}
	if err := run(bin, "--version"); err != nil {
		t.Fatalf("--version: %v", err)
	}
	if err := run(bin, "--out", out); err != nil {
		t.Fatalf("run collector: %v", err)
	}
	raw, err := os.ReadFile(out)
	if err != nil {
		t.Fatal(err)
	}
	lines := strings.Split(strings.TrimRight(string(raw), "\n"), "\n")
	if len(lines) < 1 {
		t.Fatalf("expected >=1 JSONL event, got %d lines", len(lines))
	}
	for _, ln := range lines {
		var e schema.Event
		if err := json.Unmarshal([]byte(ln), &e); err != nil {
			t.Fatalf("invalid JSONL line: %v", err)
		}
		if err := e.Validate(); err != nil {
			t.Fatalf("invalid event from real binary: %v", err)
		}
	}
}
