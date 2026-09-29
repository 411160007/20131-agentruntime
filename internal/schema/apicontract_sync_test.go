package schema

import (
	"os"
	"regexp"
	"strings"
	"testing"
)

// TestAPIDocContractMatchesGoEnums is one half of the frozen-contract
// dual-source cross-check (the Node half lives in
// scripts/apicontract-check.mjs and compares the same doc against the
// independent validator). The contract doc is the external promise: if
// the shipped enums drift away from it, the release is a lie and this
// gate must fail.
func TestAPIDocContractMatchesGoEnums(t *testing.T) {
	raw, err := os.ReadFile(filepathJoinDotDot("docs", "api-v0.md"))
	if err != nil {
		t.Skipf("contract doc not visible from this working dir: %v", err)
	}
	doc := string(raw)
	cases := []struct {
		key    string
		goList []string
	}{
		{"stages", AllStages()},
		{"types", AllEventTypes()},
		{"decisions", AllDecisions()},
		{"tiers", AllTiers()},
		{"caps", AllCapabilities()},
		{"res_classes", AllResourceClasses()},
		{"source_classes", AllSourceClasses()},
		{"rule_fields", AllMatchFields()},
		{"rule_ops", AllMatchOps()},
	}
	for _, tc := range cases {
		got := contractList(t, doc, tc.key)
		if diff := symDiff(got, tc.goList); len(diff) > 0 {
			t.Errorf("docs/api-v0.md \"%s\" diverges from Go enum: %v", tc.key, diff)
		}
	}
	// The frozen version-line shape must match the real thing.
	m := regexp.MustCompile("(?m)^version_regex: (.+)$").FindStringSubmatch(doc)
	if m == nil {
		t.Fatal("no version_regex contract line in api-v0.md")
	}
	rx, err := regexp.Compile(m[1])
	if err != nil {
		t.Fatalf("version_regex contract invalid: %v", err)
	}
	for _, good := range []string{
		"agent-collector 0.3.0-d3 linux/amd64 (go1.27.1)",
		"agent-collector 1.2.3 darwin/arm64 (go1.28)",
	} {
		if !rx.MatchString(good) {
			t.Errorf("frozen version shape rejected %q", good)
		}
	}
	for _, bad := range []string{
		"agent-collector v3 linux (go)",
		"0.3.0-d3 agent-collector linux/amd64 (go1.27.1)",
		"agent-collector 0.3.0-d3 linux (go1.27.1)",
	} {
		if rx.MatchString(bad) {
			t.Errorf("frozen version shape accepted malformed %q", bad)
		}
	}
	// hello binary uses the same shape with its own name
	helloRx, err := regexp.Compile(strings.Replace(m[1], "^agent-collector", "^hello-collector", 1))
	if err != nil {
		t.Fatal(err)
	}
	if !helloRx.MatchString("hello-collector 0.3.0-d3 windows/arm64 (go1.27.1)") {
		t.Error("version contract does not pin the hello binary sibling shape")
	}
}

func contractList(t *testing.T, doc, key string) []string {
	t.Helper()
	re := regexp.MustCompile("(?m)^" + regexp.QuoteMeta(key) + `: (.+)$`)
	m := re.FindStringSubmatch(doc)
	if m == nil {
		t.Fatalf("contract key %q not found in api-v0.md", key)
	}
	parts := strings.Split(m[1], ",")
	var out []string
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p != "" {
			out = append(out, p)
		}
	}
	if len(out) == 0 {
		t.Fatalf("contract key %q parsed empty (silent probe guard)", key)
	}
	return out
}

// filepathJoinDotDot resolves repo-relative paths from the test's cwd
// (internal/schema), without dragging the path package into contracts.
func filepathJoinDotDot(parts ...string) string {
	base := ".." + string(os.PathSeparator) + ".." + string(os.PathSeparator)
	for _, p := range parts {
		base += p + string(os.PathSeparator)
	}
	return strings.TrimSuffix(base, string(os.PathSeparator))
}
