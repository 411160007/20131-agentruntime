package schema

import (
	"fmt"
	"os"
	"reflect"
	"regexp"
	"strings"
	"testing"
)

// Intent and authority contract sync tests (stability wave, slice
// W2.2). They pin docs/schema-v2.md against the Go declarations in
// intentauthority.go, recompute the ten mechanical anchor back-checks
// and the field count programmatically (never by hand), and drive the
// fixture set: every good record round-trips, every wild shape is
// rejected before any bytes or usable values exist. Pure record and
// documentation evolution: nothing here touches runtime decision
// semantics.

func intentAnchorLines(t *testing.T, doc string) []string {
	t.Helper()
	//nolint
	hits := regexp.MustCompile("(?s)```intent-spec-anchor\n(.*?)```").FindAllStringSubmatch(doc, -1)
	if len(hits) != 1 {
		t.Fatalf("intent anchor blocks found %d, want exactly 1", len(hits))
	}
	var out []string
	for _, l := range strings.Split(hits[0][1], "\n") {
		if l = strings.TrimSpace(l); l != "" {
			out = append(out, l)
		}
	}
	return out
}

func TestIntentContractSourcesAgree(t *testing.T) {
	raw, err := os.ReadFile(filepathJoinDotDot("docs", "schema-v2.md"))
	if err != nil {
		t.Skipf("schema-v2 doc not visible: %v", err)
	}
	doc := string(raw)
	vocab := schemaV2Key(t, doc, "intent_field_vocabulary")
	if joined := strings.Join(AllIntentFields(), ", "); joined != vocab {
		t.Errorf("intent vocabulary drift:\n docs: %q\n go:   %q", vocab, joined)
	}
	count := schemaV2Key(t, doc, "intent_field_count")
	if count != "10" || len(AllIntentFields()) != 10 {
		t.Errorf("intent count: docs %q, go %d, want programmatic 10", count, len(AllIntentFields()))
	}
	if absent := schemaV2Key(t, doc, "intent_absent_semantics"); absent != "known-gap-never-fabricated" {
		t.Errorf("absent-field semantics drifted to %q", absent)
	}
	lines := intentAnchorLines(t, doc)
	if len(lines) != 10 {
		t.Fatalf("anchor line census %d, want 10", len(lines))
	}
	for i, l := range lines {
		snake := strings.ReplaceAll(strings.ToLower(l), " ", "_")
		if snake != AllIntentFields()[i] {
			t.Errorf("anchor back-check line %d (%q): snake form %q, wire token %q", i+1, l, snake, AllIntentFields()[i])
		}
	}
}

func TestAuthorityContractSourcesAgree(t *testing.T) {
	raw, err := os.ReadFile(filepathJoinDotDot("docs", "schema-v2.md"))
	if err != nil {
		t.Skipf("schema-v2 doc not visible: %v", err)
	}
	doc := string(raw)
	vocab := schemaV2Key(t, doc, "authority_origin_vocabulary")
	if joined := strings.Join(AllGrantOrigins(), ", "); joined != vocab {
		t.Errorf("origin vocabulary drift:\n docs: %q\n go:   %q", vocab, joined)
	}
	if rule := schemaV2Key(t, doc, "authority_propagation_rule"); rule != AuthorityPropagationRule {
		t.Errorf("propagation rule docs %q, go %q", rule, AuthorityPropagationRule)
	}
	if plane := schemaV2Key(t, doc, "authority_enforcement_plane"); plane != AuthorityEnforcementPlane {
		t.Errorf("enforcement plane docs %q, go %q", plane, AuthorityEnforcementPlane)
	}
	if plane := schemaV2Key(t, doc, "authority_enforcement_plane"); plane != "none-in-observation-phase" {
		t.Errorf("observation phase must not borrow any enforcement plane: %q", plane)
	}
	if carrier := schemaV2Key(t, doc, "authority_untrusted_carrier"); carrier != "untrusted" {
		t.Errorf("untrusted carrier name drifted: %q", carrier)
	}
}

// TestIntentRecordFieldShapeClosedSet pins the record geometry to the
// declared vocabulary: exactly ten fields in order, every one optional
// (omitempty = the honest absent default), and the authority link
// origin as the single required carrier inside a chain.
func TestIntentRecordFieldShapeClosedSet(t *testing.T) {
	rt := reflect.TypeOf(IntentRecord{})
	if rt.NumField() != 10 {
		t.Fatalf("IntentRecord has %d fields, want 10", rt.NumField())
	}
	fields := AllIntentFields()
	for i := 0; i < rt.NumField(); i++ {
		tag := strings.Split(rt.Field(i).Tag.Get("json"), ",")[0]
		if tag != fields[i] {
			t.Errorf("field %d: tag %q, vocabulary %q", i, tag, fields[i])
		}
		if !strings.Contains(rt.Field(i).Tag.Get("json"), "omitempty") {
			t.Errorf("field %s must be optional with an absent default", tag)
		}
	}
	lt := reflect.TypeOf(AuthorityLink{})
	if lt.NumField() != 2 {
		t.Fatalf("AuthorityLink has %d fields, want 2", lt.NumField())
	}
	if strings.Contains(lt.Field(0).Tag.Get("json"), "omitempty") {
		t.Error("link origin must stay required: a link without a named source is malformed")
	}
	if !strings.Contains(lt.Field(1).Tag.Get("json"), "omitempty") ||
		!strings.Contains(reflect.TypeOf(AuthorityChain{}).Field(1).Tag.Get("json"), "omitempty") {
		t.Error("untrusted marks are optional record bits")
	}
}

// TestIntentRoundTripAndWildReject drives the fixture pair: good
// records parse, encode, and re-parse to identical shapes (validate
// round-trip); wild records are rejected with a nil result and zero
// produced bytes. Counts are asserted programmatically.
func TestIntentRoundTripAndWildReject(t *testing.T) {
	readLines := func(name string) []string {
		raw, err := os.ReadFile(filepathJoinDotDot("testdata", "intentauthority", name))
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
	good := readLines("good.jsonl")
	wild := readLines("wild.jsonl")
	if len(good) != 3 || len(wild) != 7 {
		t.Fatalf("fixture census good=%d wild=%d, want 3/7 (programmatic)", len(good), len(wild))
	}
	produced := 0
	for i, line := range good {
		rec, err := ParseIntentRecord([]byte(line))
		if err != nil {
			t.Fatalf("good[%d] rejected: %v", i, err)
		}
		b1, err := EncodeChecked(rec)
		if err != nil {
			t.Fatalf("good[%d] encode rejected: %v", i, err)
		}
		produced += len(b1)
		rec2, err := ParseIntentRecord(b1)
		if err != nil || !reflect.DeepEqual(rec, rec2) {
			t.Errorf("good[%d] round trip unstable: %v", i, err)
		}
		b2, _ := EncodeChecked(rec2)
		if string(b1) != string(b2) {
			t.Errorf("good[%d] re-encode differs", i)
		}
	}
	if produced == 0 {
		t.Error("no bytes produced for valid records - silent fixture failure")
	}
	// Honest empty: a fully absent record is valid and encodes to {}.
	empty, err := ParseIntentRecord([]byte("{}"))
	if err != nil {
		t.Fatalf("empty record rejected: %v", err)
	}
	if b, err := EncodeChecked(empty); err != nil || string(b) != "{}" {
		t.Errorf("empty record must encode to {}, got %q err %v", b, err)
	}
	for i, line := range wild {
		rec, err := ParseIntentRecord([]byte(line))
		if err == nil || rec != nil {
			t.Errorf("wild[%d] accepted: %v", i, rec)
			continue
		}
		if b, encErr := EncodeChecked(rec); rec == nil && encErr == nil && len(b) > 0 {
			t.Errorf("wild[%d] produced bytes despite rejection", i)
		}
	}
}

// TestAuthorityStickyPropagationKeepsItsShape proves the recorded rule
// in both directions: clearing the chain mark against an untrusted
// link is rejected, while an eager set mark encodes verbatim. Positive
// and negative controls, no slogan.
func TestAuthorityStickyPropagationKeepsItsShape(t *testing.T) {
	bad := &IntentRecord{Goal: "x", Authority: &AuthorityChain{
		Links: []AuthorityLink{{Origin: OriginRuntimeInferred, Untrusted: true}},
	}}
	if err := bad.Authority.Validate(); err == nil {
		t.Error("propagation break accepted: untrusted link, cleared chain mark")
	}
	if b, err := EncodeChecked(bad); err == nil || len(b) != 0 {
		t.Errorf("rejected record must produce zero bytes, got %d", len(b))
	}
	good := &IntentRecord{Goal: "x", Authority: &AuthorityChain{
		Links:     []AuthorityLink{{Origin: OriginRuntimeInferred, Untrusted: true}},
		Untrusted: true,
	}}
	if err := good.Authority.Validate(); err != nil {
		t.Errorf("sticky chain rejected: %v", err)
	}
	for _, tok := range []GrantOrigin{"system", "User_Direct", "agent_proved", ""} {
		if err := (AuthorityLink{Origin: tok}).Validate(); err == nil {
			t.Errorf("wild origin %q accepted", tok)
		}
	}
	if got := fmt.Sprint(len(AllGrantOrigins())); got != "5" {
		t.Errorf("origin census %s, want 5", got)
	}
}
