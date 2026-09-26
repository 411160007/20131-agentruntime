//go:build darwin

package discovery

import (
	"fmt"
	"os/exec"
	"strconv"
	"strings"
)

// Snapshot collects the macOS process table by polling `ps`.
//
// Honest-scope note: the planned upgrade path is a libproc
// (proc_listpids/proc_pidpath) native poller; that requires cgo plus an
// osx SDK cross-compile toolchain and conflicts with the CGO_ENABLED=0
// architecture decision, so it is deferred. `ps -axo` reads the same
// user-space KERN_PROC snapshot libproc exposes, so discovery semantics
// are equivalent for v0; the cost is one short-lived `ps` child per scan
// instead of a syscall. Recorded as a known deviation, not a silent one.
func Snapshot() ([]ProcInfo, error) {
	// Fixed argv, no user input reaches exec.
	out, err := exec.Command("ps", "-axo", "pid=,ppid=,args=").Output()
	if err != nil {
		return nil, fmt.Errorf("discovery: ps snapshot failed: %w", err)
	}
	return ParsePsOutput(string(out)), nil
}

// ParsePsOutput turns `ps -axo pid=,ppid=,args=` text into ProcInfo rows:
// two leading integer columns, remainder is the full command line. Name
// is the basename of the first token (the launch path). Split out from
// Snapshot so the exact output shape is unit-testable with fixtures.
func ParsePsOutput(s string) []ProcInfo {
	var out []ProcInfo
	for _, line := range strings.Split(s, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		i := strings.IndexByte(line, ' ')
		if i < 0 {
			continue
		}
		rest := strings.TrimLeft(line[i:], " ")
		j := strings.IndexByte(rest, ' ')
		if j < 0 {
			continue
		}
		cmd := strings.TrimLeft(rest[j:], " ")
		pid, err1 := strconv.Atoi(line[:i])
		ppid, err2 := strconv.Atoi(rest[:j])
		if err1 != nil || err2 != nil || pid <= 0 || cmd == "" {
			continue
		}
		p := ProcInfo{PID: pid, PPID: ppid, Cmdline: cmd}
		if toks := strings.Fields(cmd); len(toks) > 0 {
			p.Name = strings.TrimSuffix(base(toks[0]), ".exe")
			if strings.Contains(toks[0], "/") {
				p.Exe = toks[0]
			}
		}
		out = append(out, p)
	}
	return out
}

func base(s string) string {
	if i := strings.LastIndexByte(s, '/'); i >= 0 {
		return s[i+1:]
	}
	return s
}

// hiddenFromUs reports whether `ps` cannot see the pid's args at all
// (kernel-level privacy or the process vanished) — coverage honesty hook.
func hiddenFromUs(pid int) bool {
	// #nosec G204 — fixed argv, integer-formatted pid.
	return exec.Command("ps", "-p", strconv.Itoa(pid), "-o", "args=").Run() != nil
}
