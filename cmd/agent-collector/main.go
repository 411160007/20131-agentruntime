// agent-collector is the Phase 0 discovery collector of the 20131 Agent
// Security Runtime: it polls the local process table, identifies AI-agent
// families (Claude Code, Codex, OpenClaw, MCP servers) from fingerprints,
// and records every finding as a validated JSONL audit line with size or
// day-cut rotation. It never blocks, signals, or otherwise interferes
// with observed processes, and performs no network I/O of any kind; the
// local audit file is the only output.
//
// Control surface (read-only): the "status" and "audit-tail"
// subcommands summarize an existing audit file. The collector never
// writes anything except its own JSONL audit stream.
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
var version = "0.3.0-d3"

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
	default:
		fmt.Fprintf(os.Stderr, "agent-collector: unknown subcommand %q (want status | audit-tail)\n", sub)
		os.Exit(2)
	}
	if err != nil {
		fmt.Fprintf(os.Stderr, "agent-collector: %v\n", err)
		os.Exit(1)
	}
}

type config struct {
	showVersion bool
	out         string
	interval    time.Duration
	once        bool
	maxCycles   int
	rotateBytes int64
	rotateDaily bool
	keepHistory int
	mode        string // observe is the ONLY mode in Phase 0 (see phase0Modes)
	trust       string // user trust file for status display (never written by the collector)
	tailN       int
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
	if err := fs.Parse(args); err != nil {
		return c, sub, err
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
