// Package discovery identifies AI-agent processes from OS process-tree
// snapshots. It is deliberately split in two:
//
//   - Platform snapshot sources (scan_linux.go, scan_darwin.go,
//     scan_windows.go) collect raw ProcInfo records using only OS-local
//     mechanisms: /proc on Linux, polling `ps` output on macOS, and
//     Toolhelp32/QueryFullProcessImageName/PEB reads on Windows.
//   - The pure detector in this file turns []ProcInfo into agent hits and
//     family relations. It never touches the OS, so identical fixture
//     process trees exercise identical matching logic on every platform.
//
// Phase 0 boundary: discovery is read-only observation. Nothing here
// signals to, injects into, terminates, or otherwise interferes with the
// processes it classifies.
package discovery

import (
	"fmt"
	"regexp"
	"strings"
)

// ProcInfo is one snapshot row: what the OS reported about a process.
// Fields the source cannot read are left empty; the detector tolerates
// partial rows and records coverage honestly (see ScanStats).
type ProcInfo struct {
	PID     int
	PPID    int
	Name    string // process/executable name as the OS reports it
	Exe     string // executable path when available
	Cmdline string // full command line when available
}

// Kind classifies a fingerprint hit. The string values match the
// schema.AgentKind vocabulary in internal/schema.
type Kind string

const (
	KindClaudeCode Kind = "claude-code"
	KindCodex      Kind = "codex"
	KindOpenClaw   Kind = "openclaw"
	KindMCPServer  Kind = "mcp-server"
)

// Fingerprint is one named matching rule. Command is matched
// case-insensitively against "name + ' ' + exe + ' ' + cmdline"; process
// names differ in case between operating systems, and command lines embed
// both lowercase package paths and capitalized executable names.
type Fingerprint struct {
	Kind    Kind
	Rule    string
	Command *regexp.Regexp
}

// maxChainDepth bounds ancestor walks so a malformed or cyclic snapshot
// (e.g. synthetic data with a PPID loop) cannot hang or over-attribute.
const maxChainDepth = 16

// DefaultFingerprints returns the built-in agent fingerprint table:
// Claude Code, Codex, OpenClaw, and MCP servers (direct pattern plus the
// parent-chain rule applied by the detector).
//
// Note on "gateway": a bare "gateway" token is far too broad (system and
// networking processes use it), so the OpenClaw rule keys on the product
// name, its npm package, and the specific "index.js gateway" launch form
// of the local gateway supervisor.
func DefaultFingerprints() []Fingerprint {
	return []Fingerprint{
		{Kind: KindClaudeCode, Rule: "claude-cli",
			Command: regexp.MustCompile(`(?i)(^|[/\\\s])claude($|[\\/\s])|(^|[/\\\s])claude\.(exe|cmd|bat|ps1)($|[\s])|@anthropic-`)},
		{Kind: KindClaudeCode, Rule: "claude-inspector",
			Command: regexp.MustCompile(`(?i)claude-code[-/](inspector|server|mcp)[a-z]*\.js`)},
		{Kind: KindCodex, Rule: "codex-cli",
			Command: regexp.MustCompile(`(?i)(^|[/\\\s])codex($|[\\/\s])|(^|[/\\\s])codex\.(exe|cmd|bat|js)($|[\s])|@openai/codex|codex-(command-runner|exec)`)},
		{Kind: KindOpenClaw, Rule: "openclaw",
			Command: regexp.MustCompile(`(?i)\bopenclaw\b|@openclaw/|index\.js[[:space:]]+gateway`)},
		{Kind: KindMCPServer, Rule: "mcp-server-binary",
			Command: regexp.MustCompile(`(?i)(^|[/\s_-])mcp[-_]server($|[^a-z])|@modelcontextprotocol/|mcp_server[./]`)},
		// mcp-parent-chain is evaluated by the detector (needs lineage),
		// not by direct command matching; Command stays nil.
		{Kind: KindMCPServer, Rule: "mcp-parent-chain"},
	}
}

// Hit is one classified agent process.
type Hit struct {
	Proc  ProcInfo
	Kind  Kind
	Rule  string // fingerprint rule name that matched
	Roots []Kind // kind(s) matched by direct fingerprints, in rule order
}

// ScanStats reports snapshot coverage so collectors can log what was
// readable and what was not — silent gaps would make the audit dishonest.
type ScanStats struct {
	Procs       int // rows in the snapshot
	NoCmdline   int // rows whose Cmdline was empty at scan time
	NoCmdlineAt int // rows whose cmdline was hidden from us by the OS (unix)
	Attributed  int // rows attached to an agent family
	TruncPPID1  int // rows dropped from attribution by the pid<=1 boundary
}

// Result is the detector output for one process snapshot.
type Result struct {
	Hits          []Hit
	MCPSiblings   []ProcInfo // mcp-like children of an agent whose parent died
	UnderAgent    map[int]int
	UnderAgentKnd map[int]Kind
	Stats         ScanStats
}

type fpGroup struct {
	kind Kind
	rule string
	re   *regexp.Regexp
}

// Detector matches fingerprints against snapshot rows and derives family
// relations. Safe for concurrent use after construction (read-only state).
type Detector struct {
	fps     []fpGroup
	mcpName *regexp.Regexp
	mcpArgs *regexp.Regexp
}

// NewDetector builds a Detector from a fingerprint table (typically
// DefaultFingerprints). Malformed rule shapes (non-nil Command only for
// direct kinds; "mcp-parent-chain" carries no Command) are accepted as-is:
// a nil-Command direct rule simply never matches via Command.
func NewDetector(fps []Fingerprint) *Detector {
	d := &Detector{
		mcpName: regexp.MustCompile(`(?i)mcp|modelcontextprotocol`),
		mcpArgs: regexp.MustCompile(`(?i)mcp[-_]server|@modelcontextprotocol/`),
	}
	for _, f := range fps {
		if f.Kind == KindMCPServer && f.Rule == "mcp-parent-chain" {
			continue
		}
		d.fps = append(d.fps, fpGroup{kind: f.Kind, rule: f.Rule, re: f.Command})
	}
	return d
}

func procHay(p ProcInfo) string {
	return p.Name + " " + p.Exe + " " + p.Cmdline
}

// Scan classifies a snapshot. pids may exclude the caller's own pid (the
// collector) to avoid self-attribution; pass 0 to include everything.
func (d *Detector) Scan(procs []ProcInfo, excludePID int) Result {
	var res Result
	res.UnderAgent = map[int]int{}
	res.UnderAgentKnd = map[int]Kind{}

	byPID := make(map[int]ProcInfo, len(procs))
	for _, p := range procs {
		byPID[p.PID] = p
	}

	// pass 1: direct fingerprints.
	kindsByPID := map[int][]Kind{}
	direct := map[int]bool{}
	for _, p := range procs {
		if p.PID == excludePID {
			continue
		}
		hay := procHay(p)
		var matched []Kind
		for _, g := range d.fps {
			if g.re == nil || !g.re.MatchString(hay) {
				continue
			}
			kind := g.kind
			matched = append(matched, kind)
			res.Hits = append(res.Hits, Hit{Proc: p, Kind: kind, Rule: g.rule})
		}
		if len(matched) > 0 {
			kindsByPID[p.PID] = append(kindsByPID[p.PID], matched...)
			direct[p.PID] = true
		}
	}

	// pass 2: ancestor attribution to the nearest agent.
	for _, p := range procs {
		if p.PID == excludePID || direct[p.PID] {
			continue
		}
		pid, kind, truncated := nearestAgent(byPID, kindsByPID, p.PID)
		if pid > 0 {
			res.UnderAgent[p.PID] = pid
			res.UnderAgentKnd[p.PID] = kind
			res.Stats.Attributed++
		}
		if truncated {
			res.Stats.TruncPPID1++
		}
	}

	// pass 3: mcp-parent-chain. A process with no agent-matching ancestor
	// but a sibling under the same parent that does match — and whose own
	// shape looks MCP-ish — is an MCP server whose supervisor exited
	// (claude/codex spawn trees reparent mcp servers).
	type kid struct {
		name, hay string
	}
	parents := map[int][]kid{}
	for _, p := range procs {
		if p.PID == excludePID {
			continue
		}
		parents[p.PPID] = append(parents[p.PPID], kid{name: p.Name, hay: procHay(p)})
	}
	hasDirectSibling := func(ppid int, selfPID int) bool {
		for _, p := range procs {
			if p.PID == selfPID || p.PID == excludePID {
				continue
			}
			if p.PPID == ppid && direct[p.PID] {
				return true
			}
		}
		return false
	}
	for _, p := range procs {
		if p.PID == excludePID || direct[p.PID] {
			continue
		}
		if _, ok := res.UnderAgent[p.PID]; ok {
			continue // already attributed as a plain family member
		}
		if p.PPID == 0 || p.PPID == 1 || !hasDirectSibling(p.PPID, p.PID) {
			continue
		}
		hay := procHay(p)
		// Candidates here already failed every direct fingerprint, so any
		// "mcp" signal left in the row (process name or args) combined with
		// an agent-matching sibling classifies the row as an MCP server.
		if d.mcpName.MatchString(p.Name) || d.mcpArgs.MatchString(hay) {
			res.MCPSiblings = append(res.MCPSiblings, p)
			res.Hits = append(res.Hits, Hit{Proc: p, Kind: KindMCPServer, Rule: "mcp-parent-chain"})
			kindsByPID[p.PID] = append(kindsByPID[p.PID], KindMCPServer)
		}
	}

	// Own matched kinds per hit row (canonical rule order).
	for i := range res.Hits {
		h := &res.Hits[i]
		if ks, ok := kindsByPID[h.Proc.PID]; ok {
			h.Roots = ks
		}
	}

	res.Stats.Procs = len(procs)
	for _, p := range procs {
		if p.Cmdline == "" {
			res.Stats.NoCmdline++
		}
	}
	return res
}

// nearestAgent walks the ancestor chain from pid's parent upward.
// Attribution is refused when the chain first reaches pid<=1 or exceeds
// maxChainDepth — the pid<=1 refusal (truncated=true) is the guard that
// keeps an init-like supervisor from claiming the entire process table.
func nearestAgent(byPID map[int]ProcInfo, kinds map[int][]Kind, fromPPID int) (int, Kind, bool) {
	pid := fromPPID
	for depth := 0; depth < maxChainDepth; depth++ {
		if pid <= 1 {
			return 0, "", true
		}
		if ks, ok := kinds[pid]; ok && len(ks) > 0 {
			return pid, ks[0], false
		}
		p, ok := byPID[pid]
		if !ok || p.PPID == pid {
			return 0, "", false
		}
		pid = p.PPID
	}
	return 0, "", true
}

// --- privacy redaction -------------------------------------------------
//
// Audit records embed short command-line fragments. Secrets frequently
// ride on command lines, so every OS-derived string that reaches an
// event attribute goes through Redact first.

var (
	flagsWithSecret = map[string]bool{
		"-i": true, "--identity": true, "-o": true, "--identityfile": true,
		"-p": true, "-w": true,
	}
	secretFlags = map[string]bool{
		"--token": true, "--password": true, "--pass": true, "--secret": true,
		"--api-key": true, "--apikey": true, "--key": true, "--credential": true,
	}
	reLongSecret = regexp.MustCompile(`(?i)(token|password|passwd|secret|api[-_]?key|authorization|credential)s?=([^\s]+)`)
	reURLUser    = regexp.MustCompile(`(?i)[a-z][a-z0-9+.-]*://[^/\s]*@[^/\s]*`)
	// reBareMail catches plain account-at-domain tokens on command lines.
	reBareMail = regexp.MustCompile(`(?i)[A-Za-z0-9._%+-]+@[A-Za-z0-9-]+(\.[A-Za-z0-9-]+)+`)
	// reAuthHeader catches HTTP header-shaped credentials ("Authorization:
	// Bearer <tok>", "-H Authorization: <tok>", "x-api-key: <tok>"): the
	// colon-space form that reLongSecret (key=value) cannot see, and the
	// dominant shape in agent tool command lines hitting adapter audits.
	reAuthHeader = regexp.MustCompile("(?i)(\\b(?:proxy-)?authorization\\s*[:=]\\s*(?:(?:bearer|basic|token)\\s+)?|x-api-key\\s*[:=]\\s*)([^\\s\"',;]+)")
)

const redactMarker = "[redacted]"

// Redact removes obvious credential material from a process string:
// user@host and scheme://user:pass@host shapes, values of known secret
// flags (--token=..., ssh -i /path), key-file paths, and HTTP
// authorization header values (Bearer/Basic/plain, plus x-api-key).
func Redact(s string) string {
	if s == "" {
		return s
	}
	s = reURLUser.ReplaceAllStringFunc(s, func(u string) string {
		i := strings.LastIndex(u, "@")
		return u[:i] + redactMarker + u[i+1:]
	})
	s = reBareMail.ReplaceAllString(s, redactMarker)
	s = reLongSecret.ReplaceAllString(s, redactMarker)
	s = reAuthHeader.ReplaceAllString(s, "$1"+redactMarker)

	toks := strings.Fields(s)
	var out []string
	skipNext := false
	for _, t := range toks {
		if skipNext {
			out = append(out, redactMarker)
			skipNext = false
			continue
		}
		low := strings.ToLower(t)
		if strings.HasPrefix(low, "--") {
			name := low
			if eq := strings.Index(name, "="); eq >= 0 {
				name = name[:eq]
			}
			if secretFlags[name] {
				if strings.Contains(t, "=") {
					out = append(out, t[:strings.Index(t, "=")+1]+redactMarker)
				} else {
					skipNext = true
					out = append(out, t)
				}
				continue
			}
		} else if len(t) == 2 && flagsWithSecret[low] {
			skipNext = true
			out = append(out, t)
			continue
		}
		if strings.Contains(t, "id_rsa") || strings.Contains(t, "id_ed25519") || strings.Contains(t, "id_srv") ||
			strings.HasSuffix(low, ".pem") || strings.HasSuffix(low, ".p12") || strings.HasSuffix(low, ".key") {
			out = append(out, redactMarker)
			continue
		}
		out = append(out, t)
	}
	return strings.Join(out, " ")
}

// TruncateRedacted returns Redact(s) cut to at most n bytes (markers may
// be visually clipped by the ellipsis, never is a secret revealed —
// redaction happens before truncation).
func TruncateRedacted(s string, n int) string {
	r := Redact(s)
	if n < 4 {
		if len(r) > n {
			return r[:n]
		}
		return r
	}
	if len(r) <= n {
		return r
	}
	return r[:n-3] + "..."
}

// String helpers used by the collector for labels.
func KindLabel(k Kind) string { return string(k) }

// HiddenFromUs reports whether the OS hides this pid's command line from
// the collector (protected process / permission), used for honest
// coverage accounting by the collector.
func HiddenFromUs(pid int) bool { return hiddenFromUs(pid) }

// ValidateFPSanity is a construction guard for callers assembling custom
// fingerprint tables: every direct rule needs a compiled Command.
func ValidateFPSanity(fps []Fingerprint) error {
	for _, f := range fps {
		if f.Rule == "" {
			return fmt.Errorf("discovery: fingerprint rule with empty name")
		}
		if f.Rule != "mcp-parent-chain" && f.Command == nil {
			return fmt.Errorf("discovery: fingerprint %s: nil Command for direct rule", f.Rule)
		}
	}
	return nil
}
