package schema

import (
	"encoding/json"
	"strings"
	"testing"
)

const (
	promoDigest = "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"
)

func promoInput() PolicyPromotionRecordInput {
	return PolicyPromotionRecordInput{
		CandidateID:       "cand.policy-a",
		CandidateVersion:  2,
		BasePolicyID:      "gw-base",
		BasePolicyVersion: 1,
		Decision:          PromoShadow,
		Rationale:         "replay shows zero would_block change on the golden set",
		DecidedBy:         "review.lead",
		DecidedAt:         "2026-10-06T14:00:00Z",
		ReplayDigest:      promoDigest,
	}
}

func TestPromotionVocabularyClosed(t *testing.T) {
	all := AllPromotionDecisions()
	if strings.Join(all, ",") != "shadow,promoted,rejected,deferred" {
		t.Fatalf("vocabulary or order drift: %v", all)
	}
	for _, tok := range all {
		if !PromotionDecision(tok).Valid() {
			t.Fatalf("declared token %q must validate", tok)
		}
	}
	for _, tok := range []string{"", "promote", "enforce", "shadow_first", "SHADOW"} {
		if PromotionDecision(tok).Valid() {
			t.Fatalf("token %q is outside the closed four and must not validate", tok)
		}
	}
}

func TestBuildPromotionRecordValid(t *testing.T) {
	rec, err := BuildPolicyPromotionRecord(promoInput())
	if err != nil {
		t.Fatalf("build: %v", err)
	}
	if err := rec.Validate(); err != nil {
		t.Fatalf("built record must validate: %v", err)
	}
	if rec.Kind != "policy.promotion" {
		t.Fatalf("kind: %q", rec.Kind)
	}
	if rec.GatesDigest != PromotionDigestAbsent {
		t.Fatalf("unstated gates citation must render the explicit absent token, got %q", rec.GatesDigest)
	}
	if rec.Stance != PromotionRecordStance || rec.EnforcementPlane != PromotionEnforcement {
		t.Fatal("pinned restatements missing")
	}
	raw, err := json.Marshal(rec)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	// Byte determinism: the record has no maps or unsorted lists.
	again, _ := BuildPolicyPromotionRecord(promoInput())
	raw2, _ := json.Marshal(again)
	if string(raw) != string(raw2) {
		t.Fatalf("record bytes drifted:\n %s\n %s", raw, raw2)
	}
}

func TestBuildPromotionRejectionsLeaveNoRecord(t *testing.T) {
	cases := map[string]func(in *PolicyPromotionRecordInput){
		"unknown decision":     func(in *PolicyPromotionRecordInput) { in.Decision = "promote" },
		"empty decision":       func(in *PolicyPromotionRecordInput) { in.Decision = "" },
		"uppercase digest":     func(in *PolicyPromotionRecordInput) { in.ReplayDigest = strings.ToUpper(promoDigest) },
		"short digest":         func(in *PolicyPromotionRecordInput) { in.ReplayDigest = promoDigest[:63] },
		"missing replay cite":  func(in *PolicyPromotionRecordInput) { in.ReplayDigest = "" },
		"bad timestamp":        func(in *PolicyPromotionRecordInput) { in.DecidedAt = "yesterday" },
		"blank rationale":      func(in *PolicyPromotionRecordInput) { in.Rationale = "   " },
		"zero candidate ver":   func(in *PolicyPromotionRecordInput) { in.CandidateVersion = 0 },
		"negative base ver":    func(in *PolicyPromotionRecordInput) { in.BasePolicyVersion = -1 },
		"bad candidate id":     func(in *PolicyPromotionRecordInput) { in.CandidateID = "bad id!" },
		"bad decided by":       func(in *PolicyPromotionRecordInput) { in.DecidedBy = "" },
		"gates digest garbage": func(in *PolicyPromotionRecordInput) { in.GatesDigest = "not-a-digest" },
	}
	for name, mut := range cases {
		in := promoInput()
		mut(&in)
		rec, err := BuildPolicyPromotionRecord(in)
		if err == nil {
			t.Fatalf("%s: must reject", name)
		}
		if rec != (PolicyPromotionRecord{}) {
			t.Fatalf("%s: rejection must leave no half record behind: %+v", name, rec)
		}
	}
}

func TestPromotionPinsNotEditable(t *testing.T) {
	rec, err := BuildPolicyPromotionRecord(promoInput())
	if err != nil {
		t.Fatalf("build: %v", err)
	}
	rec.Stance = "policy_diff: whatever"
	if err := rec.Validate(); err == nil {
		t.Fatal("a hand-mutated stance must fail validation")
	}
	rec, _ = BuildPolicyPromotionRecord(promoInput())
	rec.EnforcementPlane = "none-in-record-phase mutated"
	if err := rec.Validate(); err == nil {
		t.Fatal("a hand-mutated enforcement restatement must fail validation")
	}
}

func TestPromotionZeroEnforcementVocabulary(t *testing.T) {
	in := promoInput()
	in.GatesDigest = promoDigest
	in.Decision = PromoPromoted
	rec, err := BuildPolicyPromotionRecord(in)
	if err != nil {
		t.Fatalf("build promoted: %v", err)
	}
	raw, err := json.Marshal(rec)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	for _, banned := range []string{`"ask"`, `"deny"`, `"enforce"`, `"quarantine"`, `"rollback"`, `"effect"`, `"severity"`} {
		if strings.Contains(string(raw), banned) {
			t.Fatalf("reserved or decision-plane vocabulary present in promotion record: %s", banned)
		}
	}
}
