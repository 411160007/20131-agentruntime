// Package replay implements the read-only Historical Replay step of the
// policy simulator pipeline: a candidate policy document is applied to an
// audit corpus (and optionally the shipped golden evaluation set) and the
// verdict per event is rendered as a deterministic report.
//
// Isolation is the contract. This package holds no registry, opens no
// writer, starts no engine, and imports nothing that can reach the
// runtime decision plane; the only shipped code it reuses is the pure,
// frozen evaluator in internal/policy. A replay therefore cannot change
// what any future event is told to do: its output is a report about
// records that already exist, and every verdict it prints for a blocking
// effect is the Phase 0 observation word would_block, never an
// enforcement word. A candidate that fails the policy grammar never
// reaches evaluation at all.
package replay

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"sort"

	"20131.com/agentruntime/internal/policy"
	"20131.com/agentruntime/internal/schema"
)

// Verdict is the replay result for one corpus event, in corpus order.
// MatchedRule is empty when the policy default was applied; Default
// records that honestly instead of inventing a rule.
type Verdict struct {
	Seq            int    `json:"seq"`
	ID             string `json:"id"`
	Type           string `json:"type"`
	AgentID        string `json:"agent_id"`
	Decision       string `json:"decision"`
	Severity       int    `json:"severity"`
	MatchedRule    string `json:"matched_rule,omitempty"`
	DefaultApplied bool   `json:"default_applied"`
}

// RuleHit is one matched-rule tally line; the order of the tally is
// fixed (count desc, id asc) so two runs of the same input serialize to
// the same bytes.
type RuleHit struct {
	Rule  string `json:"rule"`
	Count int    `json:"count"`
}

// Totals summarizes a replay. Held counts corpus lines that were not
// kept (unparseable or invalid records) and verdicts the Phase 0
// emission discipline refuses to render (reserved-vocabulary effects):
// both are reported as held, never guessed into a verdict and never
// silently dropped.
type Totals struct {
	Cases      int       `json:"cases"`
	Held       int       `json:"held_lines"`
	Allow      int       `json:"allow"`
	WouldBlock int       `json:"would_block"`
	ByRule     []RuleHit `json:"by_rule"`
}

// Report is the full per-event replay report for one candidate policy.
type Report struct {
	CandidateID      string    `json:"candidate_id"`
	CandidateVersion int       `json:"candidate_version"`
	DefaultEffect    string    `json:"default_effect"`
	Sources          []string  `json:"sources"`
	Totals           Totals    `json:"totals"`
	Verdicts         []Verdict `json:"verdicts"`
	Mismatches       []string  `json:"mismatches,omitempty"`
	Stance           string    `json:"stance"`
}

// stanceLine pins the Phase 0 reading of every report: recorded, not
// enforced.
const stanceLine = "historical_replay: record-only, zero enforcement plane"

// LoadCandidate reads and validates one candidate policy document.
// It has no effect anywhere else: nothing is registered, no engine is
// constructed, and the built-in rule set is not consulted or mutated.
func LoadCandidate(path string) (*schema.Policy, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("replay: candidate %s: %w", path, err)
	}
	var p schema.Policy
	dec := json.NewDecoder(bytes.NewReader(raw))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&p); err != nil {
		return nil, fmt.Errorf("replay: candidate %s: parse: %w", path, err)
	}
	if err := p.Validate(); err != nil {
		return nil, fmt.Errorf("replay: candidate %s rejected: %w", path, err)
	}
	return &p, nil
}

// LoadCorpus reads audit JSONL corpora in the given order. A line that
// does not decode into a valid event is counted as held by the caller
// through the returned count; the events kept preserve file order and
// are returned unmodified.
func LoadCorpus(paths ...string) ([]*schema.Event, int, error) {
	var (
		events []*schema.Event
		held   int
	)
	for _, path := range paths {
		f, err := os.Open(path)
		if err != nil {
			return nil, held, fmt.Errorf("replay: corpus %s: %w", path, err)
		}
		dec := json.NewDecoder(f)
		for {
			var e schema.Event
			if err := dec.Decode(&e); err == io.EOF {
				break
			} else if err != nil {
				// Unparseable tail: held, not guessed.
				held++
				break
			}
			if err := e.Validate(); err != nil {
				held++
				continue
			}
			ev := e
			events = append(events, &ev)
		}
		f.Close()
	}
	return events, held, nil
}

// Run applies the candidate to every event and builds the report. The
// evaluator is constructed from a frozen snapshot of the candidate; the
// events and the policy are never mutated.
func Run(candidate *schema.Policy, events []*schema.Event, sources []string, held int) (*Report, error) {
	if candidate == nil {
		return nil, fmt.Errorf("replay: nil candidate")
	}
	ev, err := policy.New(candidate)
	if err != nil {
		return nil, fmt.Errorf("replay: candidate rejected: %w", err)
	}
	rep := &Report{
		CandidateID:      candidate.ID,
		CandidateVersion: candidate.Version,
		DefaultEffect:    string(candidate.DefaultEffect),
		Sources:          append([]string(nil), sources...),
		Stance:           stanceLine,
	}
	ruleCounts := make(map[string]int)
	for i, e := range events {
		out, err := ev.Evaluate(e)
		if err != nil {
			// A record that validates for storage but not for
			// evaluation is held, same as a corrupt line.
			rep.Totals.Held++
			continue
		}
		d, err := schema.DecisionFor(out.Effect)
		if err != nil {
			// A record that validates for storage but not for
			// evaluation is held, same as a corrupt line.
			rep.Totals.Held++
			continue
		}
		// Phase 0 emission discipline, identical to the runtime
		// recorder: reserved-vocabulary effects (ask) have no
		// decision line yet, so that verdict is held, never emitted.
		if err := schema.MustPhase0Decision(d); err != nil {
			rep.Totals.Held++
			continue
		}
		v := Verdict{
			Seq:            i + 1,
			ID:             e.ID,
			Type:           string(e.Type),
			AgentID:        e.AgentID,
			Decision:       string(d),
			Severity:       int(out.Severity),
			MatchedRule:    out.MatchedRule,
			DefaultApplied: out.MatchedRule == "",
		}
		if out.MatchedRule != "" {
			ruleCounts[out.MatchedRule]++
		}
		switch out.Effect {
		case schema.EffectAllow:
			rep.Totals.Allow++
		case schema.EffectWouldBlock:
			rep.Totals.WouldBlock++
		}
		rep.Verdicts = append(rep.Verdicts, v)
	}
	rep.Totals.Held += held
	rep.Totals.Cases = len(rep.Verdicts)
	for rule, count := range ruleCounts {
		rep.Totals.ByRule = append(rep.Totals.ByRule, RuleHit{Rule: rule, Count: count})
	}
	sort.Slice(rep.Totals.ByRule, func(i, j int) bool {
		if rep.Totals.ByRule[i].Count != rep.Totals.ByRule[j].Count {
			return rep.Totals.ByRule[i].Count > rep.Totals.ByRule[j].Count
		}
		return rep.Totals.ByRule[i].Rule < rep.Totals.ByRule[j].Rule
	})
	return rep, nil
}

// LabelExpect is one expected outcome from the golden label table.
type LabelExpect struct {
	Expect string `json:"expect"`
}

// LoadLabels reads the golden labels.json map (event id -> expectation).
func LoadLabels(path string) (map[string]LabelExpect, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("replay: labels %s: %w", path, err)
	}
	var m map[string]LabelExpect
	if err := json.Unmarshal(raw, &m); err != nil {
		return nil, fmt.Errorf("replay: labels %s: parse: %w", path, err)
	}
	return m, nil
}

// CheckLabels compares replay verdicts against a label table in a fixed
// order (label keys sorted) and records every mismatch as a stable
// line. A label with no replayed event is a mismatch too: the report
// never drops expectation silently.
func (r *Report) CheckLabels(labels map[string]LabelExpect) {
	byID := make(map[string]string, len(r.Verdicts))
	for _, v := range r.Verdicts {
		byID[v.ID] = v.Decision
	}
	ids := make([]string, 0, len(labels))
	for id := range labels {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	for _, id := range ids {
		got, ok := byID[id]
		if !ok {
			r.Mismatches = append(r.Mismatches, fmt.Sprintf("%s: expected %s, replayed absent", id, labels[id].Expect))
			continue
		}
		if got != labels[id].Expect {
			r.Mismatches = append(r.Mismatches, fmt.Sprintf("%s: expected %s, got %s", id, labels[id].Expect, got))
		}
	}
}

// Render serializes a report to deterministic bytes: the same inputs
// produce the same output on every run, which is the replay
// determinism gate.
func (r *Report) Render() ([]byte, error) {
	if r.Verdicts == nil {
		r.Verdicts = []Verdict{}
	}
	if r.Totals.ByRule == nil {
		r.Totals.ByRule = []RuleHit{}
	}
	b, err := json.MarshalIndent(r, "", "  ")
	if err != nil {
		return nil, err
	}
	return append(b, '\n'), nil
}
