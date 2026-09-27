// User-override merge surface. Phase 0 keeps override semantics small
// and honest:
//
//   - a user document may ADD rules (extra detections, domain
//     whitelisting via higher-priority allow rules on the domain
//     field, severity raises);
//   - a user document may NOT change a built-in rule: redeclaring a
//     built-in id requires byte-identical matcher/effect/severity/hard/
//     caps, anything else is refused;
//   - a user allow rule may not exactly shadow a hard built-in matcher
//     (same field+op+value), because that would silently downgrade a
//     non-downgradable annotation. Shadowing a hard rule with a
//     DIFFERENT broader matcher cannot be decided in general at merge
//     time; it is a documented limitation of the v0 merge surface and
//     listed as a known gap in the threat model.
package rules

import (
	"encoding/json"
	"fmt"
	"os"

	"20131.com/agentruntime/internal/schema"
)

// LoadUser reads and validates a user policy document (JSON). Returns
// (nil, nil) when path is empty (no overrides).
func LoadUser(path string) (*schema.Policy, error) {
	if path == "" {
		return nil, nil
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("rules: read user policy: %w", err)
	}
	var p schema.Policy
	if err := json.Unmarshal(raw, &p); err != nil {
		return nil, fmt.Errorf("rules: user policy parse: %w", err)
	}
	if err := p.Validate(); err != nil {
		return nil, fmt.Errorf("rules: user policy invalid: %w", err)
	}
	return &p, nil
}

// Merge returns built-in + user rules as one validated policy.
func Merge(user *schema.Policy) (*schema.Policy, error) {
	base, err := Builtin()
	if err != nil {
		return nil, err
	}
	out := *base
	out.Rules = make([]schema.Rule, len(base.Rules))
	copy(out.Rules, base.Rules)
	if user == nil {
		return &out, nil
	}
	hard := map[string]schema.Rule{}
	byID := map[string]schema.Rule{}
	for _, r := range out.Rules {
		byID[r.ID] = r
		if r.Hard {
			key := string(r.Field) + "\x00" + string(r.Op) + "\x00" + r.Value
			hard[key] = r
		}
	}
	for _, r := range user.Rules {
		if b, dup := byID[r.ID]; dup {
			if !sameSemantics(b, r) {
				return nil, fmt.Errorf("rules: user rule %q redeclares a built-in rule with changed semantics (built-ins are immutable)", r.ID)
			}
			continue // identical re-declaration is a no-op
		}
		if r.Effect == schema.EffectAllow {
			key := string(r.Field) + "\x00" + string(r.Op) + "\x00" + r.Value
			if b, blocked := hard[key]; blocked {
				return nil, fmt.Errorf("rules: user allow rule %q exactly shadows hard built-in %q (not permitted)", r.ID, b.ID)
			}
		}
		out.Rules = append(out.Rules, r)
	}
	if user.Version > out.Version {
		out.Version = user.Version
	}
	if err := out.Validate(); err != nil {
		return nil, fmt.Errorf("rules: merged policy invalid: %w", err)
	}
	return &out, nil
}

func sameSemantics(a, b schema.Rule) bool {
	if a.Field != b.Field || a.Op != b.Op || a.Value != b.Value ||
		a.Effect != b.Effect || a.Severity != b.Severity || a.Hard != b.Hard ||
		a.Priority != b.Priority {
		return false
	}
	if len(a.Caps) != len(b.Caps) {
		return false
	}
	for i := range a.Caps {
		if a.Caps[i] != b.Caps[i] {
			return false
		}
	}
	return true
}
