// Package policy implements deterministic evaluation of Events against
// Policy documents, producing the "Policy Evaluated" record of the
// pipeline: Agent Proposed -> Policy Evaluated -> Native Enforcement -> Action.
package policy

import (
	"fmt"
	"sort"
	"strings"

	"20131.com/agentruntime/internal/schema"
)

// Outcome is the result of evaluating one event against one policy.
type Outcome struct {
	Effect      schema.Effect   `json:"effect"`
	Severity    schema.Severity `json:"severity"`
	MatchedRule string          `json:"matched_rule,omitempty"` // empty = default applied
	Reason      string          `json:"reason"`
}

// Evaluator holds a validated policy and evaluates events against it.
type Evaluator struct {
	p       *schema.Policy
	ordered []schema.Rule // pre-sorted: priority desc, declaration index asc
}

// New validates p and returns an Evaluator frozen against that snapshot.
func New(p *schema.Policy) (*Evaluator, error) {
	if p == nil {
		return nil, fmt.Errorf("policy: nil policy")
	}
	if err := p.Validate(); err != nil {
		return nil, err
	}
	cp := make([]schema.Rule, len(p.Rules))
	copy(cp, p.Rules)
	sort.SliceStable(cp, func(i, j int) bool { return cp[i].Priority > cp[j].Priority })
	return &Evaluator{p: p, ordered: cp}, nil
}

// Evaluate returns the first matching rule's effect, or the default.
// It never mutates the event or policy.
func (ev *Evaluator) Evaluate(e *schema.Event) (Outcome, error) {
	if e == nil {
		return Outcome{}, fmt.Errorf("policy: nil event")
	}
	if err := e.Validate(); err != nil {
		return Outcome{}, err
	}
	for i := range ev.ordered {
		r := &ev.ordered[i]
		val, ok := fieldValue(e, r.Field)
		if !ok {
			continue
		}
		if match(r.Op, val, r.Value) {
			return Outcome{
				Effect:      r.Effect,
				Severity:    r.Severity,
				MatchedRule: r.ID,
				Reason:      fmt.Sprintf("rule %s matched %s %s %q", r.ID, string(r.Field), string(r.Op), r.Value),
			}, nil
		}
	}
	return Outcome{
		Effect:   ev.p.DefaultEffect,
		Severity: schema.SevInfo,
		Reason:   "no rule matched; default effect applied",
	}, nil
}

// fieldValue extracts the matchable value for a field from an event.
func fieldValue(e *schema.Event, f schema.MatchField) (string, bool) {
	switch f {
	case schema.FieldAgentID:
		return e.AgentID, true
	case schema.FieldType:
		return string(e.Type), true
	case schema.FieldTool, schema.FieldPath, schema.FieldDomain:
		if e.Attrs == nil {
			return "", false
		}
		v, ok := e.Attrs[string(f)]
		return v, ok
	}
	return "", false
}

// match applies one operator. All comparisons are byte-exact, case-sensitive
// (paths and domains are normalized by producers before they reach here).
func match(op schema.MatchOp, got, want string) bool {
	switch op {
	case schema.OpEquals:
		return got == want
	case schema.OpPrefix:
		return strings.HasPrefix(got, want)
	case schema.OpSuffix:
		return strings.HasSuffix(got, want)
	}
	return false
}
