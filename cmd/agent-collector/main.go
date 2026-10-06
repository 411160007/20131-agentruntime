// agent-collector is the Phase 0 discovery collector of the 20131 Agent
// Security Runtime: it polls the local process table, identifies AI-agent
// families (Claude Code, Codex, OpenClaw, MCP servers) from fingerprints,
// and records every finding as a validated JSONL audit line with size or
// day-cut rotation. It never blocks, signals, or otherwise interferes
// with observed processes, and performs no network I/O of any kind; the
// local audit file is the only output.
//
// Control surface (read-only): the "status", "audit-tail", and
// "timeline" subcommands summarize an existing audit file; "report"
// aggregates counts over it, "evidence" exports a verified bundle,
// "storage" prints the occupancy census, and "replay" judges a
// candidate policy document against audit corpora without ever
// installing it. None of them writes.
//
// Adapter surface (platform integration): "hook" receives one agent
// hook callback on stdin and records it (always exits 0, never writes
// stdout — Phase 0 pass-through); "mcp" runs a stdio JSON-RPC relay
// that forwards untouched while auditing tools/call round trips;
// "integrate" generates per-agent configuration that routes callbacks
// here. None of these can change what an observed process does; the
// collector still never performs network I/O of any kind (the MCP
// relay speaks over child-process pipes only).
package main

import (
	"flag"
	"fmt"
	"os"
	"runtime"
	"strings"
	"time"
)

// version is overridden at build time via -ldflags "-X main.version=...".
var version = "0.5.0"

func main() {
	cfg, sub, err := parseFlags(flag.CommandLine, os.Args[1:])
	if err != nil {
		fmt.Fprintf(os.Stderr, "agent-collector: %v\n", err)
		os.Exit(2)
	}
	if cfg.showVersion {
		fmt.Printf("agent-collector %s %s/%s (%s)\n", version, runtime.GOOS, runtime.GOARCH, runtime.Version())
		return
	}
	switch sub {
	case "":
		err = run(cfg)
	case "status":
		err = runStatus(cfg)
	case "audit-tail":
		err = runTail(cfg)
	case "timeline":
		err = runTimeline(cfg)
	case "report":
		err = runReport(cfg)
	case "evidence":
		err = runEvidence(cfg)
	case "storage":
		err = runStorage(cfg)
	case "replay":
		err = runReplay(cfg)
	case "hook":
		// The receiver's failure contract: never disturb the agent.
		_ = runHookSub(cfg)
		return
	case "mcp":
		err = runMCPSub(cfg, cfg.rest)
	case "integrate":
		err = runIntegrateSub(cfg)
	default:
		fmt.Fprintf(os.Stderr, "agent-collector: unknown subcommand %q (want status | audit-tail | timeline | report | evidence | storage | replay | hook | mcp | integrate)\n", sub)
		os.Exit(2)
	}
	if err != nil {
		fmt.Fprintf(os.Stderr, "agent-collector: %v\n", err)
		os.Exit(1)
	}
}

type config struct {
	showVersion   bool
	out           string
	interval      time.Duration
	once          bool
	maxCycles     int
	rotateBytes   int64
	rotateDaily   bool
	keepHistory   int
	mode          string // observe is the ONLY mode in Phase 0 (see phase0Modes)
	trust         string // user trust file for status display (never written by the collector)
	tailN         int
	agentFilter   string    // timeline: exact agent_id filter (empty = all)
	since         time.Time // timeline: RFC3339 lower bound (zero = none)
	policyVersion string    // evidence: operator-stated policy version token (required, never defaulted)
	// storage census ceilings: a zero in any of these means the operator
	// never declared that ceiling, so the display states the absence and
	// computes nothing (never a defaulted number).
	capacityBytes int64 // storage: total capacity ceiling for the pressure band
	perAgentBytes int64 // storage: per-agent byte ceiling for the scope census
	perTaskBytes  int64 // storage: per-task byte ceiling for the scope census
	// replay surface: a candidate policy document judged against audit
	// corpora; the optional label table turns the report into a golden
	// comparison. Read-only: nothing here can reach the runtime policy.
	replayPolicy string // replay: candidate policy json (required)
	replayLabels string // replay: optional labels json (expect per event id)
	replayGates  bool   // replay: print the machine-readable three-gate evaluation report instead of per-event verdicts
	// adapter surface: integrate target/dir/bin and mcp server exe plus
	// its pass-through args (rest = positional args after flag parsing).
	target string
	dir    string
	bin    string
	server string
	rest   []string
}

// phase0Modes is the complete set of runtime modes in Phase 0. The
// machine gate additionally greps that no mode string outside this set
// appears anywhere on an emission path; enforcement modes belong to a
// later phase and must be introduced together with their tests, docs,
// and the corresponding gate change.
var phase0Modes = []string{"observe"}

// knownFlags is the complete flag grammar (shared by the run form and
// the subcommand forms).
var knownFlags = map[string]bool{
	"-version": true, "--version": true, "-out": true, "--out": true,
	"-interval": true, "--interval": true, "-once": true, "--once": true,
	"-max-cycles": true, "--max-cycles": true,
	"-rotate-bytes": true, "--rotate-bytes": true,
	"-rotate-daily": true, "--rotate-daily": true,
	"-keep-history": true, "--keep-history": true,
	"-mode": true, "--mode": true,
	"-trust": true, "--trust": true,
	"-n": true, "--n": true,
	"-agent": true, "--agent": true,
	"-since": true, "--since": true,
	"-target": true, "--target": true,
	"-dir": true, "--dir": true,
	"-bin": true, "--bin": true,
	"-server": true, "--server": true,
	"-policy-version": true, "--policy-version": true,
	"-capacity-bytes": true, "--capacity-bytes": true,
	"-per-agent-bytes": true, "--per-agent-bytes": true,
	"-per-task-bytes": true, "--per-task-bytes": true,
	"-policy": true, "--policy": true,
	"-labels": true, "--labels": true,
}

// parseFlags supports `agent-collector [subcommand] [flags...]`: the
// optional leading positional selects a read-only control command.
func parseFlags(fs *flag.FlagSet, args []string) (config, string, error) {
	var c config
	sub := ""
	if len(args) > 0 && !knownFlags[args[0]] {
		if strings.HasPrefix(args[0], "-") {
			return c, "", fmt.Errorf("unknown flag %q", args[0])
		}
		sub = args[0]
		args = args[1:]
	}
	fs.BoolVar(&c.showVersion, "version", false, "print version and exit")
	fs.StringVar(&c.out, "out", "agent-audit.jsonl", "path of the local JSONL audit file (created 0600; rotated segments get a UTC-stamp suffix)")
	fs.DurationVar(&c.interval, "interval", 30*time.Second, "polling interval between scans")
	fs.BoolVar(&c.once, "once", false, "run exactly one scan cycle and exit")
	fs.IntVar(&c.maxCycles, "max-cycles", 0, "stop after N scan cycles (0 = run until signaled)")
	fs.Int64Var(&c.rotateBytes, "rotate-bytes", 10<<20, "rotate the audit file once it reaches this many bytes (0 = size rotation off)")
	fs.BoolVar(&c.rotateDaily, "rotate-daily", true, "rotate the audit file at the UTC day boundary")
	fs.IntVar(&c.keepHistory, "keep-history", 0, "keep at most N rotated segments (0 = keep all)")
	fs.StringVar(&c.mode, "mode", "observe", "runtime mode; Phase 0 is observe-only")
	fs.StringVar(&c.trust, "trust", "", "path of the user trust file listing explicit trusted agent ids (status display only)")
	fs.IntVar(&c.tailN, "n", 10, "for audit-tail: number of trailing lines")
	agent := fs.String("agent", "", "for timeline: show only this exact agent_id (empty = all)")
	sinceStr := fs.String("since", "", "for timeline: RFC3339 lower bound on event time (empty = none)")
	target := fs.String("target", "", "for integrate: adapter target (claude-code | codex | openclaw)")
	dir := fs.String("dir", "", "for integrate: the agent config directory to write into")
	bin := fs.String("bin", "", "for integrate: collector binary path embedded in generated config (default: this executable)")
	server := fs.String("server", "", "for mcp: server executable to relay to (remaining positional args go to it)")
	policyVersion := fs.String("policy-version", "", "for evidence: operator-stated policy version token (required; never defaulted)")
	capacityBytes := fs.Int64("capacity-bytes", 0, "for storage: total capacity ceiling used only to display the pressure band (0 = never declared, band not computed)")
	perAgentBytes := fs.Int64("per-agent-bytes", 0, "for storage: per-agent byte ceiling used only to display scope records (0 = never declared)")
	perTaskBytes := fs.Int64("per-task-bytes", 0, "for storage: per-task byte ceiling used only to display scope records (0 = never declared)")
	replayPolicy := fs.String("policy", "", "for replay: candidate policy json to judge against history (required; never becomes the runtime policy)")
	replayLabels := fs.String("labels", "", "for replay: optional golden label json (event id -> expected verdict)")
	replayGates := fs.Bool("gates", false, "for replay: print the machine-readable three-gate evaluation report (security / false positive / productivity / performance) instead of the per-event verdicts")
	if err := fs.Parse(args); err != nil {
		return c, sub, err
	}
	c.target = *target
	c.dir = *dir
	c.bin = *bin
	c.server = *server
	c.policyVersion = *policyVersion
	c.capacityBytes = *capacityBytes
	c.perAgentBytes = *perAgentBytes
	c.perTaskBytes = *perTaskBytes
	c.replayPolicy = *replayPolicy
	c.replayLabels = *replayLabels
	c.replayGates = *replayGates
	c.rest = fs.Args()
	c.agentFilter = *agent
	if *sinceStr != "" {
		ts, err := time.Parse(time.RFC3339, *sinceStr)
		if err != nil {
			return c, sub, fmt.Errorf("-since must be RFC3339 (e.g. 2026-09-27T09:00:00Z): %w", err)
		}
		c.since = ts.UTC()
	}
	if !isPhase0Mode(c.mode) {
		return c, sub, fmt.Errorf("mode %q is not available in this release (modes: %s)", c.mode, strings.Join(phase0Modes, ", "))
	}
	return c, sub, nil
}

func isPhase0Mode(m string) bool {
	for _, ok := range phase0Modes {
		if m == ok {
			return true
		}
	}
	return false
}
