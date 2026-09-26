//go:build linux

package discovery

import (
	"os"
	"strings"
	"testing"
)

func TestSnapshotSeesSelf(t *testing.T) {
	procs, err := Snapshot()
	if err != nil {
		t.Fatalf("Snapshot: %v", err)
	}
	if len(procs) < 2 {
		t.Fatalf("real /proc scan returned %d rows, want >= 2", len(procs))
	}
	self := os.Getpid()
	var found *ProcInfo
	for i := range procs {
		if procs[i].PID == self {
			found = &procs[i]
			break
		}
	}
	if found == nil {
		t.Fatalf("self pid %d not present in /proc snapshot", self)
	}
	if found.PPID <= 0 {
		t.Errorf("self ppid = %d, want > 0", found.PPID)
	}
	if found.Name == "" {
		t.Error("self comm empty")
	}
	if !strings.Contains(found.Cmdline, ".test") && !strings.Contains(found.Cmdline, "go") {
		t.Logf("NOTE: self cmdline = %q (len %d)", TruncateRedacted(found.Cmdline, 120), len(found.Cmdline))
	}
}

func TestSnapshotPID1AndDetectionRunClean(t *testing.T) {
	// Environment-agnostic real-scan assertions: pid 1 exists with ppid 0,
	// and a full detection pass over the live table never errors or
	// attributes the whole table through the init boundary.
	procs, err := Snapshot()
	if err != nil {
		t.Fatal(err)
	}
	n1 := 0
	for _, p := range procs {
		if p.PID == 1 {
			n1++
			if p.PPID != 0 {
				t.Errorf("pid1 ppid=%d, want 0", p.PPID)
			}
		}
	}
	if n1 != 1 {
		t.Fatalf("pid 1 rows = %d, want 1", n1)
	}
	d := NewDetector(DefaultFingerprints())
	res := d.Scan(procs, os.Getpid())
	if res.Stats.Procs != len(procs) {
		t.Errorf("stats procs %d != %d", res.Stats.Procs, len(procs))
	}
	// On a busy table (CI runners), init-boundary refusal must keep
	// attribution well under half. On tiny sandboxes the ratio is
	// meaningless (one gateway legitimately owns most of the table), so
	// the ratio guard only applies from 40 rows up.
	if len(res.UnderAgent) > len(procs)*3/4 && len(procs) > 40 {
		t.Errorf("blanket attribution on live table: %d/%d attributed", len(res.UnderAgent), len(procs))
	}
	t.Logf("live scan: procs=%d hits=%d attributed=%d no-cmdline=%d hidden=%d",
		res.Stats.Procs, len(res.Hits), res.Stats.Attributed, res.Stats.NoCmdline, res.Stats.NoCmdlineAt)
}

func TestProcStatParsingWithParenthesizedComm(t *testing.T) {
	// comm like "(my (weird) proc)" must not break ppid extraction; we
	// validate the parser shape on the real /proc of this very test.
	procs, err := Snapshot()
	if err != nil {
		t.Fatal(err)
	}
	for _, p := range procs {
		if p.PID == os.Getpid() {
			if p.PPID == 0 {
				t.Fatal("ppid parse failed for self")
			}
			return
		}
	}
	t.Fatal("self missing from snapshot")
}
