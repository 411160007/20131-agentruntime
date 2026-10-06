package replay

// The three-gate report lands the middle evaluation steps of the
// policy simulator pipeline on top of one replay: Security Evaluation,
// False Positive Analysis, Agent Productivity Analysis, and
// Performance Analysis, in machine-readable form. The inputs are a
// rendered replay Report, the candidate it was built from, and an
// optional label table; nothing else is consulted. Recovery impact
// and the promotion decision record are separate slices and are
// deliberately not present here.
//
// Isolation is unchanged from the replay package contract: no
// registry, no writer, no engine, no runtime decision plane. Every
// blocking reading is the Phase 0 observation word would_block, and
// every metric without a source is stated as a known gap instead of
// being defaulted to zero.

import (
	"encoding/json"
	"fmt"
	"sort"

	"20131.com/agentruntime/internal/schema"
)

// GatesStance pins the Phase 0 reading of the whole gate surface:
// evaluation output for humans and ledgers, never an enforcement
// input.
const GatesStance = "three-gate report: record-only, zero enforcement plane; promotion decision is not part of this report"

// statusMeasured and statusKnownGap are the only section statuses.
// A section reports measured only when every number in it was derived
// from an input that exists; otherwise it reports known_gap with a
// reason and omits its metrics entirely (absent, never a defaulted 0).
const (
	statusMeasured  = "measured"
	statusKnownGap  = "known_gap"
	reasonNoLabels  = "no label table supplied: expectations absent, false-positive rates are not computed from nothing"
	reasonNoVerdict = "no replayed verdicts: counts would be vacuous"
)

// SeverityBand is the would_block tally for one severity reading.
type SeverityBand struct {
	Severity   int `json:"severity"`
	WouldBlock int `json:"would_block"`
}

// SecurityGate is the section-254 Security Evaluation face: what the
// candidate would have decided over the corpus, band by band.
type SecurityGate struct {
	Cases      int            `json:"cases"`
	Held       int            `json:"held_lines"`
	Allow      int            `json:"allow"`
	WouldBlock int            `json:"would_block"`
	Severity   []SeverityBand `json:"severity_would_block"`
	ByRule     []RuleHit      `json:"by_rule"`
}

// FPMetrics carries the measured false-positive analysis. The two
// rates are defined explicitly: false_positive_rate = false positives
// divided by labelled verdicts whose expectation was allow;
// missed_block_rate = missed blocks divided by labelled verdicts whose
// expectation was would_block. Labels without a replayed event are
// counted, never dropped.
type FPMetrics struct {
	LabelledCases       int     `json:"labelled_cases"`
	UnlabelledVerdicts  int     `json:"unlabelled_verdicts"`
	LabelsWithoutEvent  int     `json:"labels_without_event"`
	FalsePositives      int     `json:"false_positives"`
	MissedBlocks        int     `json:"missed_blocks"`
	MatchedExpectations int     `json:"matched_expectations"`
	ExpectAllowTotal    int     `json:"expect_allow_total"`
	ExpectBlockTotal    int     `json:"expect_would_block_total"`
	FalsePositiveRate   float64 `json:"false_positive_rate"`
	MissedBlockRate     float64 `json:"missed_block_rate"`
}

// FalsePositiveGate is the labelled comparison section.
type FalsePositiveGate struct {
	Status  string     `json:"status"`
	Reason  string     `json:"reason,omitempty"`
	Metrics *FPMetrics `json:"metrics,omitempty"`
}

// AgentImpact is one agent's exposure line: of the events this agent
// produced that were replayed, how many the candidate would have
// blocked. blocked_share is that quotient (0 when the agent has no
// replayed events, which cannot happen for a listed agent).
type AgentImpact struct {
	AgentID      string  `json:"agent_id"`
	Events       int     `json:"events"`
	WouldBlock   int     `json:"would_block"`
	BlockedShare float64 `json:"blocked_share"`
}

// ProductivityGate is the Agent Productivity Analysis face: per-agent
// disturbance the candidate would have caused, as observed only.
type ProductivityGate struct {
	Status string        `json:"status"`
	Reason string        `json:"reason,omitempty"`
	Agents []AgentImpact `json:"per_agent,omitempty"`
	Total  int           `json:"agents_observed,omitempty"`
}

// PerformanceGate is the Performance Analysis face restricted to
// structural load statistics. Wall-clock timings are outside the
// byte-determinism contract of this package (two runs must render the
// same bytes), so they are stated absent by construction rather than
// emitted unstable. Per-source line boundaries are not stored on the
// replay report, so the per-file census is a declared gap instead of
// an invented split.
type PerformanceGate struct {
	Status          string `json:"status"`
	Reason          string `json:"reason,omitempty"`
	EvaluatedEvents int    `json:"evaluated_events,omitempty"`
	HeldLines       int    `json:"held_lines,omitempty"`
	CandidateRules  int    `json:"candidate_rule_count,omitempty"`
	TimingStance    string `json:"timing,omitempty"`
	PerSourceStance string `json:"per_source,omitempty"`
}

const timingAbsentReason = "wall-clock timings are outside the byte-determinism contract: structural counts only"
const perSourceAbsentReason = "per-source line boundaries are not stored on the replay report: declared gap, never an invented split"

// GatesReport is the machine-readable three-gate report built from
// one replay Report. Candidate identity and sources are copied so a
// gates document is self-describing without the verdict document next
// to it.
type GatesReport struct {
	CandidateID      string            `json:"candidate_id"`
	CandidateVersion int               `json:"candidate_version"`
	Sources          []string          `json:"sources"`
	Stance           string            `json:"stance"`
	Security         SecurityGate      `json:"security_evaluation"`
	FalsePositive    FalsePositiveGate `json:"false_positive_analysis"`
	Productivity     ProductivityGate  `json:"agent_productivity_analysis"`
	Performance      PerformanceGate   `json:"performance_analysis"`
}

// BuildGates derives the gate report from a replay report, the
// candidate, and an optional label table. It mutates nothing: the
// report, candidate, and labels are read-only inputs.
func BuildGates(rep *Report, candidate *schema.Policy, labels map[string]LabelExpect) (*GatesReport, error) {
	if rep == nil {
		return nil, fmt.Errorf("replay: gates need a report")
	}
	if candidate == nil {
		return nil, fmt.Errorf("replay: gates need the candidate the report was built from")
	}
	g := &GatesReport{
		CandidateID:      rep.CandidateID,
		CandidateVersion: rep.CandidateVersion,
		Sources:          append([]string(nil), rep.Sources...),
		Stance:           GatesStance,
	}

	// Security section: totals mirror the replay report exactly; the
	// severity banding only counts would_block verdicts (the blocking
	// reading is what an evaluation asks about), ascending by severity.
	g.Security.Cases = rep.Totals.Cases
	g.Security.Held = rep.Totals.Held
	g.Security.Allow = rep.Totals.Allow
	g.Security.WouldBlock = rep.Totals.WouldBlock
	g.Security.ByRule = append([]RuleHit(nil), rep.Totals.ByRule...)
	bands := make(map[int]int)
	for _, v := range rep.Verdicts {
		if v.Decision == "would_block" {
			bands[v.Severity]++
		}
	}
	sevs := make([]int, 0, len(bands))
	for s := range bands {
		sevs = append(sevs, s)
	}
	sort.Ints(sevs)
	for _, s := range sevs {
		g.Security.Severity = append(g.Security.Severity, SeverityBand{Severity: s, WouldBlock: bands[s]})
	}
	if g.Security.Severity == nil {
		g.Security.Severity = []SeverityBand{}
	}

	// False positive section: measured only with a label table and at
	// least one replayed verdict; otherwise an honest known gap.
	switch {
	case len(labels) == 0:
		g.FalsePositive.Status = statusKnownGap
		g.FalsePositive.Reason = reasonNoLabels
	case rep.Totals.Cases == 0:
		g.FalsePositive.Status = statusKnownGap
		g.FalsePositive.Reason = reasonNoVerdict
	default:
		m := &FPMetrics{}
		byID := make(map[string]string, len(rep.Verdicts))
		seen := make(map[string]bool, len(rep.Verdicts))
		for _, v := range rep.Verdicts {
			byID[v.ID] = v.Decision
			seen[v.ID] = true
		}
		for id, lab := range labels {
			got, ok := byID[id]
			if !ok {
				m.LabelsWithoutEvent++
				continue
			}
			m.LabelledCases++
			switch lab.Expect {
			case "allow":
				m.ExpectAllowTotal++
				if got == "allow" {
					m.MatchedExpectations++
				} else {
					m.FalsePositives++
				}
			case "would_block":
				m.ExpectBlockTotal++
				if got == "would_block" {
					m.MatchedExpectations++
				} else {
					m.MissedBlocks++
				}
			}
		}
		m.UnlabelledVerdicts = rep.Totals.Cases - m.LabelledCases
		if m.ExpectAllowTotal > 0 {
			m.FalsePositiveRate = float64(m.FalsePositives) / float64(m.ExpectAllowTotal)
		}
		if m.ExpectBlockTotal > 0 {
			m.MissedBlockRate = float64(m.MissedBlocks) / float64(m.ExpectBlockTotal)
		}
		g.FalsePositive.Status = statusMeasured
		g.FalsePositive.Metrics = m
	}

	// Productivity section: per-agent exposure, ascending by agent id
	// for stable bytes. Agents come only from replayed verdicts.
	if rep.Totals.Cases == 0 {
		g.Productivity.Status = statusKnownGap
		g.Productivity.Reason = reasonNoVerdict
	} else {
		type acc struct{ events, blocks int }
		per := make(map[string]*acc)
		for _, v := range rep.Verdicts {
			a := per[v.AgentID]
			if a == nil {
				a = &acc{}
				per[v.AgentID] = a
			}
			a.events++
			if v.Decision == "would_block" {
				a.blocks++
			}
		}
		ids := make([]string, 0, len(per))
		for id := range per {
			ids = append(ids, id)
		}
		sort.Strings(ids)
		for _, id := range ids {
			a := per[id]
			g.Productivity.Agents = append(g.Productivity.Agents, AgentImpact{
				AgentID: id, Events: a.events, WouldBlock: a.blocks,
				BlockedShare: float64(a.blocks) / float64(a.events),
			})
		}
		g.Productivity.Total = len(ids)
		g.Productivity.Status = statusMeasured
	}

	// Performance section: structural load only, plus the honest
	// timing-absence stance line.
	g.Performance.Status = statusMeasured
	g.Performance.EvaluatedEvents = rep.Totals.Cases
	g.Performance.HeldLines = rep.Totals.Held
	g.Performance.CandidateRules = len(candidate.Rules)
	g.Performance.TimingStance = timingAbsentReason
	g.Performance.PerSourceStance = perSourceAbsentReason
	return g, nil
}

// RenderGates serializes a gate report to deterministic bytes, the
// same determinism contract as Render.
func (g *GatesReport) RenderGates() ([]byte, error) {
	b, err := json.MarshalIndent(g, "", "  ")
	if err != nil {
		return nil, err
	}
	return append(b, '\n'), nil
}
