// Storage governance V2 (slice W8.1): the rotation base (size policy,
// UTC day-cut, KeepHistory) stays exactly as shipped; this file layers
// the second axis on top of it — a total-capacity hard cap plus a
// time-retention window per severity class — so audit history can never
// grow without bound along either axis.
//
// Contract preserved from the rotation base:
//   - the Writer/Log write path is untouched; governance never blocks,
//     delays, or drops a write, including Critical security events;
//   - the §270 pressure ladder (normal → retain_critical_only) is
//     recorded as observations only — no compression, aggregation, or
//     eviction policy is wired into the write path in this phase;
//   - segments holding Critical events are exempt from capacity
//     eviction; when only exempt history remains above quota the
//     governor reports an honest over-quota line instead of silently
//     dropping data;
//   - the live head file is never pruned, and may legitimately be
//     empty right after a rename-on-threshold rotation (the 0953ac1
//     gate form: an empty head is exempt from content validation but
//     keeps its 0600 mode gate); rotated archives still validate fully.
package auditlog

import (
	"bufio"
	"encoding/json"
	"os"
	"strconv"
	"strings"
	"time"

	"20131.com/agentruntime/internal/schema"
)

// Retention windows per severity class (spec §268: runtime event
// detailed 7d, security decision 30d, agent/task audit 90d, critical
// incident 180d). Stored as strings so the docs mapping line can be
// derived from these constants mechanically, never hand-copied.
const (
	StorageRetentionInfo     = "7d"
	StorageRetentionLow      = "7d"
	StorageRetentionMedium   = "30d"
	StorageRetentionHigh     = "90d"
	StorageRetentionCritical = "180d"
)

// Contract rule lines mirrored one-for-one in docs section 26.
const (
	StorageDualLimitRule    = "time-and-capacity-both-enforced-neither-infinite-growth"
	StorageCriticalHoldRule = "critical-classified-segments-never-pruned-by-capacity-pressure"
	StoragePressureResponse = "record-only-no-action"
	StorageOverQuotaHonesty = "when-only-held-segments-remain-report-over-quota-never-drop-critical"
	StorageLiveHeadRule     = "governance-never-prunes-the-live-segment-empty-head-stays-valid"
	StorageWriteBlockRule   = "pressure-never-blocks-writes-in-observation-phase"
	StorageUnreadableRule   = "unreadable-segment-is-held-never-guessed-pruned"
	StorageQuotaStance      = "tier-defaults-are-initial-engineering-defaults-not-permanent-commercial-promise"
)

// Pressure bands from usage percentage against the total quota (spec
// §270). Band responses are recorded, never enforced (§270 observation
// phase).
type StorageBand string

const (
	BandNormal             StorageBand = "normal"
	BandCompressAggregate  StorageBand = "compress_aggregate"
	BandStrongAggregation  StorageBand = "strong_aggregation"
	BandEvictOldNormal     StorageBand = "evict_old_normal"
	BandRetainCriticalOnly StorageBand = "retain_critical_only"
)

// Band upper edges, matching the §270 ladder: 0-60, 60-80, 80-90,
// 90-95, 95-100. The final edge 101 closes the top band so a
// percentage of exactly 100 (and any over-quota overshoot) lands in
// retain_critical_only.
const (
	BandEdgeCompress          = 60
	BandEdgeStrongAggregation = 80
	BandEdgeEvictOld          = 90
	BandEdgeRetainCritical    = 95
)

type bandRow struct {
	Band     StorageBand
	EdgePct  int // usage below EdgePct lands in this band
	Response string
}

// storageBandCoverage is the single registration the checker derives
// the docs vocabulary line from (same mechanical-derivation discipline
// as the cost-guard coverage table).
func storageBandCoverage() []bandRow {
	return []bandRow{
		{BandNormal, 60, StoragePressureResponse},
		{BandCompressAggregate, 80, StoragePressureResponse},
		{BandStrongAggregation, 90, StoragePressureResponse},
		{BandEvictOldNormal, 95, StoragePressureResponse},
		{BandRetainCriticalOnly, 101, StoragePressureResponse},
	}
}

// StoragePressureBand maps a usage percentage onto the §270 ladder.
func StoragePressureBand(pct int) StorageBand {
	for _, r := range storageBandCoverage() {
		if pct < r.EdgePct {
			return r.Band
		}
	}
	return BandRetainCriticalOnly
}

// Governance actions on one segment during an Enforce pass.
type GovernAction string

const (
	GovernPruneExpired      GovernAction = "prune_expired"
	GovernEvictCapacity     GovernAction = "evict_for_capacity"
	GovernHoldCritical      GovernAction = "hold_critical"
	GovernHoldLiveHead      GovernAction = "hold_live_head"
	GovernHoldUnreadable    GovernAction = "hold_unreadable"
	GovernOverQuotaCritical GovernAction = "over_quota_critical_only"
)

// Governance configures the dual limits for one Log's segment family.
// MaxTotalBytes 0 disables the capacity cap and with it the pressure
// band computation: absence is reported as a known gap, never
// fabricated as a number (P06 discipline inherited from the cost
// guard).
type Governance struct {
	MaxTotalBytes int64
	// RetainOverrides maps a severity onto a custom window in days
	// (Enterprise configurable per §268); empty uses the defaults.
	RetainOverrides map[schema.Severity]int
}

// DefaultGovernance returns §268 default windows with capacity
// enforcement off until a quota is configured.
func DefaultGovernance() Governance {
	return Governance{}
}

func (g Governance) retainDays(s schema.Severity) (int, bool) {
	if d, ok := g.RetainOverrides[s]; ok {
		return d, true
	}
	switch s {
	case schema.SevInfo:
		return 7, true
	case schema.SevLow:
		return 7, true
	case schema.SevMedium:
		return 30, true
	case schema.SevHigh:
		return 90, true
	case schema.SevCritical:
		return 180, true
	}
	return 0, false
}

// windowToken renders the docs mapping token for a severity, derived
// from the retention constants (never hand-typed at call sites).
func (g Governance) windowToken(s schema.Severity) string {
	switch s {
	case schema.SevInfo:
		return "info=" + StorageRetentionInfo
	case schema.SevLow:
		return "low=" + StorageRetentionLow
	case schema.SevMedium:
		return "medium=" + StorageRetentionMedium
	case schema.SevHigh:
		return "high=" + StorageRetentionHigh
	case schema.SevCritical:
		return "critical=" + StorageRetentionCritical
	}
	return "unclassified"
}

// GovernDecision is one recorded outcome of an Enforce pass.
type GovernDecision struct {
	Segment string
	Action  GovernAction
	Detail  string
}

// PressureReading is the §270 observation. QuotaAbsent is the honest
// no-number state when no capacity limit is configured: the band is
// NOT computed and NOT defaulted.
type PressureReading struct {
	UsedBytes   int64
	QuotaBytes  int64
	QuotaAbsent bool
	KnownGap    string
	Band        StorageBand
}

// GovernReport summarizes one Enforce pass.
type GovernReport struct {
	Pressure          PressureReading
	Decisions         []GovernDecision
	BytesBefore       int64
	BytesAfter        int64
	CriticalSeen      int // critical events counted before pruning
	CriticalHeld      int // critical events still on disk after the pass
	OverQuotaCritical bool
}

type segFacts struct {
	path       string
	bytes      int64
	latest     time.Time
	maxSev     schema.Severity
	critical   int
	lines      int
	unreadable bool
	empty      bool
	pruned     bool
}

// Observe computes the current pressure reading without touching any
// file (record-only; the write path never consults governance).
func (l *Log) Observe(g Governance) PressureReading {
	l.mu.Lock()
	defer l.mu.Unlock()
	used := l.w.Bytes()
	for _, s := range l.segments() {
		if st, err := os.Stat(s); err == nil {
			used += st.Size()
		}
	}
	if g.MaxTotalBytes <= 0 {
		return PressureReading{
			UsedBytes:   used,
			QuotaAbsent: true,
			KnownGap:    "no-capacity-quota-configured-band-not-computed",
		}
	}
	pct := int(used * 100 / g.MaxTotalBytes)
	return PressureReading{UsedBytes: used, QuotaBytes: g.MaxTotalBytes, Band: StoragePressureBand(pct)}
}

// readSegment parses just ts+severity per line — the dual-limit
// decision needs event chronology and class, never full payloads
// (Logging Privacy §272 holds by construction: metadata-only reads).
// Any malformed line marks the whole segment unreadable-held: the
// governor prunes nothing it cannot prove expired (fail-closed on
// data loss).
func readSegment(path string) segFacts {
	f := segFacts{path: path}
	st, err := os.Stat(path)
	if err != nil {
		f.unreadable = true
		return f
	}
	f.bytes = st.Size()
	if st.Size() == 0 {
		f.empty = true
		return f
	}
	fh, err := os.Open(path)
	if err != nil {
		f.unreadable = true
		return f
	}
	defer fh.Close()
	sc := bufio.NewScanner(fh)
	sc.Buffer(make([]byte, 0, 64*1024), 4*1024*1024)
	for sc.Scan() {
		var rec struct {
			TS       time.Time       `json:"ts"`
			Severity schema.Severity `json:"severity"`
		}
		line := sc.Bytes()
		if len(strings.TrimSpace(string(line))) == 0 {
			continue
		}
		if err := json.Unmarshal(line, &rec); err != nil || rec.TS.IsZero() || !rec.Severity.Valid() {
			f.unreadable = true
			return f
		}
		f.lines++
		if rec.TS.After(f.latest) {
			f.latest = rec.TS
		}
		if rec.Severity > f.maxSev {
			f.maxSev = rec.Severity
		}
		if rec.Severity == schema.SevCritical {
			f.critical++
		}
	}
	if err := sc.Err(); err != nil || f.lines == 0 {
		f.unreadable = true
	}
	return f
}

// Enforce applies both limits to rotated history, oldest segments
// first: time retention (per-class window on the newest event in the
// segment), then the capacity cap. The live head and any exempt
// segment are never deleted; exempt-but-over-quota yields the honest
// over_quota_critical_only line instead of silent drops.
func (l *Log) Enforce(g Governance, now time.Time) GovernReport {
	l.mu.Lock()
	defer l.mu.Unlock()
	rep := GovernReport{}
	segs := l.segments()
	facts := make([]segFacts, 0, len(segs))
	total := l.w.Bytes()
	for _, s := range segs {
		facts = append(facts, readSegment(s))
	}
	for _, f := range facts {
		total += f.bytes
		if f.critical > 0 && !f.unreadable {
			rep.CriticalSeen += f.critical
		}
	}
	rep.BytesBefore = total
	if g.MaxTotalBytes > 0 {
		rep.Pressure = PressureReading{UsedBytes: total, QuotaBytes: g.MaxTotalBytes,
			Band: StoragePressureBand(int(total * 100 / g.MaxTotalBytes))}
	} else {
		rep.Pressure = PressureReading{UsedBytes: total, QuotaAbsent: true,
			KnownGap: "no-capacity-quota-configured-band-not-computed"}
	}

	// Pass 1: time limit.
	for i := range facts {
		f := &facts[i]
		switch {
		case f.unreadable:
			rep.Decisions = append(rep.Decisions, GovernDecision{f.path, GovernHoldUnreadable, StorageUnreadableRule})
			continue
		case f.empty:
			// Zero bytes, zero events: pruning loses nothing.
			if err := os.Remove(f.path); err == nil {
				f.pruned = true
				rep.Decisions = append(rep.Decisions, GovernDecision{f.path, GovernPruneExpired, "empty segment, nothing to lose"})
			}
			continue
		}
		days, ok := g.retainDays(f.maxSev)
		if !ok {
			rep.Decisions = append(rep.Decisions, GovernDecision{f.path, GovernHoldUnreadable, "severity class has no retention window"})
			continue
		}
		if now.Sub(f.latest) > time.Duration(days)*24*time.Hour {
			if err := os.Remove(f.path); err == nil {
				total -= f.bytes
				f.pruned = true
				rep.Decisions = append(rep.Decisions, GovernDecision{f.path, GovernPruneExpired, g.windowToken(f.maxSev)})
			}
		}
	}

	// Pass 2: capacity limit — evict oldest non-exempt until inside
	// the cap; Critical-containing and unreadable-held segments and the
	// live head are never candidates.
	if g.MaxTotalBytes > 0 {
		for total > g.MaxTotalBytes {
			var victim *segFacts
			for i := range facts {
				f := &facts[i]
				if f.pruned || f.unreadable {
					continue
				}
				if f.critical > 0 {
					rep.Decisions = append(rep.Decisions, GovernDecision{f.path, GovernHoldCritical, StorageCriticalHoldRule})
					continue
				}
				victim = f
				break
			}
			if victim == nil {
				rep.OverQuotaCritical = true
				rep.Decisions = append(rep.Decisions, GovernDecision{l.path, GovernOverQuotaCritical, StorageOverQuotaHonesty})
				break
			}
			if err := os.Remove(victim.path); err == nil {
				total -= victim.bytes
				victim.pruned = true
				rep.Decisions = append(rep.Decisions, GovernDecision{victim.path, GovernEvictCapacity, strconv.FormatInt(g.MaxTotalBytes, 10)})
			} else {
				victim.unreadable = true // never spin on the same victim
				rep.Decisions = append(rep.Decisions, GovernDecision{victim.path, GovernHoldUnreadable, "remove failed: " + err.Error()})
			}
		}
	}

	for i := range facts {
		f := &facts[i]
		if !f.pruned && f.critical > 0 && !f.unreadable {
			rep.CriticalHeld += f.critical
		}
	}
	// The live head always stays, empty or not (0953ac1 form): the
	// hold is recorded so audits can see the exemption was exercised.
	rep.Decisions = append(rep.Decisions, GovernDecision{l.path, GovernHoldLiveHead, StorageLiveHeadRule})
	rep.BytesAfter = total
	return rep
}

// TotalBytes sums live head plus rotated history — the du-source the
// future storage subcommand (W8.3) will surface.
func (l *Log) TotalBytes() int64 {
	l.mu.Lock()
	defer l.mu.Unlock()
	total := l.w.Bytes()
	for _, s := range l.segments() {
		if st, err := os.Stat(s); err == nil {
			total += st.Size()
		}
	}
	return total
}
