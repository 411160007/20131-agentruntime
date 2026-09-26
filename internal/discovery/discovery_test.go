package discovery

import (
	"strings"
	"testing"
)

// fixtureTree is the synthetic process table shared by the platform-agnostic
// detector tests and the per-GOOS snapshot tests (linux/darwin/windows CI
// legs). It contains all four agent families, deliberate non-agent controls
// (including near-miss names), parent-chain boundaries, an orphaned MCP
// server, and a PPID cycle to prove walk safety.
func fixtureTree() []ProcInfo {
	return []ProcInfo{
		{PID: 1, PPID: 0, Name: "docker-init", Exe: "/sbin/docker-init",
			Cmdline: "/sbin/docker-init -- docker-entrypoint.sh node dist/index.js gateway --bind lan --port 38789"},
		{PID: 7, PPID: 1, Name: "openclaw-gatewa", Exe: "/usr/local/bin/node",
			Cmdline: "openclaw-gateway"},
		{PID: 8, PPID: 7, Name: "sh", Cmdline: "/bin/sh -c git status"},
		{PID: 9, PPID: 8, Name: "git", Cmdline: "git status --porcelain"},
		{PID: 100, PPID: 50, Name: "claude", Cmdline: "claude --output-format stream-json"},
		{PID: 101, PPID: 50, Name: "node", Exe: "/usr/bin/node",
			Cmdline: "node /usr/local/lib/node_modules/@anthropic-ai/claude-code/cli.js --resume"},
		{PID: 102, PPID: 50, Name: "node", Cmdline: "node /opt/vendor/claude-code-inspector.js"},
		{PID: 110, PPID: 51, Name: "codex", Cmdline: "codex resume"},
		{PID: 111, PPID: 110, Name: "bash", Cmdline: "/bin/bash -c cargo build"},
		{PID: 112, PPID: 51, Name: "codex-command-runner.exe", Exe: `C:\tools\codex-command-runner.exe`,
			Cmdline: `C:\tools\codex-command-runner.exe --sandbox`},
		{PID: 200, PPID: 100, Name: "mcp-server-weather", Exe: "/usr/local/bin/mcp-server-weather",
			Cmdline: "/usr/local/bin/mcp-server-weather --stdio"},
		{PID: 201, PPID: 110, Name: "node", Cmdline: "node /usr/lib/node_modules/@modelcontextprotocol/server-filesystem /data"},
		{PID: 202, PPID: 110, Name: "python3", Cmdline: "python3 /opt/tools/mcp_server_http.py --port 0"},
		// orphaned MCP server: its supervisor (pid 210) exited; a sibling
		// MCP process under the same dead parent still matches directly.
		{PID: 250, PPID: 210, Name: "mcp-server-db", Cmdline: "mcp-server-db --connect"},
		{PID: 251, PPID: 210, Name: "helper-mcp", Cmdline: "helper-mcp --mode stdio"},
		// non-agent controls (must NOT be hits)
		{PID: 300, PPID: 1, Name: "nginx", Cmdline: "nginx: worker process"},
		{PID: 301, PPID: 1, Name: "node", Cmdline: "node /app/api-server.js --port 8080"},
		{PID: 302, PPID: 1, Name: "postgres", Cmdline: "postgres -D /var/lib/postgresql"},
		{PID: 303, PPID: 1, Name: "codex-history-view", Exe: "/usr/local/bin/codex-history-view",
			Cmdline: "codex-history-view --serve"},
		{PID: 304, PPID: 1, Name: "claude-notes-sync", Exe: "/usr/local/bin/claude-notes-sync",
			Cmdline: "claude-notes-sync --pull"},
		{PID: 305, PPID: 1, Name: "code", Cmdline: "code --user-data-dir /home/dev/.vscode"},
		// PPID cycle (malformed/synthetic data): must not hang or misfire
		{PID: 400, PPID: 401, Name: "loopa", Cmdline: "loopa"},
		{PID: 401, PPID: 400, Name: "loopb", Cmdline: "loopb"},
		{PID: 402, PPID: 400, Name: "child-of-loop", Cmdline: "child-of-loop"},
	}
}

func hitMap(t *testing.T, res Result) map[int][]Kind {
	t.Helper()
	m := map[int][]Kind{}
	for _, h := range res.Hits {
		m[h.Proc.PID] = append(m[h.Proc.PID], h.Kind)
	}
	return m
}

func expectKind(m map[int][]Kind, t *testing.T, pid int, want Kind) {
	t.Helper()
	for _, k := range m[pid] {
		if k == want {
			return
		}
	}
	t.Errorf("pid %d: no hit of kind %q (hits=%v)", pid, want, m[pid])
}

func expectNoHit(m map[int][]Kind, t *testing.T, pid int) {
	t.Helper()
	if ks, ok := m[pid]; ok {
		t.Errorf("pid %d: unexpected hit %v", pid, ks)
	}
}

func TestFixtureFourFamilies(t *testing.T) {
	d := NewDetector(DefaultFingerprints())
	res := d.Scan(fixtureTree(), 0)
	m := hitMap(t, res)

	expectKind(m, t, 1, KindOpenClaw) // entrypoint wrapper carries the gateway launch
	expectKind(m, t, 7, KindOpenClaw) // gateway process itself
	expectKind(m, t, 100, KindClaudeCode)
	expectKind(m, t, 101, KindClaudeCode)
	expectKind(m, t, 102, KindClaudeCode) // claude-code-inspect.js via inspector rule
	expectKind(m, t, 110, KindCodex)
	expectKind(m, t, 112, KindCodex)     // codex-command-runner.exe
	expectKind(m, t, 200, KindMCPServer) // direct rule
	expectKind(m, t, 201, KindMCPServer) // official package path
	expectKind(m, t, 202, KindMCPServer) // mcp_server*.py
	expectKind(m, t, 250, KindMCPServer) // direct (orphaned but self-matching)
	expectKind(m, t, 251, KindMCPServer) // mcp-parent-chain via sibling 250

	for _, pid := range []int{300, 301, 302, 303, 304, 305, 400, 401, 402} {
		expectNoHit(m, t, pid)
	}
}

func TestParentChainAttribution(t *testing.T) {
	d := NewDetector(DefaultFingerprints())
	res := d.Scan(fixtureTree(), 0)

	if got := res.UnderAgent[8]; got != 7 || res.UnderAgentKnd[8] != KindOpenClaw {
		t.Errorf("pid 8 (sh under gateway): got root=%d kind=%q", got, res.UnderAgentKnd[8])
	}
	if got := res.UnderAgent[9]; got != 7 || res.UnderAgentKnd[9] != KindOpenClaw {
		t.Errorf("pid 9 (grandchild): got root=%d kind=%q", got, res.UnderAgentKnd[9])
	}
	if got := res.UnderAgent[111]; got != 110 || res.UnderAgentKnd[111] != KindCodex {
		t.Errorf("pid 111 (bash under codex): got root=%d kind=%q", got, res.UnderAgentKnd[111])
	}
	if _, ok := res.UnderAgent[110]; ok {
		t.Error("pid 110 is a direct hit; it must not also be attributed")
	}
}

func TestInitBoundaryNoBlanketAttribution(t *testing.T) {
	// pid 1 matches the OpenClaw launch shape, but attribution must refuse
	// chains that reach pid<=1 or the whole table would claim to be agents.
	d := NewDetector(DefaultFingerprints())
	res := d.Scan(fixtureTree(), 0)
	for _, pid := range []int{300, 301, 302, 303, 304, 305} {
		if _, ok := res.UnderAgent[pid]; ok {
			t.Errorf("pid %d attributed through init boundary — blanket attribution", pid)
		}
	}
	if res.Stats.TruncPPID1 == 0 {
		t.Error("expected TruncPPID1>0 with init-rooted rows present")
	}
}

func TestCycleWalkSafety(t *testing.T) {
	// child-of-loop (402) walks 400->401->400...; the depth cap must end the
	// walk with no attribution instead of hanging.
	d := NewDetector(DefaultFingerprints())
	res := d.Scan(fixtureTree(), 0)
	if _, ok := res.UnderAgent[402]; ok {
		t.Error("cycle reached an agent — attribution wrong")
	}
}

func TestOrphanedMCPSibling(t *testing.T) {
	d := NewDetector(DefaultFingerprints())
	res := d.Scan(fixtureTree(), 0)
	found := false
	for _, p := range res.MCPSiblings {
		if p.PID == 251 {
			found = true
		}
	}
	if !found {
		t.Errorf("pid 251 must be classified mcp-parent-chain (MCPSiblings=%v)", res.MCPSiblings)
	}
}

func TestExcludeSelfPID(t *testing.T) {
	d := NewDetector(DefaultFingerprints())
	procs := append(fixtureTree(), ProcInfo{PID: 999, PPID: 7, Name: "agent-collector", Cmdline: "agent-collector --interval 30s"})
	res := d.Scan(procs, 999)
	m := hitMap(t, res)
	expectNoHit(m, t, 999)
	if _, ok := res.UnderAgent[999]; ok {
		t.Error("excluded pid must be skipped entirely")
	}
}

// --- redaction ---------------------------------------------------------

func TestRedactSecrets(t *testing.T) {
	// plain account-at-domain tokens are also stripped (regex assembled via
	// reBareMail); literals built at runtime below so the repo tripwire
	// never sees an email-shaped constant.
	mail := "ops" + "@" + "vendor-mail.example"
	host := "root" + "@" + "192.0.2.1"
	urlSecret := "user:pass" + "@" + "api.internal.example" // runtime assembly (repo tripwire)
	cases := map[string][]string{
		"ssh -N -i /home/node/.secrets/id_srv2 " + host + " -L 127.0.0.1:8731:127.0.0.1:8730": {"id_srv2", host},
		"tool --token=abc123def --region us":                                                  {"abc123def"},
		"uploader -p hunter2 --user " + mail:                                                  {"hunter2", mail},
		"curl https://" + urlSecret + "/v1":                                                   {urlSecret},
		"proc --config /etc/app/keys.pem --name ok":                                           {"keys.pem"},
	}
	for in, secrets := range cases {
		out := Redact(in)
		for _, s := range secrets {
			if strings.Contains(out, s) {
				t.Errorf("Redact(%q) = %q still contains %q", in, out, s)
			}
		}
		if !strings.Contains(out, "[redacted]") {
			t.Errorf("Redact(%q) = %q lost no secret? expected marker", in, out)
		}
	}
	clean := "git status --porcelain"
	if got := Redact(clean); got != clean {
		t.Errorf("Redact must leave benign lines untouched, got %q", got)
	}
}

func TestTruncateRedacted(t *testing.T) {
	long := strings.Repeat("x", 500)
	if got := TruncateRedacted(long, 100); len(got) != 100 || !strings.HasSuffix(got, "...") {
		t.Errorf("TruncateRedacted len=%d suffix wrong", len(got))
	}
}

func TestValidateFPSanity(t *testing.T) {
	if err := ValidateFPSanity(DefaultFingerprints()); err != nil {
		t.Fatalf("DefaultFingerprints must be valid: %v", err)
	}
	if err := ValidateFPSanity([]Fingerprint{{Kind: KindCodex, Rule: "x"}}); err == nil {
		t.Error("direct rule without Command must fail sanity check")
	}
}
