package schema

import "fmt"

// Effect is the action a policy rule prescribes when it matches.
type Effect string

const (
	EffectAllow      Effect = "allow"
	EffectAsk        Effect = "ask"
	EffectWouldBlock Effect = "would_block"
)

func (e Effect) Valid() bool {
	switch e {
	case EffectAllow, EffectAsk, EffectWouldBlock:
		return true
	}
	return false
}

// MatchField selects which Event field a rule matches against.
type MatchField string

const (
	FieldAgentID MatchField = "agent_id"
	FieldType    MatchField = "type"
	FieldTool    MatchField = "tool"   // matched against Attrs["tool"]
	FieldPath    MatchField = "path"   // matched against Attrs["path"]
	FieldDomain  MatchField = "domain" // matched against Attrs["domain"]
)

func (m MatchField) Valid() bool {
	switch m {
	case FieldAgentID, FieldType, FieldTool, FieldPath, FieldDomain:
		return true
	}
	return false
}

// MatchOp is the comparison operator applied to a rule value.
type MatchOp string

const (
	OpEquals MatchOp = "equals"
	OpPrefix MatchOp = "prefix"
	OpSuffix MatchOp = "suffix"
)

func (o MatchOp) Valid() bool {
	switch o {
	case OpEquals, OpPrefix, OpSuffix:
		return true
	}
	return false
}

// MaxRules bounds one policy document.
const MaxRules = 256

// Rule is one deterministic matcher inside a Policy.
// A rule matches when the selected event field satisfies Op(Value).
// Rules are evaluated in priority order (higher first), ties broken by
// declaration index; the first match wins.
type Rule struct {
	ID       string     `json:"id"`
	Priority int        `json:"priority"`
	Field    MatchField `json:"field"`
	Op       MatchOp    `json:"op"`
	Value    string     `json:"value"`
	Effect   Effect     `json:"effect"`
	Severity Severity   `json:"severity"`
}

// Validate checks the rule grammar.
func (r *Rule) Validate() error {
	if !validID(r.ID) {
		return fmt.Errorf("schema: rule id %q invalid", r.ID)
	}
	if !r.Field.Valid() {
		return fmt.Errorf("schema: rule %s: unknown match field %q", r.ID, string(r.Field))
	}
	if !r.Op.Valid() {
		return fmt.Errorf("schema: rule %s: unknown operator %q", r.ID, string(r.Op))
	}
	if r.Value == "" || len(r.Value) > 512 {
		return fmt.Errorf("schema: rule %s: value empty or >512 bytes", r.ID)
	}
	if !r.Effect.Valid() {
		return fmt.Errorf("schema: rule %s: unknown effect %q", r.ID, string(r.Effect))
	}
	if !r.Severity.Valid() {
		return fmt.Errorf("schema: rule %s: severity %d out of range", r.ID, int(r.Severity))
	}
	return nil
}

// Policy is a versioned set of rules evaluated against Events.
// DefaultEffect applies when no rule matches. Phase 0 never blocks: the
// runtime records the computed effect; enforcement arrives in later phases.
type Policy struct {
	ID            string `json:"id"`
	Name          string `json:"name"`
	Version       int    `json:"version"`
	DefaultEffect Effect `json:"default_effect"`
	Rules         []Rule `json:"rules"`
}

// Validate checks the policy document grammar.
func (p *Policy) Validate() error {
	if !validID(p.ID) {
		return fmt.Errorf("schema: policy id %q invalid", p.ID)
	}
	if p.Name == "" || len(p.Name) > MaxNameLen {
		return fmt.Errorf("schema: policy %s: name empty or too long", p.ID)
	}
	if p.Version < 1 {
		return fmt.Errorf("schema: policy %s: version must be >= 1, got %d", p.ID, p.Version)
	}
	if !p.DefaultEffect.Valid() {
		return fmt.Errorf("schema: policy %s: unknown default effect %q", p.ID, string(p.DefaultEffect))
	}
	if len(p.Rules) > MaxRules {
		return fmt.Errorf("schema: policy %s: %d rules exceeds cap %d", p.ID, len(p.Rules), MaxRules)
	}
	seen := make(map[string]bool, len(p.Rules))
	for i := range p.Rules {
		if err := p.Rules[i].Validate(); err != nil {
			return err
		}
		if seen[p.Rules[i].ID] {
			return fmt.Errorf("schema: policy %s: duplicate rule id %q", p.ID, p.Rules[i].ID)
		}
		seen[p.Rules[i].ID] = true
	}
	return nil
}
