package schema

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

// TestEnumsStayInSyncWithValidator keeps the Go enums and the independent
// Node validator (scripts/validate-jsonl.mjs) from drifting apart: both
// sources must list exactly the same Stage and EventType vocabularies.
func TestEnumsStayInSyncWithValidator(t *testing.T) {
	path := filepath.Join("..", "..", "scripts", "validate-jsonl.mjs")
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Skipf("validator script not present from this working dir: %v", err)
	}
	src := string(raw)

	lists := map[string]regexp.Regexp{}
	_ = lists
	stages := extractArray(t, src, "STAGES")
	types := extractArray(t, src, "TYPES")
	decisions := extractArray(t, src, "DECISIONS")
	tiers := extractArray(t, src, "TIER")
	caps := extractArray(t, src, "CAPS")
	resClasses := extractArray(t, src, "RES_CLASSES")

	if diff := symDiff(stages, AllStages()); len(diff) > 0 {
		t.Errorf("validate-jsonl.mjs STAGES diverges from Go Stage enum: %v", diff)
	}
	if diff := symDiff(types, AllEventTypes()); len(diff) > 0 {
		t.Errorf("validate-jsonl.mjs TYPES diverges from Go EventType enum: %v", diff)
	}
	if diff := symDiff(decisions, AllDecisions()); len(diff) > 0 {
		t.Errorf("validate-jsonl.mjs DECISIONS diverges from Go Decision enum: %v", diff)
	}
	if diff := symDiff(tiers, AllTiers()); len(diff) > 0 {
		t.Errorf("validate-jsonl.mjs TIER diverges from Go Tier enum: %v", diff)
	}
	if diff := symDiff(caps, AllCapabilities()); len(diff) > 0 {
		t.Errorf("validate-jsonl.mjs CAPS diverges from Go capability vocabulary: %v", diff)
	}
	if diff := symDiff(resClasses, AllResourceClasses()); len(diff) > 0 {
		t.Errorf("validate-jsonl.mjs RES_CLASSES diverges from Go sensitivity classes: %v", diff)
	}
}

var arrayRE = map[string]*regexp.Regexp{
	"STAGES":      regexp.MustCompile(`(?s)const STAGES = \[(.*?)\];`),
	"TYPES":       regexp.MustCompile(`(?s)const TYPES = \[(.*?)\];`),
	"DECISIONS":   regexp.MustCompile(`(?s)const DECISIONS = \[(.*?)\];`),
	"TIER":        regexp.MustCompile(`(?s)const TIER = \[(.*?)\];`),
	"CAPS":        regexp.MustCompile(`(?s)const CAPS = \[(.*?)\];`),
	"RES_CLASSES": regexp.MustCompile(`(?s)const RES_CLASSES = \[(.*?)\];`),
}

func extractArray(t *testing.T, src, name string) []string {
	t.Helper()
	re, ok := arrayRE[name]
	if !ok {
		t.Fatalf("no extractor for %s", name)
	}
	m := re.FindStringSubmatch(src)
	if m == nil {
		t.Fatalf("could not locate const %s in validator script", name)
	}
	strs := regexp.MustCompile(`'([^']+)'`).FindAllStringSubmatch(m[1], -1)
	var out []string
	for _, s := range strs {
		out = append(out, s[1])
	}
	if len(out) == 0 {
		t.Fatalf("%s list parsed empty (silent probe failure guard)", name)
	}
	return out
}

func symDiff(a, b []string) []string {
	set := map[string]int{}
	for _, s := range a {
		set[s]++
	}
	for _, s := range b {
		set[s]--
	}
	var diff []string
	for k, v := range set {
		if v != 0 {
			diff = append(diff, k+strings.Repeat("!", abs(v)))
		}
	}
	return diff
}

func abs(x int) int {
	if x < 0 {
		return -x
	}
	return x
}
