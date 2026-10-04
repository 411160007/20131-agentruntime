// report_test.go + evidence_test.go - W7.3 machine gate:
// golden output snapshots, determinism, honest skip buckets,
// source-untouched proof, refuse-without-dir proof, and the
// tamper negative control through the CLI write path.
package main

import (
	"crypto/sha256"
	"encoding/hex"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
	"time"

	"20131.com/agentruntime/internal/schema"
)

func cliEvents(t *testing.T) []schema.Event {
	t.Helper()
	evs, err := timelineEvents(filepath.Join("..", "..", "testdata", "timeline-sample.jsonl"), "", time.Time{})
	if err != nil {
		t.Fatalf("load sample: %v", err)
	}
	return evs
}

// TestReportGolden pins the report renderer output byte-for-byte.
func TestReportGolden(t *testing.T) {
	lines := strings.Join(append(renderReport(cliEvents(t)), ""), "\n")
	want, err := os.ReadFile(filepath.Join("..", "..", "testdata", "report-golden.txt"))
	if err != nil {
		t.Fatalf("read golden: %v", err)
	}
	got := strings.TrimSuffix(lines, "\n") + "\n"
	if got != string(want) {
		t.Fatalf("report golden drift:\ngot:\n%s\nwant:\n%s", got, want)
	}
}

// TestEvidenceBuildTracesHonest: the one policy.decision line in the
// sample correlates to the preceding same-agent line, no skip is
// invented, and buckets read as what they are.
func TestEvidenceBuildTracesHonest(t *testing.T) {
	evs := cliEvents(t)
	traces, skips := buildTraces(evs, "builtin-v1")
	nDec := 0
	for _, e := range evs {
		if e.Type == schema.TypePolicyDecision {
			nDec++
		}
	}
	if len(traces) != nDec {
		t.Fatalf("all %d sample decisions must trace, got %d (skips=%v)", nDec, len(traces), skips)
	}
	for _, b := range evidenceSkipBuckets {
		if skips[b] != 0 {
			t.Fatalf("sample has no %s lines, got %d", b, skips[b])
		}
	}
	if traces[0].OriginEventID == traces[0].DecisionID {
		t.Fatal("origin must not self-correlate")
	}
	if traces[0].OriginEventID != "tl-04" {
		t.Fatalf("want preceding same-agent id tl-04, got %q", traces[0].OriginEventID)
	}
}

// TestEvidenceNoPriorOriginSkipped: a leading decision line has no
// preceding same-agent event - it is counted, never emitted partial.
func TestEvidenceNoPriorOriginSkipped(t *testing.T) {
	evs := cliEvents(t)
	head := evs[0]
	head.ID = "zx-01"
	head.Type = schema.TypePolicyDecision
	head.AgentID = "brand-new-agent"
	// fresh agent with only this line: no prior origin
	traces, skips := buildTraces(append([]schema.Event{head}, evs...), "builtin-v1")
	if skips["no_prior_origin"] != 1 {
		t.Fatalf("want no_prior_origin=1, got %v", skips)
	}
	for _, tr := range traces {
		if tr.DecisionID == "zx-01" {
			t.Fatal("skipped line leaked a partial trace")
		}
	}
}

func writeSampleInto(t *testing.T) (string, string) {
	t.Helper()
	dir := t.TempDir()
	src := filepath.Join(dir, "audit.jsonl")
	data, err := os.ReadFile(filepath.Join("..", "..", "testdata", "timeline-sample.jsonl"))
	if err != nil {
		t.Fatalf("read sample: %v", err)
	}
	if err := os.WriteFile(src, data, 0o600); err != nil {
		t.Fatalf("stage sample: %v", err)
	}
	return dir, src
}

func shaOf(t *testing.T, path string) string {
	t.Helper()
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("sha read: %v", err)
	}
	sum := sha256.Sum256(b)
	return hex.EncodeToString(sum[:])
}

// TestEvidenceWritesOnlyDirAndNeverSource: full run through the CLI
// path. Source sha256+mtime pinned before/after; only the four
// bundle names appear under -dir; a second run refuses (no
// overwrite); verify is green on read-back.
func TestEvidenceWritesOnlyDirAndNeverSource(t *testing.T) {
	root, src := writeSampleInto(t)
	out := filepath.Join(root, "bundle")
	if err := os.Mkdir(out, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	before := shaOf(t, src)
	mtBefore := srcMtime(t, src)
	c := config{out: src, dir: out, policyVersion: "builtin-v1"}
	if err := runEvidence(c); err != nil {
		t.Fatalf("runEvidence: %v", err)
	}
	if after := shaOf(t, src); after != before {
		t.Fatal("audit source bytes changed - export wrote back to source")
	}
	if !srcMtime(t, src).Equal(mtBefore) {
		t.Fatal("audit source mtime changed - export touched the source")
	}
	ents, err := os.ReadDir(out)
	if err != nil {
		t.Fatalf("read dir: %v", err)
	}
	var names []string
	for _, e := range ents {
		names = append(names, e.Name())
	}
	sort.Strings(names)
	want := []string{"decisions.csv", "decisions.json", "decisions.jsonl", "manifest.json"}
	if strings.Join(names, ",") != strings.Join(want, ",") {
		t.Fatalf("bundle names drift: %v", names)
	}
	// re-verify from disk contents
	man, err := os.ReadFile(filepath.Join(out, "manifest.json"))
	if err != nil {
		t.Fatalf("manifest read: %v", err)
	}
	var files []schema.EvidenceFile
	for _, n := range names {
		b, err := os.ReadFile(filepath.Join(out, n))
		if err != nil {
			t.Fatalf("read %s: %v", n, err)
		}
		files = append(files, schema.EvidenceFile{Name: n, Content: b})
	}
	if probs := schema.VerifyEvidenceBundle(man, files); len(probs) != 0 {
		t.Fatalf("written bundle must verify green: %v", probs)
	}
	// second run: refuse to overwrite, change nothing
	if err := runEvidence(c); err == nil || !strings.Contains(err.Error(), "refusing to overwrite") {
		t.Fatalf("want overwrite refusal, got %v", err)
	}
}

func srcMtime(t *testing.T, path string) time.Time {
	t.Helper()
	i, err := os.Stat(path)
	if err != nil {
		t.Fatalf("stat: %v", err)
	}
	return i.ModTime().UTC().Truncate(time.Second)
}

// TestEvidenceRefusesWithoutDirOrVersion: zero-write fail-closed.
func TestEvidenceRefusesWithoutDirOrVersion(t *testing.T) {
	root, src := writeSampleInto(t)
	ents, err := os.ReadDir(root)
	if err != nil {
		t.Fatalf("read root: %v", err)
	}
	if len(ents) != 1 {
		t.Fatalf("fixture root must hold only the audit file, got %d entries", len(ents))
	}
	if err := runEvidence(config{out: src, policyVersion: "builtin-v1"}); err == nil || !strings.Contains(err.Error(), "-dir is required") {
		t.Fatalf("want -dir refusal, got %v", err)
	}
	if err := runEvidence(config{out: src, dir: filepath.Join(root, "nope-not-exist")}); err == nil || !strings.Contains(err.Error(), "-policy-version must be stated") {
		t.Fatalf("want -policy-version refusal, got %v", err)
	}
	// still exactly one entry: refusal performed zero writes anywhere
	ents, err = os.ReadDir(root)
	if err != nil {
		t.Fatalf("re-read root: %v", err)
	}
	if len(ents) != 1 {
		t.Fatalf("refusal path wrote files: %d entries", len(ents))
	}
}

// TestEvidenceTamperNegative: one flipped byte in a written bundle
// member turns the CLI read-back verification red (negative control,
// the gate is not forever-green).
func TestEvidenceTamperNegative(t *testing.T) {
	root, src := writeSampleInto(t)
	out := filepath.Join(root, "bundle")
	if err := os.Mkdir(out, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := runEvidence(config{out: src, dir: out, policyVersion: "builtin-v1"}); err != nil {
		t.Fatalf("runEvidence: %v", err)
	}
	p := filepath.Join(out, "decisions.csv")
	b, err := os.ReadFile(p)
	if err != nil {
		t.Fatalf("read csv: %v", err)
	}
	if len(b) == 0 {
		t.Fatal("empty csv export")
	}
	b[0] ^= 0x01
	if err := os.WriteFile(p, b, 0o600); err != nil {
		t.Fatalf("tamper: %v", err)
	}
	man, err := os.ReadFile(filepath.Join(out, "manifest.json"))
	if err != nil {
		t.Fatalf("manifest: %v", err)
	}
	var files []schema.EvidenceFile
	for _, n := range []string{"decisions.csv", "decisions.json", "decisions.jsonl", "manifest.json"} {
		c2, err := os.ReadFile(filepath.Join(out, n))
		if err != nil {
			t.Fatalf("read %s: %v", n, err)
		}
		files = append(files, schema.EvidenceFile{Name: n, Content: c2})
	}
	if probs := schema.VerifyEvidenceBundle(man, files); len(probs) == 0 {
		t.Fatal("tamper slipped through - verification is forever-green")
	}
}

// TestEvidenceDeterministic: same input twice byte-identical (CLI
// level: two dirs, compare all four members).
func TestEvidenceDeterministic(t *testing.T) {
	root, src := writeSampleInto(t)
	d1 := filepath.Join(root, "b1")
	d2 := filepath.Join(root, "b2")
	for _, d := range []string{d1, d2} {
		if err := os.Mkdir(d, 0o755); err != nil {
			t.Fatalf("mkdir: %v", err)
		}
		if err := runEvidence(config{out: src, dir: d, policyVersion: "builtin-v1"}); err != nil {
			t.Fatalf("runEvidence: %v", err)
		}
	}
	for _, n := range []string{"decisions.csv", "decisions.json", "decisions.jsonl", "manifest.json"} {
		a, err := os.ReadFile(filepath.Join(d1, n))
		if err != nil {
			t.Fatalf("read a: %v", err)
		}
		b, err := os.ReadFile(filepath.Join(d2, n))
		if err != nil {
			t.Fatalf("read b: %v", err)
		}
		if string(a) != string(b) {
			t.Fatalf("%s not byte-identical across runs", n)
		}
	}
}
