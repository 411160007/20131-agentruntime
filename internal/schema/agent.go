// Package schema defines the core data model of the 20131 Agent Security
// Runtime: Agent identity records, pipeline Events, and Policy documents.
//
// The event model follows the runtime's fundamental pipeline:
//
//	Agent Proposed -> Policy Evaluated -> Native Enforcement -> Action
//
// Every record carries an explicit validation pass (Validate) before it may
// be persisted; the audit writer relies on this to keep the local audit log
// free of malformed or semantically invalid entries.
package schema

import (
	"fmt"
	"unicode/utf8"
)

// idPattern is not imported from regexp-heavy helpers on purpose: IDs are a
// small, fixed grammar so validation stays cheap on hot paths.
func validID(s string) bool {
	if len(s) == 0 || len(s) > 64 {
		return false
	}
	for i := 0; i < len(s); i++ {
		c := s[i]
		ok := c >= 'a' && c <= 'z' ||
			c >= 'A' && c <= 'Z' ||
			c >= '0' && c <= '9' ||
			c == '.' || c == '_' || c == '-'
		if !ok {
			return false
		}
	}
	first := s[0]
	if !(first >= 'a' && first <= 'z' || first >= 'A' && first <= 'Z' || first >= '0' && first <= '9') {
		return false
	}
	return true
}

// Platform identifies a supported operating-system family.
type Platform string

const (
	PlatformLinux   Platform = "linux"
	PlatformDarwin  Platform = "darwin"
	PlatformWindows Platform = "windows"
)

// Valid reports whether p is a supported platform.
func (p Platform) Valid() bool {
	switch p {
	case PlatformLinux, PlatformDarwin, PlatformWindows:
		return true
	}
	return false
}

// AgentKind classifies the family of a supervised agent process.
type AgentKind string

const (
	KindClaudeCode AgentKind = "claude-code"
	KindCodex      AgentKind = "codex"
	KindOpenClaw   AgentKind = "openclaw"
	KindMCPServer  AgentKind = "mcp-server"
	KindSystem     AgentKind = "system"
	KindUnknown    AgentKind = "unknown"
)

// Valid reports whether k is a known agent kind.
func (k AgentKind) Valid() bool {
	switch k {
	case KindClaudeCode, KindCodex, KindOpenClaw, KindMCPServer, KindSystem, KindUnknown:
		return true
	}
	return false
}

// Agent is the identity record of a supervised AI-agent process.
type Agent struct {
	ID       string    `json:"id"`
	Name     string    `json:"name"`
	Kind     AgentKind `json:"kind"`
	Platform Platform  `json:"platform"`
	Version  string    `json:"version,omitempty"`
	PID      int       `json:"pid,omitempty"`
	PPID     int       `json:"ppid,omitempty"`
}

// MaxNameLen bounds free-text fields to keep audit lines small.
const MaxNameLen = 128

// Validate checks all invariants of an Agent record.
func (a *Agent) Validate() error {
	if !validID(a.ID) {
		return fmt.Errorf("schema: agent id %q is empty, too long (>64), or contains invalid characters", a.ID)
	}
	if a.Name == "" {
		return fmt.Errorf("schema: agent %s: name is required", a.ID)
	}
	if len(a.Name) > MaxNameLen || !utf8.ValidString(a.Name) {
		return fmt.Errorf("schema: agent %s: name exceeds %d bytes or is not valid UTF-8", a.ID, MaxNameLen)
	}
	if !a.Kind.Valid() {
		return fmt.Errorf("schema: agent %s: unknown kind %q", a.ID, string(a.Kind))
	}
	if !a.Platform.Valid() {
		return fmt.Errorf("schema: agent %s: unsupported platform %q", a.ID, string(a.Platform))
	}
	if a.PID < 0 {
		return fmt.Errorf("schema: agent %s: pid must be >= 0, got %d", a.ID, a.PID)
	}
	if a.PPID < 0 {
		return fmt.Errorf("schema: agent %s: ppid must be >= 0, got %d", a.ID, a.PPID)
	}
	if len(a.Version) > MaxNameLen {
		return fmt.Errorf("schema: agent %s: version exceeds %d bytes", a.ID, MaxNameLen)
	}
	return nil
}
