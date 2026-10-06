package replay

import (
	"strings"
	"testing"

	"20131.com/agentruntime/internal/schema"
)

func diffRule(id string, prio int, sev schema.Severity, eff schema.Effect) schema.Rule {
	return schema.Rule{
		ID:       id,
		Priority: prio,
		Field:    schema.FieldAgentID,
		Op:       schema.OpEquals,
		Value:    "alpha",
		Effect:   eff,
		Severity: sev,
	}
}

func diffBase() *schema.Policy {
	return &schema.Policy{
		ID:            "gw-base",
		Name:          "GW Base",
		Version:       1,
		DefaultEffect: schema.EffectAllow,
		Rules: []schema.Rule{
			diffRule("rule-a", 10, schema.SevInfo, schema.EffectAllow),
			diffRule("rule-b", 20, schema.SevHigh, schema.EffectWouldBlock),
			diffRule("rule-c", 30, schema.SevCritical, schema.EffectWouldBlock),
		},
	}
}

func diffCand(mutate func(*schema.Policy)) *schema.Policy {
	p := diffBase()
	p.Version = 2
	mutate(p)
	return p
}

// TestDiffGoldenShape pins the full wire shape of one canonical
// diff: a single severity edit on an otherwise identical pair. If
// any key, order, or absent-vs-empty policy drifts, this byte pin
// fails - that is the diff-form golden of the slice.
func TestDiffGoldenShape(t *testing.T) {
	cand := diffCand(func(p *schema.Policy) {
		r := p.Rules[0]
		r.Severity = schema.SevLow
		p.Rules[0] = r
	})
	d, err := Diff(diffBase(), cand)
	if err != nil {
		t.Fatalf("diff: %v", err)
	}
	raw, err := d.Bytes()
	if err != nil {
		t.Fatalf("render: %v", err)
	}
	const golden = `{"base":{"id":"gw-base","name":"GW Base","version":1,"rules":3},"candidate":{"id":"gw-base","name":"GW Base","version":2,"rules":3},"added":[],"removed":[],"modified":[{"rule":"rule-a","changed":["severity"],"before":{"id":"rule-a","priority":10,"field":"agent_id","op":"equals","value":"alpha","effect":"allow","severity":0},"after":{"id":"rule-a","priority":10,"field":"agent_id","op":"equals","value":"alpha","effect":"allow","severity":1}}],"identical":false,"stance":"policy_diff: record-only, zero enforcement plane"}` + "\n"
	if string(raw) != golden {
		t.Fatalf("diff golden drift:\n got %s\nwant %s", raw, golden)
	}
}

func TestDiffAddedRemovedModifiedDefault(t *testing.T) {
	cand := diffCand(func(p *schema.Policy) {
		p.DefaultEffect = schema.EffectWouldBlock
		p.Rules = []schema.Rule{
			diffRule("rule-a", 10, schema.SevInfo, schema.EffectAllow), // unchanged
			(func() schema.Rule {
				r := diffRule("rule-b", 21, schema.SevMedium, schema.EffectWouldBlock)
				r.Priority = 21
				return r
			})(), // edited
			diffRule("rule-d", 40, schema.SevInfo, schema.EffectAllow), // added (sorts last)
		}
	})
	d, err := Diff(diffBase(), cand)
	if err != nil {
		t.Fatalf("diff: %v", err)
	}
	if len(d.Added) != 1 || d.Added[0].Rule.ID != "rule-d" || d.Added[0].DocSide != "candidate" {
		t.Fatalf("added: %+v", d.Added)
	}
	if len(d.Removed) != 1 || d.Removed[0].Rule.ID != "rule-c" || d.Removed[0].DocSide != "base" {
		t.Fatalf("removed: %+v", d.Removed)
	}
	if len(d.Modified) != 1 || d.Modified[0].Rule != "rule-b" {
		t.Fatalf("modified: %+v", d.Modified)
	}
	// Edited fields follow the normative order even when the input
	// edit order differs (priority then severity).
	want := []string{"priority", "severity"}
	if strings.Join(d.Modified[0].Changed, ",") != strings.Join(want, ",") {
		t.Fatalf("changed order: %v want %v", d.Modified[0].Changed, want)
	}
	if d.Default == nil || d.Default.Before != string(schema.EffectAllow) || d.Default.After != string(schema.EffectWouldBlock) {
		t.Fatalf("default change: %+v", d.Default)
	}
	if d.Identical {
		t.Fatal("identical must be false for a changed pair")
	}
}

func TestDiffIdenticalAndDeterministic(t *testing.T) {
	base := diffBase()
	d1, err := Diff(base, diffBase())
	if err != nil {
		t.Fatalf("diff: %v", err)
	}
	if !d1.Identical || d1.Default != nil || len(d1.Added)+len(d1.Removed)+len(d1.Modified) != 0 {
		t.Fatalf("expected empty diff: %+v", d1)
	}
	b1, err := d1.Bytes()
	if err != nil {
		t.Fatalf("render: %v", err)
	}
	d2, err := Diff(diffBase(), base)
	if err != nil {
		t.Fatalf("second diff: %v", err)
	}
	b2, err := d2.Bytes()
	if err != nil {
		t.Fatalf("second render: %v", err)
	}
	if string(b1) != string(b2) {
		t.Fatalf("byte determinism broken:\n %s\n %s", b1, b2)
	}
	// Empty lists render as [], never null: absence of change is
	// stated, not left ambiguous.
	if !strings.Contains(string(b1), `"added":[]`) || !strings.Contains(string(b1), `"modified":[]`) {
		t.Fatalf("empty lists must render []: %s", b1)
	}
}

func TestDiffRejectsInvalidInput(t *testing.T) {
	bad := diffBase()
	bad.Version = 0 // outside the policy grammar
	if d, err := Diff(bad, diffBase()); err == nil || d != nil {
		t.Fatalf("unvalidated base must be rejected with no half diff: %v %v", d, err)
	}
	if d, err := Diff(nil, diffBase()); err == nil || d != nil {
		t.Fatalf("nil document must be rejected: %v %v", d, err)
	}
	candBad := diffCand(func(p *schema.Policy) {
		p.Rules = append(p.Rules, schema.Rule{ID: "rule-z", Priority: 1, Field: "nope", Op: schema.OpEquals, Value: "x", Effect: schema.EffectAllow, Severity: schema.SevInfo})
	})
	if d, err := Diff(diffBase(), candBad); err == nil || d != nil {
		t.Fatalf("unvalidated candidate must be rejected: %v %v", d, err)
	}
}

func TestDiffZeroEnforcementVocabulary(t *testing.T) {
	cand := diffCand(func(p *schema.Policy) {
		p.Rules = append(p.Rules, schema.Rule{ID: "rule-hard", Priority: 5, Field: schema.FieldType, Op: schema.OpPrefix, Value: "rm", Effect: schema.EffectWouldBlock, Severity: schema.SevCritical, Hard: true})
	})
	d, err := Diff(diffBase(), cand)
	if err != nil {
		t.Fatalf("diff: %v", err)
	}
	raw, err := d.Bytes()
	if err != nil {
		t.Fatalf("render: %v", err)
	}
	// Reserved enforcement words never appear: the diff echoes rule
	// effects as data but introduces no decision vocabulary of its
	// own.
	for _, banned := range []string{`"deny"`, `"enforce"`, `"quarantine"`, `"rollback"`, `"promotion"`, `"applied"`} {
		if strings.Contains(string(raw), banned) {
			t.Fatalf("reserved vocabulary present in diff report: %s", banned)
		}
	}
	if !strings.Contains(string(raw), DiffStance) {
		t.Fatal("stance line must be present verbatim")
	}
}
