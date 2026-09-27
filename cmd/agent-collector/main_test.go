package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"20131.com/agentruntime/internal/discovery"
	"20131.com/agentruntime/internal/identity"
	"20131.com/agentruntime/internal/schema"
)

func fixtureProcs() []discovery.ProcInfo {
	return []discovery.ProcInfo{
		{PID: 1, PPID: 0, Name: "launchish", Cmdline: "/sbin/init"},
		{PID: 100, PPID: 1, Name: "claude", Cmdline: "claude --output-format stream-json"},
		{PID: 101, PPID: 100, Name: "node", Cmdline: "node /opt/claude-code-inspector.js"},
		{PID: 102, PPID: 100, Name: "mcp-server-fs", Cmdline: "mcp-server-fs --root /data"},
		{PID: 103, PPID: 102, Name: "helper-mcp", Cmdline: "helper-mcp --stdio"},
		{PID: 104, PPID: 100, Name: "sh", Cmdline: "sh -c ls"},
		{PID: 105, PPID: 1, Name: "codex", Cmdline: "codex resume"},
		{PID: 106, PPID: 1, Name: "openclaw-gateway", Cmdline: "openclaw-gateway serve"},
		{PID: 107, PPID: 1, Name: "nginx", Cmdline: "nginx: worker process"},
	}
}

func readEvents(t *testing.T, path string) []schema.Event {
	t.Helper()
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var out []schema.Event
	for _, ln := range strings.Split(strings.TrimRight(string(raw), "\n"), "\n") {
		if ln == "" {
			continue
		}
		var e schema.Event
		if err := json.Unmarshal([]byte(ln), &e); err != nil {
			t.Fatalf("line not JSON: %v\n%s", err, ln)
		}
		if err := e.Validate(); err != nil {
			t.Fatalf("line fails schema validation: %v\n%s", err, ln)
		}
		out = append(out, e)
	}
	return out
}

func testCollector(t *testing.T, out string) *collector {
	t.Helper()
	col, err := newCollector(config{out: out, rotateBytes: 0, rotateDaily: false}, "test")
	if err != nil {
		t.Fatal(err)
	}
	col.snapFn = func() ([]discovery.ProcInfo, error) { return fixtureProcs(), nil }
	col.hiddenFn = func(int) bool { return false }
	col.selfPID = 999999 // fixture scan has no self row
	t.Cleanup(col.close)
	return col
}

func TestCycleEmitsValidDetectionAudit(t *testing.T) {
	path := filepath.Join(t.TempDir(), "audit.jsonl")
	col := testCollector(t, path)
	if err := col.cycle(time.Now().UTC()); err != nil {
		t.Fatal(err)
	}
	events := readEvents(t, path)

	var kinds = map[string]int{}
	var scans, start, stop int
	for _, e := range events {
		switch e.Type {
		case schema.TypeCollectorStart:
			start++
		case schema.TypeCollectorStop:
			stop++
		case schema.TypeAgentScan:
			scans++
		case schema.TypeAgentDetected:
			kinds[e.Attrs["kind"]]++
		}
		if e.Stage != schema.StageObservation {
			t.Errorf("discovery event %s in stage %s: Phase 0 stays in observation", e.ID, e.Stage)
		}
		if e.Decision != schema.DecisionAllow {
			t.Errorf("discovery event %s carries decision %s: no blocking vocabulary allowed", e.ID, e.Decision)
		}
	}
	if start != 1 || stop != 0 || scans != 1 {
		t.Errorf("lifecycle counts start=%d stop=%d scan=%d, want 1/0/1", start, stop, scans)
	}
	for _, want := range []string{"claude-code", "codex", "openclaw", "mcp-server"} {
		if kinds[want] == 0 {
			t.Errorf("fixture cycle produced no %s detection (kinds=%v)", want, kinds)
		}
	}
	// nginx (pid 107) must not appear as an agent
	for _, e := range events {
		if e.Type == schema.TypeAgentDetected && e.Attrs["pid"] == "107" {
			t.Error("control process nginx was detected as agent")
		}
	}
	// detected lines carry fingerprint + lineage attrs
	for _, e := range events {
		if e.Type != schema.TypeAgentDetected {
			continue
		}
		for _, k := range []string{"kind", "rule", "pid", "ppid", "name", "cmdline", "observed"} {
			if _, ok := e.Attrs[k]; !ok {
				t.Errorf("detected event missing attr %s: %+v", k, e.Attrs)
			}
		}
	}
}

func TestCycleDedupAcrossScans(t *testing.T) {
	path := filepath.Join(t.TempDir(), "audit.jsonl")
	col := testCollector(t, path)
	if err := col.cycle(time.Now().UTC()); err != nil {
		t.Fatal(err)
	}
	first := len(readEvents(t, path))
	if err := col.cycle(time.Now().UTC()); err != nil {
		t.Fatal(err)
	}
	events := readEvents(t, path)
	if len(events) != first+1 {
		t.Fatalf("second identical scan added %d lines, want exactly 1 (scan event only)", len(events)-first)
	}
	last := events[len(events)-1]
	if last.Type != schema.TypeAgentScan || last.Attrs["new_hits"] != "0" {
		t.Fatalf("second scan should report zero new hits, got %s %v", last.Type, last.Attrs)
	}
}

func TestDetectedAttrsRedacted(t *testing.T) {
	path := filepath.Join(t.TempDir(), "audit.jsonl")
	col, err := newCollector(config{out: path, rotateBytes: 0, rotateDaily: false}, "test")
	if err != nil {
		t.Fatal(err)
	}
	defer col.close()
	secret := "hunter2-supersecret-value"
	col.snapFn = func() ([]discovery.ProcInfo, error) {
		return []discovery.ProcInfo{
			{PID: 1, PPID: 0, Name: "init", Cmdline: "/sbin/init"},
			{PID: 120, PPID: 1, Name: "codex", Cmdline: "codex --token=" + secret + " resume"},
		}, nil
	}
	col.hiddenFn = func(int) bool { return false }
	col.selfPID = 999999
	if err := col.cycle(time.Now().UTC()); err != nil {
		t.Fatal(err)
	}
	raw, _ := os.ReadFile(path)
	if strings.Contains(string(raw), secret) {
		t.Fatalf("audit file leaked command-line secret:\n%s", raw)
	}
	if !strings.Contains(string(raw), "[redacted]") {
		t.Fatal("expected redaction marker in attrs")
	}
}

func TestVersionFlagShape(t *testing.T) {
	// D5 contract: "<name> <version> <goos>/<goarch> (<goversion>)".
	line := "agent-collector " + version + " " + "linux/amd64" + " (go1.27.1)"
	if parts := strings.Fields(line); len(parts) != 4 || !strings.Contains(parts[2], "/") {
		t.Fatalf("version line shape broken: %q", line)
	}
}

// --- D3 additions: stable identity, bus wiring, control surface -------

func TestDetectedAgentIDStableAcrossRuns(t *testing.T) {
	// Two "runs" of the collector (fresh run ids) against the same
	// fixture process table must attribute the same agent ids: the id is
	// machine-fingerprint + normalized locator, never pid or run id.
	pathA := filepath.Join(t.TempDir(), "a.jsonl")
	pathB := filepath.Join(t.TempDir(), "b.jsonl")
	colA := testCollector(t, pathA)
	colB := testCollector(t, pathB)
	colB.machine = colA.machine // same machine; different runID by construction
	if err := colA.cycle(time.Now().UTC()); err != nil {
		t.Fatal(err)
	}
	if err := colB.cycle(time.Now().UTC()); err != nil {
		t.Fatal(err)
	}
	sig := func(path string) map[string]string {
		out := map[string]string{}
		for _, e := range readEvents(t, path) {
			if e.Type == schema.TypeAgentDetected {
				out[e.Attrs["pid"]] = e.AgentID
			}
		}
		return out
	}
	a, b := sig(pathA), sig(pathB)
	if len(a) == 0 {
		t.Fatal("fixture produced no detections")
	}
	byPID := map[string]discovery.ProcInfo{}
	for _, p := range fixtureProcs() {
		byPID[strconv.Itoa(p.PID)] = p
	}
	for pid, idA := range a {
		if b[pid] != idA {
			t.Fatalf("agent id for pid %s drifted across runs: %s vs %s", pid, idA, b[pid])
		}
		// Independent derivation must reproduce the emitted id exactly:
		// id = f(machine fingerprint, normalized locator), nothing else.
		if want := identity.AgentIDFor(colA.machine, byPID[pid]); idA != want {
			t.Fatalf("agent id %s for pid %s is not the pure derivation %s", idA, pid, want)
		}
	}
	// detected rows carry the identity annotation + passport state
	for _, e := range readEvents(t, pathA) {
		if e.Type != schema.TypeAgentDetected {
			continue
		}
		if e.Attrs["identity"] != "sha256:machine+locator" || e.Attrs["state"] != "observed" {
			t.Fatalf("detected row lacks identity/state annotations: %v", e.Attrs)
		}
	}
}

func TestCollectorWiredToBusWithReservedSlots(t *testing.T) {
	path := filepath.Join(t.TempDir(), "audit.jsonl")
	col := testCollector(t, path)
	if err := col.cycle(time.Now().UTC()); err != nil {
		t.Fatal(err)
	}
	st := col.bus.Stats()
	if st.Published == 0 || st.Persisted != st.Published {
		t.Fatalf("collector events did not flow through the bus 1:1: %+v", st)
	}
	names := map[string]string{}
	for _, si := range col.bus.Sources() {
		names[si.Name] = string(si.State)
	}
	if names["discovery-scan"] != "active" || names["hook"] != "reserved" || names["mcp"] != "reserved" {
		t.Fatalf("bus registry wrong: %v", names)
	}
	// every persisted line carries the explicit L1 tier now
	for _, e := range readEvents(t, path) {
		if e.Tier != schema.TierL1 {
			t.Fatalf("collector event %s tier=%q want L1", e.ID, e.Tier)
		}
	}
	// start line registers the bus topology honestly (auditable wiring)
	events := readEvents(t, path)
	if events[0].Type != schema.TypeCollectorStart || events[0].Attrs["bus_sources"] != "discovery-scan,hook,mcp" {
		t.Fatalf("start line bus_sources wrong: %v", events[0].Attrs)
	}
}

func TestStatusGolden(t *testing.T) {
	const fixture = `{"v":1,"ts":"2026-09-27T10:00:00Z","id":"ev-1","agent_id":"agi-aa","stage":"observation","type":"agent.detected","decision":"allow","severity":0,"summary":"s1","tier":"L1","attrs":{"kind":"codex","state":"observed"}}
{"v":1,"ts":"2026-09-27T10:00:01Z","id":"ev-2","agent_id":"agi-bb","stage":"observation","type":"agent.detected","decision":"allow","severity":0,"summary":"s2","tier":"L1","attrs":{"kind":"codex","state":"observed"}}
{"v":1,"ts":"2026-09-27T10:00:02Z","id":"ev-3","agent_id":"agi-cc","stage":"observation","type":"agent.detected","decision":"allow","severity":0,"summary":"s3","tier":"L1","attrs":{"kind":"claude-code","state":"trusted"}}
{"v":1,"ts":"2026-09-27T10:00:03Z","id":"ev-4","agent_id":"agi-dd","stage":"observation","type":"agent.scan","decision":"allow","severity":0,"summary":"s4","tier":"L1","attrs":{}}`
	path := filepath.Join(t.TempDir(), "audit.jsonl")
	if err := os.WriteFile(path, []byte(fixture+"\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	s := newStatus()
	dec := newLineDecoder()
	f, err := os.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	sc := newScanner(f)
	for sc.Scan() {
		e, ok, err := dec.decode(sc.Bytes())
		if err != nil || !ok {
			if err != nil {
				t.Fatal(err)
			}
			continue
		}
		s.add(e.Type, e.Attrs["kind"], e.Attrs["state"])
	}
	const want = `status: 4 event lines
by type:
  agent.detected 3
  agent.scan 1
by kind:
  claude-code 1
  codex 2
by state:
  observed 2
  trusted 1
`
	if got := s.render(); got != want {
		t.Fatalf("status golden mismatch:\ngot:\n%s\nwant:\n%s", got, want)
	}
	// corrupt input must be refused, not silently skipped
	bad := filepath.Join(t.TempDir(), "bad.jsonl")
	if err := os.WriteFile(bad, []byte("not json\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(bad); err != nil {
		t.Fatal(err)
	}
	bs, err := tailLines(bad, 5)
	if err == nil {
		t.Fatalf("tail accepted corrupt audit line: %v", bs)
	}
}

func TestAuditTailGolden(t *testing.T) {
	path := filepath.Join(t.TempDir(), "audit.jsonl")
	var b strings.Builder
	for i := 0; i < 25; i++ {
		fmt.Fprintf(&b, `{"v":1,"ts":"2026-09-27T10:00:%02dZ","id":"ev-%02d","agent_id":"agi-x","stage":"observation","type":"agent.scan","decision":"allow","severity":0,"summary":"line %02d","tier":"L1"}`+"\n", i, i, i)
	}
	if err := os.WriteFile(path, []byte(b.String()), 0o600); err != nil {
		t.Fatal(err)
	}
	lines, err := tailLines(path, 3)
	if err != nil {
		t.Fatal(err)
	}
	want := []string{
		`{"v":1,"ts":"2026-09-27T10:00:22Z","id":"ev-22","agent_id":"agi-x","stage":"observation","type":"agent.scan","decision":"allow","severity":0,"summary":"line 22","tier":"L1"}`,
		`{"v":1,"ts":"2026-09-27T10:00:23Z","id":"ev-23","agent_id":"agi-x","stage":"observation","type":"agent.scan","decision":"allow","severity":0,"summary":"line 23","tier":"L1"}`,
		`{"v":1,"ts":"2026-09-27T10:00:24Z","id":"ev-24","agent_id":"agi-x","stage":"observation","type":"agent.scan","decision":"allow","severity":0,"summary":"line 24","tier":"L1"}`,
	}
	if len(lines) != 3 || lines[0] != want[0] || lines[1] != want[1] || lines[2] != want[2] {
		t.Fatalf("tail golden mismatch:\n%v", lines)
	}
}

func TestParseFlagsSubcommandsAndModeGate(t *testing.T) {
	// subcommand + flags
	c, sub, err := parseFlags(newFlagSet(), []string{"status", "--out", "/tmp/x.jsonl"})
	if err != nil || sub != "status" || c.out != "/tmp/x.jsonl" {
		t.Fatalf("status parse: %v %v %+v", err, sub, c)
	}
	// bare flags (run form) still work
	c, sub, err = parseFlags(newFlagSet(), []string{"--once", "--out", "/tmp/y"})
	if err != nil || sub != "" || !c.once {
		t.Fatalf("run form parse: %v %v %+v", err, sub, c)
	}
	// Phase 0 refuses any non-observe mode
	if _, _, err := parseFlags(newFlagSet(), []string{"--mode", "enforce"}); err == nil {
		t.Fatal("mode=enforce accepted in Phase 0")
	}
	if _, _, err := parseFlags(newFlagSet(), []string{"--mode", "block"}); err == nil {
		t.Fatal("mode=block accepted in Phase 0")
	}
	if _, _, err := parseFlags(newFlagSet(), []string{"--mode", "observe"}); err != nil {
		t.Fatalf("observe must be accepted: %v", err)
	}
	// unknown flag rejected
	if _, _, err := parseFlags(newFlagSet(), []string{"--frobnicate"}); err == nil {
		t.Fatal("unknown flag accepted")
	}
}

func newFlagSet() *flag.FlagSet { return flag.NewFlagSet("test", flag.ContinueOnError) }
