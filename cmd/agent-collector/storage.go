// storage.go - the read-only `storage` subcommand: the section 273
// occupancy census rendered for a user. It answers "how much disk does
// 20131 actually hold here" for the members the specification names
// (runtime logs, audit, recovery, evidence, total, quota, retention,
// pressure status), plus the per-agent / per-task scope lines.
//
// This file contains no write API of any kind: the census is served by a
// read-only handle inside the audit package and printed to stdout. A
// member with no source in this release is reported as absent, never as
// zero, and a ceiling the operator never declared leaves the band
// uncomputed rather than defaulted.
package main

import (
	"fmt"
	"os"
	"time"

	"20131.com/agentruntime/internal/auditlog"
)

// renderStorage turns census lines into display lines (pure, so the
// golden snapshot test can call it directly). The member order comes
// from the census itself, which derives it from one closed registration.
func renderStorage(lines []auditlog.StorageLine) []string {
	out := make([]string, 0, len(lines)+1)
	out = append(out, "storage: read-only occupancy census")
	for _, ln := range lines {
		if ln.Detail == "" {
			out = append(out, fmt.Sprintf("  %-14s %s", ln.Member, ln.Value))
			continue
		}
		out = append(out, fmt.Sprintf("  %-14s %s | %s", ln.Member, ln.Value, ln.Detail))
	}
	return out
}

func runStorage(c config) error {
	lines, err := auditlog.StorageCensus(c.out, c.capacityBytes, c.perAgentBytes, c.perTaskBytes, time.Now())
	if err != nil {
		return err
	}
	for _, ln := range renderStorage(lines) {
		if _, err := fmt.Fprintln(os.Stdout, ln); err != nil {
			return err
		}
	}
	return nil
}
