package schema

import (
	"encoding/json"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func estimationArgs(s string) []byte { return []byte(s) }

func TestImpactEstimationFileDimensionPositive(t *testing.T) {
	in := ImpactEstimationInput{
		ActionID: "act.w111.filepos",
		ArgsRaw: estimationArgs(`{
			"path": "/etc/hosts",
			"script": "./run/setup.sh",
			"old": "../notes.md",
			"home": "~/.profile",
			"win": "C:\\Users\\op\\plan.txt",
			"nested": {"deep": ["/var/log/app.log", "/etc/hosts"]}
		}`),
	}
	rec, err := BuildImpactEstimationRecord(in)
	if err != nil {
		t.Fatalf("positive file fixture must build: %v", err)
	}
	want := []string{"../notes.md", "./run/setup.sh", "/etc/hosts", "/var/log/app.log", "C:\\Users\\op\\plan.txt", "~/.profile"}
	if strings.Join(rec.FileScope, "|") != strings.Join(want, "|") {
		t.Fatalf("file_scope derivation drifted:\n got %v\nwant %v", rec.FileScope, want)
	}
	if len(rec.NetworkScope) != 0 {
		t.Fatalf("no URL text was present, network_scope must state that: %v", rec.NetworkScope)
	}
}

func TestImpactEstimationFileDimensionNegative(t *testing.T) {
	// Bare names, prose, and non-shipped schemes are outside the
	// closed grammars: claiming them would turn an estimate into a
	// fabrication, so the record must stay silent-but-stated.
	in := ImpactEstimationInput{
		ActionID: "act.w111.fileneg",
		ArgsRaw: estimationArgs(`{
			"note": "please update data.txt and README today",
			"bare": "config.yaml",
			"tilde_alone": "~",
			"drive_alone": "C:",
			"mailto": "mailto:ops@example",
			"ftp": "ftp://files.example.com/pub",
			"count": 42,
			"flag": true
		}`),
	}
	rec, err := BuildImpactEstimationRecord(in)
	if err != nil {
		t.Fatalf("negative fixture must build: %v", err)
	}
	if len(rec.FileScope) != 0 || len(rec.NetworkScope) != 0 {
		t.Fatalf("nothing in this fixture is claimable, got files %v nets %v", rec.FileScope, rec.NetworkScope)
	}
}

func TestImpactEstimationNetworkDimensionPositive(t *testing.T) {
	in := ImpactEstimationInput{
		ActionID: "act.w111.netpos",
		ArgsRaw: estimationArgs(`{
			"endpoint": "https://API.Example.COM:8443/v1/items?x=1",
			"secret": "https://user:***@internal.example.net/p",
			"ws": "ws://stream.local:9000/feed",
			"proxy": "socks5://10.0.0.8:1080",
			"v6": "http://[::1]:3000/health",
			"again": "https://api.example.com:8443/"
		}`),
	}
	rec, err := BuildImpactEstimationRecord(in)
	if err != nil {
		t.Fatalf("positive network fixture must build: %v", err)
	}
	want := []string{"10.0.0.8", "[::1]", "api.example.com", "internal.example.net", "stream.local"}
	if strings.Join(rec.NetworkScope, "|") != strings.Join(want, "|") {
		t.Fatalf("network_scope derivation drifted:\n got %v\nwant %v", rec.NetworkScope, want)
	}
	for _, h := range rec.NetworkScope {
		if strings.Contains(h, "@") || strings.Contains(h, ":8443") {
			t.Fatalf("userinfo or port leaked into the record: %q", h)
		}
	}
	if len(rec.FileScope) != 0 {
		t.Fatalf("URL text must never double-list as paths: %v", rec.FileScope)
	}
}

func TestImpactEstimationNetworkDimensionNegative(t *testing.T) {
	in := ImpactEstimationInput{
		ActionID: "act.w111.netneg",
		ArgsRaw: estimationArgs(`{
			"schemeless": "api.example.com:8443/v1",
			"empty_host": "https://",
			"only_path": "https:///x",
			"weird": "https://user@ho st.example",
			"text": "visit our website sometime"
		}`),
	}
	rec, err := BuildImpactEstimationRecord(in)
	if err != nil {
		t.Fatalf("negative network fixture must build: %v", err)
	}
	if len(rec.NetworkScope) != 0 {
		t.Fatalf("nothing here is a closed-grammar authority, got %v", rec.NetworkScope)
	}
	if len(rec.FileScope) != 0 {
		t.Fatalf("scheme-shaped text must not fall through to the path grammar, got %v", rec.FileScope)
	}
}

func TestImpactEstimationPinnedSemanticsFields(t *testing.T) {
	rec, err := BuildImpactEstimationRecord(ImpactEstimationInput{
		ActionID: "act.w111.pins",
		ArgsRaw:  estimationArgs(`{"a": "/tmp/x"}`),
	})
	if err != nil {
		t.Fatalf("build: %v", err)
	}
	if rec.EstimateBasis != ImpactEstimationBasis {
		t.Fatalf("estimate basis drifted")
	}
	if rec.ClaimBoundary != ImpactEstimationClaimBoundary {
		t.Fatalf("claim boundary drifted")
	}
	if rec.Stance != ImpactEstimationRecordStance {
		t.Fatalf("stance drifted")
	}
	// One spelling for the observation-plane token: no second
	// dialect may enter this record family.
	for _, shipped := range []string{PromotionEnforcement, ImpactEnforcementPlane, RecoveryExecutionPlane} {
		if rec.EnforcementPlane != shipped {
			t.Fatalf("plane token must equal the shipped spelling used by every earlier record: %q vs %q", rec.EnforcementPlane, shipped)
		}
	}
	// Declared gaps: exactly six, closed-vocabulary order, derived.
	want := ImpactEstimationUnestimatedScopes()
	if len(want) != 6 || strings.Join(toStrs(want), ",") != "project_scope,process_scope,database_scope,credential_scope,device_scope,subagent_scope" {
		t.Fatalf("declared-gap list drifted: %v", want)
	}
	if len(rec.UnestimatedScopes) != 6 {
		t.Fatalf("built record must carry the six declared gaps")
	}
}

func toStrs(bs []BlastScope) []string {
	out := make([]string, len(bs))
	for i, b := range bs {
		out[i] = string(b)
	}
	return out
}

func TestImpactEstimationRejectsDriftNoHalfRecord(t *testing.T) {
	base, err := BuildImpactEstimationRecord(ImpactEstimationInput{
		ActionID: "act.w111.mutate",
		ArgsRaw:  estimationArgs(`{"p": "/etc/passwd", "u": "https://x.example"}`),
	})
	if err != nil {
		t.Fatalf("build: %v", err)
	}
	mutations := []struct {
		name string
		mut  func(*ImpactEstimationRecord)
	}{
		{"kind", func(r *ImpactEstimationRecord) { r.Kind = "impact.estimate" }},
		{"bad action id", func(r *ImpactEstimationRecord) { r.ActionID = "not an id!" }},
		{"short digest", func(r *ImpactEstimationRecord) { r.ArgsDigest = "abc" }},
		{"null file list", func(r *ImpactEstimationRecord) { r.FileScope = nil }},
		{"injected path", func(r *ImpactEstimationRecord) { r.FileScope = append([]string{"bare.txt"}, r.FileScope...) }},
		{"unsorted paths", func(r *ImpactEstimationRecord) { r.FileScope = []string{"/z", "/a"} }},
		{"userinfo host", func(r *ImpactEstimationRecord) { r.NetworkScope = []string{"***@x.example"} }},
		{"uppercase host", func(r *ImpactEstimationRecord) { r.NetworkScope = []string{"X.Example"} }},
		{"shrunk gaps", func(r *ImpactEstimationRecord) { r.UnestimatedScopes = r.UnestimatedScopes[:5] }},
		{"gap reorder", func(r *ImpactEstimationRecord) {
			r.UnestimatedScopes[0], r.UnestimatedScopes[1] = r.UnestimatedScopes[1], r.UnestimatedScopes[0]
		}},
		{"basis to fact", func(r *ImpactEstimationRecord) { r.EstimateBasis = "observed-fact" }},
		{"boundary to blocked", func(r *ImpactEstimationRecord) { r.ClaimBoundary = "renders-as-blocked" }},
		{"stance edited", func(r *ImpactEstimationRecord) { r.Stance = "enforcement-ready" }},
		{"plane promoted", func(r *ImpactEstimationRecord) { r.EnforcementPlane = "enforce-in-observation-phase" }},
	}
	for _, m := range mutations {
		cp := base
		cp.FileScope = append([]string(nil), base.FileScope...)
		cp.NetworkScope = append([]string(nil), base.NetworkScope...)
		cp.UnestimatedScopes = append([]BlastScope(nil), base.UnestimatedScopes...)
		m.mut(&cp)
		if err := cp.Validate(); err == nil {
			t.Fatalf("mutation %q must fail validation", m.name)
		}
	}
	// Build rejects bad input with the zero value, never a partial
	// record: invalid JSON, non-object, empty bytes, bad id.
	for _, bad := range []ImpactEstimationInput{
		{ActionID: "act.x", ArgsRaw: estimationArgs(`{"unterminated":`)},
		{ActionID: "act.x", ArgsRaw: estimationArgs(`[1,2]`)},
		{ActionID: "act.x", ArgsRaw: nil},
		{ActionID: "bad id!", ArgsRaw: estimationArgs(`{}`)},
	} {
		rec, err := BuildImpactEstimationRecord(bad)
		if err == nil {
			t.Fatalf("input %q must be rejected", bad.ArgsRaw)
		}
		if rec.Kind != "" || rec.ActionID != "" || rec.ArgsDigest != "" ||
			rec.FileScope != nil || rec.NetworkScope != nil || rec.UnestimatedScopes != nil ||
			rec.EstimateBasis != "" || rec.ClaimBoundary != "" || rec.Stance != "" || rec.EnforcementPlane != "" {
			t.Fatalf("rejection must return the zero value, got %+v", rec)
		}
	}
}

func TestImpactEstimationWireOrderGolden(t *testing.T) {
	rec, err := BuildImpactEstimationRecord(ImpactEstimationInput{
		ActionID: "act.w111.wire",
		ArgsRaw:  estimationArgs(`{}`),
	})
	if err != nil {
		t.Fatalf("build: %v", err)
	}
	raw, err := json.Marshal(rec)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	got := string(raw)
	order := []string{`"kind":`, `"action_id":`, `"args_digest":`, `"file_scope":`, `"network_scope":`,
		`"unestimated_scopes":`, `"estimate_basis":`, `"claim_boundary":`, `"stance":`, `"impact_estimation_enforcement_plane":`}
	pos := -1
	for _, k := range order {
		i := strings.Index(got, k)
		if i < 0 {
			t.Fatalf("wire field %s missing from %s", k, got)
		}
		if i <= pos {
			t.Fatalf("wire order drifted at %s", k)
		}
		pos = i
	}
	if !strings.Contains(got, `"file_scope":[]`) || !strings.Contains(got, `"network_scope":[]`) {
		t.Fatalf("empty derived lists must render as explicit empty arrays, never omitted: %s", got)
	}
}

func TestImpactEstimationZeroDecisionPlaneReachability(t *testing.T) {
	// Phase-0 red line: no decision-plane file references the
	// estimation surface, and the record is not an EventType.
	root := ".."
	dirs := []string{"policy", "rules", "bus", "auditlog"}
	needle := "ImpactEstimation"
	fset := token.NewFileSet()
	for _, d := range dirs {
		_ = filepath.Walk(filepath.Join(root, "internal", d), func(p string, info os.FileInfo, err error) error {
			if err != nil || info.IsDir() || !strings.HasSuffix(p, ".go") || strings.HasSuffix(p, "_test.go") {
				return nil
			}
			f, err := parser.ParseFile(fset, p, nil, 0)
			if err != nil {
				t.Errorf("parse %s: %v", p, err)
				return nil
			}
			src, _ := os.ReadFile(p)
			if strings.Contains(string(src), needle) {
				t.Errorf("decision-plane file %s references the estimation surface", p)
			}
			_ = f
			return nil
		})
	}
	ev, err := os.ReadFile("event.go") // this test runs in the schema package dir
	if err != nil {
		t.Fatalf("event vocabulary not readable: %v", err)
	}
	if strings.Contains(string(ev), "impact.estimation") {
		t.Fatalf("impact.estimation must not enter the EventType vocabulary")
	}
	// The cmd surface must not import the symbol either.
	_ = filepath.Walk(filepath.Join(root, "cmd"), func(p string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() || !strings.HasSuffix(p, ".go") {
			return nil
		}
		src, _ := os.ReadFile(p)
		if strings.Contains(string(src), needle) {
			t.Errorf("cmd file %s references the estimation surface", p)
		}
		return nil
	})
}
