// Package mcpproxy is the minimal MCP stdio pass-through proxy: it
// relays newline-delimited JSON-RPC between a client (our stdin) and a
// server subprocess (its stdin/stdout) byte-for-byte while observing
// the traffic for audit records.
//
// Structural pass-through guarantee (Phase 0, machine-audited):
//
//   - forwarding happens FIRST and UNCONDITIONALLY: every line read is
//     written to the far side before any observation touches it;
//   - observation runs on a separate goroutine fed by a bounded queue;
//     a full queue DROPS observations (counted in Dropped) and can never
//     block, stall, or alter transport bytes;
//   - request/response pairing happens exclusively on that one observer
//     goroutine and is order-independent: a response inspected before
//     its request (legal, because the two relay pumps run concurrently
//     and a server may answer while the request is still in flight)
//     lands in a bounded hold map and completes when the request's
//     observation arrives, and vice versa;
//   - no response byte is ever derived from a judgement: there is no
//     code path here that reads audit output, and the package carries
//     no interception branch of any kind (the gate greps the shipped
//     sources and unit tests prove byte identity under dangerous calls
//     and under a wedged observer).
package mcpproxy

import (
	"bufio"
	"encoding/json"
	"io"
	"sync"
	"sync/atomic"
	"time"
)

// ToolsCallMethod is the only method audited per round trip.
const ToolsCallMethod = "tools/call"

// Record describes one completed tools/call round trip.
type Record struct {
	Tool      string
	ArgsRaw   []byte // raw JSON of params.arguments (may be nil)
	Latency   time.Duration
	ReqBytes  int
	RespBytes int
}

// Observer receives completed records. It is called on the observer
// goroutine only; implementations must not call back into the proxy.
type Observer func(Record)

// Proxy relays and observes. Create with New; drive with PumpPair.
type Proxy struct {
	observe Observer // may be nil (pure relay)

	mu      sync.Mutex
	pending map[string]*wait // JSON-RPC id (compact raw) -> open call
	late    map[string]*held // id -> response observed before its request

	queue   chan obsItem
	done    chan struct{}
	dropped atomic.Int64
	seen    atomic.Int64

	closed sync.Once
}

type wait struct {
	tool string
	args []byte
	t0   time.Time
	size int
}

// held remembers a response line whose request has not been observed
// yet (see the pairing note in the package doc).
type held struct {
	t    time.Time
	size int
}

type obsItem struct {
	line []byte
	t    time.Time
}

// New builds a relay with the given observer (nil = observe nothing).
func New(observe Observer) *Proxy {
	return &Proxy{
		observe: observe,
		pending: map[string]*wait{},
		late:    map[string]*held{},
		queue:   make(chan obsItem, 1024),
		done:    make(chan struct{}),
	}
}

// Dropped reports observations lost to queue overflow (transport never
// waits on observation, so overflow is possible under load and honest).
func (p *Proxy) Dropped() int64 { return p.dropped.Load() }

// Observed counts JSON lines inspected on both directions.
func (p *Proxy) Observed() int64 { return p.seen.Load() }

// relay copies src to dst line by line, enqueuing a copy of each line
// for observation AFTER the write succeeded. Returns the last transport
// error (io.EOF is success).
func (p *Proxy) relay(dst io.Writer, src io.Reader) error {
	br := bufio.NewReader(src)
	for {
		line, err := br.ReadBytes('\n')
		if len(line) > 0 {
			if werr := fullWrite(dst, line); werr != nil {
				return werr
			}
			p.enqueue(line)
		}
		if err != nil {
			if err == io.EOF {
				return nil
			}
			return err
		}
	}
}

func fullWrite(dst io.Writer, b []byte) error {
	for len(b) > 0 {
		n, err := dst.Write(b)
		if err != nil {
			return err
		}
		b = b[n:]
	}
	return nil
}

func (p *Proxy) enqueue(line []byte) {
	if p.observe == nil {
		return
	}
	cp := append([]byte(nil), line...)
	select {
	case p.queue <- obsItem{line: cp, t: time.Now()}:
	default:
		p.dropped.Add(1)
	}
}

// observeLoop drains the queue; started by Run, exits when the queue
// is closed after both relays finish, then signals done.
func (p *Proxy) observeLoop() {
	defer close(p.done)
	for item := range p.queue {
		p.inspect(item.line, item.t)
	}
}

type msg struct {
	ID     json.RawMessage `json:"id"`
	Method string          `json:"method"`
	Params struct {
		Name      string          `json:"name"`
		Arguments json.RawMessage `json:"arguments"`
	} `json:"params"`
}

// inspect parses one transport line best-effort. Parse failures are
// observations lost, never transport errors. It runs only on the
// observer goroutine, so the pairing maps need no cross-goroutine
// ordering: whichever side of a round trip is seen first holds its
// record until the other side arrives.
func (p *Proxy) inspect(line []byte, t time.Time) {
	p.seen.Add(1)
	var m msg
	if err := json.Unmarshal(line, &m); err != nil {
		return
	}
	key := string(m.ID)
	if key == "" || key == "null" {
		return // notification or server-initiated message: not paired
	}
	if m.Method == ToolsCallMethod {
		w := &wait{tool: m.Params.Name, args: append([]byte(nil), m.Params.Arguments...), t0: t, size: len(line)}
		p.mu.Lock()
		if h, ok := p.late[key]; ok {
			delete(p.late, key) // request observed after its response
			p.mu.Unlock()
			p.deliver(w, h)
			return
		}
		if len(p.pending) < 4096 {
			p.pending[key] = w
		} else {
			p.dropped.Add(1)
		}
		p.mu.Unlock()
		return
	}
	// A line carrying an id but no tools/call method is a response (or
	// an unrelated request we do not audit).
	p.mu.Lock()
	w, ok := p.pending[key]
	if ok {
		delete(p.pending, key)
	}
	if !ok {
		// Response before request: hold it (last write wins for
		// duplicate ids, which are illegal JSON-RPC input anyway;
		// losing that duplicate is an observation loss, never a
		// transport effect).
		if len(p.late) < 4096 {
			p.late[key] = &held{t: t, size: len(line)}
		} else {
			p.dropped.Add(1)
		}
	}
	p.mu.Unlock()
	if ok {
		p.deliver(w, &held{t: t, size: len(line)})
	}
}

// deliver emits the completed round-trip record for an open request and
// its (possibly earlier-observed) response. Observation order across
// the two relays is not guaranteed, so a negative skew clamps to zero:
// latency is honest only when the request was seen first.
func (p *Proxy) deliver(w *wait, h *held) {
	if p.observe == nil {
		return
	}
	lat := h.t.Sub(w.t0)
	if lat < 0 {
		lat = 0
	}
	p.observe(Record{
		Tool:      w.tool,
		ArgsRaw:   w.args,
		Latency:   lat,
		ReqBytes:  w.size,
		RespBytes: h.size,
	})
}

// Close is idempotent queue teardown used by Run.
func (p *Proxy) Close() {
	p.closed.Do(func() { close(p.queue) })
}

// Run relays in both directions until both streams end, then drains
// observations deterministically before returning. It reports the first
// transport error (nil = clean end of stream on both paths).
//
// Observation state can never influence transported bytes: relay writes
// the line before enqueueing the copy, the observer lives on its own
// goroutine with a bounded drop-overflow queue, and nothing in this
// package feeds any observed or audited result back into a write.
func (p *Proxy) Run(clientIn io.Reader, clientOut io.Writer, serverIn io.Writer, serverOut io.Reader) error {
	observe := p.observe != nil
	if observe {
		go p.observeLoop()
	}
	errs := make(chan error, 2)
	go func() {
		e := p.relay(serverIn, clientIn)
		// Upstream stream ended: half-close the server's input so a
		// child process can reach its own EOF and exit. Closing is
		// lifecycle only — no byte ever consults an observation, and
		// write-closers that are not io.Closers (unit buffers) simply
		// stay open until their reader hits EOF.
		if c, ok := serverIn.(io.Closer); ok {
			_ = c.Close()
		}
		errs <- e
	}() // client -> server
	go func() { errs <- p.relay(clientOut, serverOut) }() // server -> client
	e1 := <-errs
	e2 := <-errs
	p.Close()
	if observe {
		<-p.done // deterministic drain: every queued observation audited
	}
	if e1 != nil {
		return e1
	}
	return e2
}
