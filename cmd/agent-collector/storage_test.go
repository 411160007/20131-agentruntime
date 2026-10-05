package main

import (
	"crypto/sha256"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
	"time"

	"20131.com/agentruntime/internal/auditlog"
	"20131.com/agentruntime/internal/schema"
)

// sevNames indexes the fixture severities by position.
var sevNames = []schema.Severity{schema.SevInfo, schema.SevMedium, schema.SevCritical}

// stev builds one valid event with a stated agent and optional stated task.
func stev(id, agent, task string, ts time.Time, sev schema.Severity) *schema.Event {
	e := &schema.Event{
		V:        schema.SchemaVersion,
		TS:       ts.UTC(),
		ID:       id,
		AgentID:  agent,
		Stage:    schema.StageObservation,
		Type:     schema.TypeAgentScan,
		Decision: schema.DecisionAllow,
		Severity: sev,
		Summary:  "storage fixture event",
	}
	if task != "" {
		e.Attrs = map[string]string{auditlog.QuotaTaskAttrKey: task}
	}
	return e
}

// writeStorFamily lays down a real segment family (live head plus at
// least one rotated segment) and returns the live path.
func writeStorFamily(t *testing.T, dir string, count int) string {
	t.Helper()
	path := filepath.Join(dir, "agent-audit.jsonl")
	log, err := auditlog.OpenLog(path, &auditlog.Rotation{MaxBytes: 320})
	if err != nil {
		t.Fatalf("open fixture log: %v", err)
	}
	base := time.Date(2026, 10, 6, 4, 0, 0, 0, time.UTC)
	for i := 0; i < count; i++ {
		agent := fmt.Sprintf("claude-code-%d", i%2)
		task := ""
		if i%3 == 0 {
			task = fmt.Sprintf("task-%d", i%2)
		}
		if err := log.WriteEvent(stev(fmt.Sprintf("st-%04d", i), agent, task, base.Add(time.Duration(i)*time.Second), sevNames[i%len(sevNames)])); err != nil {
			t.Fatalf("fixture write %d: %v", i, err)
		}
	}
	if err := log.Close(); err != nil {
		t.Fatalf("fixture close: %v", err)
	}
	return path
}

// duSumWalk is the independent ground truth: every regular file in the
// directory, summed by stat. Nothing about it is shared with the census.
func duSumWalk(t *testing.T, dir string) int64 {
	t.Helper()
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("readdir: %v", err)
	}
	var sum int64
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		st, err := e.Info()
		if err != nil {
			t.Fatalf("entry info: %v", err)
		}
		sum += st.Size()
	}
	return sum
}

func lineMap(t *testing.T, lines []auditlog.StorageLine) map[string]auditlog.StorageLine {
	t.Helper()
	m := map[string]auditlog.StorageLine{}
	for _, ln := range lines {
		if ln.Member == "scope" || ln.Member == "known_gap" {
			continue
		}
		if _, dup := m[ln.Member]; dup {
			t.Fatalf("member %q emitted twice", ln.Member)
		}
		m[ln.Member] = ln
	}
	return m
}

// TestStorageCensusMatchesDiskSum is the du cross-check: the census
// numbers must equal an independently walked disk sum, never a
// hand-stated figure.
func TestStorageCensusMatchesDiskSum(t *testing.T) {
	dir := t.TempDir()
	path := writeStorFamily(t, dir, 9)
	want := duSumWalk(t, dir)
	lines, err := auditlog.StorageCensus(path, 0, 0, 0, time.Now())
	if err != nil {
		t.Fatalf("census: %v", err)
	}
	m := lineMap(t, lines)
	for _, member := range []string{"audit", "total"} {
		got := m[member].Value
		if wantStr := fmt.Sprintf("%d bytes", want); got != wantStr {
			t.Fatalf("%s = %q, want %q (du walk)", member, got, wantStr)
		}
	}
	// The rotated segment count stated in the detail must match the
	// family actually on disk.
	segs := 0
	entries, _ := os.ReadDir(dir)
	for _, e := range entries {
		if e.Name() != filepath.Base(path) {
			segs++
		}
	}
	if !strings.Contains(m["audit"].Detail, fmt.Sprintf("%d rotated segment(s)", segs)) {
		t.Fatalf("audit detail %q does not state %d rotated segment(s)", m["audit"].Detail, segs)
	}
}

// TestStorageGoldenRenderedLines pins the display shape verbatim over a
// synthetic census (numbers authored here, so the snapshot is a shape
// pin, not a transcription of disk state).
func TestStorageGoldenRenderedLines(t *testing.T) {
	in := []auditlog.StorageLine{
		{Member: "runtime_logs", Value: auditlog.StorageViewAbsent, Detail: "no separate runtime log store exists in this release; the audit file is the only persistence exit"},
		{Member: "audit", Value: "4096 bytes", Detail: "live segment agent-audit.jsonl plus 2 rotated segment(s)"},
		{Member: "recovery", Value: auditlog.StorageViewAbsent, Detail: "recovery records are a classification plane carried on audit lines; no separately measured recovery store exists"},
		{Member: "evidence", Value: auditlog.StorageViewAbsent, Detail: "evidence bundles are written only into an operator-declared existing directory; no default evidence store path exists"},
		{Member: "total", Value: "4096 bytes", Detail: "sum over the sources that exist; absent members are stated, never counted as zero"},
		{Member: "quota", Value: "10000 bytes", Detail: "capacity ceiling declared"},
		{Member: "retention", Value: "info=7d low=7d medium=30d high=90d critical=180d", Detail: "windows derived from the shipped per-class constants (overrides are not a display-side concept)"},
		{Member: "pressure_status", Value: "normal", Detail: "4096 of 10000 bytes used; response stays " + auditlog.StoragePressureResponse},
		{Member: "scope_census", Value: "1 scope record(s)", Detail: auditlog.QuotaDualDimensionRule},
		{Member: "scope", Value: "agent log_bytes=claude-code-0 1024", Detail: "ceiling 2048 state below_ceiling response " + auditlog.QuotaResponseRule},
	}
	got := renderStorage(in)
	want := []string{
		"storage: read-only occupancy census",
		"  runtime_logs   " + auditlog.StorageViewAbsent + " | no separate runtime log store exists in this release; the audit file is the only persistence exit",
		"  audit          4096 bytes | live segment agent-audit.jsonl plus 2 rotated segment(s)",
		"  recovery       " + auditlog.StorageViewAbsent + " | recovery records are a classification plane carried on audit lines; no separately measured recovery store exists",
		"  evidence       " + auditlog.StorageViewAbsent + " | evidence bundles are written only into an operator-declared existing directory; no default evidence store path exists",
		"  total          4096 bytes | sum over the sources that exist; absent members are stated, never counted as zero",
		"  quota          10000 bytes | capacity ceiling declared",
		"  retention      info=7d low=7d medium=30d high=90d critical=180d | windows derived from the shipped per-class constants (overrides are not a display-side concept)",
		"  pressure_status normal | 4096 of 10000 bytes used; response stays " + auditlog.StoragePressureResponse,
		"  scope_census   1 scope record(s) | " + auditlog.QuotaDualDimensionRule,
		"  scope          agent log_bytes=claude-code-0 1024 | ceiling 2048 state below_ceiling response " + auditlog.QuotaResponseRule,
	}
	if len(got) != len(want) {
		t.Fatalf("rendered %d lines, want %d:\n%s", len(got), len(want), strings.Join(got, "\n"))
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("line %d drifted:\n got %q\nwant %q", i, got[i], want[i])
		}
	}
	// A line with no detail renders without the separator.
	bare := renderStorage([]auditlog.StorageLine{{Member: "total", Value: "1 bytes"}})
	if bare[1] != "  total          1 bytes" {
		t.Fatalf("detail-free line rendered %q", bare[1])
	}
}

// TestStorageCensusIsReadOnlyBeforeAndAfter proves the census never
// advances the audit plane: bytes, hashes, and mtimes are identical
// around the call, and a missing family is refused, not created.
func TestStorageCensusIsReadOnlyBeforeAndAfter(t *testing.T) {
	dir := t.TempDir()
	path := writeStorFamily(t, dir, 6)
	snap := func() map[string]string {
		out := map[string]string{}
		for _, e := range must(os.ReadDir(dir)) {
			if e.IsDir() {
				continue
			}
			b := must(os.ReadFile(filepath.Join(dir, e.Name())))
			st := must(e.Info())
			out[e.Name()] = fmt.Sprintf("%x|%d", sha256.Sum256(b), st.ModTime().UnixNano())
		}
		return out
	}
	before := snap()
	if _, err := auditlog.StorageCensus(path, 1<<20, 1<<16, 1<<16, time.Now()); err != nil {
		t.Fatalf("census: %v", err)
	}
	after := snap()
	if len(before) != len(after) {
		t.Fatalf("file count changed %d to %d (read-only rule broken)", len(before), len(after))
	}
	for k, v := range before {
		if after[k] != v {
			t.Fatalf("file %s changed around the census: %q to %q", k, v, after[k])
		}
	}
	missing := filepath.Join(dir, "absent.jsonl")
	if _, err := auditlog.StorageCensus(missing, 0, 0, 0, time.Now()); err == nil {
		t.Fatal("census on a missing family returned no error")
	}
	if _, err := os.Stat(missing); !os.IsNotExist(err) {
		t.Fatalf("census created the file it was asked to read (%v)", err)
	}
}

func must[T any](v T, err error) T {
	if err != nil {
		panic(err)
	}
	return v
}

// TestStorageAbsentMembersStatedNotZeroed pins the honest-absence rule:
// a member with no source is never rendered as a byte count.
func TestStorageAbsentMembersStatedNotZeroed(t *testing.T) {
	dir := t.TempDir()
	path := writeStorFamily(t, dir, 4)
	lines, err := auditlog.StorageCensus(path, 0, 0, 0, time.Now())
	if err != nil {
		t.Fatalf("census: %v", err)
	}
	m := lineMap(t, lines)
	for _, member := range []string{"runtime_logs", "recovery", "evidence"} {
		if m[member].Value != auditlog.StorageViewAbsent {
			t.Fatalf("%s = %q, want the absent token", member, m[member].Value)
		}
	}
	joined := strings.Join(renderStorage(lines), "\n")
	if strings.Contains(joined, "runtime_logs   0 bytes") || strings.Contains(joined, "recovery       0 bytes") {
		t.Fatalf("absent member rendered as a zero:\n%s", joined)
	}
}

// TestStorageUndeclaredCapacityLeavesBandUncomputed pins the ceiling
// absence on both the quota and the pressure member, and the band
// ladder when a ceiling is declared.
func TestStorageUndeclaredCapacityLeavesBandUncomputed(t *testing.T) {
	dir := t.TempDir()
	path := writeStorFamily(t, dir, 5)
	undeclared, err := auditlog.StorageCensus(path, 0, 0, 0, time.Now())
	if err != nil {
		t.Fatalf("census: %v", err)
	}
	m := lineMap(t, undeclared)
	if m["quota"].Value != auditlog.StorageViewAbsent || !strings.Contains(m["pressure_status"].Detail, "band not computed") {
		t.Fatalf("undeclared capacity fabricated a number: %+v %+v", m["quota"], m["pressure_status"])
	}
	total := duSumWalk(t, dir)
	declared, err := auditlog.StorageCensus(path, total*2, 0, 0, time.Now())
	if err != nil {
		t.Fatalf("census declared: %v", err)
	}
	dm := lineMap(t, declared)
	if dm["quota"].Value != fmt.Sprintf("%d bytes", total*2) {
		t.Fatalf("declared quota = %q", dm["quota"].Value)
	}
	bands := map[string]bool{
		string(auditlog.BandNormal): true, string(auditlog.BandCompressAggregate): true,
		string(auditlog.BandStrongAggregation): true, string(auditlog.BandEvictOldNormal): true,
		string(auditlog.BandRetainCriticalOnly): true,
	}
	if !bands[dm["pressure_status"].Value] {
		t.Fatalf("pressure value %q is off the shipped ladder", dm["pressure_status"].Value)
	}
}

// TestStorageScopeCensusKeepsDimensionsSeparate pins that agent and task
// scopes are displayed as their own records (never averaged) and that
// undeclared ceilings stay known gaps.
func TestStorageScopeCensusKeepsDimensionsSeparate(t *testing.T) {
	dir := t.TempDir()
	path := writeStorFamily(t, dir, 9)
	lines, err := auditlog.StorageCensus(path, 1<<20, 1<<10, 1<<12, time.Now())
	if err != nil {
		t.Fatalf("census: %v", err)
	}
	var agentLines, taskLines, gapLines []string
	for _, ln := range lines {
		switch ln.Member {
		case "scope":
			if strings.HasPrefix(ln.Value, "agent ") {
				agentLines = append(agentLines, ln.Value)
			}
			if strings.HasPrefix(ln.Value, "task ") {
				taskLines = append(taskLines, ln.Value)
			}
		case "known_gap":
			gapLines = append(gapLines, ln.Value)
		}
	}
	if len(agentLines) == 0 || len(taskLines) == 0 {
		t.Fatalf("scope census lost a dimension: agents %v tasks %v", agentLines, taskLines)
	}
	// The two dimensions are stated separately: no agent line may carry a
	// task id and no task line may carry an agent id.
	for _, ln := range agentLines {
		if strings.Contains(ln, "task ") {
			t.Fatalf("agent scope line crossed dimensions: %q", ln)
		}
	}
	for _, ln := range taskLines {
		if strings.Contains(ln, "claude-code") {
			t.Fatalf("task scope line crossed dimensions: %q", ln)
		}
	}
	sorted := append([]string{}, gapLines...)
	sort.Strings(sorted)
	wantGaps := []string{
		"quota-ceiling-not-declared:agent/event_rate",
		"quota-ceiling-not-declared:task/event_rate",
	}
	if strings.Join(sorted, ",") != strings.Join(wantGaps, ",") {
		t.Fatalf("known gaps = %v, want %v", sorted, wantGaps)
	}
}

// TestStorageMemberOrderLockedToRegistration proves the census is locked
// to the one closed member list, in spec order.
func TestStorageMemberOrderLockedToRegistration(t *testing.T) {
	dir := t.TempDir()
	path := writeStorFamily(t, dir, 3)
	lines, err := auditlog.StorageCensus(path, 0, 0, 0, time.Now())
	if err != nil {
		t.Fatalf("census: %v", err)
	}
	members := auditlog.StorageViewMemberList()
	if len(lines) < len(members) {
		t.Fatalf("census has %d lines, fewer than the %d members", len(lines), len(members))
	}
	for i, want := range members {
		if lines[i].Member != want {
			t.Fatalf("member %d = %q, want %q", i, lines[i].Member, want)
		}
	}
	// runStorage must refuse a family that does not exist rather than
	// printing an empty census.
	cfg := config{out: filepath.Join(dir, "nope.jsonl")}
	if err := runStorage(cfg); err == nil {
		t.Fatal("runStorage on a missing audit family returned no error")
	}
}
