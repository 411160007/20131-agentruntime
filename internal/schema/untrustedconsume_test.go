package schema

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

// baseIntent returns a record whose authority chain is a trusted
// user_direct grant plus an agent_provided step - the shape the
// consumption contract is tested against.
func baseIntent() *IntentRecord {
	return &IntentRecord{
		Goal:  "summarize quarterly report",
		Scope: "docs/quarterly/**",
		Authority: &AuthorityChain{
			Links: []AuthorityLink{
				{Origin: OriginUserDirect},
				{Origin: OriginAgentProvided},
			},
		},
	}
}

func mustLinksJSON(t *testing.T, links []AuthorityLink) string {
	t.Helper()
	b, err := json.Marshal(links)
	if err != nil {
		t.Fatalf("marshal links: %v", err)
	}
	return string(b)
}

func TestUntrustedConsumeYinYang(t *testing.T) {
	// YANG: an untrusted-source rewrite marks the chain and
	// preserves the original grant links byte-identically.
	base := baseIntent()
	before := mustLinksJSON(t, base.Authority.Links)
	out, err := ApplyIntentModification(base, IntentModification{
		Field:         "goal",
		NewValue:      "summarize and publish quarterly report",
		SourceOrigin:  OriginModelInterpreted,
		SourceTrusted: false,
	})
	if err != nil {
		t.Fatalf("untrusted modification rejected: %v", err)
	}
	if !out.Authority.Untrusted {
		t.Fatalf("untrusted source did not propagate to the chain mark")
	}
	tail := out.Authority.Links[len(out.Authority.Links)-1]
	if !tail.Untrusted || tail.Origin != OriginModelInterpreted {
		t.Fatalf("appended link lost its mark or origin: %+v", tail)
	}
	prefix := mustLinksJSON(t, out.Authority.Links[:len(out.Authority.Links)-1])
	if prefix != before {
		t.Fatalf("original chain links drifted:\n before %s\n after  %s", before, prefix)
	}
	if out.Goal != "summarize and publish quarterly report" {
		t.Fatalf("writable field not applied: %q", out.Goal)
	}
	if base.Goal != "summarize quarterly report" || len(base.Authority.Links) != 2 || base.Authority.Untrusted {
		t.Fatalf("base record was mutated")
	}

	// YIN: a trusted-source rewrite appends an unmarked link and
	// sets no chain mark on a clean chain.
	out2, err := ApplyIntentModification(base, IntentModification{
		Field:         "scope",
		NewValue:      "docs/quarterly/** docs/public/**",
		SourceOrigin:  OriginUserDirect,
		SourceTrusted: true,
	})
	if err != nil {
		t.Fatalf("trusted modification rejected: %v", err)
	}
	if out2.Authority.Untrusted {
		t.Fatalf("trusted source marked the chain (sticky must be one-way)")
	}
	if tail2 := out2.Authority.Links[len(out2.Authority.Links)-1]; tail2.Untrusted {
		t.Fatalf("trusted link carries an untrusted mark: %+v", tail2)
	}
}

func TestUntrustedConsumeStickyNeverLaundered(t *testing.T) {
	// A trusted modification on an already-marked chain keeps the
	// mark: consumption never clears an existing UNTRUSTED.
	base := baseIntent()
	base.Authority.Untrusted = true
	out, err := ApplyIntentModification(base, IntentModification{
		Field:         "duration",
		NewValue:      "1h",
		SourceOrigin:  OriginUserDirect,
		SourceTrusted: true,
	})
	if err != nil {
		t.Fatalf("trusted modification rejected: %v", err)
	}
	if !out.Authority.Untrusted {
		t.Fatalf("StickyClearanceRule violated: mark was laundered by a trusted write")
	}
}

func TestUntrustedConsumeRedShapes(t *testing.T) {
	cases := []struct {
		name string
		mod  IntentModification
	}{
		{"forge: untrusted claims user origin", IntentModification{Field: "goal", NewValue: "anything", SourceOrigin: OriginUserDirect, SourceTrusted: false}},
		{"authority field is not a consumption target", IntentModification{Field: "authority", NewValue: "user_direct", SourceOrigin: OriginUserDirect, SourceTrusted: true}},
		{"wild field token", IntentModification{Field: "elevate", NewValue: "x", SourceOrigin: OriginAgentProvided, SourceTrusted: true}},
		{"wild origin token", IntentModification{Field: "goal", NewValue: "x", SourceOrigin: GrantOrigin("root_grant"), SourceTrusted: true}},
	}
	for _, tc := range cases {
		base := baseIntent()
		before := mustLinksJSON(t, base.Authority.Links)
		out, err := ApplyIntentModification(base, tc.mod)
		if err == nil || out != nil {
			t.Fatalf("%s: accepted (out=%v)", tc.name, out)
		}
		if mustLinksJSON(t, base.Authority.Links) != before || base.Authority.Untrusted {
			t.Fatalf("%s: partial mutation leaked into the base record", tc.name)
		}
	}
}

func TestUntrustedConsumeLowLayerNeverAloneMovesUserBoundary(t *testing.T) {
	// Record-form assertion of "a lower layer never alone changes a
	// higher boundary": every chain reachable through this path from
	// a base carrying a trusted user_direct link still carries that
	// exact link at the same position, and no additional trusted
	// user_direct link can materialize from a non-user origin.
	base := baseIntent()
	out := base
	for _, origin := range AllGrantOrigins() {
		if origin == string(OriginUserDirect) {
			continue
		}
		var err error
		out, err = ApplyIntentModification(out, IntentModification{
			Field:         "constraints",
			NewValue:      "note",
			SourceOrigin:  GrantOrigin(origin),
			SourceTrusted: true,
		})
		if err != nil {
			t.Fatalf("low-layer trusted write rejected: %v", err)
		}
	}
	if out.Authority.Links[0] != (AuthorityLink{Origin: OriginUserDirect}) {
		t.Fatalf("user boundary link moved or changed: %+v", out.Authority.Links[0])
	}
	userTrusted := 0
	for _, l := range out.Authority.Links {
		if l.Origin == OriginUserDirect && !l.Untrusted {
			userTrusted++
		}
	}
	if userTrusted != 1 {
		t.Fatalf("trusted user_direct links multiplied through consumption: %d, want the one original", userTrusted)
	}
	if _, err := EncodeChecked(out); err != nil {
		t.Fatalf("consumed record fails the W2.2 contract: %v", err)
	}
}

func TestUntrustedConsumeGoldenRoundTrip(t *testing.T) {
	// One golden marked record and one golden clean record round-trip
	// through the real serializer; the wild sibling shapes stay
	// rejected by the parser of the same package.
	good := map[string]interface{}{
		"goal": "x",
		"authority": map[string]interface{}{
			"links": []interface{}{
				map[string]interface{}{"origin": "user_direct"},
				map[string]interface{}{"origin": "external_model_interpreted", "untrusted": true},
			},
			"untrusted": true,
		},
	}
	b, err := json.Marshal(good)
	if err != nil {
		t.Fatal(err)
	}
	rec, err := ParseIntentRecord(b)
	if err != nil {
		t.Fatalf("golden marked record rejected: %v", err)
	}
	out, err := ApplyIntentModification(rec, IntentModification{Field: "scope", NewValue: "s", SourceOrigin: OriginRuntimeInferred, SourceTrusted: false})
	if err != nil || !out.Authority.Untrusted {
		t.Fatalf("consumption on golden marked record failed: %v", err)
	}
	wild := append(b[:len(b)-2], `,"elevate": true}`...)
	if _, err := ParseIntentRecord(wild); err == nil {
		t.Fatalf("wild field record accepted by the parser")
	}
}

func TestUntrustedConsumeDocsSync(t *testing.T) {
	// Second implementation in the other direction: the five docs
	// key lines of schema-v2 section 13 must match the Go constants
	// verbatim on every test run.
	data, err := os.ReadFile(filepath.Join("..", "..", "docs", "schema-v2.md"))
	if err != nil {
		t.Fatal(err)
	}
	text := string(data)
	pins := map[string]string{
		"untrusted_consume_rule":      UntrustedConsumeRule,
		"authority_preservation_rule": AuthorityPreservationRule,
		"origin_forge_rule":           OriginForgeRule,
		"sticky_clearance_rule":       StickyClearanceRule,
		"consume_enforcement_plane":   ConsumeEnforcementPlane,
	}
	for key, want := range pins {
		line := key + ": " + want
		cnt := 0
		for i := 0; i+len(line) <= len(text); i++ {
			if text[i:i+len(line)] == line {
				cnt++
			}
		}
		if cnt != 1 {
			t.Fatalf("docs pin line %q appears %d times, want exactly 1", line, cnt)
		}
	}
}
