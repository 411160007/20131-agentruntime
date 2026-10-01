package schema

import (
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"regexp"
	"strings"
	"testing"
)

// Impact and recovery contract sync tests (stability wave, slice W2.3).
// They pin docs/schema-v2.md sections 6 and 7 against the Go
// declarations in impactrecovery.go: vocabularies are compared as
// joined strings in normative order, the anchor back-checks are
// recomputed mechanically on every run (never by hand), the scope table
// is pinned by row order against the specification's eight bullets, and
// the fixture set drives the closed sets: every good record
// round-trips, every wild shape is rejected before any bytes or usable
// values exist. The grep gates mirror the Node checker's red-shape
// assertions from this side: no wave record symbol and no
// recovery-execution vocabulary may touch the decision plane or the
// command tree while the execution plane is contracted as none. Pure
// record and documentation evolution: nothing here changes runtime
// decision semantics.

func impactAnchorLines(t *testing.T, doc string) []string {
	t.Helper()
	hits := regexp.MustCompile("(?s)```impact-spec-anchor\n(.*?)```").FindAllStringSubmatch(doc, -1)
	if len(hits) != 1 {
		t.Fatalf("impact anchor blocks found %d, want exactly 1", len(hits))
	}
	var out []string
	for _, l := range strings.Split(hits[0][1], "\n") {
		if l = strings.TrimSpace(l); l != "" {
			out = append(out, l)
		}
	}
	return out
}

func recoveryAnchorLines(t *testing.T, doc string) []string {
	t.Helper()
	hits := regexp.MustCompile("(?s)```recovery-spec-anchor\n(.*?)```").FindAllStringSubmatch(doc, -1)
	if len(hits) != 1 {
		t.Fatalf("recovery anchor blocks found %d, want exactly 1", len(hits))
	}
	var out []string
	for _, l := range strings.Split(hits[0][1], "\n") {
		if l = strings.TrimSpace(l); l != "" {
			out = append(out, l)
		}
	}
	return out
}

func TestImpactContractSourcesAgree(t *testing.T) {
	raw, err := os.ReadFile(filepathJoinDotDot("docs", "schema-v2.md"))
	if err != nil {
		t.Skipf("schema-v2 doc not visible: %v", err)
	}
	doc := string(raw)
	vocab := schemaV2Key(t, doc, "impact_field_vocabulary")
	if joined := strings.Join(AllImpactFields(), ", "); joined != vocab {
		t.Errorf("impact vocabulary drift:\n docs: %q\n go:   %q", vocab, joined)
	}
	count := schemaV2Key(t, doc, "impact_field_count")
	if count != "7" || len(AllImpactFields()) != 7 {
		t.Errorf("impact count: docs %q, go %d, want programmatic 7", count, len(AllImpactFields()))
	}
	if absent := schemaV2Key(t, doc, "impact_absent_semantics"); absent != "not-estimated-known-gap" {
		t.Errorf("impact absent-field semantics drifted to %q", absent)
	}
	if plane := schemaV2Key(t, doc, "impact_enforcement_plane"); plane != ImpactEnforcementPlane {
		t.Errorf("impact enforcement plane docs %q, go %q", plane, ImpactEnforcementPlane)
	}
	if plane := schemaV2Key(t, doc, "impact_enforcement_plane"); plane != "none-in-observation-phase" {
		t.Errorf("impact observation phase must not borrow any enforcement plane: %q", plane)
	}
	lines := impactAnchorLines(t, doc)
	if len(lines) != 7 {
		t.Fatalf("impact anchor line census %d, want 7", len(lines))
	}
	for i, l := range lines {
		snake := strings.ReplaceAll(strings.ToLower(l), " ", "_")
		if snake != AllImpactFields()[i] {
			t.Errorf("impact anchor back-check line %d (%q): snake form %q, wire token %q", i+1, l, snake, AllImpactFields()[i])
		}
	}
	svocab := schemaV2Key(t, doc, "blast_radius_scope_vocabulary")
	if joined := strings.Join(AllBlastScopes(), ", "); joined != svocab {
		t.Errorf("blast scope vocabulary drift:\n docs: %q\n go:   %q", svocab, joined)
	}
	if scount := schemaV2Key(t, doc, "blast_radius_scope_count"); scount != "8" || len(AllBlastScopes()) != 8 {
		t.Errorf("blast scope count: docs %q, go %d, want programmatic 8", scount, len(AllBlastScopes()))
	}
	// Row-order pin (honest degradation from a verbatim anchor: the
	// specification's eight scope bullets are Chinese prose, so the
	// docs table rows stand in declared order and are recounted here).
	rows := regexp.MustCompile(`(?m)^\| `+"`([a-z_]+_scope)`"+` \|[^|\n]+\| line (\d+) \|`).FindAllStringSubmatch(doc, -1)
	if len(rows) != 8 {
		t.Fatalf("scope table row census %d, want 8", len(rows))
	}
	tokens := strings.Split(svocab, ", ")
	for i, r := range rows {
		if r[1] != tokens[i] {
			t.Errorf("scope row-order pin broke at row %d: table %q, vocabulary %q", i+1, r[1], tokens[i])
		}
		if r[2] != fmt.Sprint(i+1) {
			t.Errorf("scope bullet pointer off at row %d: %q", i+1, r[2])
		}
	}
}

func TestRecoveryContractSourcesAgree(t *testing.T) {
	raw, err := os.ReadFile(filepathJoinDotDot("docs", "schema-v2.md"))
	if err != nil {
		t.Skipf("schema-v2 doc not visible: %v", err)
	}
	doc := string(raw)
	vocab := schemaV2Key(t, doc, "recovery_class_vocabulary")
	if joined := strings.Join(AllRecoveryClasses(), ", "); joined != vocab {
		t.Errorf("recovery class vocabulary drift:\n docs: %q\n go:   %q", vocab, joined)
	}
	if count := schemaV2Key(t, doc, "recovery_class_count"); count != "4" || len(AllRecoveryClasses()) != 4 {
		t.Errorf("recovery class count: docs %q, go %d, want programmatic 4", count, len(AllRecoveryClasses()))
	}
	if unc := schemaV2Key(t, doc, "recovery_unclassified_semantics"); unc != "absent-record-means-unknown-never-imply-reversible" {
		t.Errorf("unclassified-record semantics drifted to %q", unc)
	}
	plane := schemaV2Key(t, doc, "recovery_execution_plane")
	if plane != RecoveryExecutionPlane {
		t.Errorf("recovery execution plane docs %q, go %q", plane, RecoveryExecutionPlane)
	}
	if plane != "none-in-observation-phase" {
		t.Errorf("observation phase must not borrow the recovery execution plane: %q", plane)
	}
	rule := schemaV2Key(t, doc, "recovery_truthfulness_rule")
	if rule != RecoveryTruthfulnessRule {
		t.Errorf("truthfulness rule docs %q, go %q", rule, RecoveryTruthfulnessRule)
	}
	if rule != "never-claim-fully-reversible" {
		t.Errorf("truthfulness rule drifted off the recorded ban: %q", rule)
	}
	lines := recoveryAnchorLines(t, doc)
	if len(lines) != 4 {
		t.Fatalf("recovery anchor line census %d, want 4", len(lines))
	}
	toks := strings.Split(vocab, ", ")
	for i, l := range lines {
		snake := strings.ToLower(strings.ReplaceAll(strings.ReplaceAll(l, " ", "_"), "-", "_"))
		if snake != toks[i] {
			t.Errorf("recovery anchor back-check line %d (%q): snake-h form %q, wire token %q", i+1, l, snake, toks[i])
		}
	}
	// The display spellings themselves must never leak to the wire.
	for _, display := range lines {
		if RecoveryClass(display).Valid() {
			t.Errorf("spec display spelling %q accepted as a wire token", display)
		}
	}
}

// TestImpactRecoveryRecordShapeClosedSet pins record geometry to the
// declared vocabularies: exactly seven impact fields in vocabulary
// order, every one optional (omitempty = the honest "not estimated"
// default), Reversibility typed as the shared recovery class, and the
// recovery class as the single required carrier of its record.
func TestImpactRecoveryRecordShapeClosedSet(t *testing.T) {
	rt := reflect.TypeOf(ImpactRecord{})
	if rt.NumField() != 7 {
		t.Fatalf("ImpactRecord has %d fields, want 7", rt.NumField())
	}
	fields := AllImpactFields()
	for i := 0; i < rt.NumField(); i++ {
		tag := strings.Split(rt.Field(i).Tag.Get("json"), ",")[0]
		if tag != fields[i] {
			t.Errorf("impact field %d: tag %q, vocabulary %q", i, tag, fields[i])
		}
		if !strings.Contains(rt.Field(i).Tag.Get("json"), "omitempty") {
			t.Errorf("impact field %s must be optional with an absent default", tag)
		}
	}
	if rv := reflect.TypeOf(ImpactRecord{}).Field(4).Type; rv.Name() != "RecoveryClass" {
		t.Errorf("reversibility field type %q, want the shared RecoveryClass vocabulary", rv.Name())
	}
	bt := reflect.TypeOf(BlastRadiusEstimate{})
	if bt.NumField() != 2 {
		t.Fatalf("BlastRadiusEstimate has %d fields, want 2", bt.NumField())
	}
	for i := 0; i < bt.NumField(); i++ {
		if !strings.Contains(bt.Field(i).Tag.Get("json"), "omitempty") {
			t.Errorf("blast estimate part %s must be optional", bt.Field(i).Tag.Get("json"))
		}
	}
	ct := reflect.TypeOf(RecoveryRecord{})
	if ct.NumField() != 2 {
		t.Fatalf("RecoveryRecord has %d fields, want 2", ct.NumField())
	}
	if strings.Contains(ct.Field(0).Tag.Get("json"), "omitempty") {
		t.Error("recovery class must stay required: an unclassed record is malformed, not unknown")
	}
	if !strings.Contains(ct.Field(1).Tag.Get("json"), "omitempty") {
		t.Error("the transaction correlation is an optional record bit")
	}
}

// TestRecoveryClassDiscrimination proves the closed set has teeth in
// both directions: all four members validate, and every non-member -
// the spec display spellings, execution-verb coinages, the empty
// token, and near-synonyms - is rejected.
func TestRecoveryClassDiscrimination(t *testing.T) {
	for _, c := range AllRecoveryClasses() {
		if err := (RecoveryRecord{Class: RecoveryClass(c)}).Validate(); err != nil {
			t.Errorf("closed-set member %q rejected: %v", c, err)
		}
	}
	for _, wild := range []RecoveryClass{
		"", "LOCAL REVERSIBLE", "NON-REVERSIBLE", "reversible", "partial",
		"auto_rollback", "rolled_back", "fully_reversible", "Local_Reversible",
	} {
		if err := (RecoveryRecord{Class: wild}).Validate(); err == nil {
			t.Errorf("non-member class %q accepted", wild)
		}
	}
	if got := fmt.Sprint(len(AllRecoveryClasses())); got != "4" {
		t.Errorf("class census %s, want 4", got)
	}
}

// TestImpactBlastScopeDiscrimination: the eight scope tokens in, bare
// nouns and wild spellings out.
func TestImpactBlastScopeDiscrimination(t *testing.T) {
	for _, s := range AllBlastScopes() {
		if err := (&BlastRadiusEstimate{Scopes: []BlastScope{BlastScope(s)}}).Validate(); err != nil {
			t.Errorf("closed-set scope %q rejected: %v", s, err)
		}
	}
	for _, wild := range []BlastScope{"", "network", "FILE_SCOPE", "secrets", "sub_agent"} {
		if err := (&BlastRadiusEstimate{Scopes: []BlastScope{wild}}).Validate(); err == nil {
			t.Errorf("wild scope %q accepted", wild)
		}
	}
	if err := (&ImpactRecord{Reversibility: "auto_rollback_armed"}).Validate(); err == nil {
		t.Error("execution-verb reversibility pairing accepted on the impact record")
	}
	if err := (&ImpactRecord{Reversibility: RecoveryNonReversible}).Validate(); err != nil {
		t.Errorf("closed-class reversibility observation rejected: %v", err)
	}
}

func readFixtureLines(t *testing.T, name string) []string {
	t.Helper()
	raw, err := os.ReadFile(filepathJoinDotDot("testdata", "impactrecovery", name))
	if err != nil {
		t.Fatalf("fixture %s: %v", name, err)
	}
	var out []string
	for _, l := range strings.Split(string(raw), "\n") {
		if strings.TrimSpace(l) != "" {
			out = append(out, l)
		}
	}
	return out
}

// TestImpactRoundTripAndWildReject drives the impact fixtures: good
// records parse, encode, and re-parse to identical shapes; wild
// records are rejected with a nil result and zero produced bytes.
// Counts are asserted programmatically.
func TestImpactRoundTripAndWildReject(t *testing.T) {
	good := readFixtureLines(t, "good_impact.jsonl")
	if len(good) != 3 {
		t.Fatalf("impact good fixture census %d, want 3 (programmatic)", len(good))
	}
	produced := 0
	for i, line := range good {
		rec, err := ParseImpactRecord([]byte(line))
		if err != nil {
			t.Fatalf("impact good[%d] rejected: %v", i, err)
		}
		b1, err := EncodeImpactChecked(rec)
		if err != nil {
			t.Fatalf("impact good[%d] encode rejected: %v", i, err)
		}
		produced += len(b1)
		rec2, err := ParseImpactRecord(b1)
		if err != nil || !reflect.DeepEqual(rec, rec2) {
			t.Errorf("impact good[%d] round trip unstable: %v", i, err)
		}
		b2, _ := EncodeImpactChecked(rec2)
		if string(b1) != string(b2) {
			t.Errorf("impact good[%d] re-encode differs", i)
		}
	}
	if produced == 0 {
		t.Error("no bytes produced for valid records - silent fixture failure")
	}
	// Honest empty: a fully absent impact record is valid and encodes
	// to {} - every dimension simply "not estimated".
	empty, err := ParseImpactRecord([]byte("{}"))
	if err != nil {
		t.Fatalf("empty impact record rejected: %v", err)
	}
	if b, err := EncodeImpactChecked(empty); err != nil || string(b) != "{}" {
		t.Errorf("empty impact record must encode to {}, got %q err %v", b, err)
	}
}

// TestRecoveryRoundTripAndClassDiscrimination consumes the recovery
// good fixture as the four-positive-class control and proves the
// required-class rule directly.
func TestRecoveryRoundTripAndClassDiscrimination(t *testing.T) {
	good := readFixtureLines(t, "good_recovery.jsonl")
	if len(good) != 4 {
		t.Fatalf("recovery good fixture census %d, want 4 (programmatic)", len(good))
	}
	seen := map[string]int{}
	produced := 0
	for i, line := range good {
		rec, err := ParseRecoveryRecord([]byte(line))
		if err != nil {
			t.Fatalf("recovery good[%d] rejected: %v", i, err)
		}
		b1, err := EncodeRecoveryChecked(rec)
		if err != nil {
			t.Fatalf("recovery good[%d] encode rejected: %v", i, err)
		}
		produced += len(b1)
		rec2, err := ParseRecoveryRecord(b1)
		if err != nil || !reflect.DeepEqual(rec, rec2) {
			t.Errorf("recovery good[%d] round trip unstable: %v", i, err)
		}
		seen[string(rec.Class)]++
	}
	if produced == 0 {
		t.Error("no bytes produced for valid records - silent fixture failure")
	}
	if len(seen) != 4 {
		t.Errorf("class discrimination: %d distinct classes exercised, want all 4 (%v)", len(seen), seen)
	}
}

// TestImpactRecoveryWildReject drives the shared wild fixture: every
// rejected shape must be rejected by BOTH parsers (a class record is
// malformed as an impact record and vice versa), with nil results and
// zero produced bytes behind each rejection.
func TestImpactRecoveryWildReject(t *testing.T) {
	wild := readFixtureLines(t, "wild.jsonl")
	if len(wild) != 7 {
		t.Fatalf("wild fixture census %d, want 7 (programmatic)", len(wild))
	}
	for i, line := range wild {
		if rec, err := ParseImpactRecord([]byte(line)); err == nil || rec != nil {
			t.Errorf("wild[%d] accepted by the impact parser: %v", i, rec)
		}
		if rec, err := ParseRecoveryRecord([]byte(line)); err == nil || rec != nil {
			t.Errorf("wild[%d] accepted by the recovery parser: %v", i, rec)
		}
		if b, err := EncodeImpactChecked(nil); err == nil || len(b) != 0 {
			t.Fatalf("nil impact record must reject with zero bytes, got %d", len(b))
		}
		if b, err := EncodeRecoveryChecked(nil); err == nil || len(b) != 0 {
			t.Fatalf("nil recovery record must reject with zero bytes, got %d", len(b))
		}
	}
}

// TestNoWaveSymbolsOrExecutionVerbsInPlanes mirrors the Node checker's
// two structural greps from the Go side: decision-plane directories and
// the command tree may not reference the wave record symbols, and they
// may not carry recovery-execution vocabulary while the execution plane
// is contracted as none. Zero-hit baseline asserted every run; any hit
// is a deliberate contract event, never a silent one.
func TestNoWaveSymbolsOrExecutionVerbsInPlanes(t *testing.T) {
	needle := regexp.MustCompile(`GrantOrigin|AuthorityChain|IntentRecord|intentFieldWireNames|RecoveryClass|RecoveryRecord|ImpactRecord|BlastScope|BlastRadiusEstimate|blastScopeWireNames|impactFieldWireNames`)
	verbs := regexp.MustCompile(`(?i)rollback|revert|undo|compensat`)
	dirs := []string{
		filepathJoinDotDot("internal", "policy") + string(os.PathSeparator),
		filepathJoinDotDot("internal", "rules") + string(os.PathSeparator),
		filepathJoinDotDot("internal", "bus") + string(os.PathSeparator),
		filepathJoinDotDot("internal", "auditlog") + string(os.PathSeparator),
		filepathJoinDotDot("cmd") + string(os.PathSeparator),
	}
	scanned := 0
	for _, d := range dirs {
		base := strings.TrimSuffix(d, string(os.PathSeparator))
		if err := filepath.Walk(base, func(p string, info os.FileInfo, err error) error {
			if err != nil {
				return nil // missing directory: nothing to leak there
			}
			if info.IsDir() || !strings.HasSuffix(p, ".go") || strings.HasSuffix(p, "_test.go") {
				return nil
			}
			raw, rerr := os.ReadFile(p)
			if rerr != nil {
				return rerr
			}
			scanned++
			text := string(raw)
			if needle.MatchString(text) {
				t.Errorf("%s references wave record symbols", p)
			}
			if verbs.MatchString(text) {
				t.Errorf("%s carries recovery-execution vocabulary", p)
			}
			return nil
		}); err != nil {
			t.Fatalf("walk %s: %v", base, err)
		}
	}
	if scanned == 0 {
		t.Fatal("no files scanned - silent probe failure guard")
	}
	t.Logf("plane grep scanned %d non-test files, zero hits", scanned)
}
