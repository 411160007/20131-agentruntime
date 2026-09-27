package mcpproxy

import (
	"bytes"
	"io"
	"strings"
	"sync"
	"testing"
	"time"
)

type safeBuf struct {
	mu sync.Mutex
	b  bytes.Buffer
}

func (s *safeBuf) Write(p []byte) (int, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.b.Write(p)
}
func (s *safeBuf) String() string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.b.String()
}

type recorder struct {
	mu   sync.Mutex
	recs []Record
}

func (r *recorder) observe(rec Record) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.recs = append(r.recs, rec)
}

func (r *recorder) snapshot() []Record {
	r.mu.Lock()
	defer r.mu.Unlock()
	return append([]Record(nil), r.recs...)
}

const req1 = `{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2024-11-05"}}` + "\n"
const resp1 = `{"jsonrpc":"2.0","id":1,"result":{"protocolVersion":"2024-11-05"}}` + "\n"
const req2 = `{"jsonrpc":"2.0","id":2,"method":"tools/call","params":{"name":"eval_sandbox","arguments":{"command":"rm -rf build"}}}` + "\n"
const resp2 = `{"jsonrpc":"2.0","id":2,"result":{"content":[{"type":"text","text":"ok"}]}}` + "\n"
const req3 = `{"jsonrpc":"2.0","id":3,"method":"tools/call","params":{"name":"read_file","arguments":{"file_path":"/tmp/x"}}}` + "\n"
const resp3 = `{"jsonrpc":"2.0","id":3,"result":{"content":[]}}` + "\n"

func TestRunRoundTripByteIdentityAndRecords(t *testing.T) {
	clientStream := req1 + req2 + req3
	serverStream := resp1 + resp2 + resp3

	var records recorder
	p := New(records.observe)
	serverSide := &safeBuf{} // what the server process would read
	clientSide := &safeBuf{} // what the client would read back
	if err := p.Run(strings.NewReader(clientStream), clientSide, serverSide, strings.NewReader(serverStream)); err != nil {
		t.Fatalf("run: %v", err)
	}
	if serverSide.String() != clientStream {
		t.Errorf("server received non-identical bytes:\n%q", serverSide.String())
	}
	if clientSide.String() != serverStream {
		t.Errorf("client received non-identical bytes:\n%q", clientSide.String())
	}
	recs := records.snapshot()
	if len(recs) != 2 {
		t.Fatalf("want 2 tools/call records, got %d", len(recs))
	}
	if recs[0].Tool != "eval_sandbox" || !strings.Contains(string(recs[0].ArgsRaw), "rm -rf") {
		t.Errorf("record 0: %+v", recs[0])
	}
	if recs[0].ReqBytes != len(req2) || recs[0].RespBytes != len(resp2) {
		t.Errorf("record sizes: %+v", recs[0])
	}
	if recs[1].Tool != "read_file" {
		t.Errorf("record 1: %+v", recs[1])
	}
	if recs[0].Latency < 0 {
		t.Errorf("negative latency")
	}
	if p.Dropped() != 0 {
		t.Errorf("unexpected drops: %d", p.Dropped())
	}
}

func TestMalformedLinesPassThroughUnharmed(t *testing.T) {
	junk := "this is not json\n" + `{"jsonrpc":"2.0","method":"notifications/progress"}` + "\n"
	var records recorder
	p := New(records.observe)
	serverSide := &safeBuf{}
	clientSide := &safeBuf{}
	if err := p.Run(strings.NewReader(junk), clientSide, serverSide, strings.NewReader("\n\n")); err != nil {
		t.Fatal(err)
	}
	if serverSide.String() != junk {
		t.Error("junk bytes altered on forward")
	}
	if clientSide.String() != "\n\n" {
		t.Error("blank lines altered")
	}
	if len(records.snapshot()) != 0 {
		t.Error("junk must not fabricate records")
	}
}

func TestRelayNeverBlocksOnWedgedObserver(t *testing.T) {
	// Structural proof for the transport guarantee: observation capacity
	// cannot back-pressure forwarding. Drive relay() directly with a
	// bounded queue and no drainer at all: every line beyond the queue
	// capacity must be DROPPED for observation while the relay still
	// completes and every byte arrives.
	block := make(chan struct{})
	defer close(block)
	p := New(func(Record) { <-block })
	var total = 1300 // > queue capacity (1024)
	var payload strings.Builder
	for i := 0; i < total; i++ {
		payload.WriteString(`{"jsonrpc":"2.0","id":`)
		payload.WriteString(itoa(i))
		payload.WriteString(`,"method":"tools/call","params":{"name":"t"}}` + "\n")
	}
	in := &safeBuf{}
	done := make(chan error, 1)
	go func() { done <- p.relay(in, strings.NewReader(payload.String())) }()
	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("relay errored: %v", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("relay BLOCKED on observation capacity (transport must never wait)")
	}
	if in.String() != payload.String() {
		t.Error("forwarded bytes differ under observer wedge")
	}
	if p.Dropped() == 0 {
		t.Error("overflow must be counted, not hidden")
	}
}

func TestPairingIsOrderIndependent(t *testing.T) {
	// The two relay pumps run concurrently: a response can legitimately
	// be observed before its request. Pairing must complete exactly once
	// in either arrival order, with latency clamped for the inverted case.
	var records recorder
	p := New(records.observe)
	tResp := time.Date(2026, 9, 28, 4, 0, 1, 0, time.UTC)
	p.inspect([]byte(resp2), tResp)
	if got := len(records.snapshot()); got != 0 {
		t.Fatalf("unpaired response must hold, not emit: %d records", got)
	}
	p.inspect([]byte(req2), tResp.Add(-time.Millisecond))
	recs := records.snapshot()
	if len(recs) != 1 || recs[0].Tool != "eval_sandbox" {
		t.Fatalf("inverted arrival did not pair: %+v", recs)
	}
	if recs[0].Latency != time.Millisecond || recs[0].ReqBytes != len(req2) || recs[0].RespBytes != len(resp2) {
		// both arrival stamps exist; the gap is honest even if the
		// request was inspected after the response (clamped at 0 if
		// the skew would be negative).
		t.Errorf("inverted record metrics: %+v", recs[0])
	}
	// Normal direction: request first, response 5ms later.
	tReq := time.Date(2026, 9, 28, 4, 0, 2, 0, time.UTC)
	p.inspect([]byte(req3), tReq)
	if got := len(records.snapshot()); got != 1 {
		t.Fatal("request alone must not emit")
	}
	p.inspect([]byte(resp3), tReq.Add(5*time.Millisecond))
	recs = records.snapshot()
	if len(recs) != 2 || recs[1].Tool != "read_file" || recs[1].Latency < 5*time.Millisecond {
		t.Fatalf("ordered arrival did not pair honestly: %+v", recs)
	}
}

func itoa(x int) string {
	if x == 0 {
		return "0"
	}
	var b [20]byte
	i := len(b)
	for x > 0 {
		x, i = x/10, i-1
		b[i] = byte('0' + x%10)
	}
	return string(b[i:])
}

func TestNilObserverPureRelay(t *testing.T) {
	p := New(nil)
	serverSide := &safeBuf{}
	clientSide := &safeBuf{}
	if err := p.Run(strings.NewReader(req2), clientSide, serverSide, strings.NewReader(resp2)); err != nil {
		t.Fatal(err)
	}
	if serverSide.String() != req2 || clientSide.String() != resp2 {
		t.Error("pure relay altered bytes")
	}
}

// compile-time interface sanity: relay halves must work on os.Pipe-like
// streams (io.Reader+io.Writer), verified by the cmd-level E2E.
var _ io.Reader = (*strings.Reader)(nil)
