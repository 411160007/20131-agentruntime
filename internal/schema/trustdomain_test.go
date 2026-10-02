package schema

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

// TestTrustDomainVocabulariesAndRulesPin pins the two closed
// vocabularies (five levels, seven domains) in normative order and the
// four rule constants against their verbatim contract values.
func TestTrustDomainVocabulariesAndRulesPin(t *testing.T) {
	levels := AllTrustLevels()
	wantLevels := []string{"s0_normal", "s1_private", "s2_sensitive", "s3_credential", "s4_security_boundary"}
	if strings.Join(levels, ",") != strings.Join(wantLevels, ",") {
		t.Errorf("trust level vocabulary drifted, want %v got %v", wantLevels, levels)
	}
	domains := AllRunDomains()
	wantDomains := []string{"user", "agent", "tool", "sandbox", "recovery", "security_core", "external"}
	if strings.Join(domains, ",") != strings.Join(wantDomains, ",") {
		t.Errorf("run domain vocabulary drifted, want %v got %v", wantDomains, domains)
	}
	pins := [][2]string{
		{TrustDomainLegacyProjectionRule, "five-levels-project-down-to-three-never-lift-by-guess"},
		{TrustDomainCrossingRule, "cross-domain-data-movement-requires-data-boundary"},
		{TrustDomainAbsentDefault, "absent-means-unclassified-legacy-never-inferred"},
		{TrustDomainEnforcementPlane, "none-in-observation-phase"},
	}
	for i, p := range pins {
		if p[0] != p[1] {
			t.Errorf("rule constant %d drifted: %q != %q", i, p[0], p[1])
		}
	}
	// The two vocabularies are disjoint by construction: no token is
	// simultaneously a level and a domain (a fused word would let a
	// wild record satisfy either slot by accident).
	for _, l := range levels {
		for _, d := range domains {
			if l == d {
				t.Errorf("level %q collides with domain %q", l, d)
			}
		}
	}
}

// TestLegacyProjectionTableExact pins the five migration annotation
// pairs and the rejection of unknown and absent levels.
func TestLegacyProjectionTableExact(t *testing.T) {
	wants := map[TrustLevel]ResClass{
		LevelNormal:           ResLow,
		LevelPrivate:          ResMedium,
		LevelSensitive:        ResMedium,
		LevelCredential:       ResHigh,
		LevelSecurityBoundary: ResHigh,
	}
	if len(wants) != 5 {
		t.Fatal("projection fixture lost a level")
	}
	for l, want := range wants {
		got, ok := LegacyClassOf(l)
		if !ok || got != want {
			t.Errorf("LegacyClassOf(%q) = (%q,%v), want (%q,true)", l, got, ok, want)
		}
	}
	for _, bad := range []TrustLevel{"", "s5_oracle", "S0_NORMAL", "low", "medium", "high", "s0"} {
		if got, ok := LegacyClassOf(bad); ok {
			t.Errorf("LegacyClassOf(%q) projected to %q; unknown and absent levels must have no projection", bad, got)
		}
	}
}

// TestLiftIsAWindowNeverASingleValue pins the ambiguity discipline:
// medium and high lift to exactly two candidates each, low to one,
// absent and unknown to none - and the candidate sets are exactly the
// fibres of the projection table, so nothing in the five-level world
// is unreachable from the legacy world and nothing leaks between
// fibres.
func TestLiftIsAWindowNeverASingleValue(t *testing.T) {
	windows := map[ResClass][]TrustLevel{
		ResLow:    {LevelNormal},
		ResMedium: {LevelPrivate, LevelSensitive},
		ResHigh:   {LevelCredential, LevelSecurityBoundary},
	}
	for r, want := range windows {
		got := LiftCandidates(r)
		if strings.Join(toStringsLevel(got), ",") != strings.Join(toStringsLevel(want), ",") {
			t.Errorf("LiftCandidates(%q) = %v, want window %v", r, got, want)
		}
		if len(got) < 1 {
			t.Errorf("legacy class %q lifted to nothing", r)
		}
	}
	for _, bad := range []ResClass{"", "critical", "LOW", "s2_sensitive"} {
		if got := LiftCandidates(bad); got != nil {
			t.Errorf("LiftCandidates(%q) = %v; absent and unknown classes must lift to nothing", bad, got)
		}
	}
	// Fibre exactness: every level appears in exactly one window and
	// the window count of each legacy class matches the fibres of the
	// projection table.
	seen := map[TrustLevel]int{}
	for _, r := range []ResClass{ResLow, ResMedium, ResHigh} {
		for _, l := range LiftCandidates(r) {
			seen[l]++
			if c, ok := LegacyClassOf(l); !ok || c != r {
				t.Errorf("lift window for %q contains %q whose projection is not %q", r, l, r)
			}
		}
	}
	for _, l := range []TrustLevel{LevelNormal, LevelPrivate, LevelSensitive, LevelCredential, LevelSecurityBoundary} {
		if seen[l] != 1 {
			t.Errorf("level %q appears in %d lift windows, want exactly 1", l, seen[l])
		}
	}
}

func toStringsLevel(in []TrustLevel) []string {
	out := make([]string, len(in))
	for i, l := range in {
		out[i] = string(l)
	}
	return out
}

// TestProjectionLiftRoundTrip pins that projecting a level down and
// lifting back always returns the level inside its window - the
// migration annotation lines never orphan a record.
func TestProjectionLiftRoundTrip(t *testing.T) {
	for _, l := range []TrustLevel{LevelNormal, LevelPrivate, LevelSensitive, LevelCredential, LevelSecurityBoundary} {
		c, ok := LegacyClassOf(l)
		if !ok {
			t.Fatalf("level %q has no legacy projection", l)
		}
		found := false
		for _, cand := range LiftCandidates(c) {
			if cand == l {
				found = true
			}
		}
		if !found {
			t.Errorf("round trip broken: %q projects to %q but its lift window excludes it", l, c)
		}
	}
}

// TestTrustDomainAbsentMeansUnclassified pins the honest absence
// shape: empty level and empty domain are valid records meaning
// unclassified legacy, and no table or helper turns them into a
// default.
func TestTrustDomainAbsentMeansUnclassified(t *testing.T) {
	if !TrustLevel("").Valid() || !RunDomain("").Valid() {
		t.Error("absent trust level or run domain must be valid (unclassified legacy)")
	}
	if got, ok := LegacyClassOf(""); ok {
		t.Errorf("absent trust level projected to %q; absence must project to nothing", got)
	}
	if got := LiftCandidates(""); got != nil {
		t.Errorf("absent legacy class lifted to %v; absence must lift to nothing", got)
	}
	for _, bad := range []string{"s5_oracle", "trusted", "S0", "sandbox_domain", "external "} {
		if TrustLevel(bad).Valid() && RunDomain(bad).Valid() {
			t.Errorf("wild token %q validated as both a level and a domain", bad)
		}
	}
	if TrustLevel("sandbox").Valid() {
		t.Error("run domain token must not validate as a trust level")
	}
	if RunDomain("s2_sensitive").Valid() {
		t.Error("trust level token must not validate as a run domain")
	}
}

// TestTrustDomainSymbolsStayOffTheDecisionPlane scans the four
// decision-plane directories and the command tree for this slice's
// symbols while the enforcement plane reads none-in-observation-phase,
// with a planted-shape positive control so the walk proves its own
// teeth before any green is believed.
func TestTrustDomainSymbolsStayOffTheDecisionPlane(t *testing.T) {
	needle := "TrustLevel|RunDomain|AllTrustLevels|AllRunDomains|LegacyClassOf|LiftCandidates|trustToLegacyClass|TrustDomainLegacyProjectionRule|TrustDomainCrossingRule|TrustDomainAbsentDefault|TrustDomainEnforcementPlane"
	dirs := []string{
		filepath.Join("..", "policy"), filepath.Join("..", "rules"),
		filepath.Join("..", "bus"), filepath.Join("..", "auditlog"),
	}
	re := regexp.MustCompile(needle)
	for _, d := range dirs {
		entries, err := os.ReadDir(d)
		if err != nil {
			t.Fatalf("read dir %s: %v", d, err)
		}
		for _, e := range entries {
			if e.IsDir() || !strings.HasSuffix(e.Name(), ".go") || strings.HasSuffix(e.Name(), "_test.go") {
				continue
			}
			data, err := os.ReadFile(filepath.Join(d, e.Name()))
			if err != nil {
				t.Fatalf("read %s: %v", d, err)
			}
			if loc := re.FindString(string(data)); loc != "" {
				t.Errorf("decision-plane file %s references trust domain symbol %q", filepath.Join(d, e.Name()), loc)
			}
		}
	}
	cmdHits := 0
	var walkCmd func(dir string)
	walkCmd = func(dir string) {
		entries, err := os.ReadDir(dir)
		if err != nil {
			t.Fatalf("read cmd dir: %v", err)
		}
		for _, e := range entries {
			p := filepath.Join(dir, e.Name())
			if e.IsDir() {
				walkCmd(p)
				continue
			}
			if !strings.HasSuffix(e.Name(), ".go") || strings.HasSuffix(e.Name(), "_test.go") {
				continue
			}
			data, err := os.ReadFile(p)
			if err != nil {
				t.Fatalf("read %s: %v", p, err)
			}
			if loc := re.FindString(string(data)); loc != "" {
				cmdHits++
				t.Errorf("command tree file %s references trust domain symbol %q", p, loc)
			}
		}
	}
	walkCmd(filepath.Join("..", "..", "cmd"))
	if cmdHits == 0 {
		// Positive control for the walk: the same regexp must find the
		// symbols in this slice's own shipped source before any green
		// from the walk is believed.
		self, err := os.ReadFile("trustdomain.go")
		if err != nil {
			t.Fatalf("read self: %v", err)
		}
		if !re.MatchString(string(self)) {
			t.Error("positive control broken: needle matches no shipped trust domain source")
		}
	}
}

// TestTrustDomainDocsSync pins the docs/schema-v2.md section 16
// machine block against the shipped Go vocabulary, projection table,
// and rule constants - drift in either direction is a red build.
func TestTrustDomainDocsSync(t *testing.T) {
	raw, err := os.ReadFile(filepath.Join("..", "..", "docs", "schema-v2.md"))
	if err != nil {
		t.Fatalf("read docs/schema-v2.md: %v", err)
	}
	doc := string(raw)
	if !strings.Contains(doc, "## 16. Trust domain mapping contract") {
		t.Fatal("section 16 header missing from docs/schema-v2.md")
	}
	projection := []string{
		"s0_normal=" + string(ResLow),
		"s1_private=" + string(ResMedium),
		"s2_sensitive=" + string(ResMedium),
		"s3_credential=" + string(ResHigh),
		"s4_security_boundary=" + string(ResHigh),
	}
	pins := [][2]string{
		{"trust_level_vocabulary", strings.Join(AllTrustLevels(), ",")},
		{"run_domain_vocabulary", strings.Join(AllRunDomains(), ",")},
		{"trust_domain_legacy_projection", strings.Join(projection, ",")},
		{"trust_domain_crossing_rule", TrustDomainCrossingRule},
		{"trust_domain_absent_default", TrustDomainAbsentDefault},
		{"trust_domain_enforcement_plane", TrustDomainEnforcementPlane},
	}
	for _, p := range pins {
		line := p[0] + ": " + p[1]
		if !strings.Contains(doc, line) {
			t.Errorf("docs section 16 drifted, want line %q", line)
		}
	}
}
