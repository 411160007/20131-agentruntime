// mcpfixture is a deterministic canned MCP-shaped stdio server used by
// gate-d6 to prove the relay passes bytes through untouched: it answers
// every request with a fixed response table (including "successful"
// answers to dangerous-looking calls: a server behind a Phase 0 relay
// is never refused anything), stays silent on notifications, and exits
// cleanly at stdin EOF.
package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"strings"
)

// responses are keyed by request id; every value line is emitted
// verbatim on match (byte-exact table, not re-generated JSON).
var responses = map[string]string{
	`1`: `{"jsonrpc":"2.0","id":1,"result":{"protocolVersion":"2025-03-26","serverInfo":{"name":"mcpfixture"}}}`,
	`2`: `{"jsonrpc":"2.0","id":2,"result":{"content":[{"type":"text","text":"hello back"}]}}`,
	`3`: `{"jsonrpc":"2.0","id":3,"result":{"content":[{"type":"text","text":"the relay must not stop this either"}]}}`,
	`4`: `{"jsonrpc":"2.0","id":4,"result":{"content":[{"type":"text","text":"contents of the secrets file"}]}}`,
}

func main() {
	// Flush-free unbuffered stdout writes: every response line lands on
	// the pipe immediately, byte-for-byte.
	scan := bufio.NewScanner(os.Stdin)
	// 8MB tolerance deliberately exceeds the gate's oversized probe: the
	// fixture must not be the weak link when the relay is under test.
	scan.Buffer(make([]byte, 8<<20), 8<<20)
	for scan.Scan() {
		line := scan.Bytes()
		var req struct {
			ID     json.RawMessage `json:"id"`
			Method string          `json:"method"`
		}
		if err := json.Unmarshal(line, &req); err != nil {
			continue
		}
		if req.Method == "" || strings.HasPrefix(req.Method, "notifications/") || len(req.ID) == 0 {
			continue // notifications get no reply
		}
		if resp, ok := responses[string(req.ID)]; ok {
			fmt.Fprintln(os.Stdout, resp)
		}
	}
}
