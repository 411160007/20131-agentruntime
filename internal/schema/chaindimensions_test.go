package schema

import (
	"os"
	"path/filepath"
	"reflect"
	"regexp"
	"strings"
	"testing"
)

func chainObs(t *testing.T, rec ChainDimensionsRecord, dim ChainDimension) ChainDimensionObservation {
	t.Helper()
	for _, o := range rec.Observations {
		if o.Dimension == dim {
			return o
		}
	}
	t.Fatalf("record carries no observation for %q", string(dim))
	return ChainDimensionObservation{}
}

// TestChainDimensionVocabularyPin pins the closed eight-dimension
// vocabulary: count, spec order, uniqueness, membership, and the
// wild-value rejection. The order literal here is a third recount of
// the section 242 bullets (Go, docs, Node checker all must agree).
func TestChainDimensionVocabularyPin(t *testing.T) {
	want := []string{
		"intent_deviation", "capability_escalation", "data_sensitivity_escalation",
		"trust_domain_crossing", "reversibility_reduction", "blast_radius_growth",
		"destination_change", "delegation_chain",
	}
	got := AllChainDimensions()
	if len(got) != 8 {
		t.Fatalf("dimension census drifted: %d", len(got))
	}
	if strings.Join(got, ",") != strings.Join(want, ",") {
		t.Fatalf("dimension order drifted from the spec bullets:\n got %v\nwant %v", got, want)
	}
	if len(AllChainDimensionCoverage()) != 8 {
		t.Fatal("coverage table is not 8/8")
	}
	for _, w := range want {
		if !ChainDimension(w).Valid() {
			t.Errorf("vocabulary member %q rejected by Valid", w)
		}
	}
	for _, wild := range []ChainDimension{"", "sequence_risk", "Intent Deviation", "intent-deviation", "behavior_chain"} {
		if wild.Valid() {
			t.Errorf("wild dimension %q accepted", wild)
		}
	}
	for _, s := range AllChainObservationStates() {
		if !ChainObservationState(s).Valid() {
			t.Errorf("observation state %q rejected", s)
		}
	}
	for _, wild := range []ChainObservationState{"", "observed_not", "maybe", "high_risk"} {
		if wild.Valid() {
			t.Errorf("wild observation state %q accepted", wild)
		}
	}
}

// TestChainDimensionCoverageRows pins the zero-dangling rule: all
// eight rows registered in spec order, each phase legitimate, every
// anchored row naming a real later slice, exactly three computable
// rows matching ComputableChainDimensions, and the pending row
// saying so with no fake anchor.
func TestChainDimensionCoverageRows(t *testing.T) {
	rows := AllChainDimensionCoverage()
	if len(rows) != 8 {
		t.Fatalf("coverage rows %d, want 8", len(rows))
	}
	legalAnchors := map[string]bool{"W5.3": true, "W6": true, "W11": true}
	computable := []string{}
	for i, r := range rows {
		if string(r.Dimension) != chainDimensionWireNames[i] {
			t.Fatalf("coverage row %d out of spec order: %q", i, string(r.Dimension))
		}
		if !r.Phase.Valid() {
			t.Errorf("coverage row %q carries a wild phase %q", r.Dimension, r.Phase)
		}
		switch r.Phase {
		case ChainPhaseComputable:
			computable = append(computable, string(r.Dimension))
			if r.Anchor != "" {
				t.Errorf("computable row %q must not fake an anchor", r.Dimension)
			}
		case ChainPhaseAnchored:
			if !legalAnchors[r.Anchor] {
				t.Errorf("anchored row %q names a non-slice anchor %q", r.Dimension, r.Anchor)
			}
		case ChainPhasePending:
			if r.Anchor != "" {
				t.Errorf("pending row %q must stay honest about having no owner", r.Dimension)
			}
		}
	}
	if strings.Join(computable, ",") != strings.Join(ComputableChainDimensions(), ",") {
		t.Errorf("computable rows drifted from ComputableChainDimensions: %v", computable)
	}
	// Duplicate registration is a dangling table wearing a full skin.
	seen := map[ChainDimension]bool{}
	for _, r := range rows {
		if seen[r.Dimension] {
			t.Errorf("dimension %q registered twice", r.Dimension)
		}
		seen[r.Dimension] = true
	}
	if len(ComputableChainDimensions()) != 3 {
		t.Errorf("computable subset census %d, want 3", len(ComputableChainDimensions()))
	}
}

// TestIntentDeviationGolden: one chain whose steps serve two
// different recorded intents is observed with the off-root steps as
// basis; one chain serving a single intent across every step is
// not_observed; a chain that never recorded intents is
// unclassified - silence is never read as agreement.
func TestIntentDeviationGolden(t *testing.T) {
	steps := []ChainStep{
		{Seq: 1, Action: DataActionRead, Intent: "deploy the site"},
		{Seq: 2, Action: DataActionExecute, Intent: "deploy the site"},
		{Seq: 3, Action: DataActionRead, Intent: "read unrelated ssh key"},
	}
	rec, err := BuildChainDimensions(steps)
	if err != nil {
		t.Fatalf("build: %v", err)
	}
	o := chainObs(t, rec, DimensionIntentDeviation)
	if o.State != ChainStateObserved {
		t.Fatalf("deviating chain recorded %q, want observed", o.State)
	}
	if len(o.Basis) != 1 || o.Basis[0] != 3 {
		t.Errorf("deviation basis %v, want [3]", o.Basis)
	}
	quiet, err := BuildChainDimensions([]ChainStep{
		{Seq: 1, Action: DataActionDiscover, Intent: "deploy the site"},
		{Seq: 2, Action: DataActionWrite, Intent: "deploy the site"},
	})
	if err != nil {
		t.Fatalf("build: %v", err)
	}
	if o := chainObs(t, quiet, DimensionIntentDeviation); o.State != ChainStateNotObserved || len(o.Basis) != 0 {
		t.Errorf("single-intent chain recorded %q/%v, want not_observed with empty basis", o.State, o.Basis)
	}
	silent, err := BuildChainDimensions([]ChainStep{
		{Seq: 1, Action: DataActionRead},
		{Seq: 2, Action: DataActionExport},
	})
	if err != nil {
		t.Fatalf("build: %v", err)
	}
	if o := chainObs(t, silent, DimensionIntentDeviation); o.State != ChainStateUnclassified {
		t.Errorf("intentless chain recorded %q, want unclassified (absence is never inferred)", o.State)
	}
}

// TestTrustDomainCrossingGolden: adjacent steps with different
// recorded trust levels are observed with the crossing steps as
// basis; equal adjacent levels are not_observed; a chain with fewer
// than two adjacent recorded levels is unclassified.
func TestTrustDomainCrossingGolden(t *testing.T) {
	rec, err := BuildChainDimensions([]ChainStep{
		{Seq: 1, Action: DataActionRead, Trust: LevelPrivate},
		{Seq: 2, Action: DataActionRead, Trust: LevelCredential},
		{Seq: 3, Action: DataActionExport, Trust: LevelSecurityBoundary},
	})
	if err != nil {
		t.Fatalf("build: %v", err)
	}
	o := chainObs(t, rec, DimensionTrustDomainCrossing)
	if o.State != ChainStateObserved {
		t.Fatalf("crossing chain recorded %q, want observed", o.State)
	}
	if len(o.Basis) != 2 || o.Basis[0] != 2 || o.Basis[1] != 3 {
		t.Errorf("crossing basis %v, want [2 3]", o.Basis)
	}
	same, err := BuildChainDimensions([]ChainStep{
		{Seq: 1, Action: DataActionRead, Trust: LevelSensitive},
		{Seq: 2, Action: DataActionWrite, Trust: LevelSensitive},
	})
	if err != nil {
		t.Fatalf("build: %v", err)
	}
	if o := chainObs(t, same, DimensionTrustDomainCrossing); o.State != ChainStateNotObserved || len(o.Basis) != 0 {
		t.Errorf("same-domain chain recorded %q/%v, want not_observed with empty basis", o.State, o.Basis)
	}
	partial, err := BuildChainDimensions([]ChainStep{
		{Seq: 1, Action: DataActionRead, Trust: LevelNormal},
		{Seq: 2, Action: DataActionRead},
	})
	if err != nil {
		t.Fatalf("build: %v", err)
	}
	if o := chainObs(t, partial, DimensionTrustDomainCrossing); o.State != ChainStateUnclassified {
		t.Errorf("half-recorded chain recorded %q, want unclassified", o.State)
	}
}

// TestReversibilityReductionGolden consumes the W2.3 recovery
// classes only: a chain worsening from local_reversible to
// non_reversible is observed with the worsening steps as basis; an
// improving chain and a flat chain are not_observed; fewer than two
// recovery-bearing steps is unclassified.
func TestReversibilityReductionGolden(t *testing.T) {
	worse, err := BuildChainDimensions([]ChainStep{
		{Seq: 1, Action: DataActionWrite, Recovery: RecoveryLocalReversible},
		{Seq: 2, Action: DataActionPersist, Recovery: RecoveryLocalPartial},
		{Seq: 3, Action: DataActionExport, Recovery: RecoveryNonReversible},
	})
	if err != nil {
		t.Fatalf("build: %v", err)
	}
	o := chainObs(t, worse, DimensionReversibilityReduction)
	if o.State != ChainStateObserved {
		t.Fatalf("worsening chain recorded %q, want observed", o.State)
	}
	if len(o.Basis) != 2 || o.Basis[0] != 2 || o.Basis[1] != 3 {
		t.Errorf("reduction basis %v, want [2 3]", o.Basis)
	}
	better, err := BuildChainDimensions([]ChainStep{
		{Seq: 1, Action: DataActionDelete, Recovery: RecoveryNonReversible},
		{Seq: 2, Action: DataActionModify, Recovery: RecoveryLocalPartial},
	})
	if err != nil {
		t.Fatalf("build: %v", err)
	}
	if o := chainObs(t, better, DimensionReversibilityReduction); o.State != ChainStateNotObserved || len(o.Basis) != 0 {
		t.Errorf("improving chain recorded %q/%v, want not_observed (improvement is not reduction)", o.State, o.Basis)
	}
	flat, err := BuildChainDimensions([]ChainStep{
		{Seq: 1, Action: DataActionWrite, Recovery: RecoveryLocalPartial},
		{Seq: 2, Action: DataActionModify, Recovery: RecoveryLocalPartial},
	})
	if err != nil {
		t.Fatalf("build: %v", err)
	}
	if o := chainObs(t, flat, DimensionReversibilityReduction); o.State != ChainStateNotObserved {
		t.Errorf("flat chain recorded %q, want not_observed", o.State)
	}
	one, err := BuildChainDimensions([]ChainStep{
		{Seq: 1, Action: DataActionWrite, Recovery: RecoveryExternalCompensation},
	})
	if err != nil {
		t.Fatalf("build: %v", err)
	}
	if o := chainObs(t, one, DimensionReversibilityReduction); o.State != ChainStateUnclassified {
		t.Errorf("single-class chain recorded %q, want unclassified", o.State)
	}
	// The source rule is load-bearing: the only accepted recovery
	// spellings are the four W2.3 wire tokens.
	_, err = BuildChainDimensions([]ChainStep{
		{Seq: 1, Action: DataActionWrite, Recovery: "auto_rollback"},
		{Seq: 2, Action: DataActionWrite},
	})
	if err == nil {
		t.Error("recovery spelling outside the W2.3 four accepted - source rule broken")
	}
}

// TestChainDimensionsRejectionsLeaveNoRecord pins reject-before-
// value: every malformed chain leaves the zero record behind.
func TestChainDimensionsRejectionsLeaveNoRecord(t *testing.T) {
	cases := []struct {
		name  string
		steps []ChainStep
	}{
		{"empty chain", nil},
		{"zero seq", []ChainStep{{Seq: 0, Action: DataActionRead}}},
		{"gap in numbering", []ChainStep{{Seq: 1, Action: DataActionRead}, {Seq: 3, Action: DataActionWrite}}},
		{"out-of-order numbering", []ChainStep{{Seq: 2, Action: DataActionRead}, {Seq: 1, Action: DataActionWrite}}},
		{"duplicate numbering", []ChainStep{{Seq: 1, Action: DataActionRead}, {Seq: 1, Action: DataActionWrite}}},
		{"wild action", []ChainStep{{Seq: 1, Action: DataAction("teleport")}}},
		{"unclassified action", []ChainStep{{Seq: 1}}},
		{"none-word action", []ChainStep{{Seq: 1, Action: DataAction("none")}}},
		{"wild trust level", []ChainStep{{Seq: 1, Action: DataActionRead, Trust: TrustLevel("s9_olympus")}}},
		{"wild recovery class", []ChainStep{{Seq: 1, Action: DataActionRead, Recovery: RecoveryClass("undo")}}},
		{"later step wild", []ChainStep{{Seq: 1, Action: DataActionRead}, {Seq: 2, Action: DataAction("mind_control")}}},
	}
	for _, c := range cases {
		rec, err := BuildChainDimensions(c.steps)
		if err == nil {
			t.Errorf("%s: accepted", c.name)
			continue
		}
		if rec.Steps != 0 || rec.Observations != nil || rec.EnforcementPlane != "" {
			t.Errorf("%s: rejection left a partial record behind: %+v", c.name, rec)
		}
	}
}

// TestChainDimensionsRecordCarriesNoDecision pins the reflective
// shape: the step, the observation, and the record carry no field
// that smells like a verdict, and the plane line says none.
func TestChainDimensionsRecordCarriesNoDecision(t *testing.T) {
	red := regexp.MustCompile(`(?i)decision|allow|block|verdict|score|risk|severity`)
	for _, tp := range []reflect.Type{reflect.TypeOf(ChainStep{}), reflect.TypeOf(ChainDimensionObservation{}), reflect.TypeOf(ChainDimensionsRecord{})} {
		for i := 0; i < tp.NumField(); i++ {
			if red.MatchString(tp.Field(i).Name) {
				t.Errorf("%s field %q resembles a judgement - forbidden while the plane reads none", tp.Name(), tp.Field(i).Name)
			}
		}
	}
	if reflect.TypeOf(ChainDimensionsRecord{}).NumField() != 3 {
		t.Error("record field set drifted (want 3)")
	}
	if reflect.TypeOf(ChainStep{}).NumField() != 5 {
		t.Error("step field set drifted (want 5)")
	}
	if reflect.TypeOf(ChainDimensionObservation{}).NumField() != 3 {
		t.Error("observation field set drifted (want 3)")
	}
	if ChainDimensionEnforcementPlane != "none-in-observation-phase" {
		t.Error("enforcement plane line drifted")
	}
}

// TestChainDimensionsSymbolsStayOffTheDecisionPlane walks the four
// decision-plane directories and the command tree for this slice's
// symbols while the enforcement plane reads none-in-observation-
// phase, with a planted-shape positive control so the walk proves
// its own teeth before any green is believed.
func TestChainDimensionsSymbolsStayOffTheDecisionPlane(t *testing.T) {
	needle := "ChainDimension|ChainStep|ChainObservationState|ChainStateObserved|ChainStateNotObserved|ChainStateUnclassified|BuildChainDimensions|ComputableChainDimensions|ChainPhaseComputable|ChainPhaseAnchored|ChainPhasePending|ChainDimensionEnforcementPlane"
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
				t.Errorf("decision-plane file %s references chain dimension symbol %q", filepath.Join(d, e.Name()), loc)
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
				t.Errorf("command tree file %s references chain dimension symbol %q", p, loc)
			}
		}
	}
	walkCmd(filepath.Join("..", "..", "cmd"))
	if cmdHits == 0 {
		self, err := os.ReadFile("chaindimensions.go")
		if err != nil {
			t.Fatalf("read self: %v", err)
		}
		if !re.MatchString(string(self)) {
			t.Error("positive control broken: needle matches no shipped chain dimension source")
		}
	}
}

// TestChainDimensionsDocsSync pins the section 19 keys, the eight
// anchor bullet lines, and the eight coverage table rows verbatim
// against the Go constants and vocabularies, completing the
// docs<->Go<->Node-checker three-way mirror.
func TestChainDimensionsDocsSync(t *testing.T) {
	raw, err := os.ReadFile(filepath.Join("..", "..", "docs", "schema-v2.md"))
	if err != nil {
		t.Fatalf("read docs/schema-v2.md: %v", err)
	}
	doc := string(raw)
	if !strings.Contains(doc, "## 19. Behavior chain dimension record contract") {
		t.Fatal("section 19 header missing from docs/schema-v2.md")
	}
	pins := [][2]string{
		{"chain_dimension_vocabulary", strings.Join(AllChainDimensions(), ",")},
		{"chain_dimension_computable_v0", strings.Join(ComputableChainDimensions(), ",")},
		{"chain_dimension_observation_states", strings.Join(AllChainObservationStates(), ",")},
		{"chain_dimension_coverage_rule", ChainDimensionCoverageRule},
		{"chain_dimension_unclassified_rule", ChainDimensionUnclassifiedRule},
		{"chain_dimension_reversibility_source_rule", ChainDimensionReversibilitySourceRule},
		{"chain_dimension_enforcement_plane", ChainDimensionEnforcementPlane},
	}
	for _, p := range pins {
		line := p[0] + ": " + p[1]
		if !strings.Contains(doc, line) {
			t.Errorf("docs section 19 drifted, want line %q", line)
		}
	}
	// The eight section 242 bullet lines mirror the vocabulary
	// mechanically (space to underscore, lower case), in order.
	specNames := []string{
		"Intent Deviation", "Capability Escalation", "Data Sensitivity Escalation",
		"Trust Domain Crossing", "Reversibility Reduction", "Blast Radius Growth",
		"Destination Change", "Delegation Chain",
	}
	for i, n := range specNames {
		if strings.ToLower(strings.ReplaceAll(n, " ", "_")) != chainDimensionWireNames[i] {
			t.Fatalf("Go order broke against the spec bullet at %d", i)
		}
		if !strings.Contains(doc, "* "+n) {
			t.Errorf("docs anchor bullet missing: %q", n)
		}
	}
	for _, r := range AllChainDimensionCoverage() {
		row := "| `" + string(r.Dimension) + "` | " + string(r.Phase) + " | " + orDash(r.Anchor) + " |"
		if !strings.Contains(doc, row) {
			t.Errorf("docs coverage table drifted, want row %q", row)
		}
	}
}

func orDash(s string) string {
	if s == "" {
		return "-"
	}
	return s
}
