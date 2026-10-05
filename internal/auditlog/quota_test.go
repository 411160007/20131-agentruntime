package auditlog

import (
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
	"time"

	"20131.com/agentruntime/internal/schema"
)

// qev builds a valid event with a stated agent, optional stated
// task attr, and the given severity at ts.
func qev(id, agent, task string, ts time.Time, sev schema.Severity) *schema.Event {
	e := &schema.Event{
		V:        schema.SchemaVersion,
		TS:       ts.UTC(),
		ID:       id,
		AgentID:  agent,
		Stage:    schema.StageObservation,
		Type:     schema.TypeAgentScan,
		Decision: schema.DecisionAllow,
		Severity: sev,
		Summary:  "quota fixture event",
	}
	if task != "" {
		e.Attrs = map[string]string{QuotaTaskAttrKey: task}
	}
	return e
}

func findRecord(rep QuotaReport, scope, id, kind string) (QuotaRecord, bool) {
	for _, r := range rep.Records {
		if r.Scope == scope && r.ScopeID == id && r.Kind == kind {
			return r, true
		}
	}
	return QuotaRecord{}, false
}

func hasGap(rep QuotaReport, want string) bool {
	for _, g := range rep.KnownGaps {
		if strings.HasPrefix(g, want) {
			return true
		}
	}
	return false
}

// Machine judge 1a: per-agent event rate — over-ceiling positive
// control plus under-ceiling negative control, with windowed counts.
func TestQuotaEventRatePositiveAndNegative(t *testing.T) {
	dir := t.TempDir()
	l, base := openGov(t, dir, "q.jsonl", nil)
	now := time.Date(2026, 10, 6, 0, 0, 0, 0, time.UTC)
	var evs []*schema.Event
	for i := 0; i < 4; i++ {
		evs = append(evs, qev("a"+string(rune('0'+i)), "agent-a", "", now.Add(-time.Minute), schema.SevInfo))
	}
	evs = append(evs, qev("b1", "agent-b", "", now.Add(-time.Minute), schema.SevInfo))
	// agent-c sits under the ceiling INSIDE the window with ten
	// ancient lines: counting them (wrongly) would trigger it.
	evs = append(evs, qev("c-now", "agent-c", "", now.Add(-time.Minute), schema.SevInfo))
	for i := 0; i < 10; i++ {
		evs = append(evs, qev("cold"+string(rune('0'+i)), "agent-c", "", now.AddDate(0, 0, -3), schema.SevInfo))
	}
	writeSeg(t, base, "20261006T000000Z", evs...)
	rep := l.ObserveQuota(Quota{PerAgentEvents: 3, WindowSeconds: 3600}, now)
	ra, ok := findRecord(rep, QuotaScopeAgent, "agent-a", QuotaKindEventRate)
	if !ok || ra.State != QuotaAbove || !ra.Triggered {
		t.Fatalf("agent-a over ceiling not recorded: %+v ok=%v", ra, ok)
	}
	if ra.Response != "record-only-no-action" {
		t.Errorf("agent-a response drifted: %q", ra.Response)
	}
	if rb, ok := findRecord(rep, QuotaScopeAgent, "agent-b", QuotaKindEventRate); !ok || rb.Triggered || rb.Observed != 1 {
		t.Errorf("agent-b negative control broke: %+v", rb)
	}
	if rc, ok := findRecord(rep, QuotaScopeAgent, "agent-c", QuotaKindEventRate); !ok || rc.Triggered || rc.Observed != 1 {
		t.Errorf("agent-c window discipline broke (out-of-window lines counted): %+v", rc)
	}
}

// Machine judge 1b: the task dimension is attributed only from a
// stated attrs task_id; nothing is ever guessed into a default task.
func TestQuotaTaskAttributesStatedIdOnly(t *testing.T) {
	dir := t.TempDir()
	l, base := openGov(t, dir, "q.jsonl", nil)
	now := time.Date(2026, 10, 6, 0, 0, 0, 0, time.UTC)
	var evs []*schema.Event
	for i := 0; i < 3; i++ {
		evs = append(evs, qev("t"+string(rune('0'+i)), "agent-a", "task-x", now.Add(-time.Minute), schema.SevLow))
	}
	evs = append(evs, qev("notask", "agent-b", "", now.Add(-time.Minute), schema.SevLow))
	writeSeg(t, base, "20261006T000000Z", evs...)
	rep := l.ObserveQuota(Quota{PerTaskEvents: 2, PerTaskBytes: 1, WindowSeconds: 3600}, now)
	rt, ok := findRecord(rep, QuotaScopeTask, "task-x", QuotaKindEventRate)
	if !ok || rt.State != QuotaAbove || !rt.Triggered {
		t.Fatalf("task-x over ceiling not recorded: %+v ok=%v", rt, ok)
	}
	if _, ok := findRecord(rep, QuotaScopeTask, "", QuotaKindEventRate); ok {
		t.Errorf("an unstated task identity was fabricated into a record")
	}
	// Lines without the attr simply never surface on the task axis;
	// the agent axis still sees all four events.
	if rep.EventsSeen != 4 {
		t.Errorf("events seen %d, want 4", rep.EventsSeen)
	}
	if hasGap(rep, "no-stated-task_id") {
		t.Errorf("task-unattributed gap fired while stated task lines exist")
	}
}

// P06 shape: an undeclared ceiling is a stated known gap, never a
// computed record and never a zero.
func TestQuotaUndeclaredCeilingIsKnownGapNotZero(t *testing.T) {
	dir := t.TempDir()
	l, base := openGov(t, dir, "q.jsonl", nil)
	now := time.Date(2026, 10, 6, 0, 0, 0, 0, time.UTC)
	writeSeg(t, base, "20261006T000000Z", qev("e1", "agent-a", "", now.Add(-time.Minute), schema.SevInfo))
	rep := l.ObserveQuota(Quota{WindowSeconds: 3600}, now)
	if len(rep.Records) != 0 {
		t.Fatalf("records emitted for undeclared ceilings: %+v", rep.Records)
	}
	for _, want := range []string{
		"quota-ceiling-not-declared:agent/event_rate",
		"quota-ceiling-not-declared:task/event_rate",
		"quota-ceiling-not-declared:agent/log_bytes",
		"quota-ceiling-not-declared:task/log_bytes",
	} {
		if !hasGap(rep, want) {
			t.Errorf("known gap line missing: %s", want)
		}
	}
}

func TestQuotaWindowNotStatedRateNotComputed(t *testing.T) {
	dir := t.TempDir()
	l, base := openGov(t, dir, "q.jsonl", nil)
	now := time.Date(2026, 10, 6, 0, 0, 0, 0, time.UTC)
	writeSeg(t, base, "20261006T000000Z", qev("e1", "agent-a", "", now.Add(-time.Minute), schema.SevInfo))
	rep := l.ObserveQuota(Quota{PerAgentEvents: 1, PerAgentBytes: 1}, now)
	if !hasGap(rep, "rate-window-not-stated:agent/event_rate") {
		t.Errorf("missing rate-window gap line: %+v", rep.KnownGaps)
	}
	if _, ok := findRecord(rep, QuotaScopeAgent, "agent-a", QuotaKindEventRate); ok {
		t.Errorf("event rate computed without a stated window")
	}
	if _, ok := findRecord(rep, QuotaScopeAgent, "agent-a", QuotaKindLogBytes); !ok {
		t.Errorf("log_bytes dimension should still compute with bytes ceiling stated")
	}
}

// Machine judge 2: Critical conservation + honest watermark. A
// quota-triggered scope holding Critical lines gets the shipped
// over_quota_critical_only token, the counts are conserved, and no
// file is touched.
func TestQuotaCriticalConservationAndWatermark(t *testing.T) {
	dir := t.TempDir()
	l, base := openGov(t, dir, "q.jsonl", nil)
	now := time.Date(2026, 10, 6, 0, 0, 0, 0, time.UTC)
	var evs []*schema.Event
	for i := 0; i < 3; i++ {
		evs = append(evs, qev("c"+string(rune('0'+i)), "agent-x", "", now.Add(-time.Minute), schema.SevCritical))
	}
	evs = append(evs, qev("i1", "agent-y", "", now.Add(-time.Minute), schema.SevInfo))
	writeSeg(t, base, "20261006T000000Z", evs...)
	before := l.Segments()
	rep := l.ObserveQuota(Quota{PerAgentBytes: 500}, now)
	rx, ok := findRecord(rep, QuotaScopeAgent, "agent-x", QuotaKindLogBytes)
	if !ok || !rx.Triggered {
		t.Fatalf("agent-x bytes quota not triggered: %+v", rx)
	}
	if rx.Watermark != "over_quota_critical_only" {
		t.Errorf("critical-holding over-quota scope lost the honest watermark: %q", rx.Watermark)
	}
	if rx.CriticalHeld != 3 {
		t.Errorf("critical held count %d, want 3", rx.CriticalHeld)
	}
	if rep.CriticalSeen != 3 {
		t.Errorf("critical conservation broke: seen %d, want 3", rep.CriticalSeen)
	}
	if ry, ok := findRecord(rep, QuotaScopeAgent, "agent-y", QuotaKindLogBytes); ok && ry.Watermark != "" {
		t.Errorf("non-critical scope got a watermark it never earned: %+v", ry)
	}
	after := l.Segments()
	sort.Strings(before)
	sort.Strings(after)
	if strings.Join(before, ",") != strings.Join(after, ",") {
		t.Errorf("quota observation mutated the segment family: %v vs %v", before, after)
	}
}

// Machine judge: fail-closed attribution — a malformed segment is
// held whole, never guessed onto a scope; healthy segments still
// attribute, and the byte split stays conserved.
func TestQuotaMalformedSegmentHeldNotGuessed(t *testing.T) {
	dir := t.TempDir()
	l, base := openGov(t, dir, "q.jsonl", nil)
	now := time.Date(2026, 10, 6, 0, 0, 0, 0, time.UTC)
	writeSeg(t, base, "20261006T000000Z", qev("g1", "agent-bad", "", now.Add(-time.Minute), schema.SevInfo))
	// poison the segment written above with a trailing malformed line
	p := base + ".20261006T000000Z"
	f, err := os.OpenFile(p, os.O_APPEND|os.O_WRONLY, 0o600)
	if err != nil {
		t.Fatal(err)
	}
	f.WriteString("{not json}\n")
	f.Close()
	healthy := writeSeg(t, base, "20261006T010000Z", qev("g2", "agent-ok", "", now.Add(-time.Minute), schema.SevInfo))
	raw, _ := os.ReadFile(p)
	hraw, _ := os.ReadFile(healthy)
	rep := l.ObserveQuota(Quota{PerAgentBytes: 1}, now)
	if rep.SegmentsHeld != 1 {
		t.Fatalf("held segments %d, want 1", rep.SegmentsHeld)
	}
	if rep.BytesUnattributed != int64(len(raw)) {
		t.Errorf("unattributed bytes %d, want %d (whole held file)", rep.BytesUnattributed, len(raw))
	}
	if rep.BytesAttributed != int64(len(hraw)) {
		t.Errorf("attributed bytes %d, want %d (healthy file only)", rep.BytesAttributed, len(hraw))
	}
	if _, ok := findRecord(rep, QuotaScopeAgent, "agent-bad", QuotaKindLogBytes); ok {
		t.Errorf("held segment lines were guessed into a scope record")
	}
	if _, ok := findRecord(rep, QuotaScopeAgent, "agent-ok", QuotaKindLogBytes); !ok {
		t.Errorf("healthy segment attribution lost")
	}
}

// Shared-source discipline without touching agency symbols from
// this plane: the quota vocabularies are mirrored by literals and
// pinned here against the schema closed sets (drift either way is
// red in both directions).
func TestQuotaVocabulariesStaySyncedWithSchema(t *testing.T) {
	if QuotaScopeAgent != schema.AllAgencyCounterScopes()[0] || QuotaScopeTask != schema.AllAgencyCounterScopes()[1] {
		t.Errorf("quota scope pair drifted from the §261 closed pair: %v", schema.AllAgencyCounterScopes())
	}
	states := schema.AllAgencyLimitStates()
	if len(states) != 3 || string(QuotaBelow) != states[0] || string(QuotaAt) != states[1] || string(QuotaAbove) != states[2] {
		t.Errorf("quota state triad drifted from the §261 vocabulary: %v", states)
	}
	found := false
	for _, k := range schema.AllAgencyCounterKinds() {
		if k == QuotaKindEventRate {
			found = true
		}
	}
	if !found {
		t.Errorf("event_rate kind token drifted from the W6.1 counter vocabulary")
	}
	if QuotaWatermarkToken != string(GovernOverQuotaCritical) {
		t.Errorf("watermark token re-invented instead of reusing the shipped governance line")
	}
	if QuotaResponseRule != StoragePressureResponse {
		t.Errorf("response token re-invented instead of reusing the shipped record-only line")
	}
}

// Machine judge 3 (byte zero-regression of the write path, runtime
// shape): the observation never blocks a write — events land before
// and after a pass, Count and Bytes keep their rotation semantics.
func TestQuotaObservationNeverBlocksWrites(t *testing.T) {
	dir := t.TempDir()
	l, _ := openGov(t, dir, "q.jsonl", &Rotation{MaxBytes: 4096})
	now := time.Date(2026, 10, 6, 0, 0, 0, 0, time.UTC)
	for i := 0; i < 3; i++ {
		if err := l.WriteEvent(qev("w"+string(rune('0'+i)), "agent-a", "", now, schema.SevInfo)); err != nil {
			t.Fatal(err)
		}
	}
	rep := l.ObserveQuota(Quota{PerAgentBytes: 10, PerAgentEvents: 1, WindowSeconds: 60}, now)
	if _, ok := findRecord(rep, QuotaScopeAgent, "agent-a", QuotaKindLogBytes); !ok {
		t.Fatalf("live head was not scanned: %+v", rep.Records)
	}
	// head attribution must conserve against the writer counter
	if rep.BytesAttributed <= 0 {
		t.Errorf("no bytes attributed from the live head")
	}
	if err := l.WriteEvent(qev("w9", "agent-a", "", now, schema.SevLow)); err != nil {
		t.Fatalf("write after observation blocked (record-only violated): %v", err)
	}
	if l.Count() != 4 {
		t.Errorf("count after pass %d, want 4", l.Count())
	}
	// and still nothing rotated away or created besides the base file
	files, _ := filepath.Glob(filepath.Join(dir, "*"))
	if len(files) != 1 {
		t.Errorf("observation touched the file family: %v", files)
	}
}
