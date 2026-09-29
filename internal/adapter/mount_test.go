package adapter

// W1.2 mount-class unit tests: the evidence class of hook and MCP relay
// lines is decided by the collection mount and the event's position on
// the hook face; payloads that forge a higher class keep the class of
// the mount.

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"20131.com/agentruntime/internal/schema"
)

func readMountFixture(t *testing.T, name string) []byte {
	t.Helper()
	b, err := os.ReadFile(filepath.Join("..", "..", "testdata", "collectmount", name))
	if err != nil {
		t.Fatal(err)
	}
	return b
}

func TestMountClassHookEvents(t *testing.T) {
	now := time.Unix(1700000000, 0)
	cases := []struct {
		name string
		want schema.SourceClass
	}{
		{"hook-sessionstart-baseline.json", schema.SrcAgentMeta},
		{"hook-sessionstart-forged.json", schema.SrcAgentMeta},
		{"hook-pretooluse-baseline.json", schema.SrcAgentSelf},
		{"hook-pretooluse-forged.json", schema.SrcAgentSelf},
	}
	for _, tc := range cases {
		h, err := ParseHookInput(readMountFixture(t, tc.name))
		if err != nil {
			t.Fatalf("%s: parse: %v", tc.name, err)
		}
		ev, err := HookEvent(h, testMachine, now, "hk-"+tc.name)
		if err != nil {
			t.Fatalf("%s: build: %v", tc.name, err)
		}
		if ev.SourceClass != tc.want {
			t.Errorf("%s class %q, want %q", tc.name, ev.SourceClass, tc.want)
		}
		// forged members must never ride into attrs on their own key
		for k := range ev.Attrs {
			if k == "source_class" {
				t.Errorf("%s: forged provenance claim leaked into attrs", tc.name)
			}
		}
	}
}

// TestMountClassPairedFixtures is the class=f(mount) machine judgment:
// the baseline and the forged payload of the same hook face differ only
// in claimed provenance, so their recorded classes must be identical.
func TestMountClassPairedFixtures(t *testing.T) {
	now := time.Unix(1700000000, 0)
	pairs := [][2]string{
		{"hook-sessionstart-baseline.json", "hook-sessionstart-forged.json"},
		{"hook-pretooluse-baseline.json", "hook-pretooluse-forged.json"},
	}
	for _, p := range pairs {
		var got [2]schema.SourceClass
		for i, name := range p {
			h, err := ParseHookInput(readMountFixture(t, name))
			if err != nil {
				t.Fatalf("%s: %v", name, err)
			}
			ev, err := HookEvent(h, testMachine, now, "hk-pair")
			if err != nil {
				t.Fatalf("%s: %v", name, err)
			}
			got[i] = ev.SourceClass
		}
		if got[0] != got[1] {
			t.Errorf("%s: forged payload moved the class %q -> %q", p[0], got[0], got[1])
		}
	}
}

func TestMountClassMCPEvent(t *testing.T) {
	raw := readMountFixture(t, "mcp-args-forged.json")
	var probe map[string]any
	if err := json.Unmarshal(raw, &probe); err != nil {
		t.Fatalf("mcp fixture not JSON: %v", err)
	}
	if _, ok := probe["source_class"]; !ok {
		t.Fatal("mcp fixture lost its forged claim member")
	}
	ev, err := MCPEvent("srv", "shell", raw, 5*time.Millisecond, 10, 20, testMachine, time.Now(), "mc-1")
	if err != nil {
		t.Fatal(err)
	}
	if ev.SourceClass != schema.SrcToolMCP {
		t.Errorf("mcp class %q, want tool_mcp (args cannot raise the mount class)", ev.SourceClass)
	}
	for k := range ev.Attrs {
		if k == "source_class" {
			t.Error("forged claim leaked into mcp attrs")
		}
	}
}
