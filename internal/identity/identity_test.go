package identity

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"20131.com/agentruntime/internal/discovery"
)

func TestAgentIDStableAcrossScansAndRuns(t *testing.T) {
	machine := "cafe0123456789abcdef0123456789ab"
	p := discovery.ProcInfo{PID: 4242, PPID: 1, Name: "codex", Exe: "/usr/local/bin/codex", Cmdline: "codex resume"}
	first := AgentIDFor(machine, p)
	// same binary seen by a later scan: new pid, no run id anywhere in the
	// derivation -> identical agent id
	second := AgentIDFor(machine, discovery.ProcInfo{PID: 999, PPID: 1, Name: "codex", Exe: "/usr/local/bin/codex", Cmdline: "codex --token=*** resume"})
	if first != second {
		t.Fatalf("id drifted across scans: %s vs %s", first, second)
	}
	// a different machine fingerprint yields a different id
	if AgentIDFor("other", p) == first {
		t.Fatal("machine fingerprint ignored in id derivation")
	}
	// different executables never collide
	if AgentIDFor(machine, discovery.ProcInfo{Exe: "/opt/claude"}) == AgentIDFor(machine, discovery.ProcInfo{Exe: "/opt/codex"}) {
		t.Fatal("distinct exes collided")
	}
	// grammar: valid event-ID charset and length
	if !strings.HasPrefix(first, "agi-") || len(first) != 44 {
		t.Fatalf("unexpected id shape: %s", first)
	}
	for _, c := range first {
		if !(c >= 'a' && c <= 'z' || c >= '0' && c <= '9' || c == '-') {
			t.Fatalf("id contains illegal char %q", c)
		}
	}
}

func TestLocatorFallbackChain(t *testing.T) {
	// exe wins when present
	if got := NormalizeLocator(discovery.ProcInfo{Exe: `C:\Agents\Claude\claude.exe`, Cmdline: "ignored"}); got != "c:/agents/claude/claude.exe" {
		t.Fatalf("windows path normalization wrong: %q", got)
	}
	// cmdline fallback, whitespace collapsed + case folded
	p := discovery.ProcInfo{Name: "node", Cmdline: "node   /Opt/App.js"}
	if got := NormalizeLocator(p); got != "node /opt/app.js" {
		t.Fatalf("cmdline normalization wrong: %q", got)
	}
	if LocatorLabel(discovery.ProcInfo{Exe: "/x"}) != "exe" ||
		LocatorLabel(discovery.ProcInfo{Cmdline: "y z"}) != "cmdline" ||
		LocatorLabel(discovery.ProcInfo{Name: "n"}) != "name" {
		t.Fatal("locator label chain wrong")
	}
	// the fallback chain itself must be deterministic
	a := AgentIDFor("m", discovery.ProcInfo{Name: "claude"})
	b := AgentIDFor("m", discovery.ProcInfo{Name: "CLAUDE"})
	if a != b {
		t.Fatal("name-only locator not case-stable")
	}
}

func TestMachineFingerprintLocalOnly(t *testing.T) {
	fp, dedicated := MachineFingerprint()
	if fp == "" {
		t.Fatal("empty machine fingerprint")
	}
	if dedicated {
		if b, err := os.ReadFile("/etc/machine-id"); err != nil || strings.TrimSpace(string(b)) != fp {
			t.Fatal("dedicated fingerprint does not match /etc/machine-id")
		}
	} else if fp == "" || fp != fallbackHostnameOrPlaceholder() {
		t.Fatalf("fallback fingerprint unexpected: %q", fp)
	}
}

func fallbackHostnameOrPlaceholder() string {
	h, err := os.Hostname()
	if err != nil || h == "" {
		return "unknown-host"
	}
	return h
}

func TestStateTransitions(t *testing.T) {
	cases := []struct {
		from, to State
		ok       bool
	}{
		{StateKnown, StateObserved, true},    // automatic on sighting
		{StateObserved, StateKnown, true},    // forget
		{StateObserved, StateTrusted, false}, // NEVER automatic: only SetTrusted
		{StateKnown, StateTrusted, false},
		{StateTrusted, StateKnown, false},
		{StateTrusted, StateObserved, true}, // explicit revocation
		{StateObserved, StateObserved, true},
		{"bogus", StateObserved, false},
	}
	for _, tc := range cases {
		if got := CanTransition(tc.from, tc.to); got != tc.ok {
			t.Errorf("CanTransition(%s,%s)=%v want %v", tc.from, tc.to, got, tc.ok)
		}
	}
	// There is NO edge into trusted in the transition graph: nothing the
	// observation machine does can ever land on trusted.
	for _, from := range []State{StateKnown, StateObserved, StateTrusted} {
		if from == StateTrusted {
			continue // self-transition allowed, but it is not a promotion
		}
		if CanTransition(from, StateTrusted) {
			t.Errorf("transition graph has an edge into trusted from %s", from)
		}
	}
}

func TestRegistryNoAutoTrust(t *testing.T) {
	r := NewRegistry()
	if st := r.Observe("agi-aaa", "codex"); st != StateObserved {
		t.Fatalf("first sighting state=%s want observed", st)
	}
	if st := r.Observe("agi-aaa", "codex"); st != StateObserved {
		t.Fatalf("repeat sighting state=%s", st)
	}
	// explicit promotion works after a sighting
	if err := r.SetTrusted("agi-aaa"); err != nil {
		t.Fatal(err)
	}
	if r.StateOf("agi-aaa") != StateTrusted {
		t.Fatal("SetTrusted did not promote")
	}
	// observation never demotes trusted
	if st := r.Observe("agi-aaa", "codex"); st != StateTrusted {
		t.Fatalf("observation demoted trusted: %s", st)
	}
	// unobserved ids cannot be trusted at all
	if err := r.SetTrusted("agi-never-seen"); err == nil {
		t.Fatal("SetTrusted promoted an unobserved agent")
	}
	// explicit revocation path
	if err := r.Revoke("agi-aaa"); err != nil {
		t.Fatal(err)
	}
	if r.StateOf("agi-aaa") != StateObserved {
		t.Fatal("revoke failed")
	}
}

func TestTrustFileParsing(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "trust")
	if err := os.WriteFile(path, []byte("# user managed list\nagi-abc123\n\nagi-def456  # hand-picked\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	ids, err := LoadTrustFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if len(ids) != 2 || ids[0] != "agi-abc123" || ids[1] != "agi-def456" {
		t.Fatalf("trust file parse wrong: %v", ids)
	}
	if _, err := LoadTrustFile(filepath.Join(dir, "missing")); err == nil {
		t.Fatal("missing trust file read succeeded")
	}
}
