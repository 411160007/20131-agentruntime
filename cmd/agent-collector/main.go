// agent-collector is the Phase 0 discovery collector of the 20131 Agent
// Security Runtime: it polls the local process table, identifies AI-agent
// families (Claude Code, Codex, OpenClaw, MCP servers) from fingerprints,
// and records every finding as a validated JSONL audit line with size or
// day-cut rotation. It never blocks, signals, or otherwise interferes
// with observed processes, and performs no network I/O of any kind; the
// local audit file is the only output.
package main

import (
	"flag"
	"fmt"
	"os"
	"runtime"
	"time"
)

// version is overridden at build time via -ldflags "-X main.version=...".
var version = "0.2.0-d2"

func main() {
	cfg := parseFlags(flag.CommandLine, os.Args[1:])
	if cfg.showVersion {
		fmt.Printf("agent-collector %s %s/%s (%s)\n", version, runtime.GOOS, runtime.GOARCH, runtime.Version())
		return
	}
	if err := run(cfg); err != nil {
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
}

func parseFlags(fs *flag.FlagSet, args []string) config {
	var c config
	fs.BoolVar(&c.showVersion, "version", false, "print version and exit")
	fs.StringVar(&c.out, "out", "agent-audit.jsonl", "path of the local JSONL audit file (created 0600; rotated segments get a UTC-stamp suffix)")
	fs.DurationVar(&c.interval, "interval", 30*time.Second, "polling interval between scans")
	fs.BoolVar(&c.once, "once", false, "run exactly one scan cycle and exit")
	fs.IntVar(&c.maxCycles, "max-cycles", 0, "stop after N scan cycles (0 = run until signaled)")
	fs.Int64Var(&c.rotateBytes, "rotate-bytes", 10<<20, "rotate the audit file once it reaches this many bytes (0 = size rotation off)")
	fs.BoolVar(&c.rotateDaily, "rotate-daily", true, "rotate the audit file at the UTC day boundary")
	fs.IntVar(&c.keepHistory, "keep-history", 0, "keep at most N rotated segments (0 = keep all)")
	_ = fs.Parse(args)
	return c
}

func run(c config) error {
	col, err := newCollector(c, version)
	if err != nil {
		return err
	}
	defer col.close()

	interval := c.interval
	if interval < 50*time.Millisecond {
		interval = 50 * time.Millisecond
	}
	maxCycles := c.maxCycles
	if c.once {
		maxCycles = 1
	}

	stopCh := make(chan struct{}, 1)
	watchSignals(func() {
		select {
		case stopCh <- struct{}{}:
		default:
		}
	})

	for cycle := 0; ; cycle++ {
		if err := col.cycle(time.Now().UTC()); err != nil {
			// A failed snapshot is recorded; the poll continues unless the
			// audit write itself is broken (then we must not run blind).
			if lerr := col.logScanFailure(err); lerr != nil {
				_ = col.emitStop("audit write failure")
				return fmt.Errorf("cycle %d: %v (and audit write failed: %w)", col.cycles, err, lerr)
			}
		}
		if maxCycles > 0 && cycle+1 >= maxCycles {
			return col.emitStop("scan budget reached")
		}
		select {
		case <-time.After(interval):
		case <-stopCh:
			return col.emitStop("interrupted")
		}
	}
}
