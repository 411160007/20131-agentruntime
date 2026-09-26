package main

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"os"
	"os/signal"
	"runtime"
	"strconv"
	"syscall"
	"time"

	"20131.com/agentruntime/internal/auditlog"
	"20131.com/agentruntime/internal/discovery"
	"20131.com/agentruntime/internal/schema"
)

// collector wires discovery snapshots to the rotating audit log.
type collector struct {
	detector *discovery.Detector
	log      *auditlog.Log

	runID    string
	seq      int
	seen     map[string]bool
	selfPID  int
	cycles   int
	agentID  string
	started  time.Time
	out      string
	snapFn   func() ([]discovery.ProcInfo, error)
	hiddenFn func(int) bool
}

func newCollector(c config, ver string) (*collector, error) {
	rot := &auditlog.Rotation{MaxBytes: c.rotateBytes, Daily: c.rotateDaily, KeepHistory: c.keepHistory}
	if c.rotateBytes == 0 && !c.rotateDaily && c.keepHistory == 0 {
		rot = nil // explicit "no rotation" keeps OpenLog identical to the D1 writer
	}
	lg, err := auditlog.OpenLog(c.out, rot)
	if err != nil {
		return nil, err
	}
	runID := newRunID()
	col := &collector{
		detector: discovery.NewDetector(discovery.DefaultFingerprints()),
		log:      lg,
		runID:    runID,
		seen:     map[string]bool{},
		selfPID:  os.Getpid(),
		agentID:  "collector-" + runID,
		started:  time.Now().UTC(),
		out:      c.out,
		snapFn:   discovery.Snapshot,
		hiddenFn: discovery.HiddenFromUs,
	}
	if err := col.emitStart(ver); err != nil {
		lg.Close()
		return nil, fmt.Errorf("start event: %w", err)
	}
	return col, nil
}

func newRunID() string {
	b := make([]byte, 4)
	if _, err := rand.Read(b); err != nil {
		return strconv.FormatInt(time.Now().UnixNano()%(1<<32), 16)
	}
	return hex.EncodeToString(b)
}

func (c *collector) nextID() string {
	c.seq++
	return fmt.Sprintf("ev-%s-%05d", c.runID, c.seq)
}

func (c *collector) emit(typ schema.EventType, agentID, summary string, attrs map[string]string, sev schema.Severity) error {
	e := &schema.Event{
		V:        schema.SchemaVersion,
		TS:       time.Now().UTC(),
		ID:       c.nextID(),
		AgentID:  agentID,
		Stage:    schema.StageObservation,
		Type:     typ,
		Decision: schema.DecisionAllow, // Phase 0: discovery records only; no blocking vocabulary is emitted
		Severity: sev,
		Summary:  summary,
		Attrs:    attrs,
	}
	return c.log.WriteEvent(e)
}

func scanMethod() string {
	switch runtime.GOOS {
	case "linux":
		return "procfs-poll"
	case "darwin":
		return "ps-poll" // libproc upgrade path noted in scan_darwin.go
	case "windows":
		return "toolhelp32+peb-read" // ETW streaming is the Phase 1 upgrade
	default:
		return "unsupported"
	}
}

const knownGaps = "processes are discovered after launch; protected or other-user cmdlines may be unreadable (counted in coverage attrs); ETW/libproc/auditd event streams are Phase 1 scope; fingerprint misses are possible and recorded as no-agent - audit is honest, not complete"

func (c *collector) emitStart(ver string) error {
	return c.emit(schema.TypeCollectorStart, c.agentID,
		"agent-collector started: local JSONL audit only, no network, observe-only (Phase 0)",
		map[string]string{
			"version":     ver,
			"goos":        runtime.GOOS,
			"goarch":      runtime.GOARCH,
			"scan_method": scanMethod(),
			"out":         c.out,
			"known_gaps":  knownGaps,
			"self_pid":    strconv.Itoa(c.selfPID),
		}, schema.SevInfo)
}

func (c *collector) emitStop(reason string) error {
	return c.emit(schema.TypeCollectorStop, c.agentID,
		fmt.Sprintf("agent-collector stopping after %d scan cycle(s): %s", c.cycles, reason),
		map[string]string{"run": c.runID, "uptime_s": strconv.FormatInt(int64(time.Since(c.started).Seconds()), 10)},
		schema.SevInfo)
}

// logScanFailure records a failed snapshot as an observation line — the
// collector never dies or degrades silently.
func (c *collector) logScanFailure(err error) error {
	c.cycles++
	return c.emit(schema.TypeAgentScan, c.agentID,
		"scan cycle failed: "+discovery.TruncateRedacted(err.Error(), 200),
		map[string]string{"run": c.runID, "status": "snapshot-error"}, schema.SevLow)
}

// cycle performs one scan and emits detected + scan events. OS contact is
// injected through snapFn/hiddenFn so tests can drive it with fixture
// trees on every platform.
func (c *collector) cycle(now time.Time) error {
	procs, err := c.snapFn()
	if err != nil {
		return fmt.Errorf("snapshot: %w", err)
	}
	c.cycles++
	res := c.detector.Scan(procs, c.selfPID)

	hidden := 0
	for _, p := range procs {
		if p.Cmdline == "" && c.hiddenFn(p.PID) {
			hidden++
		}
	}

	newHits := 0
	for _, h := range res.Hits {
		key := strconv.Itoa(h.Proc.PID) + ":" + string(h.Kind)
		if c.seen[key] {
			continue
		}
		c.seen[key] = true
		newHits++
		if err := c.emitDetected(h, now); err != nil {
			return err
		}
	}

	summary := fmt.Sprintf("scan cycle %d: %d processes, %d agent rows (%d new), %d family members",
		c.cycles, res.Stats.Procs, len(res.Hits), newHits, res.Stats.Attributed)
	attrs := map[string]string{
		"procs":      strconv.Itoa(res.Stats.Procs),
		"hits":       strconv.Itoa(len(res.Hits)),
		"new_hits":   strconv.Itoa(newHits),
		"attributed": strconv.Itoa(res.Stats.Attributed),
		"init_trunc": strconv.Itoa(res.Stats.TruncPPID1),
		"no_cmdline": strconv.Itoa(res.Stats.NoCmdline),
		"hidden":     strconv.Itoa(hidden),
		"mode":       "observe",
		"run":        c.runID,
		"orphan_mcp": strconv.Itoa(len(res.MCPSiblings)),
	}
	return c.emit(schema.TypeAgentScan, c.agentID, summary, attrs, schema.SevInfo)
}

func (c *collector) emitDetected(h discovery.Hit, now time.Time) error {
	agentID := fmt.Sprintf("agi-%d-%s", h.Proc.PID, c.runID)
	name := discovery.TruncateRedacted(h.Proc.Name, 96)
	summary := fmt.Sprintf("detected %s agent: %s (pid %d, rule %s)", h.Kind, name, h.Proc.PID, h.Rule)
	attrs := map[string]string{
		"kind":     string(h.Kind),
		"rule":     h.Rule,
		"pid":      strconv.Itoa(h.Proc.PID),
		"ppid":     strconv.Itoa(h.Proc.PPID),
		"name":     name,
		"exe":      discovery.TruncateRedacted(h.Proc.Exe, 256),
		"cmdline":  discovery.TruncateRedacted(h.Proc.Cmdline, 320),
		"run":      c.runID,
		"observed": now.UTC().Format(time.RFC3339),
	}
	for i, r := range h.Roots {
		attrs["root"+strconv.Itoa(i)] = string(r)
	}
	return c.emit(schema.TypeAgentDetected, agentID, summary, attrs, schema.SevInfo)
}

func (c *collector) close() {
	if err := c.log.Close(); err != nil {
		fmt.Fprintf(os.Stderr, "agent-collector: close: %v\n", err)
	}
}

func watchSignals(stop func()) {
	ch := make(chan os.Signal, 1)
	signal.Notify(ch, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		<-ch
		stop()
	}()
}
