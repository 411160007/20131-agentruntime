package schema

import (
	"errors"
	"strings"
)

// Action necessity record vocabulary and constructor (spec vNext
// section 233, slice W5.1): a risk judgement that only asks whether
// an action is dangerous is incomplete, because the same action may
// still be a perfectly reasonable, necessary, or replaceable step of
// the task the user actually asked for. The spec demands that the
// system record Action Necessity Evidence for the three questions -
// is this action a reasonable step, a necessary step, and is it
// replaceable - for the intent the action claims to serve.
//
// This file is the record half of that obligation: it names the
// closed three-question vocabulary and the closed three-answer
// vocabulary, pins the rules as contract constants, and provides one
// pure constructor that produces an ActionNecessityRecord row. It is
// record-only. No file in the decision planes (policy, rules, bus,
// auditlog) may reference the symbols below while
// action_necessity_enforcement_plane reads none-in-observation-phase;
// scripts/schema-v2-check.mjs scans for that leak every run, and the
// Go sync test pins the docs contract.
//
// The constructor never evaluates necessity itself: answers and
// evidence arrive from the recorder and are validated against the
// closed sets only. What the section 233 shape examples illustrate -
// a deploy chain (git, npm, docker) answering a "deploy the site"
// intent versus reading an unrelated SSH key for that same intent -
// is fixture material in the test battery, not a hard-coded rule in
// the product: the record stores what was judged, never a judge.
// The row deliberately carries no decision field of any shape: the
// Phase 0 closed pair {allow, would_block} lives elsewhere and this
// slice borrows none of it.

// NecessityQuestion names one of the three section 233 questions.
// Declaration order is the normative order; docs/schema-v2.md
// section 18 mirrors it verbatim and the Node checker predicate pins
// both directions.
type NecessityQuestion string

// The three necessity questions in normative order (spec vNext
// section 233).
const (
	NecessityQuestionReasonable    NecessityQuestion = "reasonable_step"
	NecessityQuestionNecessary     NecessityQuestion = "necessary_step"
	NecessityQuestionSubstitutable NecessityQuestion = "substitutable_step"
)

// NecessityAnswer names the closed verdict a recorder writes for one
// question. "no_material" is an explicit honest answer - the
// recorder looked and found nothing to judge on - and is distinct
// from absence, which is governed by the absent-default rule below.
type NecessityAnswer string

// The three closed answers in normative order.
const (
	NecessityAnswerSupports   NecessityAnswer = "supports"
	NecessityAnswerRefutes    NecessityAnswer = "refutes"
	NecessityAnswerNoMaterial NecessityAnswer = "no_material"
)

// Rule constants pinned verbatim against the six-key machine block
// in docs/schema-v2.md section 18.
const (
	// ActionNecessityDangerOnlyRule pins the opening move of section
	// 233: danger alone is never the whole judgement, so necessity
	// rows exist beside risk rows rather than inside them.
	ActionNecessityDangerOnlyRule = "danger-judgement-alone-never-suffices"
	// ActionNecessityEvidenceRule is the verbatim obligation of
	// section 233 ("the system must record Action Necessity
	// Evidence"): an answer without evidence is not a record, and
	// the constructor refuses it before any value is applied.
	ActionNecessityEvidenceRule = "every-recorded-answer-requires-nonempty-evidence"
	// ActionNecessityAbsentDefault pins the honest absence shape: a
	// row with no answer entries records that nothing was assessed
	// - it never infers an answer into the hole, and absence is
	// never read as a necessity finding.
	ActionNecessityAbsentDefault = "absent-means-unassessed-never-inferred"
	// ActionNecessityEnforcementPlane pins the Phase 0 stance:
	// record only, enforcement borrowed by no plane.
	ActionNecessityEnforcementPlane = "none-in-observation-phase"
)

// AllNecessityQuestions lists the three questions in normative
// order. The docs section 18 block is pinned against this function
// by the Node checker predicate; drift in either direction is a red
// build.
func AllNecessityQuestions() []string {
	return []string{
		string(NecessityQuestionReasonable),
		string(NecessityQuestionNecessary),
		string(NecessityQuestionSubstitutable),
	}
}

// AllNecessityAnswers lists the three closed answers in normative
// order, pinned the same way as the questions.
func AllNecessityAnswers() []string {
	return []string{
		string(NecessityAnswerSupports),
		string(NecessityAnswerRefutes),
		string(NecessityAnswerNoMaterial),
	}
}

// Valid reports whether q is a member of the question vocabulary.
// The empty string is not a question: absence is recorded as a
// missing entry, never as a wildcard member.
func (q NecessityQuestion) Valid() bool {
	if q == "" {
		return false
	}
	for _, w := range AllNecessityQuestions() {
		if string(q) == w {
			return true
		}
	}
	return false
}

// Valid reports whether a is a member of the answer vocabulary. The
// empty string is not an answer.
func (a NecessityAnswer) Valid() bool {
	if a == "" {
		return false
	}
	for _, w := range AllNecessityAnswers() {
		if string(a) == w {
			return true
		}
	}
	return false
}

// NecessityAnswerEntry is one question answered with one evidence
// string. The three fields are the smallest shape that satisfies the
// section 233 obligation: which question, what the recorder decided,
// and why.
type NecessityAnswerEntry struct {
	Question NecessityQuestion
	Answer   NecessityAnswer
	Evidence string
}

// ActionNecessityRecord is the row produced for one classified
// action observed against one claimed intent. It has exactly six
// fields, none of them a decision: the checker and the Go test both
// pin the field set so no later edit can quietly bolt an outcome
// onto the necessity seed. Assessed and RefutedPresent are pure
// projections over the recorded entries - they restate what is in
// the row, they judge nothing: a row with no entries is honestly
// unassessed, and absence of the refutation flag is never a finding
// of necessity.
type ActionNecessityRecord struct {
	TaskIntent       string
	Action           DataAction
	Answers          []NecessityAnswerEntry
	Assessed         bool
	RefutedPresent   bool
	EnforcementPlane string
}

// BuildActionNecessity constructs the record row for a classified
// action of the section 15 nine against the intent text it claims to
// serve, with the answer entries the recorder produced (possibly
// none). Every rejection fires before any value is applied: an empty
// intent, an unclassified or wild action, a question outside the
// closed three, a repeated question, an answer outside the closed
// three, or any entry with empty evidence leaves no partial record
// behind. Entries keep input order; no answer is ever synthesised
// for a missing question.
func BuildActionNecessity(intent string, action DataAction, entries []NecessityAnswerEntry) (ActionNecessityRecord, error) {
	if strings.TrimSpace(intent) == "" {
		return ActionNecessityRecord{}, errors.New("action necessity: empty intent")
	}
	if !action.Valid() || action == "" {
		return ActionNecessityRecord{}, errors.New("action necessity: action must be a classified member of the nine")
	}
	seen := make(map[NecessityQuestion]bool, len(entries))
	answers := make([]NecessityAnswerEntry, 0, len(entries))
	refuted := false
	for _, e := range entries {
		if !e.Question.Valid() {
			return ActionNecessityRecord{}, errors.New("action necessity: question outside the closed three")
		}
		if seen[e.Question] {
			return ActionNecessityRecord{}, errors.New("action necessity: a question is asked at most once")
		}
		if !e.Answer.Valid() {
			return ActionNecessityRecord{}, errors.New("action necessity: answer outside the closed three")
		}
		if strings.TrimSpace(e.Evidence) == "" {
			return ActionNecessityRecord{}, errors.New("action necessity: answer without evidence is not a record")
		}
		seen[e.Question] = true
		if e.Answer == NecessityAnswerRefutes {
			refuted = true
		}
		answers = append(answers, e)
	}
	return ActionNecessityRecord{
		TaskIntent:       strings.TrimSpace(intent),
		Action:           action,
		Answers:          answers,
		Assessed:         len(answers) > 0,
		RefutedPresent:   refuted,
		EnforcementPlane: ActionNecessityEnforcementPlane,
	}, nil
}
