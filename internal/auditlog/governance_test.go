package auditlog

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"20131.com/agentruntime/internal/schema"
)

// evCls builds a valid event of the given severity at ts.
func evCls(id string, ts time.Time, sev schema.Severity) *schema.Event {
	return &schema.Event{
		V:        schema.SchemaVersion,
		TS:       ts.UTC(),
		ID:       id,
		AgentID:  "agent-gov",
		Stage:    schema.StageObservation,
		Type:     schema.TypeAgentScan,
		Decision: schema.DecisionAllow,
		Severity: sev,
		Summary:  "governance fixture event",
	}
}

// writeSeg writes a rotated-style segment file (base.<stamp> suffix so
// segments() picks it up) containing the marshaled events.
func writeSeg(t *testing.T, base string, stamp string, evs ...*schema.Event) string {
	t.Helper()
	p := base + "." + stamp
	var sb strings.Builder
	for _, e := range evs {
		raw, err := json.Marshal(e)
		if err != nil {
			t.Fatal(err)
		}
		sb.Write(raw)
		sb.WriteByte('\n')
	}
	if err := os.WriteFile(p, []byte(sb.String()), 0o600); err != nil {
		t.Fatal(err)
	}
	return p
}

func openGov(t *testing.T, dir, name string, rot *Rotation) (*Log, string) {
	t.Helper()
	path := filepath.Join(dir, name)
	l, err := OpenLog(path, rot)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { l.Close() })
	return l, path
}

func hasDecision(rep GovernReport, seg string, act GovernAction) bool {
	for _, d := range rep.Decisions {
		if d.Segment == seg && d.Action == act {
			return true
		}
	}
	return false
}

// Machine-judged fixture 1 (time axis): segments are trimmed exactly
// per the §268 class windows and nothing else.
func TestRetentionPrunesExpiredWindowsOnly(t *testing.T) {
	dir := t.TempDir()
	l, base := openGov(t, dir, "gov.jsonl", nil)
	now := time.Date(2026, 10, 5, 12, 0, 0, 0, time.UTC)
	expiredInfo := writeSeg(t, base, "20260101T000000Z", evCls("ei", now.AddDate(0, 0, -8), schema.SevInfo))
	keptInfo := writeSeg(t, base, "20260102T000000Z", evCls("ki", now.AddDate(0, 0, -6), schema.SevInfo))
	expiredMed := writeSeg(t, base, "20260103T000000Z", evCls("em", now.AddDate(0, 0, -31), schema.SevMedium))
	keptHigh := writeSeg(t, base, "20260104T000000Z", evCls("kh", now.AddDate(0, 0, -80), schema.SevHigh))
	keptCrit := writeSeg(t, base, "20260105T000000Z", evCls("kc", now.AddDate(0, 0, -100), schema.SevCritical))
	_ = l
	rep := l.Enforce(DefaultGovernance(), now)
	for p, want := range map[string]bool{expiredInfo: false, keptInfo: true, expiredMed: false, keptHigh: true, keptCrit: true} {
		_, err := os.Stat(p)
		if (err == nil) != want {
			t.Errorf("segment %s exists=%v want %v (decisions: %+v)", p, err == nil, want, rep.Decisions)
		}
	}
	if !hasDecision(rep, expiredInfo, GovernPruneExpired) {
		t.Errorf("missing prune decision for expired info: %+v", rep.Decisions)
	}
	for _, d := range rep.Decisions {
		if d.Segment == expiredInfo && d.Detail != "info="+StorageRetentionInfo {
			t.Errorf("prune detail must carry the class window token, got %q", d.Detail)
		}
	}
}

// Machine-judged fixture 2 (capacity axis + Critical preserved
// positive control): over-quota eviction takes oldest normal history
// first and never touches a segment holding Critical events; critical
// conservation counts prove nothing critical was lost.
func TestCapacityPressureHoldsCriticalPositiveControl(t *testing.T) {
	dir := t.TempDir()
	l, base := openGov(t, dir, "gov.jsonl", nil)
	now := time.Date(2026, 10, 5, 12, 0, 0, 0, time.UTC)
	old1 := writeSeg(t, base, "20260101T000000Z", evCls("c1", now.AddDate(0, 0, -1), schema.SevInfo))
	old2 := writeSeg(t, base, "20260102T000000Z", evCls("c2", now.AddDate(0, 0, -1), schema.SevInfo))
	crit := writeSeg(t, base, "20260103T000000Z",
		evCls("c3", now.AddDate(0, 0, -1), schema.SevCritical),
		evCls("c4", now.AddDate(0, 0, -2), schema.SevCritical))
	st, _ := os.Stat(crit)
	g := Governance{MaxTotalBytes: st.Size() - 10} // even the critical file alone busts the cap
	rep := l.Enforce(g, now)
	for _, p := range []string{old1, old2} {
		if _, err := os.Stat(p); err == nil {
			t.Errorf("oldest normal segment %s should have been evicted for capacity", p)
		}
	}
	if _, err := os.Stat(crit); err != nil {
		t.Fatalf("critical segment pruned — %s: %+v", StorageCriticalHoldRule, rep.Decisions)
	}
	if !hasDecision(rep, crit, GovernHoldCritical) {
		t.Errorf("missing hold_critical decision: %+v", rep.Decisions)
	}
	if rep.CriticalSeen != 2 || rep.CriticalHeld != 2 {
		t.Errorf("critical conservation broken: seen=%d held=%d", rep.CriticalSeen, rep.CriticalHeld)
	}
	if !rep.OverQuotaCritical || !hasDecision(rep, l.Path(), GovernOverQuotaCritical) {
		t.Errorf("honest watermark line missing: over=%q decisions %+v", StorageOverQuotaHonesty, rep.Decisions)
	}
	if rep.BytesAfter <= g.MaxTotalBytes {
		t.Errorf("over-quota report must stay honest, got after=%d quota=%d", rep.BytesAfter, g.MaxTotalBytes)
	}
}

// Machine-judged fixture 3: the §270 ladder boundaries are exactly
// 60/80/90/95 with the top band closing at 100+.
func TestPressureBandLadderBoundaries(t *testing.T) {
	want := []StorageBand{
		BandNormal, BandNormal,
		BandCompressAggregate, BandCompressAggregate,
		BandStrongAggregation, BandStrongAggregation,
		BandEvictOldNormal, BandEvictOldNormal,
		BandRetainCriticalOnly, BandRetainCriticalOnly, BandRetainCriticalOnly,
	}
	pcts := []int{0, 59, 60, 79, 80, 89, 90, 94, 95, 100, 140}
	for i, p := range pcts {
		if got := StoragePressureBand(p); got != want[i] {
			t.Errorf("band(%d) = %s, want %s", p, got, want[i])
		}
	}
}

// Machine-judged fixture 4: Observe is pure record — no quota means
// no band and no fabricated numbers (P06), and nothing on disk moves.
func TestObserveRecordOnlyAndQuotaAbsence(t *testing.T) {
	dir := t.TempDir()
	l, _ := openGov(t, dir, "gov.jsonl", nil)
	if err := l.WriteEvent(evCls("ob1", time.Now().UTC(), schema.SevInfo)); err != nil {
		t.Fatal(err)
	}
	before, _ := os.ReadDir(dir)
	r := l.Observe(DefaultGovernance())
	if !r.QuotaAbsent || r.Band != "" || r.KnownGap != "no-capacity-quota-configured-band-not-computed" {
		t.Fatalf("quota absence must be honest gap, got %+v", r)
	}
	after, _ := os.ReadDir(dir)
	if len(before) != len(after) {
		t.Fatalf("Observe changed the tree: %d -> %d entries", len(before), len(after))
	}
	r2 := l.Observe(Governance{MaxTotalBytes: r.UsedBytes * 2})
	if r2.Band != BandNormal || r2.QuotaAbsent {
		t.Fatalf("configured quota must compute a band, got %+v", r2)
	}
}

// Machine-judged fixture 5 (0953ac1 inheritance): a rename-on-threshold
// legitimately leaves the live head empty; Enforce never prunes it and
// content validation of the empty head is exempt (size 0 is legal for
// the live file, archives still carry events).
func TestEmptyHeadExemptionInherited(t *testing.T) {
	dir := t.TempDir()
	l, base := openGov(t, dir, "head.jsonl", &Rotation{MaxBytes: 1})
	if err := l.WriteEvent(evCls("h1", time.Now().UTC(), schema.SevInfo)); err != nil {
		t.Fatal(err)
	}
	// after rotation the head was renamed and the fresh head is empty
	if sz, err := os.Stat(base); err != nil || sz.Size() != 0 {
		t.Fatalf("fixture: expected empty live head, size=%v err=%v", sz, err)
	}
	segs := l.Segments()
	if len(segs) != 1 {
		t.Fatalf("want 1 rotated segment, got %v", segs)
	}
	now := time.Now().UTC()
	rep := l.Enforce(DefaultGovernance(), now)
	if _, err := os.Stat(base); err != nil {
		t.Fatalf("live head pruned — %s: %+v", StorageLiveHeadRule, rep.Decisions)
	}
	if st, _ := os.Stat(base); st.Size() != 0 {
		t.Errorf("empty head must stay empty, size=%d", st.Size())
	}
	if !hasDecision(rep, base, GovernHoldLiveHead) {
		t.Errorf("hold_live_head decision missing: %+v", rep.Decisions)
	}
	if unixPermBits() {
		if mode := stMode(t, base); mode != "600" {
			t.Errorf("empty head mode %s, want 600 (mode gate kept)", mode)
		}
	}
	// archive content still validates: one event line present
	raw, err := os.ReadFile(segs[0])
	if err != nil {
		t.Fatalf("archive vanished: %v (%+v)", err, rep.Decisions)
	}
	if lines := strings.Count(string(raw), "\n"); lines != 1 {
		t.Errorf("rotated archive lost events: %d lines", lines)
	}
}

func stMode(t *testing.T, p string) string {
	t.Helper()
	st, err := os.Stat(p)
	if err != nil {
		t.Fatal(err)
	}
	return fmt.Sprintf("%o", st.Mode().Perm())
}

// Machine-judged fixture 6 (§270 last line + §271 spirit): pressure
// never blocks writes; a Critical event lands on disk even when the
// cap is already exceeded.
func TestWriteNeverBlockedByPressure(t *testing.T) {
	dir := t.TempDir()
	l, _ := openGov(t, dir, "wb.jsonl", nil)
	g := Governance{MaxTotalBytes: 1}
	if err := l.WriteEvent(evCls("w1", time.Now().UTC(), schema.SevCritical)); err != nil {
		t.Fatalf("write blocked under pressure — %s: %v", StorageWriteBlockRule, err)
	}
	r := l.Observe(g)
	if r.Band != BandRetainCriticalOnly || r.UsedBytes < 1 {
		t.Fatalf("pressure ladder must read over-quota honestly, got %+v", r)
	}
}

// Machine-judged fixture 7: an unreadable segment is held, never
// guessed-pruned; foreign prefix-sharing files are untouched (prune
// discipline inherited from the rotation base).
func TestUnreadableHeldAndForeignUntouched(t *testing.T) {
	dir := t.TempDir()
	l, base := openGov(t, dir, "un.jsonl", nil)
	now := time.Date(2026, 10, 5, 12, 0, 0, 0, time.UTC)
	bad := base + ".20260101T000000Z"
	if err := os.WriteFile(bad, []byte("this is not jsonl\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	foreign := base + ".notes"
	if err := os.WriteFile(foreign, []byte("keep me"), 0o600); err != nil {
		t.Fatal(err)
	}
	rep := l.Enforce(Governance{MaxTotalBytes: 1}, now)
	if _, err := os.Stat(bad); err != nil {
		t.Fatalf("unreadable segment was deleted: %+v", rep.Decisions)
	}
	if !hasDecision(rep, bad, GovernHoldUnreadable) {
		t.Errorf("missing hold_unreadable: %+v", rep.Decisions)
	}
	if _, err := os.Stat(foreign); err != nil {
		t.Fatalf("foreign prefix file deleted: %v", err)
	}
}

// TotalBytes is the W8.3 du-source: live + rotated, nothing else.
func TestTotalBytesMatchesDisk(t *testing.T) {
	dir := t.TempDir()
	l, base := openGov(t, dir, "tb.jsonl", nil)
	if err := l.WriteEvent(evCls("t1", time.Now().UTC(), schema.SevInfo)); err != nil {
		t.Fatal(err)
	}
	writeSeg(t, base, "20260101T000000Z", evCls("t2", time.Now().UTC(), schema.SevInfo))
	got := l.TotalBytes()
	var want int64
	entries, _ := os.ReadDir(dir)
	for _, e := range entries {
		if strings.HasPrefix(e.Name(), filepath.Base(base)) {
			fi, err := e.Info()
			if err != nil {
				t.Fatal(err)
			}
			want += fi.Size()
		}
	}
	if got != want {
		t.Fatalf("TotalBytes=%d want %d (live+segments)", got, want)
	}
}
