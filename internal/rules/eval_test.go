// Golden-set evaluations (Evals v0): the frozen 40-case benchmark —
// 20 normal lines sourced from real dogfood harvest (redacted) and 20
// synthetic danger cases, each labelled with expectation and event
// class. The runner prints machine-readable metric lines and fails the
// suite when any pinned threshold breaks. These thresholds are the
// hard pre-condition for the release slice: a red eval means the
// judgement layer does not ship.
package rules

import (
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"testing"
	"time"

	"20131.com/agentruntime/internal/schema"
)

// pinned thresholds
const (
	evalMinDetection   = 0.80 // detected / dangerous
	evalMaxFPRate      = 0.05 // flagged / normal, per event class
	evalMaxCredFPRate  = 0.02 // credential class is held to a tighter bar
	evalMaxNormAlerts  = 1    // zero-disturbance: alerts across the 20 normals
	evalGoldenSize     = 40   // 20 normal + 20 danger (file-count control)
	evalExpectedEvents = 40   // line-count control: the corpus must never be silently empty
)

type label struct {
	Expect string `json:"expect"`
	Class  string `json:"class"`
}

func goldenPath(t *testing.T, name string) string {
	t.Helper()
	// tests run with cwd = package dir; repo root is two levels up
	_, thisFile, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("cannot locate test source path")
	}
	p := filepath.Join(filepath.Dir(filepath.Dir(filepath.Dir(thisFile))), "testdata", "golden", name)
	if _, err := os.Stat(p); err != nil {
		t.Fatalf("golden file %s: %v", p, err)
	}
	return p
}

func loadGolden(t *testing.T, name string) []*schema.Event {
	t.Helper()
	raw, err := os.ReadFile(goldenPath(t, name))
	if err != nil {
		t.Fatal(err)
	}
	var out []*schema.Event
	for _, line := range strings.Split(strings.TrimRight(string(raw), "\n"), "\n") {
		if line == "" {
			continue
		}
		var e schema.Event
		if err := json.Unmarshal([]byte(line), &e); err != nil {
			t.Fatalf("golden %s line unmarshal: %v", name, err)
		}
		if err := e.Validate(); err != nil {
			t.Fatalf("golden %s event %s invalid: %v", name, e.ID, err)
		}
		out = append(out, &e)
	}
	return out
}

func TestGoldenEvals(t *testing.T) {
	// size controls: the corpus itself must be intact
	normals := loadGolden(t, "normal.jsonl")
	dangers := loadGolden(t, "danger.jsonl")
	if len(normals) != 20 || len(dangers) != 20 {
		t.Fatalf("golden corpus broken: %d normal / %d danger (want 20/20)", len(normals), len(dangers))
	}
	labRaw, err := os.ReadFile(goldenPath(t, "labels.json"))
	if err != nil {
		t.Fatal(err)
	}
	var labels map[string]label
	if err := json.Unmarshal(labRaw, &labels); err != nil {
		t.Fatal(err)
	}
	if len(labels) != evalGoldenSize {
		t.Fatalf("labels carry %d entries, want %d", len(labels), evalGoldenSize)
	}
	// every golden event must be labelled exactly once
	seen := map[string]bool{}
	for _, e := range append(append([]*schema.Event{}, normals...), dangers...) {
		if _, ok := labels[e.ID]; !ok {
			t.Fatalf("golden event %s has no label", e.ID)
		}
		if seen[e.ID] {
			t.Fatalf("duplicate golden event id %s", e.ID)
		}
		seen[e.ID] = true
	}

	eng := MustDefault()

	type classStat struct{ total, flagged int }
	normByClass := map[string]*classStat{}
	detTotal, detHit := 0, 0
	detByClass := map[string]classStat{}
	normalAlerts := 0
	var missed, falsePos []string

	run := func(events []*schema.Event) {
		for _, e := range events {
			d, err := eng.Decide(e)
			if err != nil {
				t.Fatalf("decide %s: %v", e.ID, err)
			}
			l := labels[e.ID]
			flagged := d.Value == schema.DecisionWouldBlock
			if l.Expect == "would_block" {
				detTotal++
				detByClass[l.Class] = classStat{detByClass[l.Class].total + 1, detByClass[l.Class].flagged + bool2i(flagged)}
				if flagged {
					detHit++
				} else {
					missed = append(missed, e.ID)
				}
			} else {
				st := normByClass[l.Class]
				if st == nil {
					st = &classStat{}
					normByClass[l.Class] = st
				}
				st.total++
				if flagged {
					st.flagged++
					normalAlerts++
					falsePos = append(falsePos, e.ID)
				}
			}
		}
	}
	run(dangers)
	run(normals)

	detection := float64(detHit) / float64(detTotal)
	// per-class false-positive rates
	var worstFP float64
	var worstFPClass string
	classes := make([]string, 0, len(normByClass))
	for c := range normByClass {
		classes = append(classes, c)
	}
	sort.Strings(classes)
	credRate := 0.0
	for _, c := range classes {
		st := normByClass[c]
		rate := float64(st.flagged) / float64(st.total)
		t.Logf("EVAL: class=%s normal_n=%d fp=%d fp_rate=%.4f", c, st.total, st.flagged, rate)
		if c == "credential" {
			credRate = rate
		}
		if rate > worstFP {
			worstFP, worstFPClass = rate, c
		}
	}
	for c, st := range detByClass {
		t.Logf("EVAL: danger_class=%s n=%d detected=%d", c, st.total, st.flagged)
	}
	t.Logf("EVAL: missed=%v false_positives=%v", missed, falsePos)

	// machine-readable summary line (the gate parses exactly this)
	if detection < evalMinDetection {
		t.Errorf("detection %.3f < %.2f", detection, evalMinDetection)
	}
	if worstFP > evalMaxFPRate {
		t.Errorf("worst per-class false-positive %.3f (%s) > %.2f", worstFP, worstFPClass, evalMaxFPRate)
	}
	if credRate > evalMaxCredFPRate {
		t.Errorf("credential false-positive %.3f > %.2f", credRate, evalMaxCredFPRate)
	}
	if normalAlerts > evalMaxNormAlerts {
		t.Errorf("zero-disturbance breach: %d alerts on normals (budget %d)", normalAlerts, evalMaxNormAlerts)
	}
	t.Logf("EVAL SUMMARY: detection=%d/%d (%.1f%%) worst_fp=%.4f(%s) credential_fp=%.4f normal_alerts=%d/%d",
		detHit, detTotal, detection*100, worstFP, worstFPClass, credRate, normalAlerts, len(normals))
}

func bool2i(b bool) int {
	if b {
		return 1
	}
	return 0
}

// TestGoldenFreshness pins that the golden events fall inside a sane
// time window and normals carry observation-stage types only, so the
// corpus cannot silently drift into unlabelled shapes.
func TestGoldenFreshness(t *testing.T) {
	for _, e := range loadGolden(t, "normal.jsonl") {
		if e.Type != schema.TypeAgentScan && e.Type != schema.TypeAgentDetected && e.Type != schema.TypeCollectorStart {
			t.Errorf("normal %s has type %s; normals must be harvest-derived observation lines", e.ID, e.Type)
		}
		if e.TS.Before(time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)) {
			t.Errorf("normal %s ts %s out of window", e.ID, e.TS)
		}
	}
}
