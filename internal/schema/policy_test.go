package schema

import (
	"fmt"
	"strings"
	"testing"
)

func validPolicy() *Policy {
	return &Policy{
		ID:            "policy-01",
		Name:          "baseline",
		Version:       1,
		DefaultEffect: EffectAllow,
		Rules: []Rule{
			{ID: "r1", Priority: 10, Field: FieldPath, Op: OpPrefix, Value: "/etc/", Effect: EffectWouldBlock, Severity: SevHigh},
			{ID: "r2", Priority: 5, Field: FieldType, Op: OpEquals, Value: "tool.call", Effect: EffectAsk, Severity: SevMedium},
		},
	}
}

func TestPolicyValid(t *testing.T) {
	if err := validPolicy().Validate(); err != nil {
		t.Fatalf("expected valid, got %v", err)
	}
	p := validPolicy()
	p.Rules = nil // zero rules is legal: default-only policy
	if err := p.Validate(); err != nil {
		t.Fatalf("default-only policy must be valid, got %v", err)
	}
}

func TestPolicyInvalid(t *testing.T) {
	cases := []struct {
		name string
		mut  func(*Policy)
	}{
		{"empty id", func(p *Policy) { p.ID = "" }},
		{"empty name", func(p *Policy) { p.Name = "" }},
		{"zero version", func(p *Policy) { p.Version = 0 }},
		{"negative version", func(p *Policy) { p.Version = -1 }},
		{"bad default effect", func(p *Policy) { p.DefaultEffect = Effect("nuke") }},
		{"duplicate rule id", func(p *Policy) {
			p.Rules = append(p.Rules, Rule{ID: "r1", Priority: 1, Field: FieldType, Op: OpEquals, Value: "x", Effect: EffectAllow, Severity: SevInfo})
		}},
		{"rule bad field", func(p *Policy) { p.Rules[0].Field = MatchField("vibes") }},
		{"rule bad op", func(p *Policy) { p.Rules[0].Op = MatchOp("regex") }},
		{"rule empty value", func(p *Policy) { p.Rules[0].Value = "" }},
		{"rule value too long", func(p *Policy) { p.Rules[0].Value = strings.Repeat("a", 513) }},
		{"rule bad effect", func(p *Policy) { p.Rules[0].Effect = Effect("yolo") }},
		{"rule bad severity", func(p *Policy) { p.Rules[0].Severity = Severity(9) }},
		{"too many rules", func(p *Policy) {
			rules := make([]Rule, MaxRules+1)
			for i := range rules {
				rules[i] = Rule{ID: fmt.Sprintf("rr-%d", i), Priority: i, Field: FieldType, Op: OpEquals, Value: "t", Effect: EffectAllow, Severity: SevInfo}
			}
			p.Rules = rules
		}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			p := validPolicy()
			tc.mut(p)
			if err := p.Validate(); err == nil {
				t.Fatalf("expected error for %s", tc.name)
			}
		})
	}
}

func TestRuleBoundary(t *testing.T) {
	p := validPolicy()
	p.Rules = make([]Rule, MaxRules)
	for i := range p.Rules {
		p.Rules[i] = Rule{ID: fmt.Sprintf("rr-%d", i), Priority: i, Field: FieldType, Op: OpEquals, Value: "t", Effect: EffectAllow, Severity: SevInfo}
	}
	if err := p.Validate(); err != nil {
		t.Fatalf("exactly MaxRules must pass, got %v", err)
	}
}
