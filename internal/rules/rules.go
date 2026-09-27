// Package rules is the Phase 0 security judgement layer: the built-in
// rule set (a frozen JSON policy document carried inside the binary),
// the user-override merge surface, and the decision emitter that turns
// an observed event into a policy.decision audit line.
//
// Phase 0 boundary (observation without enforcement): the engine only
// COMPUTES decisions. A matched rule records decision=would_block plus
// severity and, for non-downgradable rules, the hard annotation on the
// emitted audit line; nothing in this package can prevent, pause, or
// alter the behaviour of any process, file operation, or network path.
// The decision vocabulary at emission time is pinned to
// {allow, would_block} through the shared schema guard, so an
// enforcement value cannot enter the audit stream even by accident.
//
// Fast Path (continuously proven): evaluation is pure in-process rule
// matching — zero LLM calls, zero network, bounded tables. The unit
// suite replays decisions in a loop and fails if average cost drifts
// above the per-event budget, and the gate asserts the import closure
// of this package contains no net/* or crypto/tls package.
package rules

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"sort"
	"sync"

	"20131.com/agentruntime/internal/policy"
	"20131.com/agentruntime/internal/schema"
)

// builtinPolicyJSON is the built-in 12-rule Phase 0 document. It is
// embedded verbatim (stdlib-only, no external file dependency): the
// binary carries its own default policy. Priorities: hard rules share
// 100, ties break by declaration order below, which keeps multi-match
// events deterministic. The id ↔ threat mapping lives in
// docs/threat-model.md and is cross-checked by the gate in both
// directions.
const builtinPolicyJSON = `{
  "id": "builtin-phase0",
  "name": "20131 built-in Phase 0 rule set",
  "version": 1,
  "default_effect": "allow",
  "rules": [
    {"id": "cred.ssh",       "priority": 100, "field": "path",    "op": "contains", "value": "/.ssh/",       "effect": "would_block", "severity": 4, "hard": true, "caps": ["credential.access", "file.read"]},
    {"id": "cred.aws",       "priority": 100, "field": "path",    "op": "contains", "value": "/.aws/",       "effect": "would_block", "severity": 4, "hard": true, "caps": ["credential.access", "file.read"]},
    {"id": "cred.browser",   "priority": 100, "field": "path",    "op": "contains", "value": "Login Data",   "effect": "would_block", "severity": 4, "hard": true, "caps": ["credential.access", "file.read"]},
    {"id": "destroy.rmrf",   "priority": 100, "field": "cmdline", "op": "contains", "value": "rm -rf",       "effect": "would_block", "severity": 4, "hard": true, "caps": ["shell.exec", "file.delete"]},
    {"id": "destroy.disk",   "priority": 100, "field": "cmdline", "op": "contains", "value": "mkfs.",        "effect": "would_block", "severity": 4, "hard": true, "caps": ["shell.exec", "file.delete"]},
    {"id": "exec.remotepipe","priority": 100, "field": "cmdline", "op": "contains", "value": "| sh",         "effect": "would_block", "severity": 4, "hard": true, "caps": ["shell.exec", "net.outbound", "proc.spawn"]},
    {"id": "audit.tamper",   "priority": 100, "field": "path",    "op": "contains", "value": "agent-audit",  "effect": "would_block", "severity": 4, "hard": true, "caps": ["file.write", "file.delete"]},
    {"id": "path.sudoers",   "priority": 100, "field": "path",    "op": "contains", "value": "etc/sudoers",  "effect": "would_block", "severity": 4, "hard": true, "caps": ["file.write"]},
    {"id": "cred.dotenv",    "priority": 80,  "field": "path",    "op": "suffix",   "value": ".env",         "effect": "would_block", "severity": 3, "caps": ["credential.access", "file.read"]},
    {"id": "mcp.eval",       "priority": 70,  "field": "tool",    "op": "contains", "value": "eval",         "effect": "would_block", "severity": 3, "caps": ["mcp.tool"]},
    {"id": "agent.masquerade","priority": 50, "field": "exe",     "op": "contains", "value": "/tmp/",        "effect": "would_block", "severity": 2, "caps": ["process.inspect"]},
    {"id": "net.egress",     "priority": 40,  "field": "type",    "op": "equals",   "value": "network.intent","effect": "would_block", "severity": 2, "caps": ["net.outbound"]}
  ]
}`

// BuiltinRuleCount pins the built-in rule count for gate assertions.
const BuiltinRuleCount = 12

var (
	builtinOnce  sync.Once
	builtinCache *schema.Policy
	builtinErr   error
)

// Builtin returns the parsed, validated built-in policy (cached; the
// parse happens once).
func Builtin() (*schema.Policy, error) {
	builtinOnce.Do(func() {
		var p schema.Policy
		if err := json.Unmarshal([]byte(builtinPolicyJSON), &p); err != nil {
			builtinErr = fmt.Errorf("rules: built-in policy unmarshal: %w", err)
			return
		}
		if err := p.Validate(); err != nil {
			builtinErr = fmt.Errorf("rules: built-in policy invalid: %w", err)
			return
		}
		if len(p.Rules) != BuiltinRuleCount {
			builtinErr = fmt.Errorf("rules: built-in policy carries %d rules, want %d", len(p.Rules), BuiltinRuleCount)
			return
		}
		builtinCache = &p
	})
	return builtinCache, builtinErr
}

// Decision is one computed judgement for one event.
type Decision struct {
	Value    schema.Decision `json:"decision"`
	Severity schema.Severity `json:"severity"`
	RuleID   string          `json:"rule,omitempty"` // empty = default applied
	Hard     bool            `json:"hard,omitempty"`
	Reason   string          `json:"reason"`
}

// Engine evaluates events against one frozen policy snapshot.
type Engine struct {
	ev   *policy.Evaluator
	byID map[string]*schema.Rule
}

// New builds an Engine from a validated policy.
func New(p *schema.Policy) (*Engine, error) {
	ev, err := policy.New(p)
	if err != nil {
		return nil, err
	}
	byID := make(map[string]*schema.Rule, len(p.Rules))
	for i := range p.Rules {
		byID[p.Rules[i].ID] = &p.Rules[i]
	}
	return &Engine{ev: ev, byID: byID}, nil
}

// MustDefault returns the built-in engine or panics; the built-in
// document is a compile-time constant, so any error is a programming
// bug the unit tests catch before release.
func MustDefault() *Engine {
	p, err := Builtin()
	if err != nil {
		panic(err)
	}
	e, err := New(p)
	if err != nil {
		panic(err)
	}
	return e
}

// Decide computes the judgement for one event without mutating it.
func (e *Engine) Decide(ev *schema.Event) (Decision, error) {
	out, err := e.ev.Evaluate(ev)
	if err != nil {
		return Decision{}, err
	}
	val, err := schema.DecisionFor(out.Effect)
	if err != nil {
		return Decision{}, err
	}
	// Phase 0 emission guard: even a valid contract effect (ask is
	// reserved vocabulary) must not reach a decision line yet.
	if err := schema.MustPhase0Decision(val); err != nil {
		return Decision{}, err
	}
	d := Decision{Value: val, Severity: out.Severity, Reason: out.Reason}
	if r, ok := e.byID[out.MatchedRule]; ok {
		d.RuleID = r.ID
		d.Hard = r.Hard
		d.Severity = r.Severity
	}
	return d, nil
}

// DecisionEvent renders a Decision as an auditable policy.decision
// line (stage=evaluated) attached to the source event. The source is
// never mutated. The line carries the rule id, the hard annotation,
// and — when the matched rule guards capabilities — the first of its
// capability tokens plus that capability's default resource class, so
// downstream tooling can rank findings without the policy document.
//
// Deterministic: same source event in, same decision id out, so
// replays do not fabricate "new" findings.
func DecisionEvent(src *schema.Event, d Decision) (*schema.Event, error) {
	if src == nil {
		return nil, fmt.Errorf("rules: nil source event")
	}
	if err := schema.MustPhase0Decision(d.Value); err != nil {
		return nil, err
	}
	sum := d.Reason
	attrs := map[string]string{"hard": fmt.Sprintf("%t", d.Hard)}
	if d.RuleID != "" {
		attrs["rule"] = d.RuleID
		if r, ok := ruleByID(d.RuleID); ok && len(r.Caps) > 0 {
			cap0 := r.Caps[0]
			attrs["cap"] = cap0
			if c, ok := schema.Cap(cap0).ClassOf(); ok {
				attrs["res_class"] = string(c)
			}
		}
	}
	if len(sum) > 512 {
		sum = sum[:509] + "..."
	}
	h := sha256.Sum256([]byte(src.ID + "|" + d.RuleID + "|" + string(d.Value)))
	return &schema.Event{
		V:        schema.SchemaVersion,
		TS:       src.TS,
		ID:       "dec-" + hex.EncodeToString(h[:8]),
		AgentID:  src.AgentID,
		Stage:    schema.StageEvaluated,
		Type:     schema.TypePolicyDecision,
		Decision: d.Value,
		Severity: d.Severity,
		Summary:  sum,
		Tier:     schema.TierL1, // decision lines persist; they are never re-fed to the engine
		Attrs:    attrs,
	}, nil
}

// ruleByID looks up a built-in rule for annotation rendering.
func ruleByID(id string) (*schema.Rule, bool) {
	p, err := Builtin()
	if err != nil {
		return nil, false
	}
	for i := range p.Rules {
		if p.Rules[i].ID == id {
			return &p.Rules[i], true
		}
	}
	return nil, false
}

// RuleIDs returns the built-in rule ids in declaration order (used by
// the gate cross-check against the threat model).
func RuleIDs() []string {
	p, err := Builtin()
	if err != nil {
		return nil
	}
	out := make([]string, 0, len(p.Rules))
	for i := range p.Rules {
		out = append(out, p.Rules[i].ID)
	}
	return out
}

// HardRuleIDs returns built-in ids carrying the non-downgradable mark.
func HardRuleIDs() []string {
	p, err := Builtin()
	if err != nil {
		return nil
	}
	var out []string
	for i := range p.Rules {
		if p.Rules[i].Hard {
			out = append(out, p.Rules[i].ID)
		}
	}
	sort.Strings(out)
	return out
}

// CapTokensUsed returns the sorted set of capability tokens referenced
// by the built-in rule set (table-vs-rules cross-check surface).
func CapTokensUsed() []string {
	p, err := Builtin()
	if err != nil {
		return nil
	}
	set := map[string]bool{}
	for i := range p.Rules {
		for _, c := range p.Rules[i].Caps {
			set[c] = true
		}
	}
	out := make([]string, 0, len(set))
	for c := range set {
		out = append(out, c)
	}
	sort.Strings(out)
	return out
}
