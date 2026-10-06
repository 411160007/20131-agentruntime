// replay.go - the read-only `replay` subcommand: the section 254
// Historical Replay step rendered for a user. A candidate policy
// document plus one or more audit corpora produce a deterministic
// per-event verdict report on stdout; passing --labels turns the report
// into a golden comparison as well.
//
// This file contains no write API of any kind. The candidate never
// becomes the runtime policy: replay judges records that already
// exist, so a candidate can be evaluated against history before anyone
// considers promoting it. Every blocking verdict is the Phase 0
// observation word, never an enforcement word.
package main

import (
	"fmt"
	"os"

	"20131.com/agentruntime/internal/replay"
)

func runReplay(c config) error {
	if c.replayPolicy == "" {
		return fmt.Errorf("replay: --policy <candidate.json> is required")
	}
	if len(c.rest) == 0 {
		return fmt.Errorf("replay: pass audit corpora as positional arguments")
	}
	candidate, err := replay.LoadCandidate(c.replayPolicy)
	if err != nil {
		return err
	}
	events, held, err := replay.LoadCorpus(c.rest...)
	if err != nil {
		return err
	}
	rep, err := replay.Run(candidate, events, c.rest, held)
	if err != nil {
		return err
	}
	if c.replayLabels != "" {
		labels, err := replay.LoadLabels(c.replayLabels)
		if err != nil {
			return err
		}
		rep.CheckLabels(labels)
	}
	out, err := rep.Render()
	if err != nil {
		return err
	}
	if _, err := os.Stdout.Write(out); err != nil {
		return err
	}
	if len(rep.Mismatches) > 0 {
		return fmt.Errorf("replay: %d label mismatches (see report)", len(rep.Mismatches))
	}
	return nil
}
