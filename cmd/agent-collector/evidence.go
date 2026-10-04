// evidence.go - the `evidence` subcommand: the user-visible read
// surface of the W7.1 trace constructor plus the W7.2 integrity
// envelope (the section 307 item 15 "enterprise can export complete
// decision/evidence" realisation form, Phase 0 honest scope).
//
// Write discipline (machine-gated): the audit source is opened
// read-only and is NEVER written back; output files land ONLY inside
// an explicit -dir that already exists, and an existing file is
// refused, not overwritten. Without -dir the command performs zero
// filesystem writes and refuses before doing anything else.
package main

import (
	"fmt"
	"os"
	"path/filepath"

	"20131.com/agentruntime/internal/schema"
)

// evidenceSkipBuckets is the closed set of honest skip reasons a
// policy.decision line can have while being traced. A skipped line
// NEVER appears as a partial trace and never reads as "no decision".
var evidenceSkipBuckets = []string{"no_prior_origin", "self_correlation", "trace_rejected"}

// buildTraces derives traces for every policy.decision line.
// originEventID is the id of the most recent preceding line of the
// SAME agent (correlation to what the decision followed; a decision
// with no preceding same-agent line is skipped, never self-linked).
// policyVersion is operator-stated (an un-stated version is a
// recorder error, not a gap, mirroring the W7.1 stance). Pure over
// its inputs: deterministic, no I/O.
func buildTraces(events []schema.Event, policyVersion string) ([]schema.DecisionTrace, map[string]int) {
	var traces []schema.DecisionTrace
	skips := map[string]int{}
	lastByAgent := map[string]string{}
	for _, e := range events {
		if e.Type != schema.TypePolicyDecision {
			lastByAgent[e.AgentID] = e.ID
			continue
		}
		origin := lastByAgent[e.AgentID]
		switch {
		case origin == "" || e.ID == "":
			skips["no_prior_origin"]++
		case origin == e.ID:
			skips["self_correlation"]++
		default:
			ev := e
			t, err := schema.BuildDecisionTrace(&ev, origin, policyVersion)
			if err != nil {
				skips["trace_rejected"]++
			} else {
				traces = append(traces, *t)
			}
		}
		lastByAgent[e.AgentID] = e.ID
	}
	return traces, skips
}

// renderEvidenceSummary is the stdout side of the command (pure, so
// the golden test pins it). It reports counts, skip buckets, bundle
// file list, and verification outcome - never bundle contents.
func renderEvidenceSummary(source string, traceCount int, skips map[string]int, dir string, names []string, verified bool) []string {
	out := []string{fmt.Sprintf("evidence: source=%s traces=%d", source, traceCount)}
	for _, b := range evidenceSkipBuckets {
		out = append(out, fmt.Sprintf("  skipped %-16s %d", b+"=", skips[b]))
	}
	if dir == "" {
		out = append(out, "  wrote: 0 file(s) (no -dir given; command refuses to write)")
		return out
	}
	out = append(out, fmt.Sprintf("  wrote: %d file(s) into %s", len(names), dir))
	for _, n := range names {
		out = append(out, "  + "+n)
	}
	out = append(out, fmt.Sprintf("  integrity: verify=%v (manifest hashes re-checked after write)", verified))
	return out
}

func runEvidence(c config) error {
	if c.policyVersion == "" {
		return fmt.Errorf("evidence: -policy-version must be stated by the operator (an un-stated policy version is a recorder error, not a gap)")
	}
	if c.dir == "" {
		return fmt.Errorf("evidence: -dir is required and must already exist (the command performs zero filesystem writes without an explicit output directory)")
	}
	info, err := os.Stat(c.dir)
	if err != nil || !info.IsDir() {
		return fmt.Errorf("evidence: -dir %q is not an existing directory (refusing to create output paths)", c.dir)
	}
	evs, err := timelineEvents(c.out, c.agentFilter, c.since)
	if err != nil {
		return err
	}
	traces, skips := buildTraces(evs, c.policyVersion)
	bundle, err := schema.BuildEvidenceBundle(traces, c.out)
	if err != nil {
		return fmt.Errorf("evidence: %w", err)
	}
	files := bundle.Files()
	// Verify the in-memory envelope before touching the filesystem:
	// a bundle that fails its own manifest never gets written.
	if probs := schema.VerifyEvidenceBundle(bundle.Manifest, files); len(probs) > 0 {
		return fmt.Errorf("evidence: bundle failed integrity verification before write (%d problem(s): %s)", len(probs), probs[0])
	}
	names := make([]string, 0, len(files))
	for _, f := range files {
		names = append(names, f.Name)
	}
	for _, f := range files {
		path := filepath.Join(c.dir, f.Name)
		if _, err := os.Stat(path); err == nil {
			return fmt.Errorf("evidence: refusing to overwrite existing %s (remove it deliberately or choose another -dir)", path)
		}
		if err := os.WriteFile(path, f.Content, 0o600); err != nil {
			return fmt.Errorf("evidence: write %s: %w", path, err)
		}
	}
	// Re-verify from what actually landed on disk (the audit source
	// stays untouched; only files inside -dir are re-read).
	written := make([]schema.EvidenceFile, 0, len(files))
	for _, f := range files {
		b, err := os.ReadFile(filepath.Join(c.dir, f.Name))
		if err != nil {
			return fmt.Errorf("evidence: read-back %s: %w", f.Name, err)
		}
		written = append(written, schema.EvidenceFile{Name: f.Name, Content: b})
	}
	man := written[0].Content
	probs := schema.VerifyEvidenceBundle(man, written)
	for _, ln := range renderEvidenceSummary(c.out, len(traces), skips, c.dir, names, len(probs) == 0) {
		if _, err := fmt.Fprintln(os.Stdout, ln); err != nil {
			return err
		}
	}
	if len(probs) > 0 {
		return fmt.Errorf("evidence: written bundle failed verification: %s", probs[0])
	}
	return nil
}
