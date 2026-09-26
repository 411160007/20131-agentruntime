//go:build linux

package discovery

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

// Snapshot reads the local process table from /proc: /proc/<pid>/comm,
// /proc/<pid>/stat (field 4 = PPID), /proc/<pid>/cmdline (NUL-separated),
// and /proc/<pid>/exe (readlink). This is the polling source for the
// "Linux proc" leg; auditd integration is a later privilege-dependent
// upgrade and is not required for discovery.
//
// Partial reads are tolerated: a process that exits between readdir and
// open is simply skipped or recorded with whatever fields were readable,
// and coverage lands in ScanStats via the empty-Cmdline count.
func Snapshot() ([]ProcInfo, error) {
	dirEntries, err := os.ReadDir("/proc")
	if err != nil {
		return nil, fmt.Errorf("discovery: read /proc: %w", err)
	}
	var out []ProcInfo
	for _, de := range dirEntries {
		if !de.IsDir() {
			continue
		}
		pid, err := strconv.Atoi(de.Name())
		if err != nil {
			continue
		}
		out = append(out, procInfoFromProcOS(pid))
	}
	if len(out) == 0 {
		return nil, fmt.Errorf("discovery: /proc yielded no numeric pid directories")
	}
	return out, nil
}

func procInfoFromProcOS(pid int) ProcInfo {
	base := "/proc/" + strconv.Itoa(pid)
	p := ProcInfo{PID: pid}
	if b, err := os.ReadFile(filepath.Join(base, "comm")); err == nil {
		p.Name = strings.TrimSpace(string(b))
	}
	if b, err := os.ReadFile(filepath.Join(base, "stat")); err == nil {
		// comm itself may contain spaces and parentheses; fields start
		// after the LAST ')' so field indexing is stable.
		if i := bytes.LastIndexByte(b, ')'); i >= 0 && i+3 < len(b) {
			rest := strings.Fields(string(b[i+2:]))
			if len(rest) >= 2 {
				if pp, err := strconv.Atoi(rest[1]); err == nil {
					p.PPID = pp
				}
			}
		}
	}
	if b, err := os.ReadFile(filepath.Join(base, "cmdline")); err == nil {
		args := bytes.Split(b, []byte{0})
		// trailing NUL leaves one empty element
		for len(args) > 0 && len(args[len(args)-1]) == 0 {
			args = args[:len(args)-1]
		}
		var parts []string
		for _, a := range args {
			parts = append(parts, string(a))
		}
		p.Cmdline = strings.Join(parts, " ")
	}
	if exe, err := os.Readlink(filepath.Join(base, "exe")); err == nil {
		p.Exe = exe
	}
	return p
}

// hiddenFromUs reports whether a /proc read failure means the process
// exists but hides its cmdline from us (true for other users' processes:
// the file reads back empty), used to log coverage honestly.
func hiddenFromUs(pid int) bool {
	b, err := os.ReadFile("/proc/" + strconv.Itoa(pid) + "/cmdline")
	if err != nil {
		return true
	}
	return len(b) == 0
}
