package schema

import (
	"encoding/json"
	"strings"
	"testing"
)

// Wire tests for the memory-provenance observation field riding the
// event contract (slice W12.2 follow-up). The field is record-only:
// a well-stated record builds, a rejected input invalidates the whole
// event line and leaves no half record behind, and absence is
// single-meaning — no shipped rule reads the field, so a recorded
// claim never infers a grant (Memory is not Authorization).

func memoryWireEvent(t *testing.T, raw string) *Event {
	t.Helper()
	var e Event
	if err := json.Unmarshal([]byte(raw), &e); err != nil {
		t.Fatalf("wire must decode (field is part of the contract): %v", err)
	}
	return &e
}

func TestMemoryWireValidRecordBuilds(t *testing.T) {
	e := memoryWireEvent(t, `{"v":1,"ts":"2026-10-14T06:30:00Z","id":"gw-1","agent_id":"agi-wire","stage":"action","type":"file.access","decision":"allow","severity":0,"summary":"write long-term memory note","tier":"L2","memory_provenance":{"origin":"recorder-cli","author":"agi-wire","timestamp":"2026-10-14T06:30:00Z","trust":"stated-by-recorder","scope":"workspace:memory","ttl":"90d","integrity":"checked-on-write","surface":"long_term_memory","reverification":"reverified"}}`)
	if err := e.Validate(); err != nil {
		t.Fatalf("complete stated record must validate: %v", err)
	}
	if e.Memory == nil {
		t.Fatal("memory_provenance must decode into the typed input")
	}
	rec, err := BuildMemoryProvenance(*e.Memory)
	if err != nil {
		t.Fatalf("validated event record must build: %v", err)
	}
	if rec.MemoryNotAuthorization != MemoryNotAuthorizationRule || rec.EnforcementPlane != MemoryEnforcementPlane {
		t.Fatal("constructor-pinned restatements must come from the constructor, never the wire")
	}
}

func TestMemoryWireRejectionsLeaveNoHalfRecord(t *testing.T) {
	base := `{"v":1,"ts":"2026-10-14T06:30:00Z","id":"gw-2","agent_id":"agi-wire","stage":"action","type":"file.access","decision":"allow","severity":0,"summary":"scratchpad note quoting a prior approval","tier":"L2","memory_provenance":%s}`
	for name, mem := range map[string]string{
		"empty attribute": `{"origin":"  ","author":"a","timestamp":"t","trust":"x","scope":"s","ttl":"1d","integrity":"i","surface":"scratchpad","reverification":"not_reverified"}`,
		"surface off-set": `{"origin":"o","author":"a","timestamp":"t","trust":"x","scope":"s","ttl":"1d","integrity":"i","surface":"dream_store","reverification":"reverified"}`,
		"silent reverif":  `{"origin":"o","author":"a","timestamp":"t","trust":"x","scope":"s","ttl":"1d","integrity":"i","surface":"scratchpad","reverification":""}`,
		"empty object":    `{}`,
	} {
		e := memoryWireEvent(t, strings.Replace(base, "%s", mem, 1))
		if err := e.Validate(); err == nil {
			t.Fatalf("%s: event must be rejected, not degraded", name)
		}
	}
}

func TestMemoryWireAbsentIsNeverInferred(t *testing.T) {
	e := memoryWireEvent(t, `{"v":1,"ts":"2026-10-14T06:30:00Z","id":"gw-3","agent_id":"agi-wire","stage":"observation","type":"session.start","decision":"allow","severity":0,"summary":"plain session line"}`)
	if e.Memory != nil {
		t.Fatal("absent memory_provenance must decode to nil — silence is not a record")
	}
	if err := e.Validate(); err != nil {
		t.Fatalf("legacy-shaped line must still validate: %v", err)
	}
}
