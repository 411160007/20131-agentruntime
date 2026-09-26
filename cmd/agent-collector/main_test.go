package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"20131.com/agentruntime/internal/discovery"
	"20131.com/agentruntime/internal/schema"
)

func fixtureProcs() []discovery.ProcInfo {
	return []discovery.ProcInfo{
		{PID: 1, PPID: 0, Name: "launchish", Cmdline: "/sbin/init"},
		{PID: 100, PPID: 1, Name: "claude", Cmdline: "claude --output-format stream-json"},
		{PID: 101, PPID: 100, Name: "node", Cmdline: "node /opt/claude-code-inspector.js"},
		{PID: 102, PPID: 100, Name: "mcp-server-fs", Cmdline: "mcp-server-fs --root /data"},
		{PID: 103, PPID: 102, Name: "helper-mcp", Cmdline: "helper-mcp --stdio"},
		{PID: 104, PPID: 100, Name: "sh", Cmdline: "sh -c ls"},
		{PID: 105, PPID: 1, Name: "codex", Cmdline: "codex resume"},
		{PID: 106, PPID: 1, Name: "openclaw-gateway", Cmdline: "openclaw-gateway serve"},
		{PID: 107, PPID: 1, Name: "nginx", Cmdline: "nginx: worker process"},
	}
}

func readEvents(t *testing.T, path string) []schema.Event {
	t.Helper()
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var out []schema.Event
	for _, ln := range strings.Split(strings.TrimRight(string(raw), "\n"), "\n") {
		if ln == "" {
			continue
		}
		var e schema.Event
		if err := json.Unmarshal([]byte(ln), &e); err != nil {
			t.Fatalf("line not JSON: %v\n%s", err, ln)
		}
		if err := e.Validate(); err != nil {
			t.Fatalf("line fails schema validation: %v\n%s", err, ln)
		}
		out = append(out, e)
	}
	return out
}

func testCollector(t *testing.T, out string) *collector {
	t.Helper()
	col, err := newCollector(config{out: out, rotateBytes: 0, rotateDaily: false}, "test")
	if err != nil {
		t.Fatal(err)
	}
	col.snapFn = func() ([]discovery.ProcInfo, error) { return fixtureProcs(), nil }
	col.hiddenFn = func(int) bool { return false }
	col.selfPID = 999999 // fixture scan has no self row
	t.Cleanup(col.close)
	return col
}

func TestCycleEmitsValidDetectionAudit(t *testing.T) {
	path := filepath.Join(t.TempDir(), "audit.jsonl")
	col := testCollector(t, path)
	if err := col.cycle(time.Now().UTC()); err != nil {
		t.Fatal(err)
	}
	events := readEvents(t, path)

	var kinds = map[string]int{}
	var scans, start, stop int
	for _, e := range events {
		switch e.Type {
		case schema.TypeCollectorStart:
			start++
		case schema.TypeCollectorStop:
			stop++
		case schema.TypeAgentScan:
			scans++
		case schema.TypeAgentDetected:
			kinds[e.Attrs["kind"]]++
		}
		if e.Stage != schema.StageObservation {
			t.Errorf("discovery event %s in stage %s: Phase 0 stays in observation", e.ID, e.Stage)
		}
		if e.Decision != schema.DecisionAllow {
			t.Errorf("discovery event %s carries decision %s: no blocking vocabulary allowed", e.ID, e.Decision)
		}
	}
	if start != 1 || stop != 0 || scans != 1 {
		t.Errorf("lifecycle counts start=%d stop=%d scan=%d, want 1/0/1", start, stop, scans)
	}
	for _, want := range []string{"claude-code", "codex", "openclaw", "mcp-server"} {
		if kinds[want] == 0 {
			t.Errorf("fixture cycle produced no %s detection (kinds=%v)", want, kinds)
		}
	}
	// nginx (pid 107) must not appear as an agent
	for _, e := range events {
		if e.Type == schema.TypeAgentDetected && e.Attrs["pid"] == "107" {
			t.Error("control process nginx was detected as agent")
		}
	}
	// detected lines carry fingerprint + lineage attrs
	for _, e := range events {
		if e.Type != schema.TypeAgentDetected {
			continue
		}
		for _, k := range []string{"kind", "rule", "pid", "ppid", "name", "cmdline", "observed"} {
			if _, ok := e.Attrs[k]; !ok {
				t.Errorf("detected event missing attr %s: %+v", k, e.Attrs)
			}
		}
	}
}

func TestCycleDedupAcrossScans(t *testing.T) {
	path := filepath.Join(t.TempDir(), "audit.jsonl")
	col := testCollector(t, path)
	if err := col.cycle(time.Now().UTC()); err != nil {
		t.Fatal(err)
	}
	first := len(readEvents(t, path))
	if err := col.cycle(time.Now().UTC()); err != nil {
		t.Fatal(err)
	}
	events := readEvents(t, path)
	if len(events) != first+1 {
		t.Fatalf("second identical scan added %d lines, want exactly 1 (scan event only)", len(events)-first)
	}
	last := events[len(events)-1]
	if last.Type != schema.TypeAgentScan || last.Attrs["new_hits"] != "0" {
		t.Fatalf("second scan should report zero new hits, got %s %v", last.Type, last.Attrs)
	}
}

func TestDetectedAttrsRedacted(t *testing.T) {
	path := filepath.Join(t.TempDir(), "audit.jsonl")
	col, err := newCollector(config{out: path, rotateBytes: 0, rotateDaily: false}, "test")
	if err != nil {
		t.Fatal(err)
	}
	defer col.close()
	secret := "hunter2-supersecret-value"
	col.snapFn = func() ([]discovery.ProcInfo, error) {
		return []discovery.ProcInfo{
			{PID: 1, PPID: 0, Name: "init", Cmdline: "/sbin/init"},
			{PID: 120, PPID: 1, Name: "codex", Cmdline: "codex --token=" + secret + " resume"},
		}, nil
	}
	col.hiddenFn = func(int) bool { return false }
	col.selfPID = 999999
	if err := col.cycle(time.Now().UTC()); err != nil {
		t.Fatal(err)
	}
	raw, _ := os.ReadFile(path)
	if strings.Contains(string(raw), secret) {
		t.Fatalf("audit file leaked command-line secret:\n%s", raw)
	}
	if !strings.Contains(string(raw), "[redacted]") {
		t.Fatal("expected redaction marker in attrs")
	}
}

func TestVersionFlagShape(t *testing.T) {
	// D5 contract: "<name> <version> <goos>/<goarch> (<goversion>)".
	line := "agent-collector " + version + " " + "linux/amd64" + " (go1.27.1)"
	if parts := strings.Fields(line); len(parts) != 4 || !strings.Contains(parts[2], "/") {
		t.Fatalf("version line shape broken: %q", line)
	}
}
