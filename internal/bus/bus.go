// Package bus is the core event bus: an in-process fan-in pipeline that
// merges events from multiple registered sources into ONE audit sink (the
// local JSONL writer), applying tier routing along the way.
//
// Tier semantics (additive schema extension):
//
//	L0  dropped at the bus; only counted, never persisted
//	L1  persisted directly (the pre-tier default path)
//	L2/L3 persisted AND delivered in order to every attached rule-engine
//	    consumer (the security judgement layer attaches there; with no
//	    consumer attached, delivery is a bounded no-op beyond counters)
//
// Sources: discovery snapshot producers register actively; the hook and
// mcp adapter surfaces register as RESERVED slots — visible in the
// registry so the wiring is real, but rejecting events until their slice
// ships. Reserved slots are never silently dropped: every publish attempt
// against one increments that source's Rejected counter.
//
// Guarantees pinned by unit tests:
//   - count conservation: Published == Persisted + DroppedL0 + Rejected,
//     observed through the sink as well;
//   - deterministic order: per-source FIFO preserved through the sink,
//     including under concurrent producers;
//   - slow consumers lose nothing: engine delivery is a blocking send on
//     a bounded channel (backpressure instead of drops), and the audit
//     persist never starves while consumers lag;
//   - zero network: the only sink used by the collector writes local
//     files (import closure is os + encoding/json at most).
package bus

import (
	"errors"
	"fmt"
	"sync"

	"20131.com/agentruntime/internal/schema"
)

// ErrReserved is returned when an event is published to a reserved
// (not-yet-live) source slot.
var ErrReserved = errors.New("bus: source slot is reserved and not active")

// Sink is the persistence exit — the same contract both auditlog.Writer
// and auditlog.Log already satisfy.
type Sink interface {
	WriteEvent(e *schema.Event) error
}

// SourceState distinguishes live from reserved registry slots.
type SourceState string

const (
	StateActive   SourceState = "active"
	StateReserved SourceState = "reserved"
)

// SourceInfo is a snapshot row of the registry for status surfaces.
type SourceInfo struct {
	Name      string
	State     SourceState
	Published int64
	Rejected  int64
	DroppedL0 int64
	LastSeq   int64
}

// Source is a handle held by one producer.
type Source struct {
	b    *Bus
	name string

	mu       sync.Mutex
	reserved bool
	seq      int64
	pub      int64
	rej      int64
	dropL0   int64
}

// Name returns the registry key.
func (s *Source) Name() string { return s.name }

// Publish validates and routes one event. The sequence number accepted
// here (under the bus lock) defines the global order the sink observes.
func (s *Source) Publish(e *schema.Event) error {
	if s == nil || e == nil {
		return errors.New("bus: nil publish")
	}
	s.mu.Lock()
	reserved := s.reserved
	s.mu.Unlock()

	s.b.mu.Lock()
	defer s.b.mu.Unlock()

	s.b.seq++ // acceptance order == global deterministic order
	if reserved {
		s.b.rejected++
		s.bump(0, 1, 0)
		return ErrReserved
	}
	if err := e.Validate(); err != nil {
		s.b.rejected++
		s.b.rejectedBy[s.name]++
		s.bump(0, 1, 0)
		return err
	}
	// Phase 0 emission invariant: runtime decisions stay inside
	// {allow, would_block}. Invalid events never reach the sink.
	if err := schema.MustPhase0Decision(e.Decision); err != nil {
		s.b.rejected++
		s.b.rejectedBy[s.name]++
		s.bump(0, 1, 0)
		return err
	}
	s.b.published++
	s.bump(1, 0, 0)

	switch e.Tier.EffectiveTier() {
	case schema.TierL0:
		s.b.droppedL0++
		s.bump(0, 0, 1)
		return nil
	default:
		if err := s.b.sink.WriteEvent(e); err != nil {
			s.b.persistErr++
			return fmt.Errorf("bus: sink: %w", err)
		}
		s.b.persisted++
	}
	if e.Tier == schema.TierL2 || e.Tier == schema.TierL3 {
		s.b.delivered++
		for _, ch := range s.b.engine {
			ch <- e // blocking: slow consumers get backpressure, never loss
		}
	}
	return nil
}

// bump updates per-source counters; must be called with b.mu held so
// source counters share the bus serialization point (no torn totals).
func (s *Source) bump(pub, rej, drop int64) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.seq++
	s.pub += pub
	s.rej += rej
	s.dropL0 += drop
}

// Stats is a point-in-time copy of bus counters.
type Stats struct {
	Published   int64
	Persisted   int64
	DroppedL0   int64
	Rejected    int64
	Delivered   int64 // tier L2/L3 hand-offs to engine consumers
	PersistErrs int64
	Seq         int64
}

// Bus wires sources, tier routing, the audit sink, and engine consumers.
// Not safe for concurrent use of lifecycle methods (Register/Attach);
// Publish is safe from many goroutines.
type Bus struct {
	mu   sync.Mutex
	sink Sink
	seq  int64

	published, persisted, droppedL0, rejected, delivered, persistErr int64
	rejectedBy                                                       map[string]int64

	sources map[string]*Source
	engine  []chan *schema.Event
}

// New builds a bus over one sink (the single-pipe guarantee: exactly the
// audit writer given here, for every source).
func New(sink Sink) *Bus {
	return &Bus{sink: sink, sources: map[string]*Source{}, rejectedBy: map[string]int64{}}
}

// Register adds a live source. Duplicate names fail closed.
func (b *Bus) Register(name string) (*Source, error) {
	return b.register(name, false)
}

// RegisterReserved adds a wired-but-inert slot (hook/mcp surfaces until
// their slice ships). Publishes against it are rejected and counted.
func (b *Bus) RegisterReserved(name string) (*Source, error) {
	return b.register(name, true)
}

func (b *Bus) register(name string, reserved bool) (*Source, error) {
	if name == "" {
		return nil, errors.New("bus: empty source name")
	}
	b.mu.Lock()
	defer b.mu.Unlock()
	if _, dup := b.sources[name]; dup {
		return nil, fmt.Errorf("bus: source %q already registered", name)
	}
	s := &Source{b: b, name: name, reserved: reserved}
	b.sources[name] = s
	return s, nil
}

// AttachEngine adds a rule-engine consumer channel (capacity is the
// caller's buffer; sends block when full = backpressure not drops).
// Returns the unsubscribe closure.
func (b *Bus) AttachEngine(ch chan *schema.Event) func() {
	if ch == nil {
		return func() {}
	}
	b.mu.Lock()
	b.engine = append(b.engine, ch)
	b.mu.Unlock()
	return func() {
		b.mu.Lock()
		defer b.mu.Unlock()
		for i, c := range b.engine {
			if c == ch {
				b.engine = append(b.engine[:i], b.engine[i+1:]...)
				return
			}
		}
	}
}

// Stats returns current counters.
func (b *Bus) Stats() Stats {
	b.mu.Lock()
	defer b.mu.Unlock()
	return Stats{
		Published: b.published, Persisted: b.persisted, DroppedL0: b.droppedL0,
		Rejected: b.rejected, Delivered: b.delivered, PersistErrs: b.persistErr, Seq: b.seq,
	}
}

// Sources lists registry rows in insertion-independent (sorted-by-name)
// deterministic order for status rendering.
func (b *Bus) Sources() []SourceInfo {
	b.mu.Lock()
	defer b.mu.Unlock()
	out := make([]SourceInfo, 0, len(b.sources))
	for _, s := range b.sources {
		s.mu.Lock()
		out = append(out, SourceInfo{
			Name: s.name,
			State: func() SourceState {
				if s.reserved {
					return StateReserved
				}
				return StateActive
			}(),
			Published: s.pub, Rejected: s.rej, DroppedL0: s.dropL0, LastSeq: s.seq,
		})
		s.mu.Unlock()
	}
	sortSourceInfos(out)
	return out
}

func sortSourceInfos(v []SourceInfo) {
	for i := 1; i < len(v); i++ {
		for j := i; j > 0 && v[j].Name < v[j-1].Name; j-- {
			v[j], v[j-1] = v[j-1], v[j]
		}
	}
}
