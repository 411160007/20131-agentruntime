// report.go - the read-only `report` subcommand: a plain-text
// aggregate over one local audit JSONL file (status/audit-tail/
// timeline family, additive). It never writes anywhere: the audit
// source is opened read-only and every counter is derived from
// validated lines only. Unknown buckets never collapse into zero -
// each census row states what it counted.
package main

import (
	"fmt"
	"os"
	"sort"

	"20131.com/agentruntime/internal/schema"
)

// renderReport turns validated events into report lines (pure, so the
// golden snapshot test can call it directly). Deterministic by
// construction: every map is sorted by count desc, then key asc.
func renderReport(events []schema.Event) []string {
	byDecision := map[string]int{}
	byType := map[string]int{}
	bySeverity := map[int]int{}
	byAgent := map[string]int{}
	decisions := 0
	for _, e := range events {
		byDecision[string(e.Decision)]++
		byType[string(e.Type)]++
		bySeverity[int(e.Severity)]++
		byAgent[e.AgentID]++
		if e.Type == schema.TypePolicyDecision {
			decisions++
		}
	}
	out := make([]string, 0, len(byDecision)+len(byType)+len(bySeverity)+len(byAgent)+8)
	out = append(out, fmt.Sprintf("report: %d event(s), %d policy.decision line(s)", len(events), decisions))
	out = append(out, "decisions:")
	for _, k := range sortedCountKeysStr(byDecision) {
		out = append(out, fmt.Sprintf("  %-16s %d", k, byDecision[k]))
	}
	out = append(out, "event types:")
	for _, k := range sortedCountKeysStr(byType) {
		out = append(out, fmt.Sprintf("  %-24s %d", k, byType[k]))
	}
	out = append(out, "severity census:")
	sevs := make([]int, 0, len(bySeverity))
	for s := range bySeverity {
		sevs = append(sevs, s)
	}
	sort.Ints(sevs)
	for _, s := range sevs {
		out = append(out, fmt.Sprintf("  severity=%d %d", s, bySeverity[s]))
	}
	out = append(out, fmt.Sprintf("agents: %d distinct", len(byAgent)))
	for _, k := range sortedCountKeysStr(byAgent) {
		out = append(out, fmt.Sprintf("  %-24s %d", k, byAgent[k]))
	}
	return out
}

// sortedCountKeysStr orders a count map deterministically:
// count descending, then key ascending.
func sortedCountKeysStr(m map[string]int) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Slice(keys, func(i, j int) bool {
		if m[keys[i]] != m[keys[j]] {
			return m[keys[i]] > m[keys[j]]
		}
		return keys[i] < keys[j]
	})
	return keys
}

func runReport(c config) error {
	evs, err := timelineEvents(c.out, c.agentFilter, c.since)
	if err != nil {
		return err
	}
	for _, ln := range renderReport(evs) {
		if _, err := fmt.Fprintln(os.Stdout, ln); err != nil {
			return err
		}
	}
	return nil
}
