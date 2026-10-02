package schema

// Intent-contract inflow face for the intent wave (slice W3.1). The
// record shape for intents already exists (slice W2.2: IntentRecord,
// ParseIntentRecord, EncodeChecked). What this slice adds is the only
// other thing the owner specification's plan-analysis section needs
// before any later inference work can happen: the channels through
// which a stated intent enters the record, and the single unambiguous
// meaning of "not stated".
//
// Two inflow channels are contracted here. The CLI/file channel feeds
// bytes straight into the existing parser: any shape the parser
// rejects (unknown field names, malformed authority chains, forged
// propagation) produces a rejection with no usable record, matching
// the zero-bytes-on-rejection shape the rest of the package uses. The
// hook-payload channel is deliberately narrower: the task field maps
// to the goal field and nothing else. There is no key the hook
// payload can send that reaches any other intent field, so the
// authority chain cannot be raised through this channel at all - not
// "checked and rejected on suspicion", but structurally unreachable.
//
// When neither channel reports anything, the result is the honest
// empty record: every field absent, carrying the known-gap meaning.
// An agent that does not self-report is recorded as not-self-reported;
// deriving an inferred shape from observed behavior is a later slice
// and no code path here fabricates a plan, a goal, or an authority
// claim to fill the hole.
//
// Like every other contract in this package this is an observation-
// phase shape only. Nothing in the evaluator, the rule engine, the
// bus, the audit writer, or the command tree calls any inflow symbol;
// wiring one in is a deliberate later-phase change, never a silent
// one. docs/schema-v2.md section 14 is the human-facing contract; the
// sync test in this package pins the two sources verbatim, and the
// Node structural checker mirrors the five contract keys and the
// three-source channel vocabulary from the other side.

import (
	"bytes"
	"encoding/json"
	"fmt"
)

// IntentInflowSource names the channel through which an intent (or
// its honest absence) entered the record. The three tokens are a
// closed set. Declaration order is normative: the contract file, this
// list, and the Node checker compare joined strings verbatim, in
// order.
type IntentInflowSource string

const (
	// InflowSourceCLIFile: bytes supplied through the CLI/file
	// channel, parsed by the existing intent parser.
	InflowSourceCLIFile IntentInflowSource = "cli_file"

	// InflowSourceHookTask: a hook payload whose task field was
	// present and non-empty; it flowed into the goal field only.
	InflowSourceHookTask IntentInflowSource = "hook_task"

	// InflowSourceAbsent: neither channel reported a stated intent.
	// This is a positive record of absence, never a placeholder for
	// an inferred one.
	InflowSourceAbsent IntentInflowSource = "absent_not_reported"
)

// AllIntentInflowSources lists every inflow source in normative
// declaration order. scripts/schema-v2-check.mjs and
// docs/schema-v2.md mirror this list verbatim; the sync tests fail on
// any drift.
func AllIntentInflowSources() []string {
	return []string{
		string(InflowSourceCLIFile), string(InflowSourceHookTask),
		string(InflowSourceAbsent),
	}
}

// Valid reports whether s is one of the closed set of inflow sources.
func (s IntentInflowSource) Valid() bool {
	switch s {
	case InflowSourceCLIFile, InflowSourceHookTask, InflowSourceAbsent:
		return true
	}
	return false
}

// The four recorded rule strings of the inflow contract. Each
// constant here, its docs key line, and the Node checker predicate
// must agree verbatim; drift is a sync-test failure, not a comment.
const (
	// HookTaskMappingRule: the hook payload's task field maps to the
	// intent goal field and nothing else. No other intent field has
	// a hook-payload write path.
	HookTaskMappingRule = "task-field-maps-to-goal-and-nothing-else"

	// InflowAbsentDefault: an intent that was not reported means
	// exactly one thing - not reported, a known gap. Fabricating a
	// value for an unreported field is outside the contract; the
	// inferred shape that later slices may build starts from this
	// honest empty record, never replaces it.
	InflowAbsentDefault = "not-reported-is-known-gap-never-fabricated"

	// InflowForgeRule: neither inflow channel can escalate
	// authority. The file channel inherits every parser rejection
	// (unknown fields, malformed chains); the hook channel has no
	// path to the authority chain at all. A payload that tries to
	// smuggle intent fields through the hook channel is rejected
	// before any value is applied.
	InflowForgeRule = "inflow-channel-never-escalates-authority"

	// InflowEnforcementPlane names the (currently empty) plane that
	// would act on inflow shape at decision time. In the observation
	// phase the inflow result is recorded, not enforced; enabling a
	// consumer is a later-phase decision.
	InflowEnforcementPlane = "none-in-observation-phase"
)

// IntentInflow is the result of one inflow evaluation: which channel
// produced the record, the record itself, and whether the
// known-gap note applies. It is a value struct; on any rejection the
// caller receives an error and must not hold a record - the
// zero-value Record field is nil exactly so a half-trusted rejection
// cannot travel downstream, matching the parser contract.
type IntentInflow struct {
	Source   IntentInflowSource
	Record   *IntentRecord
	KnownGap bool
}

// AbsentIntentRecord returns the honest empty record: every field
// absent, which under the omitempty wire shape encodes to a single
// empty JSON object. It carries no goal, no scope, no chain, and no
// fabricated default of any kind.
func AbsentIntentRecord() *IntentRecord {
	return &IntentRecord{}
}

// InflowFromFile evaluates the CLI/file channel. Bytes are fed to
// the existing parser unchanged; on any parse or chain error the
// result is a rejection with no record (zero-value IntentInflow) and
// a non-nil error. A successfully parsed record is returned as-is:
// this channel does not fill in defaults, so empty fields keep
// meaning "not reported".
func InflowFromFile(data []byte) (IntentInflow, error) {
	rec, err := ParseIntentRecord(data)
	if err != nil {
		return IntentInflow{}, fmt.Errorf("cli/file intent inflow rejected: %w", err)
	}
	return IntentInflow{Source: InflowSourceCLIFile, Record: rec}, nil
}

// hookPayload is the strict shape of the hook-payload channel: the
// only accepted key is task, and it must be a string. Disallowing
// unknown fields means a payload that smuggles goal, authority, or
// any other intent wire name through the hook channel is rejected
// before any value is applied - the escalation shape raises nothing.
type hookPayload struct {
	Task *string `json:"task"`
}

// decodeHookPayload performs the strict decode behind the hook
// channel: object shape enforced, unknown keys rejected, and the
// task type checked by the decoder itself. It exists as a separate
// helper so the sync test can assert the rejection happens before
// any value of a smuggled field is ever observable.
func decodeHookPayload(data []byte) (hookPayload, error) {
	dec := json.NewDecoder(bytes.NewReader(data))
	dec.DisallowUnknownFields()
	var p hookPayload
	if err := dec.Decode(&p); err != nil {
		return hookPayload{}, err
	}
	return p, nil
}

// InflowFromHookPayload evaluates the hook-payload channel. A
// payload with a non-empty task string produces a record whose goal
// is that string and nothing else (source hook_task). A well-formed
// payload with no task (or an empty task) is the honest absent case
// (source absent, KnownGap true, empty record). Any malformed
// payload, wrong task type, or extra key is a rejection with no
// record.
func InflowFromHookPayload(data []byte) (IntentInflow, error) {
	p, err := decodeHookPayload(data)
	if err != nil {
		return IntentInflow{}, fmt.Errorf("hook intent inflow rejected: %w", err)
	}
	if p.Task == nil || *p.Task == "" {
		return IntentInflow{Source: InflowSourceAbsent, Record: AbsentIntentRecord(), KnownGap: true}, nil
	}
	return IntentInflow{Source: InflowSourceHookTask, Record: &IntentRecord{Goal: *p.Task}}, nil
}

// Stated reports whether the inflow carries any reported intent
// content at all. Absent inflow is false by definition; any record
// with at least one reported field is true. This is a record-position
// helper with zero decision effect.
func (f IntentInflow) Stated() bool {
	if f.Record == nil || f.Source == InflowSourceAbsent {
		return false
	}
	r := f.Record
	return r.Goal != "" || r.Scope != "" || r.ExpectedOutcome != "" ||
		len(r.ExpectedActions) > 0 || len(r.AllowedResources) > 0 ||
		len(r.SensitiveResources) > 0 || r.ForbiddenScope != "" ||
		r.Authority != nil || r.Duration != "" || len(r.Constraints) > 0
}
