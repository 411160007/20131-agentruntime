package rules

import (
	"encoding/json"
	"sync"
	"testing"
	"time"

	"20131.com/agentruntime/internal/bus"
	"20131.com/agentruntime/internal/schema"
)

// recSink records everything the bus persists (test double for the
// audit writer; the real wiring uses auditlog directly).
type recSink struct {
	mu    sync.Mutex
	lines []*schema.Event
}

func (r *recSink) WriteEvent(e *schema.Event) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.lines = append(r.lines, e)
	return nil
}

func (r *recSink) count() int {
	r.mu.Lock()
	defer r.mu.Unlock()
	return len(r.lines)
}

// TestBusEngineDelivery proves the reserved consumption site end to
// end: an L2 event published on the bus is persisted once, delivered
// in order to the attached engine consumer, and renders a validated
// policy.decision audit line (would_block, hard annotation carried).
func TestBusEngineDelivery(t *testing.T) {
	sink := &recSink{}
	b := bus.New(sink)
	engineCh := make(chan *schema.Event, 8)
	eng := MustDefault()
	var emitted []*schema.Event
	var muE sync.Mutex
	stop := StartConsumer(eng, engineCh, func(e *schema.Event) {
		muE.Lock()
		emitted = append(emitted, e)
		muE.Unlock()
	})
	defer stop()
	detach := b.AttachEngine(engineCh)
	defer detach()

	s2, err := b.Register("test-observe")
	if err != nil {
		t.Fatal(err)
	}
	mk := func(id, path string) *schema.Event {
		return ev(t, id, schema.TypeFileAccess, map[string]string{"path": path})
	}
	l2 := mk("b1", "/home/dev/.ssh/id_rsa")
	l2.Tier = schema.TierL2
	l1 := mk("b2", "/home/dev/docs/readme.md")
	l1.Tier = schema.TierL1
	if err := s2.Publish(l2); err != nil {
		t.Fatal(err)
	}
	if err := s2.Publish(l1); err != nil {
		t.Fatal(err)
	}
	// delivery is synchronous on the bus side (blocking send); the
	// consumer goroutine needs a scheduling tick.
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		muE.Lock()
		n := len(emitted)
		muE.Unlock()
		if n >= 1 {
			break
		}
		time.Sleep(time.Millisecond)
	}
	muE.Lock()
	defer muE.Unlock()
	if len(emitted) != 1 {
		t.Fatalf("engine saw %d events, want exactly 1 (only L2 is delivered)", len(emitted))
	}
	d := emitted[0]
	if d.Type != schema.TypePolicyDecision || d.Decision != schema.DecisionWouldBlock || d.Attrs["rule"] != "cred.ssh" || d.Attrs["hard"] != "true" {
		t.Errorf("decision line wrong: %+v", d)
	}
	if err := d.Validate(); err != nil {
		t.Errorf("decision line must validate: %v", err)
	}
	if sink.count() != 2 {
		t.Errorf("bus persisted %d lines, want 2 (L1+L2, decisions via sink callback not re-fed)", sink.count())
	}
	st := b.Stats()
	if st.Delivered != 1 {
		t.Errorf("bus delivered counter %d, want 1", st.Delivered)
	}
	// JSON round trip: the decision line must serialize as one valid
	// audit JSONL record (what the production sink writes).
	raw, err := json.Marshal(d)
	if err != nil {
		t.Fatal(err)
	}
	var back schema.Event
	if err := json.Unmarshal(raw, &back); err != nil || back.ID != d.ID {
		t.Errorf("decision line round trip broken: %v", err)
	}
}

// TestMultiMatchDeterminism pins that an event shaped to hit several
// built-in rules resolves deterministically to the earliest
// hard-rule in declaration order (documented priority semantics).
func TestMultiMatchDeterminism(t *testing.T) {
	eng := MustDefault()
	e := ev(t, "m1", schema.TypeToolCall, map[string]string{
		"path":    "/root/.aws/credentials",
		"cmdline": "rm -rf /mnt/data && mkfs.xfs /dev/sdc",
	})
	d1, err := eng.Decide(e)
	if err != nil || d1.RuleID != "cred.aws" {
		t.Fatalf("multi-match first decision wrong: %+v err=%v", d1, err)
	}
	d2, _ := eng.Decide(e)
	if d1.RuleID != d2.RuleID || d1.Value != schema.DecisionWouldBlock || !d1.Hard {
		t.Errorf("multi-match not deterministic/hard: %+v", d2)
	}
}
