package schema

import (
	"fmt"
	"strings"
	"time"
)

// Policy promotion record (spec v2 section 254, slice W9.3 record
// surface): the pipeline that section 254 lays out ends at a Promotion
// Decision - after replay, security evaluation, false-positive,
// productivity, performance, and recovery-impact analysis, a human
// (or an explicitly authorised review) decides whether a candidate
// policy moves from shadow to promotion, and that decision is a
// ledger event, not a runtime effect.
//
// This slice lands the recording surface only:
//
//   - the record shape with the section 254 identity fields (which
//     candidate, against which base, what decision, why, by whom,
//     when, citing which replay);
//   - a closed four-word decision vocabulary derived from section
//     254's own ordering (new rules prefer shadow first, then
//     promotion): shadow
//     (enter observation first), promoted (the promotion was
//     decided), rejected (the candidate was turned down), deferred
//     (analysis pending, nothing decided). Silence is never a
//     decision word and an unrecorded state must be stated as
//     deferred with a reason, not left blank;
//   - two constructor-pinned restatements no caller can edit: the
//     record-only stance, and the pre-pinned discipline that the
//     Phase 1 gate (the no-promotion-record, no-enforcement rule)
//     will later read - absent a
//     promoted record, no enforcement plane is admissible. Pinning
//     the sentence here is documentation of the future gate's input
//     contract; nothing in this slice consults, enforces, or wires
//     it anywhere.
//
// What the record never carries: an effect, a severity, a runtime
// pointer, or a policy body. The digests bind the decision to the
// evidence that justified it (a replay report and, when present, the
// gates report); they are lowercase hex sha256 shapes validated
// structurally, never resolved or followed. Absence is single-meaning:
// an optional gates digest that is not stated renders as the explicit
// "absent" token, never as an empty string a later reader could
// confuse with "not computed because I did not look".
//
// docs/schema-v2.md section 30 is the human-facing contract. The
// record is not an EventType and enters no decision-plane file
// (policy, rules, bus, auditlog) while
// promotion_enforcement_plane reads none-in-record-phase.

// PromotionDecision is one closed decision word of the section 254
// promotion step. Declaration order is normative (shadow precedes
// promotion in the spec's own sentence); the section 30 anchor
// bullets and any later checker mirror derive the wire tokens from
// it and fail on drift in either direction.
type PromotionDecision string

// The four decision words.
const (
	PromoShadow   PromotionDecision = "shadow"
	PromoPromoted PromotionDecision = "promoted"
	PromoRejected PromotionDecision = "rejected"
	PromoDeferred PromotionDecision = "deferred"
)

// promotionDecisionWireNames lists the four tokens in normative
// order. AllPromotionDecisions mirrors it; nobody recounts them.
var promotionDecisionWireNames = []string{"shadow", "promoted", "rejected", "deferred"}

// AllPromotionDecisions returns the four decision tokens in
// declaration order. The count and order are machine-asserted.
func AllPromotionDecisions() []string {
	out := make([]string, len(promotionDecisionWireNames))
	copy(out, promotionDecisionWireNames)
	return out
}

// Valid reports whether d is one of the closed four.
func (d PromotionDecision) Valid() bool {
	for _, n := range promotionDecisionWireNames {
		if string(d) == n {
			return true
		}
	}
	return false
}

// PromotionDigestAbsent is the explicit token for "no gates report
// was cited": an optional field must state its absence, never leave
// an empty string that is ambiguous between "absent" and "unknown".
const PromotionDigestAbsent = "absent"

// PromotionRecordStance and PromotionEnforcementPlane are the two
// constructor-pinned restatements. They are values, not fields a
// caller supplies.
const (
	PromotionRecordStance = "policy_promotion_record: record-only, zero enforcement plane"
	PromotionEnforcement  = "none-in-record-phase; absent a promoted record no enforcement plane is admissible (pre-pinned Phase 1 discipline, not wired here)"
)

// PolicyPromotionRecord is one promotion decision as recorded. Field
// order is the wire order and is part of the contract.
type PolicyPromotionRecord struct {
	Kind              string            `json:"kind"`
	CandidateID       string            `json:"candidate_id"`
	CandidateVersion  int               `json:"candidate_version"`
	BasePolicyID      string            `json:"base_policy_id"`
	BasePolicyVersion int               `json:"base_policy_version"`
	Decision          PromotionDecision `json:"decision"`
	Rationale         string            `json:"rationale"`
	DecidedBy         string            `json:"decided_by"`
	DecidedAt         string            `json:"decided_at"`
	ReplayDigest      string            `json:"replay_digest"`
	GatesDigest       string            `json:"gates_digest"`
	Stance            string            `json:"stance"`
	EnforcementPlane  string            `json:"promotion_enforcement_plane"`
}

// PolicyPromotionRecordInput is what a caller supplies; the pinned
// restatements and the kind marker are not among it.
type PolicyPromotionRecordInput struct {
	CandidateID       string
	CandidateVersion  int
	BasePolicyID      string
	BasePolicyVersion int
	Decision          PromotionDecision
	Rationale         string
	DecidedBy         string
	DecidedAt         string
	ReplayDigest      string
	GatesDigest       string // optional: empty renders PromotionDigestAbsent
}

// validDigest checks the lowercase-hex sha256 shape without ever
// touching a filesystem or a network: it validates a citation, not
// its target.
func validDigest(s string) bool {
	if len(s) != 64 {
		return false
	}
	for i := 0; i < len(s); i++ {
		c := s[i]
		if !(c >= '0' && c <= '9' || c >= 'a' && c <= 'f') {
			return false
		}
	}
	return true
}

// Validate checks the record grammar. A hand-constructed record must
// pass the same gate a build produces, so Build only pins values and
// Validate does all the judging.
func (r *PolicyPromotionRecord) Validate() error {
	if r.Kind != "policy.promotion" {
		return fmt.Errorf("promotion record: kind %q invalid", r.Kind)
	}
	for _, req := range []struct {
		name  string
		value string
	}{
		{"candidate_id", r.CandidateID},
		{"base_policy_id", r.BasePolicyID},
		{"decided_by", r.DecidedBy},
		{"rationale", r.Rationale},
	} {
		if strings.TrimSpace(req.value) == "" {
			return fmt.Errorf("promotion record: field %q must record text (absence is only admissible when stated explicitly)", req.name)
		}
	}
	if !validID(r.CandidateID) {
		return fmt.Errorf("promotion record: candidate_id %q invalid", r.CandidateID)
	}
	if !validID(r.BasePolicyID) {
		return fmt.Errorf("promotion record: base_policy_id %q invalid", r.BasePolicyID)
	}
	if !validID(r.DecidedBy) {
		return fmt.Errorf("promotion record: decided_by %q invalid", r.DecidedBy)
	}
	if r.CandidateVersion < 1 {
		return fmt.Errorf("promotion record: candidate_version must be >= 1, got %d", r.CandidateVersion)
	}
	if r.BasePolicyVersion < 1 {
		return fmt.Errorf("promotion record: base_policy_version must be >= 1, got %d", r.BasePolicyVersion)
	}
	if !r.Decision.Valid() {
		return fmt.Errorf("promotion record: decision %q is outside the closed four", string(r.Decision))
	}
	if _, err := time.Parse(time.RFC3339, r.DecidedAt); err != nil {
		return fmt.Errorf("promotion record: decided_at %q is not an RFC3339 timestamp", r.DecidedAt)
	}
	if !validDigest(r.ReplayDigest) {
		return fmt.Errorf("promotion record: replay_digest must be 64 lowercase hex characters (the record must cite the replay it rests on)")
	}
	if r.GatesDigest != PromotionDigestAbsent && !validDigest(r.GatesDigest) {
		return fmt.Errorf("promotion record: gates_digest must be 64 lowercase hex characters or the explicit %q token", PromotionDigestAbsent)
	}
	if r.Stance != PromotionRecordStance {
		return fmt.Errorf("promotion record: stance must be the pinned record-only line, got %q", r.Stance)
	}
	if r.EnforcementPlane != PromotionEnforcement {
		return fmt.Errorf("promotion record: promotion_enforcement_plane must be the pinned none-in-record-phase line, got %q", r.EnforcementPlane)
	}
	return nil
}

// BuildPolicyPromotionRecord produces one validated record or no
// record at all: every rejection returns the zero value, never a
// half-filled line.
func BuildPolicyPromotionRecord(in PolicyPromotionRecordInput) (PolicyPromotionRecord, error) {
	gates := strings.TrimSpace(in.GatesDigest)
	if gates == "" {
		gates = PromotionDigestAbsent
	}
	rec := PolicyPromotionRecord{
		Kind:              "policy.promotion",
		CandidateID:       in.CandidateID,
		CandidateVersion:  in.CandidateVersion,
		BasePolicyID:      in.BasePolicyID,
		BasePolicyVersion: in.BasePolicyVersion,
		Decision:          in.Decision,
		Rationale:         in.Rationale,
		DecidedBy:         in.DecidedBy,
		DecidedAt:         in.DecidedAt,
		ReplayDigest:      in.ReplayDigest,
		GatesDigest:       gates,
		Stance:            PromotionRecordStance,
		EnforcementPlane:  PromotionEnforcement,
	}
	if err := rec.Validate(); err != nil {
		return PolicyPromotionRecord{}, err
	}
	return rec, nil
}
