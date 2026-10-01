package schema

// Evidence and profile record shapes for the stability-wave schema
// contracts (slice W2.4, first piece). Like the intent, authority,
// impact, and recovery records these are observation-phase RECORD
// contracts: nothing in the runtime emits them, nothing in the
// evaluator, the rule engine, the bus, or the audit writer consumes
// them. The specification's evidence-integrity section ("key evidence
// must carry these fields") and the two profile sections ("the profile
// may only serve as risk and compatibility input, it may never replace
// a hard security boundary") are lifted here as record promises, not
// as enforcement. docs/schema-v2.md is the human-facing contract; the
// sync tests in this package pin the two sources verbatim, and the
// Node structural checker mirrors them from the other side.

import (
	"bytes"
	"encoding/json"
	"fmt"
)

// evidenceFieldWireNames lists the seven wire tokens of the evidence
// contract in normative order, derived mechanically (lower case, runs
// of spaces and slashes collapsed to single underscores) from the
// verbatim anchor block in docs/schema-v2.md. The slash collapse is
// the one step wider than the intent/impact rule, because the
// specification spells the seventh requirement "Actor / Agent
// Identity"; the docs anchor block pins the full mapping and the count
// is recomputed on every run, never recounted by hand.
var evidenceFieldWireNames = []string{
	"timestamp", "source", "integrity_hash", "policy_version",
	"decision_id", "event_correlation_id", "actor_agent_identity",
}

// AllEvidenceFields returns the seven-field vocabulary in normative
// order. scripts/schema-v2-check.mjs and docs/schema-v2.md mirror this
// list verbatim; the sync tests fail on any drift.
func AllEvidenceFields() []string {
	out := make([]string, len(evidenceFieldWireNames))
	copy(out, evidenceFieldWireNames)
	return out
}

// EvidenceEnforcementPlane keeps the plane discipline shared by the
// wave records: in the observation phase no decision surface consumes
// evidence fields. Each record keeps its own constant so each can
// evolve its plane line independently.
const EvidenceEnforcementPlane = "none-in-observation-phase"

// EvidenceContextRetentionRule lifts the specification's export clause
// ("key context must never be deleted to make exporting easier") into
// a recorded promise. It is a record-level rule only: this slice adds
// no exporter and touches no export path.
const EvidenceContextRetentionRule = "must-not-drop-key-context-for-export"

// EvidenceSignatureEnvelope states the phasing of the specification's
// conditional clause "establish signatures or verifiable evidence
// envelopes when needed". No signature or seal field exists in this
// contract; adding the envelope is a deliberate later-phase change
// with its own slice, never a silent field appearing here.
const EvidenceSignatureEnvelope = "deferred-later-phase"

// EvidenceRecord is the integrity surface for one key piece of
// evidence. Unlike the nullable wave records, all seven fields are
// REQUIRED here: the specification says key evidence must carry them,
// so a half-formed record is malformed, not partially trusted. Event
// correlation is recorded in EventCorrelationID; when a later slice
// needs transaction-level grouping it reuses the RecoveryRecord
// transaction token rather than forking a synonym here (one-vocabulary
// per wave, the recovery-slice precedent).
type EvidenceRecord struct {
	Timestamp          string `json:"timestamp"`
	Source             string `json:"source"`
	IntegrityHash      string `json:"integrity_hash"`
	PolicyVersion      string `json:"policy_version"`
	DecisionID         string `json:"decision_id"`
	EventCorrelationID string `json:"event_correlation_id"`
	ActorAgentIdentity string `json:"actor_agent_identity"`
}

// Validate rejects any missing member of the closed seven-field set.
// A fabricated-completeness shape (a hash whose source is never
// named, an empty stand-in) fails here before any bytes exist.
func (r *EvidenceRecord) Validate() error {
	if r == nil {
		return fmt.Errorf("nil evidence record")
	}
	if err := requireEvidenceField("timestamp", r.Timestamp); err != nil {
		return err
	}
	if err := requireEvidenceField("source", r.Source); err != nil {
		return err
	}
	if err := requireEvidenceField("integrity_hash", r.IntegrityHash); err != nil {
		return err
	}
	if err := requireEvidenceField("policy_version", r.PolicyVersion); err != nil {
		return err
	}
	if err := requireEvidenceField("decision_id", r.DecisionID); err != nil {
		return err
	}
	if err := requireEvidenceField("event_correlation_id", r.EventCorrelationID); err != nil {
		return err
	}
	return requireEvidenceField("actor_agent_identity", r.ActorAgentIdentity)
}

func requireEvidenceField(name, value string) error {
	if value == "" {
		return fmt.Errorf("evidence field %q is required for key evidence", name)
	}
	return nil
}

// ParseEvidenceRecord decodes one evidence record, rejecting any field
// name outside the closed seven-token vocabulary before trusting a
// value, then validating the required members. On any error it
// returns a nil record: a rejected record never holds usable bytes.
func ParseEvidenceRecord(data []byte) (*EvidenceRecord, error) {
	dec := json.NewDecoder(bytes.NewReader(data))
	dec.DisallowUnknownFields()
	var rec EvidenceRecord
	if err := dec.Decode(&rec); err != nil {
		return nil, err
	}
	if err := rec.Validate(); err != nil {
		return nil, err
	}
	return &rec, nil
}

// EncodeEvidenceChecked validates before serializing; rejected records
// produce no bytes.
func EncodeEvidenceChecked(rec *EvidenceRecord) ([]byte, error) {
	if err := rec.Validate(); err != nil {
		return nil, err
	}
	return json.Marshal(rec)
}

// profileScopeWireNames names the two profile kinds assigned in-repo:
// the user-level profile and the per-agent behavior profile. The
// specification introduces them as section titles ("Personal Security
// Profile V2", "Agent Behavior Profile") whose display spellings do
// not mechanically reduce to these short tokens, so the back-check
// here is the row-order-pinned degraded form already established by
// the blast-radius scope table: the docs table pins row order against
// the two specification sections, and the honest note says so.
var profileScopeWireNames = []string{"personal", "agent"}

// AllProfileScopes returns the two scope tokens in normative order.
func AllProfileScopes() []string {
	out := make([]string, len(profileScopeWireNames))
	copy(out, profileScopeWireNames)
	return out
}

// ProfileScope names one of the two profile kinds assigned in-repo
// (the wave does not fork the display spellings of the specification
// section titles; the docs table pins row order, the honest-degradation
// precedent set by the blast-radius scope table).
type ProfileScope string

const (
	ProfileScopePersonal ProfileScope = "personal"
	ProfileScopeAgent    ProfileScope = "agent"
)

// Valid reports whether s is one of the two closed scopes.
func (s ProfileScope) Valid() bool {
	return s == ProfileScopePersonal || s == ProfileScopeAgent
}

// profilePersonalFieldWireNames lists the twelve wire tokens of the
// user-level profile, derived mechanically (lower case, spaces to
// single underscores) from the verbatim anchor block copied from the
// specification's personal-profile text listing. "Common MCPs"
// reduces to common_mcps by the same rule as every other line.
var profilePersonalFieldWireNames = []string{
	"common_agents", "common_tools", "common_mcps", "common_skills",
	"projects", "servers", "domains", "normal_workflows",
	"sensitive_assets", "hard_deny", "preferred_security_mode",
	"compatibility_notes",
}

// AllProfilePersonalFields returns the twelve-token vocabulary in
// normative order.
func AllProfilePersonalFields() []string {
	out := make([]string, len(profilePersonalFieldWireNames))
	copy(out, profilePersonalFieldWireNames)
	return out
}

// profileAgentFieldWireNames lists the eleven wire tokens of the
// per-agent behavior profile, derived by the same mechanical rule from
// the specification's agent-behavior bullet list. Common tools is one
// token shared by both scopes on purpose: the wave does not fork
// synonymous vocabularies per scope.
var profileAgentFieldWireNames = []string{
	"common_tasks", "common_tools", "common_processes", "common_files",
	"common_network", "normal_sequences", "known_deviations",
	"compatibility_issues", "validated_fixes", "confidence",
	"trust_decay",
}

// AllProfileAgentFields returns the eleven-token vocabulary in
// normative order.
func AllProfileAgentFields() []string {
	out := make([]string, len(profileAgentFieldWireNames))
	copy(out, profileAgentFieldWireNames)
	return out
}

// ProfileEnforcementPlane records that no decision surface consumes
// profile fields in the observation phase. ProfileHardBoundaryRule is
// the specification's own limit ("may never replace a hard security
// boundary") lifted verbatim into a promise: profiles are input-only
// risk and compatibility context, and the later slices that feed them
// into judgement inherit this line as a precondition.
// ProfileAbsentSemantics keeps the honest-absent discipline: a field
// nobody has observed yet is empty (a known gap), never guessed, and
// an absent profile record never implies a permissive default.
const ProfileEnforcementPlane = "none-in-observation-phase"

// ProfileHardBoundaryRule is enforced as its own line for the same
// stability reason.
const ProfileHardBoundaryRule = "never-replaces-hard-security-boundaries"

// ProfileAbsentSemantics names the honest-empty meaning shared by the
// two scopes.
const ProfileAbsentSemantics = "not-yet-observed-known-gap"

// ProfileNumberSemantics pins that confidence and trust decay stay
// recorded text, not scores.
const ProfileNumberSemantics = "recorded-text-no-numeric-score"

// PersonalProfileRecord is the record-form minimal skeleton of the
// user-level long-term profile. Every field is optional at the record
// level - the profile is something that accumulates, so absence means
// not yet observed (a known gap), never estimated-empty. Scalar
// observations stay free-form strings: numeric confidence or decay
// scoring is a later-phase surface and would imply judgement inputs
// this record must not carry.
type PersonalProfileRecord struct {
	CommonAgents          []string `json:"common_agents,omitempty"`
	CommonTools           []string `json:"common_tools,omitempty"`
	CommonMcps            []string `json:"common_mcps,omitempty"`
	CommonSkills          []string `json:"common_skills,omitempty"`
	Projects              []string `json:"projects,omitempty"`
	Servers               []string `json:"servers,omitempty"`
	Domains               []string `json:"domains,omitempty"`
	NormalWorkflows       []string `json:"normal_workflows,omitempty"`
	SensitiveAssets       []string `json:"sensitive_assets,omitempty"`
	HardDeny              []string `json:"hard_deny,omitempty"`
	PreferredSecurityMode string   `json:"preferred_security_mode,omitempty"`
	CompatibilityNotes    []string `json:"compatibility_notes,omitempty"`
}

// Validate is structural: the vocabulary itself is enforced at parse
// time (unknown field names are rejected before any value is trusted).
// There is nothing further to reject in the record-form skeleton, and
// inventing semantic checks here would pre-borrow later phases.
func (r *PersonalProfileRecord) Validate() error {
	if r == nil {
		return fmt.Errorf("nil personal profile record")
	}
	return nil
}

// ParsePersonalProfile decodes one user-level profile record with the
// closed-vocabulary gate. On any error it returns a nil record.
func ParsePersonalProfile(data []byte) (*PersonalProfileRecord, error) {
	dec := json.NewDecoder(bytes.NewReader(data))
	dec.DisallowUnknownFields()
	var rec PersonalProfileRecord
	if err := dec.Decode(&rec); err != nil {
		return nil, err
	}
	if err := rec.Validate(); err != nil {
		return nil, err
	}
	return &rec, nil
}

// AgentProfileRecord is the record-form minimal skeleton of the
// per-agent behavior profile: the observed-common lists plus the two
// scalar observation fields the specification names (confidence and
// trust decay), recorded as text. Known deviations and validated
// fixes live here as plain accumulators; nothing reads them yet.
type AgentProfileRecord struct {
	CommonTasks         []string `json:"common_tasks,omitempty"`
	CommonTools         []string `json:"common_tools,omitempty"`
	CommonProcesses     []string `json:"common_processes,omitempty"`
	CommonFiles         []string `json:"common_files,omitempty"`
	CommonNetwork       []string `json:"common_network,omitempty"`
	NormalSequences     []string `json:"normal_sequences,omitempty"`
	KnownDeviations     []string `json:"known_deviations,omitempty"`
	CompatibilityIssues []string `json:"compatibility_issues,omitempty"`
	ValidatedFixes      []string `json:"validated_fixes,omitempty"`
	Confidence          string   `json:"confidence,omitempty"`
	TrustDecay          string   `json:"trust_decay,omitempty"`
}

// Validate is structural for the same reason as the personal profile.
func (r *AgentProfileRecord) Validate() error {
	if r == nil {
		return fmt.Errorf("nil agent profile record")
	}
	return nil
}

// ParseAgentProfile decodes one agent behavior profile record with the
// closed-vocabulary gate. On any error it returns a nil record.
func ParseAgentProfile(data []byte) (*AgentProfileRecord, error) {
	dec := json.NewDecoder(bytes.NewReader(data))
	dec.DisallowUnknownFields()
	var rec AgentProfileRecord
	if err := dec.Decode(&rec); err != nil {
		return nil, err
	}
	if err := rec.Validate(); err != nil {
		return nil, err
	}
	return &rec, nil
}

// EncodePersonalProfileChecked validates before serializing; rejected
// records produce no bytes.
func EncodePersonalProfileChecked(rec *PersonalProfileRecord) ([]byte, error) {
	if err := rec.Validate(); err != nil {
		return nil, err
	}
	return json.Marshal(rec)
}

// EncodeAgentProfileChecked validates before serializing; rejected
// records produce no bytes.
func EncodeAgentProfileChecked(rec *AgentProfileRecord) ([]byte, error) {
	if err := rec.Validate(); err != nil {
		return nil, err
	}
	return json.Marshal(rec)
}
