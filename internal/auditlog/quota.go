// Per-agent and per-task storage quota observation (slice W8.2,
// spec §271). The rotation base (writer.go) and the dual-limit
// governor (governance.go) stay exactly as shipped; this file adds
// the quota lens on top: one anomalous agent must never eat the
// whole disk with its own audit lines, and the observation says so
// in records only.
//
// Collection-source discipline (§271 shared source, W6.1 doctrine):
// the quota never runs a second event collector. Everything it
// counts is derived from the already-stored audit lines — the same
// bytes the W6.1 recorders count when they stream, re-counted here
// from the written record. Agency guard symbols stay off this plane
// (the schema package pins that boundary); the two-scope pair and
// the three-state vocabulary are mirrored by literal tokens and the
// Go test in this package pins them against the schema scope pair and state triad
// accessors, so drift in either direction is a
// red build from the schema side too.
//
// Phase 0 stance (inherited from W8.1): every ceiling is
// record-only. No quota observation blocks, drops, delays, or prunes
// anything — the write path never consults this file. Critical
// events are exempt from quota pressure reporting by construction:
// a scope above its quota while holding Critical lines gets the
// honest over_quota_critical_only watermark line (the W8.1 token,
// reused, never re-invented) instead of any silent-drop fiction.
// An undeclared ceiling is a stated known gap, never a zero (P06,
// inherited from the cost guard). The task dimension is attributed
// only when an audit line explicitly states attrs["task_id"]; a line
// without it is unattributed, never guessed into a default task
// (the W7.1 trace coverage doctrine: absence named, not fabricated).
package auditlog

import (
	"bufio"
	"encoding/json"
	"os"
	"time"

	"20131.com/agentruntime/internal/schema"
)

// Scope pair tokens mirroring the §261 closed pair (agent, task) in
// normative order. The literals are pinned against the schema
// vocabulary by TestQuotaVocabulariesStaySyncedWithSchema.
const (
	QuotaScopeAgent = "agent"
	QuotaScopeTask  = "task"
)

// Quota kinds in taskbook order: the event-rate protection of
// §271 first, then the disk-consumption it exists to bound.
const (
	QuotaKindEventRate = "event_rate"
	QuotaKindLogBytes  = "log_bytes"
)

// Limit-state tokens mirroring the §261 closed triad. Same pin
// discipline as the scope pair.
type QuotaState string

const (
	QuotaBelow QuotaState = "below_ceiling"
	QuotaAt    QuotaState = "at_ceiling"
	QuotaAbove QuotaState = "above_ceiling"
)

// Contract rule lines mirrored one-for-one in docs section 27.
const (
	// QuotaDualDimensionRule pins §271: agent and task are separate
	// quota dimensions; neither is averaged away into the other
	// (§234 stance, restated from the W6.1 scope doctrine).
	QuotaDualDimensionRule = "per-agent-and-per-task-quoted-separately-never-averaged"
	// QuotaSharedSourceRule pins the collection-source ban: quota
	// observations are re-counts of stored audit lines, never a
	// second collector, never a parallel event stream.
	QuotaSharedSourceRule = "quota-recounts-stored-audit-lines-never-a-second-collector"
	// QuotaCriticalExemptRule restates §271 "keep critical events"
	// on the quota axis: quota pressure never reports a Critical
	// drop, it reports the watermark instead.
	QuotaCriticalExemptRule = "critical-events-never-quota-dropped-report-watermark"
	// QuotaUnattributedRule pins the honesty of missing identity:
	// a line without the dimension's stated id lands in an
	// unattributed bucket and a known-gap line, never a default.
	QuotaUnattributedRule = "line-without-stated-id-unattributed-never-defaulted"
	// QuotaAbsentCeilingRule is the P06 shape for quotas: an
	// undeclared ceiling is not computed and not defaulted to zero.
	QuotaAbsentCeilingRule = "undeclared-ceiling-not-computed-stated-known-gap"
	// QuotaResponseRule reuses the shipped record-only token
	// verbatim (StoragePressureResponse): one response vocabulary,
	// not two dialects of the same stance.
	QuotaResponseRule = StoragePressureResponse
	// QuotaWatermarkToken reuses the W8.1 honest-line token (already
	// shipped on the governance plane via GovernOverQuotaCritical):
	// a quota-triggered scope that still holds Critical lines gets
	// exactly this watermark and no drop fiction.
	QuotaWatermarkToken = string(GovernOverQuotaCritical)
	// QuotaTaskAttrKey names the only shape under which a task
	// dimension is attributed: the stated attrs key. Nothing else
	// casts a line into a task.
	QuotaTaskAttrKey = "task_id"
)

// Quota states the declared ceilings. Zero in any field means the
// ceiling was never declared: no record is computed for that
// dimension and the report says so as a known gap. WindowSeconds
// bounds the event-rate half-life of the rate dimension; with no
// window stated, the rate cannot be computed and the event-rate
// gaps are reported even if a ceiling exists.
type Quota struct {
	PerAgentEvents uint64
	PerTaskEvents  uint64
	PerAgentBytes  int64
	PerTaskBytes   int64
	WindowSeconds  int64
}

func (q Quota) ceiling(scope, kind string) (uint64, bool) {
	switch {
	case scope == QuotaScopeAgent && kind == QuotaKindEventRate:
		return q.PerAgentEvents, q.PerAgentEvents > 0
	case scope == QuotaScopeTask && kind == QuotaKindEventRate:
		return q.PerTaskEvents, q.PerTaskEvents > 0
	case scope == QuotaScopeAgent && kind == QuotaKindLogBytes:
		return uint64(q.PerAgentBytes), q.PerAgentBytes > 0
	case scope == QuotaScopeTask && kind == QuotaKindLogBytes:
		return uint64(q.PerTaskBytes), q.PerTaskBytes > 0
	}
	return 0, false
}

// QuotaRecord is one recorded ceiling comparison. The state is
// computed from the two numbers, never stated by a caller; the
// record carries no action field at all — the only response value
// is the reused record-only token. CriticalHeld and Watermark are
// the exemption accounting: when a triggered scope still holds
// Critical lines the watermark token rides with the record.
type QuotaRecord struct {
	Scope        string     `json:"scope"`
	ScopeID      string     `json:"scope_id"`
	Kind         string     `json:"kind"`
	Ceiling      uint64     `json:"ceiling"`
	Observed     uint64     `json:"observed"`
	State        QuotaState `json:"quota_state"`
	Triggered    bool       `json:"quota_triggered"`
	Response     string     `json:"response"`
	CriticalHeld int        `json:"critical_held_events"`
	Watermark    string     `json:"over_quota_watermark,omitempty"`
}

// QuotaReport is the whole observation of one pass. Counts are
// conservation counts over what was scanned: EventsSeen equals the
// events attributed plus the unattributed, CriticalSeen counts all
// Critical lines read, and BytesAttributed+BytesUnattributed equals
// the scanned bytes of readable files. A segment with any malformed
// line is held whole: its bytes go unattributed, never guessed onto
// a scope (StorageUnreadableRule on the quota axis).
type QuotaReport struct {
	Records           []QuotaRecord `json:"records"`
	KnownGaps         []string      `json:"known_gaps"`
	EventsSeen        int           `json:"events_seen"`
	CriticalSeen      int           `json:"critical_seen_events"`
	BytesAttributed   int64         `json:"bytes_attributed"`
	BytesUnattributed int64         `json:"bytes_unattributed"`
	SegmentsHeld      int           `json:"segments_held_unreadable"`
}

// quotaLine is the metadata-only read of one stored audit line:
// time, severity, agent identity, the stated task attr — nothing
// else. Logging Privacy §272 holds by construction because the
// decode shape has no field that could carry a payload.
type quotaLine struct {
	TS    time.Time
	Sev   schema.Severity
	Agent string
	Task  string
	bytes int
}

type quotaScope struct {
	inWindow int
	all      int
	critical int
	bytes    int64
}

type quotaBucket struct {
	agent map[string]*quotaScope
	task  map[string]*quotaScope
}

func newQuotaBucket() quotaBucket {
	return quotaBucket{agent: map[string]*quotaScope{}, task: map[string]*quotaScope{}}
}

// scanQuota reads one file's lines, metadata-only. ok=false means a
// malformed line was met: the caller holds the WHOLE file
// unattributed (fail-closed, nothing already parsed from it is
// counted — the partial read is discarded exactly like the
// governance readSegment discards its facts).
func scanQuota(path string) (lines []quotaLine, ok bool) {
	fh, err := os.Open(path)
	if err != nil {
		return nil, false
	}
	defer fh.Close()
	sc := bufio.NewScanner(fh)
	sc.Buffer(make([]byte, 0, 64*1024), 4*1024*1024)
	for sc.Scan() {
		raw := sc.Bytes()
		if len(trimSpaceBytes(raw)) == 0 {
			continue
		}
		var q quotaLine
		q.bytes = len(raw) + 1 // the newline the writer appends
		var probe struct {
			TS       time.Time         `json:"ts"`
			Severity schema.Severity   `json:"severity"`
			AgentID  string            `json:"agent_id"`
			Attrs    map[string]string `json:"attrs"`
		}
		if err := json.Unmarshal(raw, &probe); err != nil {
			return nil, false
		}
		if probe.TS.IsZero() || !probe.Severity.Valid() || probe.AgentID == "" {
			return nil, false
		}
		q.TS = probe.TS
		q.Sev = probe.Severity
		q.Agent = probe.AgentID
		if probe.Attrs != nil {
			q.Task = probe.Attrs[QuotaTaskAttrKey]
		}
		lines = append(lines, q)
	}
	if err := sc.Err(); err != nil {
		return nil, false
	}
	return lines, true
}

func trimSpaceBytes(b []byte) []byte {
	out := b
	for len(out) > 0 && (out[0] == ' ' || out[0] == '\t' || out[0] == '\n' || out[0] == '\r') {
		out = out[1:]
	}
	for len(out) > 0 && (out[len(out)-1] == ' ' || out[len(out)-1] == '\t' || out[len(out)-1] == '\n' || out[len(out)-1] == '\r') {
		out = out[:len(out)-1]
	}
	return out
}

func (b *quotaBucket) add(scope map[string]*quotaScope, id string, l quotaLine, inWindow bool) {
	if id == "" {
		return
	}
	s := scope[id]
	if s == nil {
		s = &quotaScope{}
		scope[id] = s
	}
	s.all++
	if inWindow {
		s.inWindow++
	}
	s.bytes += int64(l.bytes)
	if l.Sev == schema.SevCritical {
		s.critical++
	}
}

// ObserveQuota computes the per-agent/per-task quota observations
// for one pass. It reads only: no file is created, renamed, or
// deleted, and the write path is never consulted (record-only, the
// §270/§271 observation stance together).
func (l *Log) ObserveQuota(q Quota, now time.Time) QuotaReport {
	l.mu.Lock()
	defer l.mu.Unlock()
	rep := QuotaReport{}
	b := newQuotaBucket()
	rateWindowOK := q.WindowSeconds > 0
	var windowStart time.Time
	if rateWindowOK {
		windowStart = now.Add(-time.Duration(q.WindowSeconds) * time.Second)
	}
	paths := append([]string{}, l.segments()...)
	// The live head is scanned too (it is where the anomalous agent
	// is eating right now); reading it is safe — the append handle
	// and a sequential read do not conflict, and any torn tail line
	// fails closed into the held-not-guessed bucket like any
	// malformed segment.
	paths = append(paths, l.path)
	for _, p := range paths {
		lines, ok := scanQuota(p)
		if !ok {
			st, serr := os.Stat(p)
			size := int64(0)
			if serr == nil {
				size = st.Size()
			}
			rep.BytesUnattributed += size
			rep.SegmentsHeld++
			continue
		}
		var fileBytes int64
		for _, ln := range lines {
			fileBytes += int64(ln.bytes)
			rep.EventsSeen++
			if ln.Sev == schema.SevCritical {
				rep.CriticalSeen++
			}
			inWin := rateWindowOK && !ln.TS.Before(windowStart) && !ln.TS.After(now)
			b.add(b.agent, ln.Agent, ln, inWin)
			if ln.Task != "" {
				b.add(b.task, ln.Task, ln, inWin)
			}
		}
		rep.BytesAttributed += fileBytes
	}
	// Lines whose agent was present (contract requires it) but whose
	// task id was never stated are unattributed on the task axis.
	// State the gap once, never per line, never a default task.
	if rep.EventsSeen > 0 && len(b.task) == 0 {
		rep.KnownGaps = append(rep.KnownGaps, "no-stated-task_id-lines-task-dimension-unattributed")
	}

	emit := func(scope, kind string, observed func(*quotaScope) uint64) {
		var m map[string]*quotaScope
		if scope == QuotaScopeAgent {
			m = b.agent
		} else {
			m = b.task
		}
		ceiling, declared := q.ceiling(scope, kind)
		if !declared {
			rep.KnownGaps = append(rep.KnownGaps, "quota-ceiling-not-declared:"+scope+"/"+kind)
			return
		}
		if kind == QuotaKindEventRate && !rateWindowOK {
			rep.KnownGaps = append(rep.KnownGaps, "rate-window-not-stated:"+scope+"/"+kind)
			return
		}
		for id, s := range m {
			obs := observed(s)
			state := QuotaBelow
			switch {
			case obs > ceiling:
				state = QuotaAbove
			case obs == ceiling:
				state = QuotaAt
			}
			rec := QuotaRecord{
				Scope: scope, ScopeID: id, Kind: kind,
				Ceiling: ceiling, Observed: obs,
				State: state, Triggered: state != QuotaBelow,
				Response:     QuotaResponseRule,
				CriticalHeld: s.critical,
			}
			if rec.Triggered && s.critical > 0 {
				rec.Watermark = QuotaWatermarkToken
			}
			rep.Records = append(rep.Records, rec)
		}
	}
	emit(QuotaScopeAgent, QuotaKindEventRate, func(s *quotaScope) uint64 { return uint64(s.inWindow) })
	emit(QuotaScopeTask, QuotaKindEventRate, func(s *quotaScope) uint64 { return uint64(s.inWindow) })
	emit(QuotaScopeAgent, QuotaKindLogBytes, func(s *quotaScope) uint64 { return uint64(s.bytes) })
	emit(QuotaScopeTask, QuotaKindLogBytes, func(s *quotaScope) uint64 { return uint64(s.bytes) })
	return rep
}
