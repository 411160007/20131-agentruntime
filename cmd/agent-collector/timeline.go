// timeline.go — the read-only CLI timeline (Phase 0 has no GUI):
// renders an audit JSONL file as an aligned terminal event timeline.
// Optional filters: --agent (exact agent_id) and --since (RFC3339).
// Like every control command it never writes: corrupt input is
// refused, not echoed as audit truth.
package main

import (
	"fmt"
	"io"
	"os"
	"time"

	"20131.com/agentruntime/internal/schema"
)

// renderTimeline turns validated events into timeline lines (pure, so
// the golden snapshot test can call it directly).
func renderTimeline(events []schema.Event) []string {
	out := make([]string, 0, len(events)+1)
	out = append(out, fmt.Sprintf("timeline: %d event(s)", len(events)))
	for _, e := range events {
		out = append(out, fmt.Sprintf("%s  %-11s  %-16s  %-24s  %s",
			e.TS.UTC().Format(time.RFC3339), string(e.Decision), string(e.Type), e.AgentID, e.Summary))
	}
	return out
}

func timelineEvents(path, agent string, since time.Time) ([]schema.Event, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	dec := newLineDecoder()
	sc := newScanner(f)
	var out []schema.Event
	for sc.Scan() {
		e, ok, err := dec.decode(sc.Bytes())
		if err != nil {
			return nil, fmt.Errorf("timeline: %w", err)
		}
		if !ok {
			continue
		}
		if agent != "" && e.AgentID != agent {
			continue
		}
		if !since.IsZero() && e.TS.Before(since) {
			continue
		}
		out = append(out, e)
	}
	if err := sc.Err(); err != nil {
		return nil, fmt.Errorf("timeline: %w", err)
	}
	return out, nil
}

func runTimeline(c config) error {
	evs, err := timelineEvents(c.out, c.agentFilter, c.since)
	if err != nil {
		return err
	}
	return writeTimeline(os.Stdout, evs)
}

func writeTimeline(w io.Writer, evs []schema.Event) error {
	for _, ln := range renderTimeline(evs) {
		if _, err := fmt.Fprintln(w, ln); err != nil {
			return err
		}
	}
	return nil
}
