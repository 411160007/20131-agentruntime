//go:build darwin

package discovery

import (
	"os"
	"strings"
	"testing"
)

// psFixture is synthetic `ps -axo pid=,ppid=,args=` output covering the
// four agent families plus controls, run through the real darwin parse
// path (fixture process-tree discovery test, darwin shape).
const psFixture = `  1  0 /sbin/launchd
300  1 /usr/sbin/nginx -g daemon off;
301  1 node /app/api-server.js --port 8080
410 400 node /Users/dev/.nvm/versions/node/v22/bin/../lib/node_modules/@anthropic-ai/claude-code/cli.js --print
411 400 claude --output-format stream-json
412 400 /usr/local/bin/codex exec resume
413 400 openclaw-gateway
414 413 node /opt/claude-code-inspector.js
415 400 node /Users/dev/.npm/@modelcontextprotocol/server-filesystem /data
416 413 python3 /opt/tools/mcp_server_http.py --port 0
417  1 /usr/local/bin/codex-history-view --serve
`

func TestDarwinPsParseFixtureDiscovery(t *testing.T) {
	procs := ParsePsOutput(psFixture)
	if len(procs) == 0 {
		t.Fatal("ParsePsOutput produced no rows")
	}
	d := NewDetector(DefaultFingerprints())
	res := d.Scan(procs, 0)
	kinds := map[Kind][]int{}
	for _, h := range res.Hits {
		kinds[h.Kind] = append(kinds[h.Kind], h.Proc.PID)
	}
	for _, want := range []Kind{KindClaudeCode, KindCodex, KindOpenClaw, KindMCPServer} {
		if len(kinds[want]) == 0 {
			t.Errorf("darwin parse path found no %q agent (hits=%v)", want, kinds)
		}
	}
	for _, h := range res.Hits {
		if h.Proc.PID == 1 || h.Proc.PID == 300 || h.Proc.PID == 301 || h.Proc.PID == 417 {
			t.Errorf("control row %d (%s) matched %s via darwin parse", h.Proc.PID, h.Proc.Name, h.Kind)
		}
	}
	// launchd/agents: attribution must not blanket the table through pid 1.
	if res.Stats.Attributed > len(procs)/2 {
		t.Errorf("over-attribution: %d/%d", res.Stats.Attributed, len(procs))
	}
}

func TestDarwinParseNamesAndExe(t *testing.T) {
	procs := ParsePsOutput(psFixture)
	byPID := map[int]ProcInfo{}
	for _, p := range procs {
		byPID[p.PID] = p
	}
	if p := byPID[413]; p.Name != "openclaw-gateway" || p.PPID != 400 {
		t.Errorf("413 parse wrong: %+v", p)
	}
	if p := byPID[411]; p.Exe != "" {
		t.Errorf("411 has no path in args; Exe should be empty, got %q", p.Exe)
	}
}

// TestDarwinLiveSnapshot runs only on a real macOS runner (CI matrix):
// self pid must appear with sane lineage.
func TestDarwinLiveSnapshot(t *testing.T) {
	procs, err := Snapshot()
	if err != nil {
		t.Fatalf("Snapshot: %v", err)
	}
	self := os.Getpid()
	for _, p := range procs {
		if p.PID == self {
			if p.PPID <= 0 || !strings.Contains(strings.ToLower(p.Cmdline), "test") {
				t.Errorf("self row odd: %+v", p)
			}
			return
		}
	}
	t.Fatalf("self pid %d missing from ps snapshot (%d rows)", self, len(procs))
}
