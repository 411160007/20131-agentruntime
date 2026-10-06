package replay

// Policy diff lands the rule-set comparison half of slice W9.3's
// record surface: given a base policy and a candidate policy (both
// already validated - LoadCandidate is the only admission door), it
// renders which rules were added, which were removed, and which
// survived the change with edited fields, in machine-readable form.
//
// The diff is a report about two documents, nothing else. Isolation
// is unchanged from the replay package contract: no registry, no
// writer, no engine, no runtime decision plane, and the built-in rule
// set is not consulted. A blocking reading of a changed rule is
// stated with the Phase 0 observation word would_block only where the
// rule's own effect field says so; the diff itself never emits an
// effect, never merges, never applies.
//
// Determinism is the contract: rule lists are ordered by rule id and
// edited-field lists follow one fixed field order, so two runs over
// the same pair serialize to identical bytes.

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"

	"20131.com/agentruntime/internal/schema"
)

// DiffStance pins the Phase 0 reading of every diff: recorded, not
// enforced.
const DiffStance = "policy_diff: record-only, zero enforcement plane"

// RuleState classifies one rule's fate between the two documents.
// The three words are closed; declaration order is the render order
// of the section headers.
const (
	DiffAdded    = "added"
	DiffRemoved  = "removed"
	DiffModified = "modified"
)

// ruleFieldOrder is the normative comparison order for edited fields.
// It mirrors the schema.Rule wire order so a reader can align a
// changed-field list with the record layout without a second table.
var ruleFieldOrder = []string{"priority", "field", "op", "value", "effect", "severity", "hard", "caps"}

// RuleEntry is one rule rendered for the diff: the full rule as it
// exists in its document. Rendering the whole rule (not a digest)
// keeps the report self-explanatory for the promotion review that
// consumes it later.
type RuleEntry struct {
	Rule    schema.Rule `json:"rule"`
	DocSide string      `json:"doc_side"` // "base" or "candidate"
}

// ModifiedRule names the edited fields of one rule that exists on
// both sides. Before and After carry the two full documents' views
// so a reviewer never has to re-open the sources to see what changed.
type ModifiedRule struct {
	Rule    string      `json:"rule"`
	Changed []string    `json:"changed"`
	Before  schema.Rule `json:"before"`
	After   schema.Rule `json:"after"`
}

// DefaultEffectChange records a changed policy default. Absent from
// the report means unchanged; the changed pair is stated only when
// the two documents genuinely disagree.
type DefaultEffectChange struct {
	Before string `json:"before"`
	After  string `json:"after"`
}

// PolicyDiff is the deterministic rule-set diff of one base/candidate
// pair. Meta (id/name/version) is recorded on both heads for
// citation; a diff across different policy ids is admissible and is
// simply stated by the two heads being different documents.
type PolicyDiff struct {
	Base      DiffHead             `json:"base"`
	Candidate DiffHead             `json:"candidate"`
	Added     []RuleEntry          `json:"added"`
	Removed   []RuleEntry          `json:"removed"`
	Modified  []ModifiedRule       `json:"modified"`
	Default   *DefaultEffectChange `json:"default_effect_change,omitempty"`
	Identical bool                 `json:"identical"`
	Stance    string               `json:"stance"`
}

// DiffHead cites one of the two documents.
type DiffHead struct {
	ID      string `json:"id"`
	Name    string `json:"name"`
	Version int    `json:"version"`
	Rules   int    `json:"rules"`
}

// Diff computes the rule-set diff of two validated policies. It
// refuses unvalidated input on its own threshold (both documents must
// pass the grammar) so the pure function is safe to call from any
// future read-only surface without an implicit trust assumption.
func Diff(before, after *schema.Policy) (*PolicyDiff, error) {
	if before == nil || after == nil {
		return nil, fmt.Errorf("replay: diff: nil policy document")
	}
	if err := before.Validate(); err != nil {
		return nil, fmt.Errorf("replay: diff base rejected: %w", err)
	}
	if err := after.Validate(); err != nil {
		return nil, fmt.Errorf("replay: diff candidate rejected: %w", err)
	}
	d := &PolicyDiff{
		Base:      headOf(before),
		Candidate: headOf(after),
		Added:     []RuleEntry{},
		Removed:   []RuleEntry{},
		Modified:  []ModifiedRule{},
		Stance:    DiffStance,
	}
	beforeIdx := indexRules(before.Rules)
	afterIdx := indexRules(after.Rules)

	ids := make([]string, 0, len(before.Rules)+len(after.Rules))
	seen := map[string]bool{}
	for id := range beforeIdx {
		if !seen[id] {
			seen[id] = true
			ids = append(ids, id)
		}
	}
	for id := range afterIdx {
		if !seen[id] {
			seen[id] = true
			ids = append(ids, id)
		}
	}
	sort.Strings(ids)

	for _, id := range ids {
		b, inBase := beforeIdx[id]
		a, inAfter := afterIdx[id]
		switch {
		case inBase && !inAfter:
			d.Removed = append(d.Removed, RuleEntry{Rule: b, DocSide: "base"})
		case !inBase && inAfter:
			d.Added = append(d.Added, RuleEntry{Rule: a, DocSide: "candidate"})
		default:
			if changed := changedFields(b, a); len(changed) > 0 {
				d.Modified = append(d.Modified, ModifiedRule{Rule: id, Changed: changed, Before: b, After: a})
			}
		}
	}
	if string(before.DefaultEffect) != string(after.DefaultEffect) {
		d.Default = &DefaultEffectChange{Before: string(before.DefaultEffect), After: string(after.DefaultEffect)}
	}
	d.Identical = len(d.Added) == 0 && len(d.Removed) == 0 && len(d.Modified) == 0 && d.Default == nil
	return d, nil
}

func headOf(p *schema.Policy) DiffHead {
	return DiffHead{ID: p.ID, Name: p.Name, Version: p.Version, Rules: len(p.Rules)}
}

func indexRules(rules []schema.Rule) map[string]schema.Rule {
	out := make(map[string]schema.Rule, len(rules))
	for _, r := range rules {
		out[r.ID] = r
	}
	return out
}

// changedFields compares two same-id rules in the normative field
// order and returns only the disagreeing field names.
func changedFields(b, a schema.Rule) []string {
	var out []string
	if b.Priority != a.Priority {
		out = append(out, "priority")
	}
	if string(b.Field) != string(a.Field) {
		out = append(out, "field")
	}
	if string(b.Op) != string(a.Op) {
		out = append(out, "op")
	}
	if b.Value != a.Value {
		out = append(out, "value")
	}
	if string(b.Effect) != string(a.Effect) {
		out = append(out, "effect")
	}
	if b.Severity != a.Severity {
		out = append(out, "severity")
	}
	if b.Hard != a.Hard {
		out = append(out, "hard")
	}
	if strings.Join(b.Caps, "\x00") != strings.Join(a.Caps, "\x00") {
		out = append(out, "caps")
	}
	return out
}

// Bytes renders the diff deterministically (no indentation drift, no
// map iteration): two runs over the same pair produce identical
// bytes, asserted per run by the replay package tests.
func (d *PolicyDiff) Bytes() ([]byte, error) {
	if d == nil {
		return nil, fmt.Errorf("replay: diff: nil report")
	}
	raw, err := json.Marshal(d)
	if err != nil {
		return nil, fmt.Errorf("replay: diff render: %w", err)
	}
	return append(raw, '\n'), nil
}
