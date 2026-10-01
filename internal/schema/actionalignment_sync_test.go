package schema

import (
	"bufio"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

// Intent alignment contract sync tests (alignment wave, slice W3.2).
// They pin docs/schema-v2.md section 12 against the Go declarations in
// actionalignment.go: the four-class vocabulary is compared as a joined
// string in normative order, the anchor back-check is recomputed
// mechanically on every run (never by hand), the record field list is
// pinned by row order against the docs table, and the fixture set
// drives the closed sets: every good record round-trips, every wild
// shape - including every malice-flavoured coinage, because the two
// honesty equations of section 232 are structural - is rejected before
// any bytes or usable values exist. The grep gates mirror the Node
// checker's red-shape assertions from this side: no alignment record
// symbol may touch the decision planes or the command tree while the
// enforcement plane is contracted as none, and the Phase 0 decision
// closed pair {allow, would_block} must be exactly what this slice
// found and exactly what it leaves behind. Pure record and
// documentation evolution: nothing here changes runtime decision
// semantics.

func alignmentAnchorLines(t *testing.T, doc string) []string {
	t.Helper()
	hits := regexp.MustCompile("(?s)```alignment-spec-anchor\n(.*?)```").FindAllStringSubmatch(doc, -1)
	if len(hits) != 1 {
		t.Fatalf("alignment anchor blocks found %d, want exactly 1", len(hits))
	}
	var out []string
	for _, l := range strings.Split(hits[0][1], "\n") {
		if l = strings.TrimSpace(l); l != "" {
			out = append(out, l)
		}
	}
	return out
}

func TestAlignmentContractSourcesAgree(t *testing.T) {
	doc := readRepoFile(t, filepath.Join("docs", "schema-v2.md"))
	vocab := schemaV2Key(t, doc, "alignment_class_vocabulary")
	if joined := strings.Join(AllActionClasses(), ", "); joined != vocab {
		t.Errorf("alignment vocabulary drift:\n docs: %q\n go:   %q", vocab, joined)
	}
	count := schemaV2Key(t, doc, "alignment_class_count")
	if count != "4" || len(AllActionClasses()) != 4 {
		t.Errorf("alignment count: docs %q, go %d, want programmatic 4", count, len(AllActionClasses()))
	}
	if absent := schemaV2Key(t, doc, "alignment_absent_semantics"); absent != AlignmentAbsentSemantics {
		t.Errorf("alignment absent semantics docs %q, go %q", absent, AlignmentAbsentSemantics)
	}
	if mal := schemaV2Key(t, doc, "alignment_malicious_rule"); mal != AlignmentMaliciousRule {
		t.Errorf("malicious-equation rule docs %q, go %q", mal, AlignmentMaliciousRule)
	}
	if esc := schemaV2Key(t, doc, "alignment_escalation_precondition"); esc != AlignmentEscalationPrecondition {
		t.Errorf("escalation precondition docs %q, go %q", esc, AlignmentEscalationPrecondition)
	}
	if plane := schemaV2Key(t, doc, "alignment_enforcement_plane"); plane != AlignmentEnforcementPlane {
		t.Errorf("alignment enforcement plane docs %q, go %q", plane, AlignmentEnforcementPlane)
	}
	if plane := schemaV2Key(t, doc, "alignment_enforcement_plane"); plane != "none-in-observation-phase" {
		t.Errorf("alignment observation phase must not borrow any enforcement plane: %q", plane)
	}
	lines := alignmentAnchorLines(t, doc)
	if len(lines) != 4 {
		t.Fatalf("alignment anchor line census %d, want 4", len(lines))
	}
	for i, l := range lines {
		if strings.ToLower(l) != AllActionClasses()[i] {
			t.Errorf("alignment anchor back-check line %d (%q): lower form %q, wire token %q",
				i+1, l, strings.ToLower(l), AllActionClasses()[i])
		}
	}
	// Record field table row-order pin: docs rows stand in declared
	// order and are recounted against the Go list here.
	secAt := strings.Index(doc, "## 12. Intent alignment record contract")
	if secAt < 0 {
		t.Fatalf("alignment section header missing from docs")
	}
	rows := regexp.MustCompile(`(?m)^\| `+"`([a-z_]+)`"+` \| (yes|no) \|[^|\n]*\|`).FindAllStringSubmatch(doc[secAt:], -1)
	if len(rows) != 5 {
		t.Fatalf("alignment field table row census %d, want 5", len(rows))
	}
	for i, r := range rows {
		if r[1] != AllAlignmentRecordFields()[i] {
			t.Errorf("alignment field row-order pin broke at row %d: docs %q, go %q", i+1, r[1], AllAlignmentRecordFields()[i])
		}
	}
}

func TestAlignmentGoodFixturesRoundTrip(t *testing.T) {
	raw, err := os.ReadFile(filepathJoinDotDot("testdata", "actionalignment", "good_alignment.jsonl"))
	if err != nil {
		t.Fatalf("read good fixtures: %v", err)
	}
	want := AllActionClasses()
	lines := nonEmptyLines(string(raw))
	if len(lines) != 4 {
		t.Fatalf("good fixture line census %d, want 4 (one per class, declared order)", len(lines))
	}
	for i, line := range lines {
		rec, err := ParseAlignmentRecord([]byte(line))
		if err != nil {
			t.Fatalf("good line %d rejected: %v", i+1, err)
		}
		if string(rec.Class) != want[i] {
			t.Errorf("good line %d class %q, want %q (row-order pin)", i+1, rec.Class, want[i])
		}
		out, err := EncodeAlignmentChecked(rec)
		if err != nil || string(out) != line {
			t.Errorf("good line %d did not round-trip verbatim: out=%q err=%v", i+1, string(out), err)
		}
	}
}

func TestAlignmentWildFixturesRejected(t *testing.T) {
	raw, err := os.ReadFile(filepathJoinDotDot("testdata", "actionalignment", "wild_alignment.jsonl"))
	if err != nil {
		t.Fatalf("read wild fixtures: %v", err)
	}
	lines := nonEmptyLines(string(raw))
	if len(lines) < 4 {
		t.Fatalf("wild fixture census %d, want at least 4 (one negative per class)", len(lines))
	}
	// One negative golden per class is machine-pinned by class token:
	// upper-case respelling (direct), near-miss coinage (inferred),
	// the malice confusion the honesty equations forbid (uncertain),
	// and a distance synonym (unrelated).
	seen := map[string]bool{}
	for i, line := range lines {
		rec, err := ParseAlignmentRecord([]byte(line))
		if err == nil || rec != nil {
			t.Errorf("wild line %d accepted: %v", i+1, err)
		}
		for _, k := range []string{`"DIRECT"`, `"inference"`, `"malicious"`, `"irrelevant"`} {
			if strings.Contains(line, k) {
				seen[k] = true
			}
		}
	}
	for _, k := range []string{`"DIRECT"`, `"inference"`, `"malicious"`, `"irrelevant"`} {
		if !seen[k] {
			t.Errorf("wild fixture set lost its negative golden for class against %s", k)
		}
	}
}

func TestAlignmentEncodeProducesNoBytesOnRejection(t *testing.T) {
	bad := &AlignmentRecord{Class: ActionClass("malicious")}
	out, err := EncodeAlignmentChecked(bad)
	if err == nil {
		t.Errorf("encode accepted a malice-shaped class: %s", string(out))
	}
	if out != nil {
		t.Errorf("rejection must return nil bytes, got %q", string(out))
	}
	var nilRec *AlignmentRecord
	if out, err := EncodeAlignmentChecked(nilRec); err == nil || out != nil {
		t.Errorf("nil record must reject with nil bytes (err=%v)", err)
	}
}

func TestAlignmentSymbolsStayOffTheDecisionPlane(t *testing.T) {
	needle := "ActionClass|AlignmentRecord|alignmentClassWireNames|alignmentRecordFieldWireNames|AlignmentAbsentSemantics|AlignmentMaliciousRule|AlignmentEscalationPrecondition|AlignmentEnforcementPlane"
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
				t.Fatalf("read %s: %v", e.Name(), err)
			}
			if loc := re.FindString(string(data)); loc != "" {
				t.Errorf("decision-plane file %s references alignment record symbol %q", filepath.Join(d, e.Name()), loc)
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
				t.Errorf("command tree file %s references alignment record symbol %q", p, loc)
			}
		}
	}
	walkCmd(filepath.Join("..", "..", "cmd"))
	if cmdHits == 0 {
		// positive control for the walk itself: a planted reference in
		// this package's own file is found by the same regexp.
		if !re.MatchString(string(mustReadFile(t, "actionalignment.go"))) {
			t.Errorf("positive control broken: needle matches no shipped alignment source")
		}
	}
}

func TestAlignmentLeavesDecisionSetUntouched(t *testing.T) {
	// Phase 0 red line: the runtime decision closed pair is exactly
	// {allow, would_block} before and after this slice, and no
	// alignment token may show up in any decision vocabulary.
	pair := Phase0RuntimeDecisions()
	if len(pair) != 2 || pair[0] != DecisionAllow || pair[1] != DecisionWouldBlock {
		t.Fatalf("phase-0 runtime decisions drifted: %v", pair)
	}
	all := strings.Join(AllDecisions(), ", ")
	for _, cls := range AllActionClasses() {
		if strings.Contains(all, cls) {
			t.Errorf("alignment class %q leaked into the decision vocabulary: %q", cls, all)
		}
	}
	doc := readRepoFile(t, filepath.Join("docs", "schema-v2.md"))
	if p0 := schemaV2Key(t, doc, "decision_phase0_runtime_set"); p0 != "allow, would_block" {
		t.Errorf("docs phase-0 runtime set drifted to %q", p0)
	}
}

func nonEmptyLines(s string) []string {
	var out []string
	sc := bufio.NewScanner(strings.NewReader(s))
	sc.Buffer(make([]byte, 1<<20), 1<<20)
	for sc.Scan() {
		if l := strings.TrimSpace(sc.Text()); l != "" {
			out = append(out, l)
		}
	}
	return out
}

func mustReadFile(t *testing.T, name string) []byte {
	t.Helper()
	data, err := os.ReadFile(name)
	if err != nil {
		t.Fatalf("read %s: %v", name, err)
	}
	return data
}
