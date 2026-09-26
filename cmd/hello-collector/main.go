// hello-collector is the Phase 0 skeleton collector of the 20131 Agent
// Security Runtime: it demonstrates the core event model end to end and
// writes a local JSONL audit file. It performs no network I/O of any kind;
// the audit file on the local disk is the only output.
package main

import (
	"flag"
	"fmt"
	"os"
	"runtime"

	"20131.com/agentruntime/internal/auditlog"
	"20131.com/agentruntime/internal/policy"
	"20131.com/agentruntime/internal/schema"
)

// version is overridden at build time via -ldflags "-X main.version=...".
var version = "0.1.0-d1"

func currentPlatform() schema.Platform { return schema.Platform(runtime.GOOS) }

func main() {
	var (
		showVersion = flag.Bool("version", false, "print version and exit")
		out         = flag.String("out", "agent-events.jsonl", "path of the local JSONL audit file (created with 0600)")
		name        = flag.String("agent-name", "hello-collector", "agent display name recorded in the demo events")
	)
	flag.Parse()

	if *showVersion {
		fmt.Printf("hello-collector %s %s/%s (%s)\n", version, runtime.GOOS, runtime.GOARCH, runtime.Version())
		return
	}

	ev, err := policy.New(DemoPolicy())
	if err != nil {
		fmt.Fprintf(os.Stderr, "hello-collector: demo policy invalid: %v\n", err)
		os.Exit(1)
	}
	events, err := PipelineEvents(DemoAgent(*name), ev)
	if err != nil {
		fmt.Fprintf(os.Stderr, "hello-collector: pipeline construction failed: %v\n", err)
		os.Exit(1)
	}

	w, err := auditlog.Open(*out)
	if err != nil {
		fmt.Fprintf(os.Stderr, "hello-collector: %v\n", err)
		os.Exit(1)
	}
	defer w.Close()

	for _, e := range events {
		if err := w.WriteEvent(e); err != nil {
			fmt.Fprintf(os.Stderr, "hello-collector: write event %s: %v\n", e.ID, err)
			os.Exit(1)
		}
	}
	fmt.Printf("hello-collector: wrote %d events (%d bytes) to %s\n", w.Count(), w.Bytes(), w.Path())
}
