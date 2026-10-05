// storageview.go - the read-only occupancy census behind the user-visible
// storage command (slice W8.3, specification section 273).
//
// Two properties define this file. First, it is a reader: the live segment
// is opened O_RDONLY with no create flag, so asking for the census cannot
// advance the audit plane, cannot materialise a file that was absent, and
// cannot disturb a concurrent append handle. Second, it reuses the shipped
// observation code over the same segment family: every byte and every
// per-scope count here is a re-count of already-durable audit lines taken
// through the exact paths the observation slices already use. There is no
// second source and no second dialect.
//
// The lines returned carry plain string fields only, so the command file
// never needs to reach into an observation type.
package auditlog

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"time"

	"20131.com/agentruntime/internal/schema"
)

// storageViewMembers is the closed section-273 member list, in the order
// the specification states it. The docs key pins this exact order.
var storageViewMembers = []string{
	"runtime_logs", "audit", "recovery", "evidence",
	"total", "quota", "retention", "pressure_status",
}

// StorageViewAbsent is the value a member carries when the release has no
// source for it. It is never rendered as a number: an absent member is
// stated, not zeroed.
const StorageViewAbsent = "not-present-in-this-release"

// StorageView rules, stated once here and mirrored in the docs section.
const (
	// StorageViewReadOnlyRule is the read-side contract of the census.
	StorageViewReadOnlyRule = "storage-census-opens-audit-read-only-never-creates-never-advances"
	// StorageViewSharedSourceRule restates the shared-source doctrine on
	// the display side: the census re-counts durable audit lines.
	StorageViewSharedSourceRule = StorageViewReadOnlyRule + ";" + QuotaSharedSourceRule
	// StorageViewAbsenceRule pins the honest-zero ban for members.
	StorageViewAbsenceRule = "member-without-source-reported-absent-never-rendered-as-zero"
	// StorageViewCeilingRule is the ceiling stance of section 271, aliased
	// from the shipped quota token: one dialect, not two.
	StorageViewCeilingRule = QuotaAbsentCeilingRule
	// StorageViewEnforcementPlane keeps the surface on the observation
	// stance, same token as every other Phase 0 surface.
	StorageViewEnforcementPlane = "none-in-observation-phase"
	// StorageViewRetentionAliasRule pins that the retention line renders
	// the shipped window mapping instead of a second window table.
	StorageViewRetentionAliasRule = "renders-section-26-retention-mapping-never-a-second-window-table"
	// StorageViewBandAliasRule pins the same stance for the pressure band:
	// the shipped ladder vocabulary and nothing else.
	StorageViewBandAliasRule = "renders-section-26-pressure-band-vocabulary-never-a-second-ladder"
)

// StorageLine is one rendered census line.
type StorageLine struct {
	Member string
	Value  string
	Detail string
}

// openReadOnlyLog builds a Log around a read-only handle on an already
// existing live segment. Nothing in the observation path writes: the
// census only stats and sequentially reads files, exactly as a rotation
// check does, and the handle carries no create flag.
func openReadOnlyLog(path string) (*Log, error) {
	if path == "" {
		return nil, fmt.Errorf("auditlog: empty path")
	}
	f, err := os.Open(path) // O_RDONLY, no create
	if err != nil {
		return nil, fmt.Errorf("auditlog: read-only open %s: %w", path, err)
	}
	st, err := f.Stat()
	if err != nil {
		f.Close()
		return nil, fmt.Errorf("auditlog: stat %s: %w", path, err)
	}
	return &Log{path: path, w: &Writer{f: f, path: path, written: st.Size()}}, nil
}

// StorageCensus produces the section-273 member lines for one audit
// segment family. A capacity of zero means the operator never declared a
// ceiling: the band is then not computed and not defaulted, and the quota
// and pressure members say so. The same holds per scope.
func StorageCensus(path string, capacityBytes, perAgentBytes, perTaskBytes int64, now time.Time) ([]StorageLine, error) {
	l, err := openReadOnlyLog(path)
	if err != nil {
		return nil, err
	}
	defer l.Close()

	total := l.TotalBytes()
	rotated := l.Segments()
	pressure := l.Observe(Governance{MaxTotalBytes: capacityBytes})
	quota := l.ObserveQuota(Quota{PerAgentBytes: perAgentBytes, PerTaskBytes: perTaskBytes}, now)

	lines := []StorageLine{
		{Member: "runtime_logs", Value: StorageViewAbsent,
			Detail: "no separate runtime log store exists in this release; the audit file is the only persistence exit"},
		{Member: "audit", Value: strconv.FormatInt(total, 10) + " bytes",
			Detail: fmt.Sprintf("live segment %s plus %d rotated segment(s)", filepath.Base(path), len(rotated))},
		{Member: "recovery", Value: StorageViewAbsent,
			Detail: "recovery records are a classification plane carried on audit lines; no separately measured recovery store exists"},
		{Member: "evidence", Value: StorageViewAbsent,
			Detail: "evidence bundles are written only into an operator-declared existing directory; no default evidence store path exists"},
		{Member: "total", Value: strconv.FormatInt(total, 10) + " bytes",
			Detail: "sum over the sources that exist; absent members are stated, never counted as zero"},
	}

	quotaDetail := "capacity ceiling declared"
	if pressure.QuotaAbsent {
		quotaDetail = "no capacity ceiling declared: " + pressure.KnownGap
	}
	lines = append(lines,
		StorageLine{Member: "quota",
			Value:  quotaValue(capacityBytes, pressure.QuotaAbsent),
			Detail: quotaDetail},
		StorageLine{Member: "retention",
			Value:  retentionCensus(),
			Detail: "windows derived from the shipped per-class constants (overrides are not a display-side concept)"},
		StorageLine{Member: "pressure_status",
			Value:  pressureValue(pressure),
			Detail: pressureDetail(pressure)},
	)

	lines = append(lines, scopeLines(quota)...)
	if err := checkStorageViewOrder(lines); err != nil {
		return nil, err
	}
	return lines, nil
}

// checkStorageViewOrder keeps the emitted census locked to the one closed
// member registration: a member added, dropped, or reordered fails here
// instead of drifting into the display.
func checkStorageViewOrder(lines []StorageLine) error {
	if len(lines) < len(storageViewMembers) {
		return fmt.Errorf("auditlog: storage census has %d lines, want at least %d members", len(lines), len(storageViewMembers))
	}
	for i, m := range storageViewMembers {
		if lines[i].Member != m {
			return fmt.Errorf("auditlog: storage census member %d is %q, want %q", i, lines[i].Member, m)
		}
	}
	return nil
}

// StorageViewMemberList exposes the closed member registration so the
// docs mirror and the tests compare against the same single source.
func StorageViewMemberList() []string {
	out := make([]string, len(storageViewMembers))
	copy(out, storageViewMembers)
	return out
}

// quotaValue states the declared ceiling or its honest absence.
func quotaValue(capacityBytes int64, absent bool) string {
	if absent {
		return StorageViewAbsent
	}
	return strconv.FormatInt(capacityBytes, 10) + " bytes"
}

// retentionCensus renders the five-class window line from the shipped
// constants, never from hand-typed text.
func retentionCensus() string {
	g := Governance{}
	return g.windowToken(schema.SevInfo) + " " +
		g.windowToken(schema.SevLow) + " " +
		g.windowToken(schema.SevMedium) + " " +
		g.windowToken(schema.SevHigh) + " " +
		g.windowToken(schema.SevCritical)
}

func pressureValue(p PressureReading) string {
	if p.QuotaAbsent {
		return StorageViewAbsent
	}
	return string(p.Band)
}

func pressureDetail(p PressureReading) string {
	if p.QuotaAbsent {
		return "band not computed: " + p.KnownGap
	}
	return fmt.Sprintf("%d of %d bytes used; response stays %s",
		p.UsedBytes, p.QuotaBytes, StoragePressureResponse)
}

// scopeLines turns the per-scope re-count into display lines, ordered
// deterministically (scope, kind, observed descending, id ascending) so
// the same bytes always render the same lines.
func scopeLines(r QuotaReport) []StorageLine {
	recs := make([]QuotaRecord, len(r.Records))
	copy(recs, r.Records)
	sort.SliceStable(recs, func(i, j int) bool {
		a, b := recs[i], recs[j]
		if a.Scope != b.Scope {
			return a.Scope < b.Scope
		}
		if a.Kind != b.Kind {
			return a.Kind < b.Kind
		}
		if a.Observed != b.Observed {
			return a.Observed > b.Observed
		}
		return a.ScopeID < b.ScopeID
	})
	out := make([]StorageLine, 0, len(recs)+len(r.KnownGaps)+1)
	out = append(out, StorageLine{Member: "scope_census",
		Value:  fmt.Sprintf("%d scope record(s)", len(recs)),
		Detail: QuotaDualDimensionRule})
	for _, rec := range recs {
		out = append(out, StorageLine{
			Member: "scope",
			Value:  fmt.Sprintf("%s %s=%s %d", rec.Scope, rec.Kind, rec.ScopeID, rec.Observed),
			Detail: fmt.Sprintf("ceiling %d state %s response %s", rec.Ceiling, rec.State, rec.Response),
		})
	}
	gaps := make([]string, len(r.KnownGaps))
	copy(gaps, r.KnownGaps)
	sort.Strings(gaps)
	for _, g := range gaps {
		out = append(out, StorageLine{Member: "known_gap", Value: g, Detail: QuotaAbsentCeilingRule})
	}
	return out
}
