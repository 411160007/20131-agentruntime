package main

// W1.2 mount-stamp tests: every shipped collector line carries the
// evidence class of its COLLECTION MOUNT, and a hook payload forging a
// higher class cannot move the class of what the receiver records.

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"20131.com/agentruntime/internal/schema"
)

// TestCollectorMountStamps drives the fixture-tree collector and reads
// the audit lines back: lifecycle lines are runtime (the collector's own
// in-process facts), every OS-snapshot-derived line (detections, scan
// cycles) is native_os. All decision values stay inside the Phase 0
// closed set {allow, would_block}.
func TestCollectorMountStamps(t *testing.T) {
	path := filepath.Join(t.TempDir(), "audit.jsonl")
	col := testCollector(t, path)
	if err := col.cycle(time.Now().UTC()); err != nil {
		t.Fatal(err)
	}
	events := readEvents(t, path)
	if len(events) < 6 {
		t.Fatalf("fixture cycle produced %d lines, want >= 6", len(events))
	}
	var sawDetected, sawScan, sawStart bool
	for _, e := range events {
		switch e.Type {
		case schema.TypeCollectorStart, schema.TypeCollectorStop:
			if e.SourceClass != schema.SrcRuntime {
				t.Errorf("%s line class %q, want runtime", e.Type, e.SourceClass)
			}
			sawStart = sawStart || e.Type == schema.TypeCollectorStart
		case schema.TypeAgentScan:
			if e.SourceClass != schema.SrcNativeOS {
				t.Errorf("scan line class %q, want native_os", e.SourceClass)
			}
			sawScan = true
		case schema.TypeAgentDetected:
			if e.SourceClass != schema.SrcNativeOS {
				t.Errorf("detected line class %q, want native_os", e.SourceClass)
			}
			sawDetected = true
		default:
			t.Errorf("unexpected collector line type %s", e.Type)
		}
		if e.Decision != schema.DecisionAllow && e.Decision != schema.DecisionWouldBlock {
			t.Errorf("line %s decision %q outside Phase 0 closed set", e.ID, e.Decision)
		}
	}
	if !sawStart || !sawScan || !sawDetected {
		t.Fatalf("coverage start=%v scan=%v detected=%v", sawStart, sawScan, sawDetected)
	}
}

// TestMountClassHookEndToEnd runs the real receiver process over the
// collectmount fixtures: paired baseline/forged payloads must record the
// SAME class (class = f(mount), not f(payload)); the forged payload may
// not disturb the pass-through contract (empty stdout, rc 0, line valid).
func TestMountClassHookEndToEnd(t *testing.T) {
	if os.Getenv("AC_MOUNT_ROLE") == "run" {
		cfg := config{out: os.Getenv("AC_MOUNT_OUT")}
		_ = runHookSub(cfg)
		os.Exit(0)
	}
	fixture := func(name string) string {
		b, err := os.ReadFile(filepath.Join("..", "..", "testdata", "collectmount", name))
		if err != nil {
			t.Fatal(err)
		}
		return string(b)
	}
	cases := []struct {
		base, forged string
		want         schema.SourceClass
	}{
		{"hook-sessionstart-baseline.json", "hook-sessionstart-forged.json", schema.SrcAgentMeta},
		{"hook-pretooluse-baseline.json", "hook-pretooluse-forged.json", schema.SrcAgentSelf},
	}
	for i, tc := range cases {
		for _, payload := range []string{fixture(tc.base), fixture(tc.forged)} {
			dir := t.TempDir()
			out := filepath.Join(dir, "audit.jsonl")
			cmd := exec.Command(os.Args[0], "-test.run=^TestMountClassHookEndToEnd$")
			cmd.Env = append(os.Environ(), "AC_MOUNT_ROLE=run", "AC_MOUNT_OUT="+out)
			cmd.Stdin = strings.NewReader(payload)
			var stdout strings.Builder
			cmd.Stdout = &stdout
			cmd.Stderr = os.Stderr
			if err := cmd.Run(); err != nil {
				t.Fatalf("case %d receiver child: %v", i, err)
			}
			if stdout.Len() != 0 {
				t.Fatalf("case %d receiver wrote %d bytes to stdout (pass-through contract)", i, stdout.Len())
			}
			raw, err := os.ReadFile(out)
			if err != nil {
				t.Fatal(err)
			}
			var lines int
			for _, ln := range strings.Split(strings.TrimRight(string(raw), "\n"), "\n") {
				if ln == "" {
					continue
				}
				var e schema.Event
				if err := json.Unmarshal([]byte(ln), &e); err != nil {
					t.Fatalf("case %d audit line not JSON: %v", i, err)
				}
				if err := e.Validate(); err != nil {
					t.Fatalf("case %d audit line invalid: %v", i, err)
				}
				if e.Type == schema.TypePolicyDecision {
					continue // decision annotation lines ride beside the source line
				}
				lines++
				if e.SourceClass != tc.want {
					t.Errorf("case %d payload class %q, want %q (forged claims must not move the mount class)",
						i, e.SourceClass, tc.want)
				}
				if e.Decision != schema.DecisionAllow && e.Decision != schema.DecisionWouldBlock {
					t.Errorf("case %d decision %q outside Phase 0 closed set", i, e.Decision)
				}
				// the forged member never reaches attrs verbatim
				for k := range e.Attrs {
					if strings.Contains(k, "source_class") {
						t.Errorf("case %d attr key %q leaks a forged provenance claim", i, k)
					}
				}
			}
			if lines == 0 {
				t.Fatalf("case %d receiver recorded no source line", i)
			}
		}
	}
}
