package schema

import (
	"encoding/json"
	"strings"
	"testing"
	"time"
)

func mustTierTS(s string) time.Time {
	v, err := time.Parse(time.RFC3339Nano, s)
	if err != nil {
		panic(err)
	}
	return v
}

// validEvent is a minimal good record; tests mutate one aspect at a time.
func tierCapEvent() *Event {
	return &Event{
		V:        SchemaVersion,
		TS:       mustTierTS("2026-09-27T12:00:00Z"),
		ID:       "ev-tier-1",
		AgentID:  "agi-0123456789abcdef0123456789abcdef01234567",
		Stage:    StageObservation,
		Type:     TypeAgentScan,
		Decision: DecisionAllow,
		Severity: SevInfo,
		Summary:  "tier/cap probe",
	}
}

// TestTierValidationMatrix is the table-driven machine judgement for the
// additive tier field: absent stays valid (legacy L1 default), every
// vocabulary member validates, wild values are rejected on both sides of
// the enum boundary (case, empty-ish, out-of-range levels).
func TestTierValidationMatrix(t *testing.T) {
	cases := []struct {
		name string
		tier Tier
		ok   bool
	}{
		{"absent-legacy", "", true},
		{"L0", TierL0, true},
		{"L1", TierL1, true},
		{"L2", TierL2, true},
		{"L3", TierL3, true},
		{"wild-l4", "L4", false},
		{"wild-l9", Tier("L9"), false},
		{"lowercase", "l1", false},
		{"bare-digit", "1", false},
		{"empty-ish-space", Tier(" "), false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			e := tierCapEvent()
			e.Tier = tc.tier
			err := e.Validate()
			if tc.ok && err != nil {
				t.Fatalf("tier %q rejected: %v", string(tc.tier), err)
			}
			if !tc.ok && err == nil {
				t.Fatalf("wild tier %q accepted", string(tc.tier))
			}
		})
	}
	// legacy default resolution
	if got := Tier("").EffectiveTier(); got != TierL1 {
		t.Errorf("absent tier must default to L1, got %q", got)
	}
}

// TestLegacyLineWithoutTierStillValid pins the additive non-breaking
// property at schema level: unmarshalling a pre-tier JSONL line yields a
// valid event routed as L1.
func TestLegacyLineWithoutTierStillValid(t *testing.T) {
	legacy := `{"v":1,"ts":"2026-09-27T15:52:32.103006806Z","id":"ev-aaa111","agent_id":"agent-hello-collector","stage":"observation","type":"collector.start","decision":"allow","severity":0,"summary":"hello-collector started (local JSONL audit only, no network)","attrs":{"version":"x"}}`
	var e Event
	if err := json.Unmarshal([]byte(legacy), &e); err != nil {
		t.Fatal(err)
	}
	if err := e.Validate(); err != nil {
		t.Fatalf("legacy line must validate: %v", err)
	}
	if e.Tier != "" || e.Tier.EffectiveTier() != TierL1 {
		t.Fatalf("absent tier must mean L1, got %q", e.Tier)
	}
}

func TestCapabilityVocabulary(t *testing.T) {
	table := CapabilityTable()
	if len(table) == 0 || len(table) > MaxCapabilities {
		t.Fatalf("capability table size %d outside (0, %d]", len(table), MaxCapabilities)
	}
	for _, d := range table {
		if !d.NonTransitive {
			t.Errorf("capability %q must be annotated non-transitive (subprocess != inherited grant)", d.Name)
		}
		if !d.DefaultClass.Valid() || d.DefaultClass == "" {
			t.Errorf("capability %q has invalid default class %q", d.Name, d.DefaultClass)
		}
	}
	// wild value rejected
	if Cap("file.DELETE").Valid() || Cap("rm -rf").Valid() || Cap("").Valid() {
		t.Error("wild capability must be invalid")
	}
	// class cap: exactly three levels
	if len(AllResourceClasses()) != 3 {
		t.Fatalf("resource sensitivity must be three levels, got %v", AllResourceClasses())
	}
	// parse list: cap enforced + wild rejected + duplicates rejected
	_, err := ParseCapabilityList(strings.Join(append(AllCapabilities(), "extra.bogus"), ","))
	if err == nil {
		t.Fatal("ParseCapabilityList accepted a wild value")
	}
	if got, err := ParseCapabilityList("file.read, shell.exec"); err != nil || len(got) != 2 {
		t.Fatalf("valid list rejected: %v", err)
	}
	if _, err := ParseCapabilityList("file.read,file.read"); err == nil {
		t.Fatal("duplicate capability accepted")
	}
	if _, err := ParseCapabilityList(strings.Repeat("env.read,", MaxCapabilities) + "env.read"); err == nil {
		t.Fatalf("capability list beyond cap %d accepted", MaxCapabilities)
	}
}

func TestEventAttrsCapAndResClassValidation(t *testing.T) {
	good := tierCapEvent()
	good.Attrs = map[string]string{"cap": "shell.exec", "res_class": "high"}
	if err := good.Validate(); err != nil {
		t.Fatalf("vocabulary attrs rejected: %v", err)
	}
	badCap := tierCapEvent()
	badCap.Attrs = map[string]string{"cap": "rm -rf /"}
	if err := badCap.Validate(); err == nil || !strings.Contains(err.Error(), "cap") {
		t.Fatalf("wild cap attr accepted or wrong error: %v", err)
	}
	badClass := tierCapEvent()
	badClass.Attrs = map[string]string{"res_class": "critical"}
	if err := badClass.Validate(); err == nil {
		t.Fatal("wild res_class accepted")
	}
	// empty res_class = unclassified and valid
	unclass := tierCapEvent()
	unclass.Attrs = map[string]string{"res_class": ""}
	if err := unclass.Validate(); err != nil {
		t.Fatalf("unclassified res_class rejected: %v", err)
	}
}

func TestPhase0DecisionVocabularyInvariant(t *testing.T) {
	// Contract vocabulary is a superset...
	if len(AllDecisions()) != 3 {
		t.Fatalf("decision contract vocabulary changed: %v", AllDecisions())
	}
	// ...but the runtime set is exactly {allow, would_block}.
	rs := Phase0RuntimeDecisions()
	if len(rs) != 2 {
		t.Fatalf("phase-0 runtime set must have 2 values, got %v", rs)
	}
	for _, d := range rs {
		if err := MustPhase0Decision(d); err != nil {
			t.Errorf("runtime decision %q rejected: %v", d, err)
		}
	}
	if err := MustPhase0Decision(DecisionAsk); err == nil {
		t.Error("ask must be outside the phase-0 runtime vocabulary")
	}
	if err := MustPhase0Decision(Decision("block")); err == nil {
		t.Error("block must be outside the phase-0 runtime vocabulary")
	}
}
