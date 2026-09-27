package rules

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"20131.com/agentruntime/internal/schema"
)

func TestMergeAdditiveAndWhitelist(t *testing.T) {
	user := &schema.Policy{
		ID: "user-x", Name: "user overrides", Version: 2,
		DefaultEffect: schema.EffectAllow,
		Rules: []schema.Rule{
			{ID: "allow.github", Priority: 60, Field: schema.FieldDomain, Op: schema.OpEquals, Value: "api.github.com", Effect: schema.EffectAllow, Severity: schema.SevInfo},
			{ID: "local.devpipe", Priority: 45, Field: schema.FieldCmdline, Op: schema.OpContains, Value: "npm run dev", Effect: schema.EffectWouldBlock, Severity: schema.SevMedium},
		},
	}
	merged, err := Merge(user)
	if err != nil {
		t.Fatal(err)
	}
	if len(merged.Rules) != BuiltinRuleCount+2 {
		t.Fatalf("merged rule count %d", len(merged.Rules))
	}
	eng, err := New(merged)
	if err != nil {
		t.Fatal(err)
	}
	net := ev(t, "w1", schema.TypeNetworkIntent, map[string]string{"domain": "api.github.com"})
	d, err := eng.Decide(net)
	if err != nil || d.Value != schema.DecisionAllow {
		t.Fatalf("domain whitelist did not pre-empt net.egress: %+v err=%v", d, err)
	}
	other := ev(t, "w2", schema.TypeNetworkIntent, map[string]string{"domain": "telemetry.otherhost.dev"})
	if d2, _ := eng.Decide(other); d2.Value != schema.DecisionWouldBlock || d2.RuleID != "net.egress" {
		t.Errorf("non-whitelisted egress not flagged: %+v", d2)
	}
}

func TestMergeRefusesDowngradeAndShadow(t *testing.T) {
	// change a built-in's semantics: refused
	changed := &schema.Policy{ID: "u", Name: "n", Version: 1, DefaultEffect: schema.EffectAllow,
		Rules: []schema.Rule{{ID: "cred.ssh", Priority: 100, Field: schema.FieldPath, Op: schema.OpContains, Value: "/.ssh/", Effect: schema.EffectAllow, Severity: schema.SevInfo}}}
	if _, err := Merge(changed); err == nil {
		t.Error("built-in redeclaration with changed semantics accepted")
	}
	// exact shadow of a hard rule: refused
	shadow := &schema.Policy{ID: "u", Name: "n", Version: 1, DefaultEffect: schema.EffectAllow,
		Rules: []schema.Rule{{ID: "sshpass", Priority: 200, Field: schema.FieldPath, Op: schema.OpContains, Value: "/.ssh/", Effect: schema.EffectAllow, Severity: schema.SevInfo}}}
	if _, err := Merge(shadow); err == nil {
		t.Error("exact allow-shadow of hard rule accepted")
	}
	// identical re-declaration: accepted as no-op
	bp, _ := Builtin()
	same := &schema.Policy{ID: "u", Name: "n", Version: 1, DefaultEffect: schema.EffectAllow, Rules: []schema.Rule{bp.Rules[0]}}
	m, err := Merge(same)
	if err != nil || len(m.Rules) != BuiltinRuleCount {
		t.Errorf("identical re-declaration must be a no-op merge (err=%v n=%d)", err, len(m.Rules))
	}
	// wild cap token in a user rule: refused by schema grammar
	wild := &schema.Policy{ID: "u", Name: "n", Version: 1, DefaultEffect: schema.EffectAllow,
		Rules: []schema.Rule{{ID: "w", Priority: 10, Field: schema.FieldPath, Op: schema.OpPrefix, Value: "/x", Effect: schema.EffectWouldBlock, Severity: schema.SevLow, Caps: []string{"not.a.cap"}}}}
	if _, err := Merge(wild); err == nil {
		t.Error("wild cap token accepted")
	}
	// nil / empty path user file: pure built-in
	if m2, err := Merge(nil); err != nil || len(m2.Rules) != BuiltinRuleCount {
		t.Errorf("nil merge broken: %v", err)
	}
	if p, err := LoadUser(""); err != nil || p != nil {
		t.Errorf("empty path must load (nil, nil): %v %v", err, p)
	}
	// real file round trip (LoadUser on valid + corrupt inputs)
	dir := t.TempDir()
	good := filepath.Join(dir, "u.json")
	raw, _ := json.Marshal(map[string]any{"id": "u2", "name": "n", "version": 1, "default_effect": "allow",
		"rules": []map[string]any{{"id": "r", "priority": 10, "field": "path", "op": "prefix", "value": "/opt", "effect": "would_block", "severity": 1}}})
	os.WriteFile(good, raw, 0o600)
	p, err := LoadUser(good)
	if err != nil || len(p.Rules) != 1 {
		t.Fatalf("LoadUser valid file: %v", err)
	}
	os.WriteFile(good, []byte("not json"), 0o600)
	if _, err := LoadUser(good); err == nil {
		t.Error("corrupt user policy accepted")
	}
	if _, err := LoadUser(filepath.Join(dir, "missing.json")); err == nil {
		t.Error("missing user policy silently accepted")
	}
}

// TestBuiltinTablesCrossCheck re-proves (from the Go side) what the
// gate greps: every cap token used by a built-in rule exists in the
// capability vocabulary, every rule field/op is in the enum lists, and
// every rule carries the exact hard shape when flagged.
func TestBuiltinTablesCrossCheck(t *testing.T) {
	p, err := Builtin()
	if err != nil {
		t.Fatal(err)
	}
	fields := map[string]bool{}
	for _, f := range schema.AllMatchFields() {
		fields[f] = true
	}
	ops := map[string]bool{}
	for _, o := range schema.AllMatchOps() {
		ops[o] = true
	}
	used := 0
	for _, r := range p.Rules {
		if !fields[string(r.Field)] {
			t.Errorf("rule %s field %q outside enum", r.ID, r.Field)
		}
		if !ops[string(r.Op)] {
			t.Errorf("rule %s op %q outside enum", r.ID, r.Op)
		}
		for _, c := range r.Caps {
			if !schema.Capability(c).Valid() {
				t.Errorf("rule %s cap %q outside vocabulary", r.ID, c)
			} else {
				used++
			}
		}
	}
	if used == 0 {
		t.Error("built-in rules carry zero cap references; cross-check vacuous")
	}
	toks := CapTokensUsed()
	if len(toks) < 5 {
		t.Errorf("distinct cap tokens %d, want >= 5", len(toks))
	}
	if len(HardRuleIDs()) < 5 {
		t.Error("hard set smaller than designed")
	}
}
