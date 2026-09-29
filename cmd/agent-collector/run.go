package main

import (
	"bufio"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/signal"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"syscall"
	"time"

	"20131.com/agentruntime/internal/auditlog"
	"20131.com/agentruntime/internal/bus"
	"20131.com/agentruntime/internal/discovery"
	"20131.com/agentruntime/internal/identity"
	"20131.com/agentruntime/internal/schema"
)

// collector wires discovery snapshots to the audit log THROUGH THE EVENT
// BUS: one active source (discovery-scan), plus reserved adapter slots,
// plus tier routing into the single persistence pipe.
type collector struct {
	detector *discovery.Detector
	log      *auditlog.Log
	bus      *bus.Bus
	src      *bus.Source
	reg      *identity.Registry
	machine  string // machine fingerprint component of stable ids

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
	b := bus.New(lg)
	src, err := b.Register("discovery-scan")
	if err != nil {
		lg.Close()
		return nil, err
	}
	// Reserved adapter slots: wired in the registry, inert until their
	// slice ships; every publish attempt is rejected and counted, never
	// silently dropped.
	for _, slot := range []string{"hook", "mcp"} {
		if _, err := b.RegisterReserved(slot); err != nil {
			lg.Close()
			return nil, err
		}
	}
	runID := newRunID()
	machine, _ := identity.MachineFingerprint()
	col := &collector{
		detector: discovery.NewDetector(discovery.DefaultFingerprints()),
		log:      lg,
		bus:      b,
		src:      src,
		reg:      identity.NewRegistry(),
		machine:  machine,
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

// emitAt builds one collector event and stamps the evidence class of the
// COLLECTION MOUNT it came from: schema.SrcNativeOS for lines whose
// evidence is the OS process snapshot itself (detections, scan cycles,
// snapshot failures), schema.SrcRuntime for the collector's own
// in-process lifecycle lines (start/stop). The class is a property of
// where the event was collected, never of payload content — and nothing
// downstream reads it in this phase (gate-d7 mount-split assertion).
func (c *collector) emitAt(class schema.SourceClass, typ schema.EventType, agentID, summary string, attrs map[string]string, sev schema.Severity) error {
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
		Tier:     schema.TierL1, // observation events persist directly
		Attrs:    attrs,
	}
	e.SourceClass = class
	// The bus validates, applies the Phase 0 decision invariant, counts
	// per source, and writes through the single audit pipe.
	return c.src.Publish(e)
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

const knownGaps = "processes are discovered after launch; protected or other-user cmdlines may be unreadable (counted in coverage attrs); ETW/libproc/auditd event streams are Phase 1 scope; fingerprint misses are possible and recorded as no-agent - audit is honest, not complete; stable agent ids derive from machine fingerprint + exe-or-cmdline-or-name, so a reinstalled binary at the same locator keeps its id"

func (c *collector) emitStart(ver string) error {
	return c.emitAt(schema.SrcRuntime, schema.TypeCollectorStart, c.agentID,
		"agent-collector started: local JSONL audit only, no network, observe-only (Phase 0)",
		map[string]string{
			"version":     ver,
			"goos":        runtime.GOOS,
			"goarch":      runtime.GOARCH,
			"scan_method": scanMethod(),
			"out":         c.out,
			"known_gaps":  knownGaps,
			"self_pid":    strconv.Itoa(c.selfPID),
			"bus_sources": strings.Join(busSourceNames(c.bus), ","),
		}, schema.SevInfo)
}

func busSourceNames(b *bus.Bus) []string {
	out := []string{}
	for _, s := range b.Sources() {
		out = append(out, s.Name)
	}
	return out
}

func (c *collector) emitStop(reason string) error {
	return c.emitAt(schema.SrcRuntime, schema.TypeCollectorStop, c.agentID,
		fmt.Sprintf("agent-collector stopping after %d scan cycle(s): %s", c.cycles, reason),
		map[string]string{"run": c.runID, "uptime_s": strconv.FormatInt(int64(time.Since(c.started).Seconds()), 10)},
		schema.SevInfo)
}

// logScanFailure records a failed snapshot as an observation line — the
// collector never dies or degrades silently.
func (c *collector) logScanFailure(err error) error {
	c.cycles++
	return c.emitAt(schema.SrcNativeOS, schema.TypeAgentScan, c.agentID,
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
		// Stable identity: the same executable shape yields the same
		// agent id across scans AND across collector runs, so the audit
		// trail links sightings. pid:kind remains the per-run novelty
		// key (a relaunch of the same binary is a new process).
		aid := identity.AgentIDFor(c.machine, h.Proc)
		state := c.reg.Observe(aid, string(h.Kind))
		key := strconv.Itoa(h.Proc.PID) + ":" + string(h.Kind)
		if c.seen[key] {
			continue
		}
		c.seen[key] = true
		newHits++
		if err := c.emitDetected(h, now, aid, string(state)); err != nil {
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
	return c.emitAt(schema.SrcNativeOS, schema.TypeAgentScan, c.agentID, summary, attrs, schema.SevInfo)
}

func (c *collector) emitDetected(h discovery.Hit, now time.Time, agentID, state string) error {
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
		"identity": "sha256:machine+locator",
		"locator":  identity.LocatorLabel(h.Proc),
		"state":    state,
		"run":      c.runID,
		"observed": now.UTC().Format(time.RFC3339),
	}
	for i, r := range h.Roots {
		attrs["root"+strconv.Itoa(i)] = string(r)
	}
	return c.emitAt(schema.SrcNativeOS, schema.TypeAgentDetected, agentID, summary, attrs, schema.SevInfo)
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

// --- control surface (read-only) ---------------------------------------

// runStatus aggregates an audit file by event type, agent kind, and
// passport state. It writes to stdout only; the audit file is opened
// read-only.
func runStatus(c config) error {
	trusted := map[string]bool{}
	if c.trust != "" {
		ids, err := identity.LoadTrustFile(c.trust)
		if err != nil {
			return fmt.Errorf("trust file: %w", err)
		}
		for _, id := range ids {
			trusted[id] = true
		}
	}
	f, err := os.Open(c.out)
	if err != nil {
		return err
	}
	defer f.Close()
	s := newStatus()
	dec := newLineDecoder()
	sc := newScanner(f)
	for sc.Scan() {
		e, ok, err := dec.decode(sc.Bytes())
		if err != nil {
			return fmt.Errorf("status: %w", err)
		}
		if !ok {
			continue
		}
		state := e.Attrs["state"]
		if trusted[e.AgentID] {
			state = "trusted"
		}
		s.add(e.Type, e.Attrs["kind"], state)
	}
	if err := sc.Err(); err != nil {
		return fmt.Errorf("status: %w", err)
	}
	os.Stdout.WriteString(s.render())
	return nil
}

type statusAgg struct {
	byType  map[string]int
	byKind  map[string]int
	byState map[string]int
	lines   int
}

func newStatus() *statusAgg {
	return &statusAgg{byType: map[string]int{}, byKind: map[string]int{}, byState: map[string]int{}}
}

func (s *statusAgg) add(typ schema.EventType, kind, state string) {
	s.lines++
	s.byType[string(typ)]++
	if kind != "" {
		s.byKind[kind]++
	}
	if state != "" {
		s.byState[state]++
	}
}

func renderSection(title string, m map[string]int) string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	var b strings.Builder
	fmt.Fprintf(&b, "%s\n", title)
	for _, k := range keys {
		fmt.Fprintf(&b, "  %s %d\n", k, m[k])
	}
	return b.String()
}

func (s *statusAgg) render() string {
	var b strings.Builder
	fmt.Fprintf(&b, "status: %d event lines\n", s.lines)
	b.WriteString(renderSection("by type:", s.byType))
	b.WriteString(renderSection("by kind:", s.byKind))
	b.WriteString(renderSection("by state:", s.byState))
	return b.String()
}

// runTail prints the trailing N lines of the audit file verbatim
// (read-only; rotated history untouched; no formatting, so operators can
// pipe it straight into validate-jsonl.mjs).
func runTail(c config) error {
	if c.tailN <= 0 {
		return fmt.Errorf("audit-tail: -n must be > 0, got %d", c.tailN)
	}
	lines, err := tailLines(c.out, c.tailN)
	if err != nil {
		return err
	}
	for _, ln := range lines {
		fmt.Println(ln)
	}
	return nil
}

func tailLines(path string, n int) ([]string, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	var out []string
	dec := newLineDecoder()
	sc := newScanner(f)
	for sc.Scan() {
		// Validate every line we intend to print: the control surface
		// never echoes corrupt records as if they were audit truth.
		_, ok, err := dec.decode(sc.Bytes())
		if err != nil {
			return nil, fmt.Errorf("audit-tail: %w", err)
		}
		if !ok {
			continue
		}
		out = append(out, sc.Text())
		if len(out) > n {
			out = out[1:]
		}
	}
	if err := sc.Err(); err != nil {
		return nil, err
	}
	return out, nil
}

// newScanner builds a line scanner with an audit-sized buffer (a single
// JSONL line may hold a long redacted cmdline, never more than 1 MiB).
func newScanner(r io.Reader) *bufio.Scanner {
	sc := bufio.NewScanner(r)
	sc.Buffer(make([]byte, 0, 64*1024), 1024*1024)
	return sc
}

// lineDecoder re-validates JSONL lines so callers never echo corrupt
// records as audit truth.
func newLineDecoder() *lineDecoder { return &lineDecoder{} }

type lineDecoder struct{}

func (d *lineDecoder) decode(b []byte) (schema.Event, bool, error) {
	t := strings.TrimSpace(string(b))
	if t == "" {
		return schema.Event{}, false, nil
	}
	var e schema.Event
	if err := json.Unmarshal([]byte(t), &e); err != nil {
		return schema.Event{}, false, fmt.Errorf("invalid JSONL line: %w", err)
	}
	if err := e.Validate(); err != nil {
		return schema.Event{}, false, fmt.Errorf("invalid event: %w", err)
	}
	return e, true, nil
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
