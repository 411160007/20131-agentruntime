package schema

import (
	"os"
	"path/filepath"
	"reflect"
	"regexp"
	"strings"
	"testing"
)

// TestExportWrapVocabularyAndRulesPin pins the eight-word packaging
// vocabulary in normative order, the four rule constant values, and
// the deliberate cross-vocabulary fact that "share" is a member of
// both the packaging family and the nine data actions with distinct
// meanings.
func TestExportWrapVocabularyAndRulesPin(t *testing.T) {
	want := []string{"compress", "encode", "encrypt", "archive", "copy", "upload", "send", "share"}
	got := AllPackagingWords()
	if len(got) != 8 || strings.Join(got, ",") != strings.Join(want, ",") {
		t.Fatalf("packaging vocabulary drifted: %v", got)
	}
	if ExportPackagingSensitivityRule != "packaging-never-lowers-sensitivity" {
		t.Errorf("sensitivity rule drifted: %q", ExportPackagingSensitivityRule)
	}
	if ExportRejudgementIndependenceRule != "read-allow-never-carries-to-export" {
		t.Errorf("independence rule drifted: %q", ExportRejudgementIndependenceRule)
	}
	if ExportRejudgementAbsentDefault != "absent-means-no-packaging-observed-never-inferred" {
		t.Errorf("absent default drifted: %q", ExportRejudgementAbsentDefault)
	}
	if ExportRejudgementEnforcementPlane != "none-in-observation-phase" {
		t.Errorf("enforcement plane drifted: %q", ExportRejudgementEnforcementPlane)
	}
	// "share" lives in both closed sets on purpose; the tables must
	// stay separate functions with separate meanings.
	if PackagingWord("share").Valid() != true {
		t.Error("share must be a valid packaging word")
	}
	if DataAction("share").Valid() != true {
		t.Error("share must remain a valid data action")
	}
	if len(AllPackagingWords()) == len(AllDataActions()) {
		t.Error("vocabulary sizes must stay distinguishable (8 vs 9)")
	}
}

// TestPackagingWordValidity accepts each of the eight and rejects the
// wild shapes: empty string, upper-case respelling, near-miss words,
// and data-action-only tokens.
func TestPackagingWordValidity(t *testing.T) {
	for _, w := range AllPackagingWords() {
		if !PackagingWord(w).Valid() {
			t.Errorf("closed word rejected: %q", w)
		}
	}
	wild := []string{"", "COMPRESS", "Compress", "zip", "encrypt_", "decompress", "export", "persist", " share", "share "}
	for _, w := range wild {
		if PackagingWord(w).Valid() {
			t.Errorf("wild shape accepted: %q", w)
		}
	}
}

// TestBuildExportRejudgementGoldenFacet is the two-row golden the
// wave owed: same target, a READ row and an EXPORT row recorded side
// by side. Both rows exist independently, only the export row is
// flagged for independent re-judgement, and neither row carries any
// decision - an allow recorded next to the read never leaks into the
// export row because no row can carry an allow at all.
func TestBuildExportRejudgementGoldenFacet(t *testing.T) {
	read, err := BuildExportRejudgement("secrets/api.key", DataActionRead, nil)
	if err != nil {
		t.Fatalf("read row: %v", err)
	}
	export, err := BuildExportRejudgement("secrets/api.key", DataActionExport,
		[]PackagingWord{PackagingCompress, PackagingEncrypt})
	if err != nil {
		t.Fatalf("export row: %v", err)
	}
	if read.IndependentRejudgement {
		t.Error("read row must not be flagged for export re-judgement")
	}
	if !export.IndependentRejudgement {
		t.Error("export row must always be flagged for independent re-judgement")
	}
	if !read.SensitivityPreserved || !export.SensitivityPreserved {
		t.Error("both rows must carry the sensitivity-preserving constant")
	}
	if len(export.Packaging) != 2 || export.Packaging[0] != PackagingCompress || export.Packaging[1] != PackagingEncrypt {
		t.Errorf("export row lost packaging annotation order/content: %v", export.Packaging)
	}
	if len(read.Packaging) != 0 {
		t.Errorf("absent packaging must record empty, never inferred: %v", read.Packaging)
	}
	if read.Target != export.Target {
		t.Error("golden requires the same target on both rows")
	}
}

// TestEachPackagingWordHasProAndConFixtures gives every one of the
// eight words at least one acceptance fixture (inside an export row)
// and one rejection fixture (respelled off the closed set), so the
// vocabulary's teeth are per-word, not collective.
func TestEachPackagingWordHasProAndConFixtures(t *testing.T) {
	for _, w := range AllPackagingWords() {
		row, err := BuildExportRejudgement("docs/report.txt", DataActionExport, []PackagingWord{PackagingWord(w)})
		if err != nil || len(row.Packaging) != 1 || row.Packaging[0] != PackagingWord(w) {
			t.Fatalf("word %q failed its positive fixture: err=%v row=%+v", w, err, row)
		}
		if _, err := BuildExportRejudgement("docs/report.txt", DataActionExport, []PackagingWord{PackagingWord("x" + w)}); err == nil {
			t.Fatalf("word %q failed its negative fixture: respelling accepted", w)
		}
	}
	// Duplicates collapse, first-occurrence order kept.
	row, err := BuildExportRejudgement("t", DataActionExport, []PackagingWord{PackagingSend, PackagingCopy, PackagingSend})
	if err != nil || len(row.Packaging) != 2 || row.Packaging[0] != PackagingSend || row.Packaging[1] != PackagingCopy {
		t.Errorf("dedupe/order pin broken: err=%v row=%+v", err, row)
	}
}

// TestExportRejudgementRejectionsLeaveNoRecord pins the before-value
// rejection shape: every red input returns the zero struct plus an
// error - no partial row escapes the constructor.
func TestExportRejudgementRejectionsLeaveNoRecord(t *testing.T) {
	cases := []struct {
		name   string
		target string
		action DataAction
		wraps  []PackagingWord
	}{
		{"empty target", "", DataActionExport, nil},
		{"whitespace target", "   ", DataActionExport, nil},
		{"absent class", "t", "", nil},
		{"wild class", "t", DataAction("transmit"), nil},
		{"wild word", "t", DataActionExport, []PackagingWord{"transmit"}},
		{"empty word", "t", DataActionExport, []PackagingWord{""}},
	}
	for _, c := range cases {
		row, err := BuildExportRejudgement(c.target, c.action, c.wraps)
		if err == nil {
			t.Errorf("%s: rejection missing", c.name)
		}
		if row.Target != "" || row.Action != "" || len(row.Packaging) != 0 ||
			row.IndependentRejudgement || row.SensitivityPreserved || row.EnforcementPlane != "" {
			t.Errorf("%s: partial record leaked past rejection: %+v", c.name, row)
		}
	}
}

// TestExportRejudgementFieldSetCarriesNoDecision reflects over the
// record so no later edit can bolt an outcome onto the seed: exactly
// six fields, and no field name anywhere resembling a decision.
func TestExportRejudgementFieldSetCarriesNoDecision(t *testing.T) {
	tp := reflect.TypeOf(ExportRejudgementRecord{})
	if tp.NumField() != 6 {
		t.Fatalf("field set drifted (want 6): %d", tp.NumField())
	}
	red := regexp.MustCompile(`(?i)decision|allow|block|verdict|outcome`)
	for i := 0; i < tp.NumField(); i++ {
		if red.MatchString(tp.Field(i).Name) {
			t.Errorf("field %q resembles a decision - forbidden while the plane reads none", tp.Field(i).Name)
		}
	}
}

// TestExportRejudgementSymbolsStayOffTheDecisionPlane walks the four
// decision-plane directories and the command tree for this slice's
// symbols while the enforcement plane reads none-in-observation-phase,
// with a planted-shape positive control so the walk proves its own
// teeth before any green is believed.
func TestExportRejudgementSymbolsStayOffTheDecisionPlane(t *testing.T) {
	needle := "PackagingWord|AllPackagingWords|ExportPackagingSensitivityRule|ExportRejudgementIndependenceRule|ExportRejudgementAbsentDefault|ExportRejudgementEnforcementPlane|ExportRejudgementRecord|BuildExportRejudgement"
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
				t.Errorf("decision-plane file %s references export wrap symbol %q", filepath.Join(d, e.Name()), loc)
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
				t.Errorf("command tree file %s references export wrap symbol %q", p, loc)
			}
		}
	}
	walkCmd(filepath.Join("..", "..", "cmd"))
	if cmdHits == 0 {
		// Positive control: the same regexp must find the symbols in
		// this slice's own shipped source before any green from the
		// walk is believed.
		self, err := os.ReadFile("exportwrap.go")
		if err != nil {
			t.Fatalf("read self: %v", err)
		}
		if !re.MatchString(string(self)) {
			t.Error("positive control broken: needle matches no shipped export wrap source")
		}
	}
}
