package bus

import (
	"fmt"
	"path/filepath"
	"strconv"
	"sync"
	"testing"
	"time"

	"20131.com/agentruntime/internal/auditlog"
	"20131.com/agentruntime/internal/schema"
)

func ev(id string, tier schema.Tier) *schema.Event {
	return &schema.Event{
		V:        schema.SchemaVersion,
		TS:       time.Date(2026, 9, 27, 12, 0, 0, 0, time.UTC),
		ID:       id,
		AgentID:  "agi-testsrc",
		Stage:    schema.StageObservation,
		Type:     schema.TypeAgentScan,
		Decision: schema.DecisionAllow,
		Severity: schema.SevInfo,
		Summary:  "bus probe " + id,
		Tier:     tier,
	}
}

// countingSink records order and tier of everything persisted.
type countingSink struct {
	ids   []string
	tiers []schema.Tier
	fail  error
}

func (s *countingSink) WriteEvent(e *schema.Event) error {
	if s.fail != nil {
		return s.fail
	}
	s.ids = append(s.ids, e.ID)
	s.tiers = append(s.tiers, e.Tier)
	return nil
}

func TestMultiSourceCountConservationAndOrder(t *testing.T) {
	sink := &countingSink{}
	b := New(sink)
	a, _ := b.Register("alpha")
	c, _ := b.Register("gamma")

	for i := 0; i < 5; i++ {
		if err := a.Publish(ev(fmt.Sprintf("a-%d", i), schema.TierL1)); err != nil {
			t.Fatal(err)
		}
	}
	// an L0 event is dropped but counted, never persisted
	if err := a.Publish(ev("a-drop", schema.TierL0)); err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 3; i++ {
		if err := c.Publish(ev(fmt.Sprintf("c-%d", i), schema.TierL1)); err != nil {
			t.Fatal(err)
		}
	}

	st := b.Stats()
	if st.Published != 9 || st.Persisted != 8 || st.DroppedL0 != 1 || st.Rejected != 0 {
		t.Fatalf("bus counters: %+v", st)
	}
	if len(sink.ids) != int(st.Persisted) {
		t.Fatalf("sink saw %d lines, bus says persisted %d", len(sink.ids), st.Persisted)
	}
	// per-source FIFO preserved through the fan-in pipe
	want := []string{"a-0", "a-1", "a-2", "a-3", "a-4", "c-0", "c-1", "c-2"}
	for i := range want {
		if sink.ids[i] != want[i] {
			t.Fatalf("order divergence at %d: got %v want %v", i, sink.ids, want)
		}
	}
	// per-source counters sum to totals
	var sumPub int64
	for _, si := range b.Sources() {
		sumPub += si.Published
	}
	if sumPub != st.Published {
		t.Fatalf("source counters sum %d != published %d", sumPub, st.Published)
	}
}

func TestConcurrentProducersPreservePerSourceFIFO(t *testing.T) {
	sink := &countingSink{}
	b := New(sink)
	const nsrc, nper = 4, 50
	srcs := make([]*Source, nsrc)
	for i := range srcs {
		s, err := b.Register("src-" + strconv.Itoa(i))
		if err != nil {
			t.Fatal(err)
		}
		srcs[i] = s
	}
	var wg sync.WaitGroup
	for i, s := range srcs {
		wg.Add(1)
		go func(i int, s *Source) {
			defer wg.Done()
			for j := 0; j < nper; j++ {
				if err := s.Publish(ev(fmt.Sprintf("s%d-%03d", i, j), schema.TierL1)); err != nil {
					t.Errorf("publish: %v", err)
				}
			}
		}(i, s)
	}
	wg.Wait()
	if got := len(sink.ids); got != nsrc*nper {
		t.Fatalf("persisted %d, want %d", got, nsrc*nper)
	}
	// per-source subsequence order must be ascending (deterministic FIFO)
	last := map[int]int{}
	for _, id := range sink.ids {
		var si, j int
		if _, err := fmt.Sscanf(id, "s%d-%d", &si, &j); err != nil {
			t.Fatalf("bad id %q", id)
		}
		prev, seen := last[si]
		if !seen {
			prev = -1
		}
		if j <= prev {
			t.Fatalf("source %d emitted out of order: %s after %d", si, id, prev)
		}
		last[si] = j
	}
}

func TestSlowEngineConsumerLosesNothing(t *testing.T) {
	sink := &countingSink{}
	b := New(sink)
	s, _ := b.Register("prod")

	// tiny buffer on purpose: the consumer reads slower than production
	// (sleeps), so non-blocking drop semantics would lose events.
	ch := make(chan *schema.Event, 2)
	detach := b.AttachEngine(ch)
	defer detach()
	done := make(chan int, 1)
	go func() {
		count := 0
		for range ch {
			time.Sleep(time.Millisecond) // slow consumer
			count++
		}
		done <- count
	}()

	const n = 25
	for i := 0; i < n; i++ {
		if err := s.Publish(ev(fmt.Sprintf("e-%d", i), schema.TierL2)); err != nil {
			t.Fatal(err)
		}
	}
	// L1 must not be forwarded to the engine
	if err := s.Publish(ev("e-l1", schema.TierL1)); err != nil {
		t.Fatal(err)
	}
	close(ch)
	got := <-done
	if got != n {
		t.Fatalf("engine consumer got %d, want %d (slow consumer lost events)", got, n)
	}
	st := b.Stats()
	if st.Persisted != n+1 || st.Delivered != n {
		t.Fatalf("counters: %+v", st)
	}
}

func TestReservedSlotsRejectAndCount(t *testing.T) {
	sink := &countingSink{}
	b := New(sink)
	hook, err := b.RegisterReserved("hook")
	if err != nil {
		t.Fatal(err)
	}
	if err := hook.Publish(ev("x", schema.TierL1)); err != ErrReserved {
		t.Fatalf("reserved slot accepted a publish: %v", err)
	}
	if len(sink.ids) != 0 {
		t.Fatal("reserved slot persisted something")
	}
	st := b.Stats()
	if st.Rejected != 1 || st.Published != 0 {
		t.Fatalf("reserved counters: %+v", st)
	}
	var found *SourceInfo
	for _, si := range b.Sources() {
		if si.Name == "hook" {
			s := si
			found = &s
		}
	}
	if found == nil || found.State != StateReserved || found.Rejected != 1 {
		t.Fatalf("registry row for reserved slot wrong: %+v", found)
	}
}

func TestInvalidEventRejectedBeforeSink(t *testing.T) {
	// reuse the D1 zero-byte guarantee end to end: an invalid event must
	// not move a single byte into a real audit file.
	path := filepath.Join(t.TempDir(), "audit.jsonl")
	lg, err := auditlog.OpenLog(path, nil)
	if err != nil {
		t.Fatal(err)
	}
	b := New(lg)
	s, _ := b.Register("live")
	bad := ev("bad", schema.TierL1)
	bad.Summary = "" // violates the field contract
	if err := s.Publish(bad); err == nil {
		t.Fatal("invalid event accepted")
	}
	st := b.Stats()
	if st.Published != 0 || st.Rejected != 1 || st.Persisted != 0 {
		t.Fatalf("invalid-event counters: %+v", st)
	}
	if err := lg.Close(); err != nil {
		t.Fatal(err)
	}
	// the writer's start-of-file invariant still holds (empty file only)
	lg2, err := auditlog.OpenLog(path, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer lg2.Close()
	if lg2.Bytes() != 0 {
		t.Fatalf("invalid event wrote %d bytes", lg2.Bytes())
	}
}

func TestPhase0DecisionGateInBus(t *testing.T) {
	sink := &countingSink{}
	b := New(sink)
	s, _ := b.Register("live")
	ask := ev("ask-line", schema.TierL1)
	ask.Decision = schema.DecisionAsk // valid contract vocabulary...
	if err := s.Publish(ask); err == nil {
		t.Fatal("bus persisted an ask decision: phase-0 runtime set violated")
	}
	if len(sink.ids) != 0 {
		t.Fatal("ask event reached the sink")
	}
	// would_block IS allowed (recorded, never enforced)
	wb := ev("wb", schema.TierL1)
	wb.Decision = schema.DecisionWouldBlock
	if err := s.Publish(wb); err != nil {
		t.Fatalf("would_block rejected: %v", err)
	}
}

func TestDuplicateRegisterFailsClosed(t *testing.T) {
	b := New(&countingSink{})
	if _, err := b.Register("x"); err != nil {
		t.Fatal(err)
	}
	if _, err := b.Register("x"); err == nil {
		t.Fatal("duplicate source name accepted")
	}
}
