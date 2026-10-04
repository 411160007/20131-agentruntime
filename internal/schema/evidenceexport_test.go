package schema

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// sampleExportTraces returns three fixed traces: one full, one
// with attr-backed fields unrecorded, one minimal. Values are
// copied literals - the export slice adds no judgement.
func sampleExportTraces() []DecisionTrace {
	return []DecisionTrace{
		{
			DecisionID: "d-1", Timestamp: "2026-10-04T20:00:00Z", Agent: "ag-1",
			Action: "exec", RiskFactors: "medium", PolicyRule: "r-9",
			Decision: "allow", PolicyVersion: "v2.0", EnforcementMode: "record_only_phase0",
			OriginEventID: "e-1", Capability: "shell", Resource: "host_fs",
			KnownGapFields:   []string{"intent", "outcome", "recovery_state", "task"},
			UnrecordedFields: []string{},
		},
		{
			DecisionID: "d-2", Timestamp: "2026-10-05T01:02:03Z", Agent: "ag-2",
			Action: "read", RiskFactors: "low", PolicyRule: "",
			Decision: "allow", PolicyVersion: "v2.0", EnforcementMode: "record_only_phase0",
			OriginEventID:    "e-2",
			KnownGapFields:   []string{"intent", "outcome", "recovery_state", "task"},
			UnrecordedFields: []string{"capability", "resource"},
		},
	}
}

func TestExportTracesJSONLByteIdenticalTwice(t *testing.T) {
	tr := sampleExportTraces()
	a, err := ExportTracesJSONL(tr)
	if err != nil {
		t.Fatalf("first export: %v", err)
	}
	b, err := ExportTracesJSONL(tr)
	if err != nil {
		t.Fatalf("second export: %v", err)
	}
	if string(a) != string(b) {
		t.Fatal("same input exported twice is not byte-identical")
	}
	lines := strings.Split(strings.TrimRight(string(a), "\n"), "\n")
	if len(lines) != 2 {
		t.Fatalf("jsonl line census %d, want 2", len(lines))
	}
	for _, ln := range lines {
		var m map[string]interface{}
		if err := json.Unmarshal([]byte(ln), &m); err != nil {
			t.Fatalf("jsonl line unparsable: %v", err)
		}
	}
}

func TestExportTracesCSVColumnOrderAndAbsentCells(t *testing.T) {
	tr := sampleExportTraces()
	c, err := ExportTracesCSV(tr)
	if err != nil {
		t.Fatalf("csv export: %v", err)
	}
	lines := strings.Split(strings.TrimRight(string(c), "\n"), "\n")
	if len(lines) != 3 {
		t.Fatalf("csv line census %d, want 3 (header+2)", len(lines))
	}
	hdr := strings.Split(lines[1-1], ",")
	cols := append(AllTraceFields(), "origin_event_id")
	if len(hdr) != len(cols) {
		t.Fatalf("csv header census %d, want %d", len(hdr), len(cols))
	}
	for i, h := range hdr {
		if h != cols[i] {
			t.Fatalf("csv header pin broke at %d: %q vs %q", i, h, cols[i])
		}
	}
	// Row 2 has empty policy_rule and uncarried attr fields: every
	// one must read the literal absent token, never "".
	row := strings.Split(lines[2], ",")
	wantAbsent := map[string]bool{"task": true, "intent": true, "outcome": true, "recovery_state": true, "capability": true, "resource": true, "policy_rule": true}
	for i, cell := range row {
		if wantAbsent[cols[i]] && cell != EvidenceAbsentCellToken {
			t.Fatalf("csv absent cell for %q is %q, want literal absent token", cols[i], cell)
		}
		if !wantAbsent[cols[i]] && cell == EvidenceAbsentCellToken {
			t.Fatalf("csv cell for %q wrongly reads absent", cols[i])
		}
	}
}

func TestExportTracesJSONValidArrayTwiceIdentical(t *testing.T) {
	tr := sampleExportTraces()
	a, err := ExportTracesJSON(tr)
	if err != nil {
		t.Fatal(err)
	}
	b, err := ExportTracesJSON(tr)
	if err != nil {
		t.Fatal(err)
	}
	if string(a) != string(b) {
		t.Fatal("json export not byte-identical")
	}
	var back []map[string]interface{}
	if err := json.Unmarshal(a, &back); err != nil {
		t.Fatalf("json export unparsable: %v", err)
	}
	if len(back) != 2 {
		t.Fatalf("json array census %d, want 2", len(back))
	}
	// known_gap fields stay OMITTED on the wire (struct omitempty
	// covers capability/resource); decision_id always present.
	if _, ok := back[1]["capability"]; ok {
		t.Fatal("unrecorded capability leaked into json")
	}
	if _, ok := back[0]["decision_id"]; !ok {
		t.Fatal("decision_id missing from json row")
	}
}

func TestBuildEvidenceManifestRejectsEmptySource(t *testing.T) {
	files := []EvidenceFile{{Name: "decisions.jsonl", Content: []byte("x\n")}}
	if _, err := BuildEvidenceManifest("", sampleExportTraces(), files); err == nil {
		t.Fatal("empty source accepted - a guessed source is a fabricated integrity field")
	}
	if _, err := BuildEvidenceManifest("s", sampleExportTraces(), []EvidenceFile{{Name: "manifest.json"}}); err == nil {
		t.Fatal("manifest pinning itself accepted")
	}
	if _, err := BuildEvidenceManifest("s", sampleExportTraces(), []EvidenceFile{{Name: "a.jsonl"}, {Name: "a.jsonl"}}); err == nil {
		t.Fatal("duplicate file name accepted")
	}
}

func TestManifestTimestampDerivedNotWallClock(t *testing.T) {
	tr := sampleExportTraces()
	body, err := ExportTracesJSONL(tr)
	if err != nil {
		t.Fatal(err)
	}
	man, err := BuildEvidenceManifest("audit-stream-x", tr, []EvidenceFile{{Name: "decisions.jsonl", Content: body}})
	if err != nil {
		t.Fatal(err)
	}
	var m evidenceManifest
	if err := json.Unmarshal(man, &m); err != nil {
		t.Fatal(err)
	}
	if m.SourceTimestamp != "2026-10-05T01:02:03Z" {
		t.Fatalf("envelope timestamp %q is not the latest trace timestamp", m.SourceTimestamp)
	}
	if m.TimestampRule != EvidenceTimestampRule {
		t.Fatal("timestamp rule line drifted")
	}
	if m.Source != "audit-stream-x" {
		t.Fatal("source not pinned")
	}
	// Zero traces: envelope timestamp is ABSENT (omitted), never a
	// fabricated default.
	man2, err := BuildEvidenceManifest("empty-src", nil, []EvidenceFile{{Name: "decisions.jsonl"}})
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(man2), "source_timestamp") {
		t.Fatal("empty evidence set fabricated a source_timestamp")
	}
}

func TestBundleManifestFilesOneToOne(t *testing.T) {
	tr := sampleExportTraces()
	eb, err := BuildEvidenceBundle(tr, "suite-x")
	if err != nil {
		t.Fatalf("bundle: %v", err)
	}
	files := eb.Files()
	if len(files) != 4 {
		t.Fatalf("bundle file census %d, want 4", len(files))
	}
	if probs := VerifyEvidenceBundle(eb.Manifest, files); len(probs) != 0 {
		t.Fatalf("clean bundle verifies red: %v", probs)
	}
	// byte-identical rebuild of the whole bundle
	eb2, err := BuildEvidenceBundle(tr, "suite-x")
	if err != nil {
		t.Fatal(err)
	}
	if string(eb2.Manifest) != string(eb.Manifest) || string(eb2.DecisionsCSV) != string(eb.DecisionsCSV) {
		t.Fatal("bundle not byte-identical across rebuilds")
	}
}

func TestVerifyBundleTamperGoesRed(t *testing.T) {
	tr := sampleExportTraces()
	eb, err := BuildEvidenceBundle(tr, "suite-y")
	if err != nil {
		t.Fatal(err)
	}
	files := eb.Files()
	for i := range files {
		if files[i].Name == "decisions.csv" {
			files[i].Content = []byte(strings.Replace(string(files[i].Content), "d-2", "d-9", 1))
		}
	}
	probs := VerifyEvidenceBundle(eb.Manifest, files)
	if len(probs) != 1 || !strings.Contains(probs[0], "integrity hash mismatch for decisions.csv") {
		t.Fatalf("single-byte tamper verdict wrong: %v", probs)
	}
	// missing file
	var kept []EvidenceFile
	for _, f := range eb.Files() {
		if f.Name != "decisions.json" {
			kept = append(kept, f)
		}
	}
	probs = VerifyEvidenceBundle(eb.Manifest, kept)
	if len(probs) != 1 || !strings.Contains(probs[0], "absent: decisions.json") {
		t.Fatalf("missing-file verdict wrong: %v", probs)
	}
	// extra file
	probs = VerifyEvidenceBundle(eb.Manifest, append(eb.Files(), EvidenceFile{Name: "ghost.jsonl", Content: []byte("hi\n")}))
	if len(probs) != 1 || !strings.Contains(probs[0], "not pinned in manifest: ghost.jsonl") {
		t.Fatalf("extra-file verdict wrong: %v", probs)
	}
	// forged manifest: unparsable + version drift
	if probs := VerifyEvidenceBundle([]byte("{not json"), eb.Files()); len(probs) != 1 {
		t.Fatalf("unparsable manifest verdict wrong: %v", probs)
	}
	if probs := VerifyEvidenceBundle([]byte(`{"manifest_version":"v9","files":[]}`), eb.Files()); len(probs) != 1 {
		t.Fatalf("version drift verdict wrong: %v", probs)
	}
}

func TestExportZeroWriteBackToSourceFile(t *testing.T) {
	dir := t.TempDir()
	src := filepath.Join(dir, "policy-decision.jsonl")
	body := `{"id":"d-1","type":"policy.decision"}` + "\n"
	if err := os.WriteFile(src, []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
	st1, err := os.Stat(src)
	if err != nil {
		t.Fatal(err)
	}
	before := sha256Hex([]byte(body))
	tr := sampleExportTraces()
	if _, err := BuildEvidenceBundle(tr, src); err != nil {
		t.Fatal(err)
	}
	raw, err := os.ReadFile(src)
	if err != nil {
		t.Fatal(err)
	}
	st2, err := os.Stat(src)
	if err != nil {
		t.Fatal(err)
	}
	if sha256Hex(raw) != before {
		t.Fatal("source audit file content changed after export - write-back happened")
	}
	if !st1.ModTime().Equal(st2.ModTime()) || st1.Size() != st2.Size() {
		t.Fatal("source audit file mtime/size changed after export")
	}
	// Traces themselves must not be mutated either.
	if tr[0].DecisionID != "d-1" || len(tr) != 2 {
		t.Fatal("input traces mutated by export")
	}
}

func TestEvidenceVocabCensusesAndStances(t *testing.T) {
	if got := AllEvidenceFormats(); len(got) != 5 || got[0] != "jsonl" {
		t.Fatalf("format vocabulary drifted: %v", got)
	}
	if got := AllEvidenceMembers(); len(got) != 7 || got[4] != "policies.json" {
		t.Fatalf("member vocabulary drifted: %v", got)
	}
	if got := AllEvidenceIntegrityFields(); len(got) != 7 || got[6] != "actor_identity" {
		t.Fatalf("integrity vocabulary drifted: %v", got)
	}
	fc := AllEvidenceFormatCoverage()
	mc := AllEvidenceMemberCoverage()
	if len(fc) != 5 || len(mc) != 7 {
		t.Fatal("coverage registration census drifted")
	}
	fCensus := map[EvidenceFormatStance]int{}
	for _, r := range fc {
		if !r.Stance.valid() {
			t.Fatalf("wild stance on format row %q", r.Format)
		}
		fCensus[r.Stance]++
	}
	if fCensus[StanceDeliveredV0] != 3 || fCensus[StanceGapAbsent] != 2 || fCensus[StanceInlineV0] != 0 {
		t.Fatalf("format stance census drifted: %+v", fCensus)
	}
	mCensus := map[EvidenceFormatStance]int{}
	for _, r := range mc {
		if !r.Stance.valid() {
			t.Fatalf("wild stance on member row %q", r.Member)
		}
		mCensus[r.Stance]++
	}
	if mCensus[StanceDeliveredV0] != 2 || mCensus[StanceInlineV0] != 1 || mCensus[StanceGapAbsent] != 4 {
		t.Fatalf("member stance census drifted: %+v", mCensus)
	}
	for _, name := range []string{"jsonl", "csv", "json", "pdf", "signed_bundle"} {
		if _, ok := FormatStanceOf(name); !ok {
			t.Fatalf("format %q not registered", name)
		}
	}
	if _, ok := FormatStanceOf("xml"); ok {
		t.Fatal("wild format accepted")
	}
	if _, ok := MemberStanceOf("hashes"); !ok {
		t.Fatal("hashes member not registered")
	}
	if v := TraceCellValue(sampleExportTraces()[0], "nope"); v != "" {
		t.Fatal("non-251 column silently produced a value")
	}
}
