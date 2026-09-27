// Package identity gives observed agents STABLE identifiers and tracks
// their passport state.
//
// Two scans — or two collector runs — that see the same local executable
// must produce the same agent_id, so the audit trail links sightings
// instead of minting a fresh pseudo-id per process. The id is a pure
// hash of two local inputs:
//
//	agent_id = "agi-" + sha256(machineFingerprint | "\x00" | normalizedLocator)[0:40]
//
// The machine fingerprint is derived from files the OS maintains
// (/etc/machine-id, /var/lib/dbus/machine-id) with a hostname fallback
// for platforms without them; nothing here touches the network.
//
// Passport states follow the identity model: known -> observed ->
// trusted. Observation is automatic (a discovery hit promotes known to
// observed). Promotion to trusted NEVER happens automatically: it
// requires an explicit user trust file listing the agent id, so a
// detected binary can never earn trust by behaving well.
package identity

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"sort"
	"strings"

	"20131.com/agentruntime/internal/discovery"
)

// MachineFingerprint reads the local machine identity sources. The bool
// reports whether a dedicated machine-id file was found (false means the
// fallback chain ended at hostname, which is weaker but still local and
// stable for this host).
func MachineFingerprint() (string, bool) {
	for _, p := range []string{"/etc/machine-id", "/var/lib/dbus/machine-id"} {
		if b, err := os.ReadFile(p); err == nil {
			s := strings.TrimSpace(string(b))
			if s != "" {
				return s, true
			}
		}
	}
	h, err := os.Hostname()
	if err != nil || h == "" {
		h = "unknown-host"
	}
	return h, false
}

// NormalizeLocator canonicalizes an OS-derived locator (exe path first,
// then cmdline, then name) so two sightings of the same process shape
// hash identically across scans: separators unify to '/', repeated or
// surrounding whitespace collapses, and case folds — case-folding can
// only ever MERGE ids (two spellings of one path), never split one
// spelling across scans.
func NormalizeLocator(p discovery.ProcInfo) string {
	raw := p.Exe
	if raw == "" {
		raw = p.Cmdline
	}
	if raw == "" {
		raw = p.Name
	}
	raw = strings.ReplaceAll(raw, "\\", "/")
	raw = strings.ToLower(strings.Join(strings.Fields(raw), " "))
	return raw
}

// AgentID derives the stable id from machine fingerprint + locator.
func AgentID(machineID, locator string) string {
	h := sha256.New()
	h.Write([]byte(machineID))
	h.Write([]byte{0})
	h.Write([]byte(locator))
	return "agi-" + hex.EncodeToString(h.Sum(nil))[:40]
}

// AgentIDFor is the convenience join of NormalizeLocator + AgentID.
func AgentIDFor(machineID string, p discovery.ProcInfo) string {
	return AgentID(machineID, NormalizeLocator(p))
}

// LocatorLabel marks the fallback chain used, for audit attrs honesty.
func LocatorLabel(p discovery.ProcInfo) string {
	switch {
	case p.Exe != "":
		return "exe"
	case p.Cmdline != "":
		return "cmdline"
	default:
		return "name"
	}
}

// --- passport state machine -------------------------------------------

// State is one passport state.
type State string

const (
	StateKnown    State = "known"
	StateObserved State = "observed"
	StateTrusted  State = "trusted"
)

// Valid reports whether s is a passport state.
func (s State) Valid() bool {
	switch s {
	case StateKnown, StateObserved, StateTrusted:
		return true
	}
	return false
}

// AllStates lists passport states in promotion order.
func AllStates() []string {
	return []string{string(StateKnown), string(StateObserved), string(StateTrusted)}
}

// legalTransitions is the complete promotion/revocation graph. Anything
// absent is rejected — most importantly there is NO edge into trusted:
// trusted is reachable only through SetTrusted (explicit user config).
var legalTransitions = map[State][]State{
	StateKnown:    {StateObserved},
	StateObserved: {StateKnown},
	StateTrusted:  {StateObserved}, // revocation only; never auto-promotion
}

// CanTransition reports whether from->to is a legal machine transition.
func CanTransition(from, to State) bool {
	if !from.Valid() || !to.Valid() {
		return false
	}
	if from == to {
		return true
	}
	for _, t := range legalTransitions[from] {
		if t == to {
			return true
		}
	}
	return false
}

// Passport is one agent's identity record.
type Passport struct {
	ID      string
	State   State
	Kind    string
	Hits    int
	FirstTS string
	LastTS  string
}

// Registry tracks passports for a collector run.
type Registry struct {
	byID map[string]*Passport
}

// NewRegistry builds an empty registry.
func NewRegistry() *Registry { return &Registry{byID: map[string]*Passport{}} }

// Observe registers a sighting: first contact lands at observed (the
// automatic promotion), trusted rows are NEVER demoted by observation.
func (r *Registry) Observe(id, kind string) State {
	p, ok := r.byID[id]
	if !ok {
		r.byID[id] = &Passport{ID: id, State: StateObserved, Kind: kind}
		return StateObserved
	}
	if p.State == StateKnown {
		p.State = StateObserved
	}
	p.Kind = kind
	p.Hits++
	return p.State
}

// StateOf returns the current state (unknown ids read as known).
func (r *Registry) StateOf(id string) State {
	if p, ok := r.byID[id]; ok {
		return p.State
	}
	return StateKnown
}

// SetTrusted applies the explicit user decision; it is the ONLY path
// into trusted and it deliberately bypasses the observation transition
// graph (which has no edge into trusted). Requirements: the agent must
// have been sighted at least once (registry row exists) and must have
// reached observed — an agent stuck at known cannot be trusted.
func (r *Registry) SetTrusted(id string) error {
	p, ok := r.byID[id]
	if !ok {
		return fmt.Errorf("identity: agent %s not observed; trust requires a prior sighting", id)
	}
	if p.State == StateKnown {
		return fmt.Errorf("identity: agent %s is known but never observed; refusing trust", id)
	}
	p.State = StateTrusted
	return nil
}

// Revoke demotes trusted -> observed (explicit user action).
func (r *Registry) Revoke(id string) error {
	p, ok := r.byID[id]
	if !ok {
		return fmt.Errorf("identity: agent %s unknown", id)
	}
	if !CanTransition(p.State, StateObserved) {
		return fmt.Errorf("identity: illegal transition %s -> observed for %s", p.State, id)
	}
	p.State = StateObserved
	return nil
}

// List returns passports sorted by id (deterministic status output).
func (r *Registry) List() []Passport {
	out := make([]Passport, 0, len(r.byID))
	for _, p := range r.byID {
		out = append(out, *p)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ID < out[j].ID })
	return out
}

// --- user trust file ---------------------------------------------------

// LoadTrustFile reads the user trust list: one agent id per line,
// '#' comments and blanks ignored. It only READS user configuration —
// the collector never writes it, so trust cannot be self-granted.
func LoadTrustFile(path string) ([]string, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var ids []string
	for _, ln := range strings.Split(string(b), "\n") {
		ln = strings.TrimSpace(strings.SplitN(ln, "#", 2)[0])
		if ln != "" {
			ids = append(ids, ln)
		}
	}
	return ids, nil
}
