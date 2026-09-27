// Capability vocabulary v0: the closed set of capability tokens an event
// may reference through attrs["cap"], plus the three resource
// sensitivity classes (spec vNext section 19) in attrs["res_class"].
//
// Phase 0 boundary: this package EXPRESSES capabilities and their
// sensitivity; it does not enforce grants. Per spec vNext section 23,
// grants are NON-TRANSITIVE: a subprocess spawned via proc.spawn or
// shell.exec does NOT inherit its parent's grants. Every table entry
// carries that annotation, and the unit tests pin it so a future edit
// cannot silently mark a capability transitive.
package schema

import (
	"fmt"
	"strings"
)

// Capability names one supervised kind of agent-side effect.
type Capability string

// MaxCapabilities caps the built-in capability table so the vocabulary
// stays auditable and per-rule lookups stay small.
const MaxCapabilities = 64

// Resource class levels (spec vNext section 19): three levels, no more.
const (
	ResLow    = "low"
	ResMedium = "medium"
	ResHigh   = "high"
)

// ResClass is the resource sensitivity level of the target of a
// capability use. Absent (empty) means unclassified and is valid;
// anything outside {low, medium, high} is rejected.
type ResClass string

func (r ResClass) Valid() bool {
	switch r {
	case "", ResLow, ResMedium, ResHigh:
		return true
	}
	return false
}

// AllResourceClasses lists the three sensitivity levels in order.
func AllResourceClasses() []string { return []string{ResLow, ResMedium, ResHigh} }

// CapDef is one vocabulary entry: the token, its default resource
// sensitivity, and the section 23 non-transitivity annotation.
type CapDef struct {
	Name          Capability
	DefaultClass  ResClass
	NonTransitive bool
}

// capabilityTable is the built-in vocabulary. Order is fixed; the sync
// tests, the Node validator, and docs/api-v0.md mirror it exactly.
var capabilityTable = []CapDef{
	{Cap("file.read"), ResMedium, true},
	{Cap("file.write"), ResHigh, true},
	{Cap("file.delete"), ResHigh, true},
	{Cap("net.outbound"), ResHigh, true},
	{Cap("net.listen"), ResMedium, true},
	{Cap("proc.spawn"), ResMedium, true},
	{Cap("shell.exec"), ResHigh, true},
	{Cap("mcp.tool"), ResMedium, true},
	{Cap("credential.access"), ResHigh, true},
	{Cap("env.read"), ResLow, true},
	{Cap("process.inspect"), ResLow, true},
}

// Cap is the constructor for vocabulary keys (keeps the table readable).
func Cap(s string) Capability { return Capability(s) }

// Valid reports whether c is in the built-in vocabulary.
func (c Capability) Valid() bool {
	for i := range capabilityTable {
		if capabilityTable[i].Name == c {
			return true
		}
	}
	return false
}

// ClassOf returns the default sensitivity class for a capability.
func (c Capability) ClassOf() (ResClass, bool) {
	for i := range capabilityTable {
		if capabilityTable[i].Name == c {
			return capabilityTable[i].DefaultClass, true
		}
	}
	return "", false
}

// NonTransitive reports the section 23 grant semantics: a child process
// launched under a capability does NOT inherit any grant held by its
// parent. v0's answer is uniformly true; the lookup exists so callers
// express the semantics explicitly instead of assuming.
func (c Capability) NonTransitive() bool {
	for i := range capabilityTable {
		if capabilityTable[i].Name == c {
			return capabilityTable[i].NonTransitive
		}
	}
	return false
}

// CapabilityTable returns a copy of the built-in vocabulary.
func CapabilityTable() []CapDef {
	out := make([]CapDef, len(capabilityTable))
	copy(out, capabilityTable)
	return out
}

// AllCapabilities lists every vocabulary token in declaration order.
func AllCapabilities() []string {
	out := make([]string, 0, len(capabilityTable))
	for i := range capabilityTable {
		out = append(out, string(capabilityTable[i].Name))
	}
	return out
}

// ParseCapabilityList splits a comma-separated grant list, enforcing the
// table cap and rejecting wild values, duplicates, and empty items.
func ParseCapabilityList(s string) ([]Capability, error) {
	var out []Capability
	for _, part := range strings.Split(s, ",") {
		p := strings.TrimSpace(part)
		if p == "" {
			continue
		}
		c := Capability(p)
		if !c.Valid() {
			return nil, fmt.Errorf("schema: unknown capability %q", p)
		}
		for _, prev := range out {
			if prev == c {
				return nil, fmt.Errorf("schema: duplicate capability %q", p)
			}
		}
		out = append(out, c)
		if len(out) > MaxCapabilities {
			return nil, fmt.Errorf("schema: capability list exceeds cap %d", MaxCapabilities)
		}
	}
	return out, nil
}
