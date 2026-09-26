//go:build windows

package discovery

import (
	"os"
	"strings"
	"testing"
)

// winFixture is a synthetic windows-shaped process table (Toolhelp rows
// carry basename+exe path+PEB cmdline): four agent families and controls,
// run through the same pure detector the windows Snapshot() feeds.
func winFixture() []ProcInfo {
	return []ProcInfo{
		{PID: 4, PPID: 0, Name: "System", Cmdline: ""},
		{PID: 900, PPID: 700, Name: "claude.exe", Exe: `C:\Users\dev\AppData\Roaming\npm\claude.exe`,
			Cmdline: `C:\Users\dev\AppData\Roaming\npm\claude.exe --output-format stream-json`},
		{PID: 901, PPID: 700, Name: "node.exe", Exe: `C:\Program Files\nodejs\node.exe`,
			Cmdline: `node.exe C:\Users\dev\AppData\Local\nvm\v22.14.0\node_modules\@anthropic-ai\claude-code\cli.js`},
		{PID: 902, PPID: 900, Name: "mcp-server-fs.exe", Exe: `C:\tools\mcp-server-fs.exe`,
			Cmdline: `C:\tools\mcp-server-fs.exe --root D:\proj`},
		{PID: 903, PPID: 700, Name: "codex.exe", Cmdline: `C:\npm\prefix\codex.exe resume`},
		{PID: 904, PPID: 903, Name: "cmd.exe", Cmdline: `cmd /c cargo build`},
		{PID: 905, PPID: 700, Name: "openclaw-gateway.exe", Exe: `D:\agents\openclaw-gateway.exe`,
			Cmdline: `D:\agents\openclaw-gateway.exe`},
		{PID: 906, PPID: 700, Name: "chrome.exe", Cmdline: `chrome.exe --profile-directory Default`},
		{PID: 907, PPID: 1000, Name: "orphan-helper-mcp.exe", Cmdline: `orphan-helper-mcp.exe --stdio`},
		{PID: 908, PPID: 1000, Name: "mcp-server-db.exe", Cmdline: `mcp-server-db.exe --connect`},
	}
}

func TestWindowsFixtureDiscoveryShapes(t *testing.T) {
	d := NewDetector(DefaultFingerprints())
	res := d.Scan(winFixture(), 0)
	kinds := map[int][]Kind{}
	for _, h := range res.Hits {
		kinds[h.Proc.PID] = append(kinds[h.Proc.PID], h.Kind)
	}
	expectKind(kinds, t, 900, KindClaudeCode)
	expectKind(kinds, t, 901, KindClaudeCode) // @anthropic- in path
	expectKind(kinds, t, 902, KindMCPServer)
	expectKind(kinds, t, 903, KindCodex)
	expectKind(kinds, t, 905, KindOpenClaw)
	expectKind(kinds, t, 908, KindMCPServer)
	expectKind(kinds, t, 907, KindMCPServer) // parent-chain via sibling 908
	expectNoHit(kinds, t, 4)
	expectNoHit(kinds, t, 906) // chrome must never be an agent
	if got := res.UnderAgent[904]; got != 903 || res.UnderAgentKnd[904] != KindCodex {
		t.Errorf("cmd.exe under codex: got root=%d kind=%q", got, res.UnderAgentKnd[904])
	}
	if _, ok := res.UnderAgent[902]; ok {
		t.Error("902 is a direct hit and must not also be attributed")
	}
}

// TestWindowsLiveSnapshot runs only on a real windows runner (CI matrix).
func TestWindowsLiveSnapshot(t *testing.T) {
	procs, err := Snapshot()
	if err != nil {
		t.Fatalf("Snapshot: %v", err)
	}
	if len(procs) < 3 {
		t.Fatalf("live snapshot too small: %d rows", len(procs))
	}
	self := os.Getpid()
	for _, p := range procs {
		if p.PID == self {
			if !strings.Contains(strings.ToLower(p.Name), "test") {
				t.Errorf("self name unexpected: %q", p.Name)
			}
			return
		}
	}
	t.Fatalf("self pid %d missing from toolhelp snapshot", self)
}
