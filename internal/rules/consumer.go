// Bus consumer adapter: the rule-engine delivery slot built by the
// event bus (tier L2/L3, in order, backpressure instead of drops)
// lands here. The consumer never mutates the source event and never
// touches anything the observed agent is doing — it renders decision
// lines and hands them to a caller-provided sink (the audit writer in
// production wiring, a recorder in tests).
//
// This is the consumption site the core completion slice reserved;
// today the collector's discovery source emits L1 observation lines
// (persist only), so the adapter runs attached in tests and wires into
// live traffic when the hook/mcp sources (their own slice) publish at
// L2 and above.
package rules

import "20131.com/agentruntime/internal/schema"

// StartConsumer drains in until it is closed or stop is called,
// deciding every delivered event and emitting its decision line
// through sink. Returns the stop function; calling it after the
// channel is closed is harmless.
func StartConsumer(eng *Engine, in <-chan *schema.Event, sink func(*schema.Event)) (stop func()) {
	done := make(chan struct{})
	stopped := make(chan struct{})
	go func() {
		defer close(stopped)
		for {
			select {
			case <-done:
				return
			case ev, ok := <-in:
				if !ok {
					return
				}
				d, err := eng.Decide(ev)
				if err != nil {
					continue // undecidable input is dropped, never forged
				}
				line, err := DecisionEvent(ev, d)
				if err != nil {
					continue
				}
				if sink != nil {
					sink(line)
				}
			}
		}
	}()
	var once int
	return func() {
		once++
		if once == 1 {
			close(done)
			<-stopped
		}
	}
}
