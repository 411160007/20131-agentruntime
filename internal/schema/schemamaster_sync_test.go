package schema

// Master table sync tests (stability wave, closing slice). They pin the
// twenty-eight cell census of docs/schema-v2.md section 11 as a second,
// independent implementation: same cell set, same closed three-state
// machine, same sufficiency thresholds as the Node structural check, and
// an explicit creator-slice map so a docs-only rewrite cannot satisfy
// both implementations by accident. Schema and documentation evolution
// only: nothing here changes runtime decision semantics.

import (
	"os"
	"regexp"
	"strings"
	"testing"
)

var masterWaveCells = []string{
	"decision", "intent", "authority", "impact", "recovery", "evidence", "profile",
}

var masterWaveElements = []string{"version", "compatibility", "migration", "validation"}

// masterWaveCreators records the wave history: which build slice wrote
// which schema's element sections. Changing a schema's contract without
// moving this map (and the docs evidence lines together) is drift.
var masterWaveCreators = map[string]string{
	"decision":  "W2.1",
	"intent":    "W2.2",
	"authority": "W2.2",
	"impact":    "W2.3",
	"recovery":  "W2.3",
	"evidence":  "W2.4",
	"profile":   "W2.4",
}

func schemaV2Doc(t *testing.T) string {
	t.Helper()
	raw, err := os.ReadFile(filepathJoinDotDot("docs", "schema-v2.md"))
	if err != nil {
		t.Skipf("schema-v2 doc not visible: %v", err)
	}
	return string(raw)
}

func masterCells(t *testing.T, doc string) []map[string]string {
	t.Helper()
	var out []map[string]string
	for _, kv := range schemaV2Blocks(t, doc) {
		if _, ok := kv["master_cell"]; ok {
			out = append(out, kv)
		}
	}
	return out
}

func TestSchemaMasterCellCensus(t *testing.T) {
	doc := schemaV2Doc(t)
	cells := masterCells(t, doc)
	if len(cells) != 28 {
		t.Fatalf("master cell census %d, want 28", len(cells))
	}
	seen := map[string]bool{}
	states := map[string]bool{"preexisting": true, "this_wave": true, "planned_build": true}
	for _, kv := range cells {
		cell := strings.TrimSpace(kv["master_cell"])
		if seen[cell] {
			t.Errorf("duplicate master_cell: %s", cell)
			continue
		}
		seen[cell] = true
		parts := strings.Split(cell, "/")
		if len(parts) != 2 {
			t.Errorf("malformed master_cell %q", cell)
			continue
		}
		schemaName, element := parts[0], parts[1]
		if !masterContains(masterWaveCells, schemaName) {
			t.Errorf("master_cell on off-wave schema: %s", cell)
			continue
		}
		if !masterContains(masterWaveElements, element) {
			t.Errorf("master_cell with unknown element: %s", cell)
			continue
		}
		st, ok := kv["state"]
		if !ok || !states[st] {
			t.Errorf("wild master_cell state at %s: %q", cell, st)
			continue
		}
		ev := kv["evidence"]
		if strings.TrimSpace(ev) == "" {
			t.Errorf("master_cell without evidence: %s", cell)
			continue
		}
		switch st {
		case "planned_build":
			if !regexp.MustCompile(`\bW\d+\.\d+\b`).MatchString(ev) {
				t.Errorf("planned_build cell missing W commitment point: %s", cell)
			}
		case "this_wave":
			if !strings.Contains(ev, masterWaveCreators[schemaName]) {
				t.Errorf("this_wave cell %s missing creator %s", cell, masterWaveCreators[schemaName])
			}
		case "preexisting":
			if !strings.Contains(ev, "api-v0") {
				t.Errorf("preexisting cell must point at api-v0: %s", cell)
			}
		}
	}
	for _, s := range masterWaveCells {
		for _, e := range masterWaveElements {
			if !seen[s+"/"+e] {
				t.Errorf("missing master_cell: %s/%s", s, e)
			}
		}
	}
}

func TestSchemaMasterCellSufficiency(t *testing.T) {
	doc := schemaV2Doc(t)
	for _, kv := range masterCells(t, doc) {
		if kv["state"] == "planned_build" {
			continue
		}
		cell := kv["master_cell"]
		parts := strings.Split(cell, "/")
		if len(parts) != 2 {
			continue
		}
		schemaName, element := parts[0], parts[1]
		if !masterContains(masterWaveCells, schemaName) {
			continue
		}
		head := regexp.MustCompile("(?m)^## \\d+\\. ([A-Za-z]+) schema - complete contract").
			FindAllStringSubmatchIndex(doc, -1)
		var body string
		found := false
		for _, h := range head {
			if strings.ToLower(doc[h[2]:h[3]]) != schemaName {
				continue
			}
			found = true
			start := h[1]
			end := len(doc)
			if j := regexp.MustCompile("(?m)^## ").FindStringIndex(doc[start:]); j != nil {
				end = start + j[0]
			}
			body = doc[start:end]
		}
		if !found {
			t.Errorf("cell %s: no complete-contract section for schema", cell)
			continue
		}
		el := strings.ToUpper(element[:1]) + element[1:]
		k := strings.Index(body, "#### "+el+"\n")
		if k < 0 {
			t.Errorf("cell %s: element section missing", cell)
			continue
		}
		rest := body[k+len("#### "+el+"\n"):]
		if m := regexp.MustCompile("(?m)^#{2,4} ").FindStringIndex(rest); m != nil {
			rest = rest[:m[0]]
		}
		sec := body[k : k+len("#### "+el+"\n")+len(rest)]
		lines := 0
		listItem := false
		for _, l := range strings.Split(sec, "\n") {
			if strings.TrimSpace(l) == "" {
				continue
			}
			lines++
			if strings.HasPrefix(strings.TrimSpace(l), "- ") {
				listItem = true
			}
		}
		spans := regexp.MustCompile("`[^`\n]+`").FindAllString(sec, -1)
		nonSpace := len(strings.Join(strings.Fields(sec), ""))
		if lines < 3 || !listItem || len(spans) < 2 || nonSpace < 120 {
			t.Errorf("cell %s target not substantive: lines=%d list=%v spans=%d chars=%d",
				cell, lines, listItem, len(spans), nonSpace)
		}
	}
}

func TestSchemaMasterFifteenNameCoverage(t *testing.T) {
	doc := schemaV2Doc(t)
	re := regexp.MustCompile("(?m)^### 11\\.1 Coverage against the fifteen schema names")
	start := re.FindStringIndex(doc)
	if start == nil {
		t.Fatal("fifteen-name coverage note missing")
	}
	rest := doc[start[0]:]
	if j := regexp.MustCompile("(?m)^### ").FindStringIndex(rest[1:]); j != nil {
		rest = rest[:j[0]+1]
	}
	for _, name := range []string{
		"Event", "Identity", "Capability", "Policy", "Risk",
		"Agent Control API", "Platform Adapter API", "External Intelligence API",
	} {
		if !strings.Contains(rest, "`"+name+"`") {
			t.Errorf("off-wave schema name missing from coverage note: %s", name)
		}
	}
}

func masterContains(hay []string, needle string) bool {
	for _, h := range hay {
		if h == needle {
			return true
		}
	}
	return false
}
