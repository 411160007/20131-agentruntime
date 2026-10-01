package schema

// Sync tests for the evidence and profile contracts (slice W2.4 first
// piece): docs, Go declarations, and the Node checker must agree
// verbatim, and the fixture pairs prove the gates have teeth (good
// shapes parse, wild shapes are rejected with zero bytes).

import (
	"encoding/json"
	"fmt"
	"os"
	"reflect"
	"regexp"
	"strings"
	"testing"
)

func readRepoFile(t *testing.T, rel string) string {
	t.Helper()
	parts := strings.Split(rel, "/")
	b, err := os.ReadFile(filepathJoinDotDot(parts...))
	if err != nil {
		t.Fatalf("read %s: %v", rel, err)
	}
	return string(b)
}

func docsKey(t *testing.T, docs, key string) string {
	t.Helper()
	re := regexp.MustCompile(`(?m)^` + regexp.QuoteMeta(key) + `: (.+)$`)
	ms := re.FindAllStringSubmatch(docs, -1)
	if len(ms) != 1 {
		t.Fatalf("key %s must appear exactly once, found %d", key, len(ms))
	}
	return strings.TrimSpace(ms[0][1])
}

func anchorLines(t *testing.T, docs, block string, want int) []string {
	t.Helper()
	re := regexp.MustCompile("(?s)```" + regexp.QuoteMeta(block) + "\n(.*?)```")
	m := re.FindStringSubmatch(docs)
	if m == nil {
		t.Fatalf("anchor block %s not found", block)
	}
	var lines []string
	for _, l := range strings.Split(strings.TrimSpace(m[1]), "\n") {
		if s := strings.TrimSpace(l); s != "" {
			lines = append(lines, s)
		}
	}
	if len(lines) != want {
		t.Fatalf("anchor %s census %d want %d", block, len(lines), want)
	}
	return lines
}

// anchorToken derives a wire token mechanically: lower case, runs of
// spaces and slashes collapsed to single underscores. Passing "/" is
// required only by the evidence seventh line; the other anchors stay
// correct under the same rule.
func anchorToken(line string) string {
	s := strings.ToLower(line)
	s = strings.ReplaceAll(s, "/", " ")
	for strings.Contains(s, "  ") {
		s = strings.ReplaceAll(s, "  ", " ")
	}
	return strings.ReplaceAll(s, " ", "_")
}

func TestEvidenceAnchorMirror(t *testing.T) {
	docs := readRepoFile(t, "docs/schema-v2.md")
	vocab := strings.Split(docsKey(t, docs, "evidence_field_vocabulary"), ", ")
	count := docsKey(t, docs, "evidence_field_count")
	lines := anchorLines(t, docs, "evidence-spec-anchor", 7)
	if fmt.Sprint(len(vocab)) != count {
		t.Errorf("docs count %s drifts from vocabulary length %d", count, len(vocab))
	}
	goList := AllEvidenceFields()
	if strings.Join(goList, ", ") != strings.Join(vocab, ", ") {
		t.Errorf("Go vocabulary drifts from docs: %v vs %v", goList, vocab)
	}
	for i, l := range lines {
		if got := anchorToken(l); got != vocab[i] {
			t.Errorf("anchor back-check broke at line %d: %q -> %q want %q", i+1, l, got, vocab[i])
		}
	}
}

func TestEvidencePlaneStringsMirror(t *testing.T) {
	docs := readRepoFile(t, "docs/schema-v2.md")
	pairs := [][2]string{
		{"evidence_enforcement_plane", EvidenceEnforcementPlane},
		{"evidence_context_retention_rule", EvidenceContextRetentionRule},
		{"evidence_signature_envelope_phase", EvidenceSignatureEnvelope},
		{"evidence_required_semantics", "all-seven-required-for-key-evidence"},
	}
	for _, p := range pairs {
		if got := docsKey(t, docs, p[0]); got != p[1] {
			t.Errorf("docs %s = %q, Go/contract = %q", p[0], got, p[1])
		}
	}
}

func TestEvidenceRequiredByReflection(t *testing.T) {
	typ := reflect.TypeOf(EvidenceRecord{})
	if typ.NumField() != 7 {
		t.Fatalf("evidence record has %d fields, want 7", typ.NumField())
	}
	names := make([]string, 0, typ.NumField())
	for i := 0; i < typ.NumField(); i++ {
		tag := typ.Field(i).Tag.Get("json")
		if strings.Contains(tag, "omitempty") {
			t.Errorf("field %s carries omitempty: key evidence fields are required", typ.Field(i).Name)
		}
		names = append(names, strings.Split(tag, ",")[0])
	}
	if !reflect.DeepEqual(names, AllEvidenceFields()) {
		t.Errorf("struct field order drifts from vocabulary: %v", names)
	}
}

func runEvidenceFixtures(t *testing.T, path string, wantOK bool) {
	t.Helper()
	data := readRepoFile(t, path)
	for i, line := range strings.Split(strings.TrimRight(data, "\n"), "\n") {
		rec, err := ParseEvidenceRecord([]byte(line))
		if wantOK && err != nil {
			t.Errorf("%s line %d rejected: %v", path, i+1, err)
		}
		if !wantOK {
			if err == nil {
				t.Errorf("%s line %d accepted: wild shape must be rejected", path, i+1)
			}
			if rec != nil {
				t.Errorf("%s line %d returned a record on rejection", path, i+1)
			}
		}
	}
}

func TestEvidenceFixturePair(t *testing.T) {
	runEvidenceFixtures(t, "testdata/evidenceprofile/good_evidence.jsonl", true)
	runEvidenceFixtures(t, "testdata/evidenceprofile/wild_evidence.jsonl", false)
}

func TestEvidenceEncodeCheckedZeroBytes(t *testing.T) {
	rec := &EvidenceRecord{Timestamp: "t", Source: "s", IntegrityHash: "h",
		PolicyVersion: "p", DecisionID: "d", EventCorrelationID: "e"}
	b, err := EncodeEvidenceChecked(rec)
	if err == nil || len(b) != 0 {
		t.Errorf("incomplete record must produce error and zero bytes")
	}
	rec.ActorAgentIdentity = "a"
	b, err = EncodeEvidenceChecked(rec)
	if err != nil {
		t.Fatalf("complete record rejected: %v", err)
	}
	var back EvidenceRecord
	if err := json.Unmarshal(b, &back); err != nil || back != *rec {
		t.Errorf("round trip drifted: %v", err)
	}
}

func TestProfileAnchorMirror(t *testing.T) {
	docs := readRepoFile(t, "docs/schema-v2.md")
	cases := []struct {
		key   string
		block string
		goSet []string
		n     int
	}{
		{"profile_personal_field_vocabulary", "profile-personal-spec-anchor", AllProfilePersonalFields(), 12},
		{"profile_agent_field_vocabulary", "profile-agent-spec-anchor", AllProfileAgentFields(), 11},
	}
	for _, c := range cases {
		vocab := strings.Split(docsKey(t, docs, c.key), ", ")
		if len(vocab) != c.n {
			t.Fatalf("%s has %d tokens, want %d", c.key, len(vocab), c.n)
		}
		if strings.Join(vocab, ", ") != strings.Join(c.goSet, ", ") {
			t.Errorf("%s: Go list drifts from docs", c.key)
		}
		lines := anchorLines(t, docs, c.block, c.n)
		for i, l := range lines {
			if got := anchorToken(l); got != vocab[i] {
				t.Errorf("%s back-check broke at line %d: %q -> %q want %q", c.block, i+1, l, got, vocab[i])
			}
		}
	}
	if strings.Split(docsKey(t, docs, "profile_scope_vocabulary"), ", ")[0] != "personal" {
		t.Errorf("scope vocabulary order drifted")
	}
	scopes := []ProfileScope{ProfileScopePersonal, ProfileScopeAgent}
	for _, s := range scopes {
		if !s.Valid() {
			t.Errorf("scope %q must be valid", s)
		}
	}
	if ProfileScope("global").Valid() {
		t.Errorf("wild scope accepted")
	}
	// shared-token discipline: common_tools appears in both scopes
	pt := AllProfilePersonalFields()
	at := AllProfileAgentFields()
	if idxOf(pt, "common_tools") < 0 || idxOf(at, "common_tools") < 0 {
		t.Errorf("shared tools token forked")
	}
}

func idxOf(list []string, want string) int {
	for i, x := range list {
		if x == want {
			return i
		}
	}
	return -1
}

func TestProfileOptionalByReflection(t *testing.T) {
	for _, typ := range []reflect.Type{reflect.TypeOf(PersonalProfileRecord{}), reflect.TypeOf(AgentProfileRecord{})} {
		for i := 0; i < typ.NumField(); i++ {
			tag := typ.Field(i).Tag.Get("json")
			if !strings.Contains(tag, "omitempty") {
				t.Errorf("%s.%s lacks omitempty: profile fields accumulate, absence is honest", typ.Name(), typ.Field(i).Name)
			}
		}
	}
	if n := reflect.TypeOf(PersonalProfileRecord{}).NumField(); n != 12 {
		t.Errorf("personal record has %d fields, want 12", n)
	}
	if n := reflect.TypeOf(AgentProfileRecord{}).NumField(); n != 11 {
		t.Errorf("agent record has %d fields, want 11", n)
	}
}

func TestProfilePlaneStringsMirror(t *testing.T) {
	docs := readRepoFile(t, "docs/schema-v2.md")
	pairs := [][2]string{
		{"profile_enforcement_plane", ProfileEnforcementPlane},
		{"profile_hard_boundary_rule", ProfileHardBoundaryRule},
		{"profile_absent_semantics", ProfileAbsentSemantics},
		{"profile_number_semantics", ProfileNumberSemantics},
	}
	for _, p := range pairs {
		if got := docsKey(t, docs, p[0]); got != p[1] {
			t.Errorf("docs %s = %q, Go/contract = %q", p[0], got, p[1])
		}
	}
}

func runProfileFixtures(t *testing.T, path string, personal bool, wantOK bool) {
	t.Helper()
	data := readRepoFile(t, path)
	for i, line := range strings.Split(strings.TrimRight(data, "\n"), "\n") {
		var err error
		if personal {
			_, err = ParsePersonalProfile([]byte(line))
		} else {
			_, err = ParseAgentProfile([]byte(line))
		}
		if wantOK && err != nil {
			t.Errorf("%s line %d rejected: %v", path, i+1, err)
		}
		if !wantOK && err == nil {
			t.Errorf("%s line %d accepted: wild shape must be rejected", path, i+1)
		}
	}
}

func TestProfileFixturePairs(t *testing.T) {
	runProfileFixtures(t, "testdata/profile/good_personal.jsonl", true, true)
	runProfileFixtures(t, "testdata/profile/good_agent.jsonl", false, true)
	runProfileFixtures(t, "testdata/profile/wild_profile.jsonl", true, false)
}

func TestProfileEmptyRecordIsHonest(t *testing.T) {
	// an empty profile record parses (accumulated-nothing is honest at
	// the record level) and encodes byte-clean.
	b, err := EncodePersonalProfileChecked(&PersonalProfileRecord{})
	if err != nil || string(b) != "{}" {
		t.Errorf("empty personal record must encode to {}, got %q err %v", b, err)
	}
}
