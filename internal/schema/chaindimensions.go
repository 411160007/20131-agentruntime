package schema

// Behavior Chain dimension vocabulary and the record shape of the
// first three computable dimensions (spec vNext section 242, slice
// W5.2): a behavior chain is more than the sum of its single-step
// risks - the spec names eight dimensions the chain analysis must
// compute, and shows that a sequence of individually plausible steps
// (read a credential, archive it, encode it, send it out) can be
// high risk overall while every point looks low risk alone.
//
// This slice lands the honest first move: the closed eight-dimension
// vocabulary pinned against the spec bullet lines, an 8/8 coverage
// registration table in which every non-computable dimension names
// either a real later-slice anchor (W5.3, W6, W11) or an explicit
// pending gap - never a silent hole - and pure constructors that
// compute the first three computable dimensions from recorded chain
// steps:
//
//   - Intent Deviation: recorded steps carry the intent text they
//     claim to serve; more than one distinct intent in the chain is
//     recorded as an observation with the off-root steps as basis.
//   - Trust Domain Crossing: adjacent steps whose recorded trust
//     levels differ are recorded with the crossing steps as basis.
//     The level vocabulary is section 16's, consumed unchanged.
//   - Reversibility Reduction: steps carry the optional recovery
//     class of slice W2.3; a chain whose recorded classes get worse
//     over time (lower on the four-class truthfulness scale) is
//     recorded with the worsening steps as basis. No new reversibility
//     vocabulary exists in this file - the recovery classes are the
//     only source, exactly as the taskbook's consumption note
//     requires.
//
// It is record-only. The constructors never judge danger, never
// score, never decide: they restate what the recorded steps carry.
// Absence is single-meaning: a dimension whose backing data was
// never recorded reads unclassified - never not_observed, never a
// default. No file in the decision planes (policy, rules, bus,
// auditlog) and no command-tree file may reference the symbols
// below while chain_dimension_enforcement_plane reads
// none-in-observation-phase; scripts/schema-v2-check.mjs scans for
// that leak every run, the Go tests in this package pin the docs
// contract, and the Node checker mirrors the coverage table row by
// row. docs/schema-v2.md section 19 is the human-facing contract.

import (
	"errors"
	"fmt"
	"strings"
)

// ChainDimension names one of the eight behavior-chain dimensions of
// spec vNext section 242. Declaration order is the spec bullet order
// and is normative: the section 19 anchor bullets, the coverage
// table, and the Node checker are all pinned row by row against it.
type ChainDimension string

// The eight section 242 dimensions in spec order.
const (
	DimensionIntentDeviation        ChainDimension = "intent_deviation"
	DimensionCapabilityEscalation   ChainDimension = "capability_escalation"
	DimensionDataSensitivityEscal   ChainDimension = "data_sensitivity_escalation"
	DimensionTrustDomainCrossing    ChainDimension = "trust_domain_crossing"
	DimensionReversibilityReduction ChainDimension = "reversibility_reduction"
	DimensionBlastRadiusGrowth      ChainDimension = "blast_radius_growth"
	DimensionDestinationChange      ChainDimension = "destination_change"
	DimensionDelegationChain        ChainDimension = "delegation_chain"
)

// chainDimensionWireNames lists the eight dimension tokens in
// normative (spec) order. AllChainDimensions mirrors it; the docs
// anchor bullets and the Node checker derive the same list from the
// section 19 bullet lines and fail on any drift in either direction.
var chainDimensionWireNames = []string{
	"intent_deviation", "capability_escalation", "data_sensitivity_escalation",
	"trust_domain_crossing", "reversibility_reduction", "blast_radius_growth",
	"destination_change", "delegation_chain",
}

// AllChainDimensions returns the eight dimension tokens in spec
// order. The count and the order are machine-asserted on every run;
// nobody recounts them by hand.
func AllChainDimensions() []string {
	out := make([]string, len(chainDimensionWireNames))
	copy(out, chainDimensionWireNames)
	return out
}

// Valid reports whether d is a member of the closed eight-dimension
// vocabulary. The empty string is not a dimension: absence is
// recorded as a missing row or an unclassified state, never as a
// wildcard member.
func (d ChainDimension) Valid() bool {
	if d == "" {
		return false
	}
	for _, w := range chainDimensionWireNames {
		if string(d) == w {
			return true
		}
	}
	return false
}

// ChainDimensionPhase names how one dimension stands in the 8/8
// coverage registration: computed by this slice from recorded step
// data, deferred to a named later slice, or honestly pending with no
// owner yet. Every coverage row carries exactly one phase; a row
// with no phase is a dangling hole, and holes are red builds here.
type ChainDimensionPhase string

// The three coverage phases.
const (
	ChainPhaseComputable ChainDimensionPhase = "computable_v0"
	ChainPhaseAnchored   ChainDimensionPhase = "anchored"
	ChainPhasePending    ChainDimensionPhase = "pending"
)

// Valid reports whether p is one of the three coverage phases.
func (p ChainDimensionPhase) Valid() bool {
	switch p {
	case ChainPhaseComputable, ChainPhaseAnchored, ChainPhasePending:
		return true
	}
	return false
}

// ChainDimensionCoverageRow registers one dimension's standing:
// its phase and, for anchored rows, the later slice that will
// compute it (the taskbook's anchor note; enforcement stays out of
// scope for every row while the plane reads none).
type ChainDimensionCoverageRow struct {
	Dimension ChainDimension
	Phase     ChainDimensionPhase
	Anchor    string
}

// chainDimensionCoverage is the normative 8/8 registration, in spec
// order. The three computable rows are this slice's dimensions; the
// five deferred rows name their real anchors (W5.3 delegation
// observation, W6 least-agency counting, W11 blast-radius
// estimation) or stand as explicit pending gaps (destination change
// has no owner slice yet and says so). docs/schema-v2.md section 19
// mirrors these eight rows verbatim, row order included, and the
// Node checker pins them pair by pair.
func chainDimensionCoverage() []ChainDimensionCoverageRow {
	return []ChainDimensionCoverageRow{
		{DimensionIntentDeviation, ChainPhaseComputable, ""},
		{DimensionCapabilityEscalation, ChainPhaseAnchored, "W6"},
		{DimensionDataSensitivityEscal, ChainPhaseAnchored, "W6"},
		{DimensionTrustDomainCrossing, ChainPhaseComputable, ""},
		{DimensionReversibilityReduction, ChainPhaseComputable, ""},
		{DimensionBlastRadiusGrowth, ChainPhaseAnchored, "W11"},
		{DimensionDestinationChange, ChainPhasePending, ""},
		{DimensionDelegationChain, ChainPhaseAnchored, "W5.3"},
	}
}

// AllChainDimensionCoverage returns a copy of the eight registration
// rows in normative order.
func AllChainDimensionCoverage() []ChainDimensionCoverageRow {
	return chainDimensionCoverage()
}

// ComputableChainDimensions lists the three dimensions this slice
// computes, in spec order, as the wire-token subset the docs
// chain_dimension_computable_v0 line mirrors.
func ComputableChainDimensions() []string {
	return []string{
		string(DimensionIntentDeviation),
		string(DimensionTrustDomainCrossing),
		string(DimensionReversibilityReduction),
	}
}

// ChainObservationState is the closed tri-state one computed
// dimension of one chain can hold. observed and not_observed are
// findings over recorded data; unclassified is the honest shape when
// the dimension's backing data was never recorded: a chain whose
// steps carry no trust levels has said nothing about trust domain
// crossings, and no reader may turn that silence into either a
// finding or a clean bill of health.
type ChainObservationState string

// The three observation states in normative order.
const (
	ChainStateObserved     ChainObservationState = "observed"
	ChainStateNotObserved  ChainObservationState = "not_observed"
	ChainStateUnclassified ChainObservationState = "unclassified"
)

// AllChainObservationStates lists the three states in normative
// order.
func AllChainObservationStates() []string {
	return []string{
		string(ChainStateObserved), string(ChainStateNotObserved),
		string(ChainStateUnclassified),
	}
}

// Valid reports whether s is one of the three states. The empty
// string is not a state.
func (s ChainObservationState) Valid() bool {
	if s == "" {
		return false
	}
	for _, w := range AllChainObservationStates() {
		if string(s) == w {
			return true
		}
	}
	return false
}

// Rule constants pinned verbatim against the seven-key machine block
// in docs/schema-v2.md section 19.
const (
	// ChainDimensionCoverageRule pins the zero-dangling obligation:
	// all eight section 242 dimensions carry a registration row, and
	// an unregistered dimension is a build failure, not a footnote.
	ChainDimensionCoverageRule = "eight-of-eight-rows-registered-never-dangling"
	// ChainDimensionUnclassifiedRule pins the honest silence shape:
	// missing backing data yields unclassified, never not_observed,
	// because "nothing recorded" is not evidence that nothing
	// happened.
	ChainDimensionUnclassifiedRule = "missing-step-data-yields-unclassified-never-not-observed"
	// ChainDimensionReversibilitySourceRule pins the W2.3
	// consumption contract: reversibility reduction reads only the
	// four recovery classes, so chain slices and schema slices share
	// one scale instead of inventing rival ones.
	ChainDimensionReversibilitySourceRule = "reversibility-reduction-reads-recovery-classes-only"
	// ChainDimensionEnforcementPlane pins the Phase 0 stance: the
	// three computable dimensions are record projections; no plane
	// borrows them to decide anything yet.
	ChainDimensionEnforcementPlane = "none-in-observation-phase"
)

// ChainStep is one recorded step of one behavior chain. Seq is the
// step's 1-based position and must be gap-free: chain analysis is
// order-sensitive, so the constructor refuses holes, duplicates, and
// disorder before applying any value. Action is one of the section
// 15 nine. Intent, Trust, and Recovery are the optional recorded
// observations the three computable dimensions consume; each empty
// field means that dimension's data was not recorded for this step,
// never a default value. The step carries no decision, score, or
// severity field and the reflective test pins that.
type ChainStep struct {
	Seq      int           `json:"seq"`
	Action   DataAction    `json:"action"`
	Intent   string        `json:"intent,omitempty"`
	Trust    TrustLevel    `json:"trust_level,omitempty"`
	Recovery RecoveryClass `json:"recovery,omitempty"`
}

// Validate checks one step against the closed vocabularies it
// borrows: a classified member of the nine, a trust level of the
// five (or absent), and a recovery class of the four (or absent).
func (s ChainStep) Validate() error {
	if s.Seq <= 0 {
		return fmt.Errorf("chain step seq must be positive, got %d", s.Seq)
	}
	if s.Action == "" || !s.Action.Valid() {
		return errors.New("chain step action must be a classified member of the nine")
	}
	if !s.Trust.Valid() {
		return fmt.Errorf("chain step trust level %q is outside the closed five", string(s.Trust))
	}
	if s.Recovery != "" && !s.Recovery.Valid() {
		return fmt.Errorf("chain step recovery class %q is outside the closed four", string(s.Recovery))
	}
	return nil
}

// ChainDimensionObservation is one computed dimension's finding over
// one chain: the state plus the step sequences the finding is based
// on (empty basis means the state came from the recorded data as a
// whole, or that there was no data to classify).
type ChainDimensionObservation struct {
	Dimension ChainDimension        `json:"dimension"`
	State     ChainObservationState `json:"state"`
	Basis     []int                 `json:"basis,omitempty"`
}

// ChainDimensionsRecord is the row BuildChainDimensions produces:
// the step count, the three computed observations in spec order, and
// the enforcement-plane restatement. It has no decision field of any
// shape; the Phase 0 closed pair {allow, would_block} lives
// elsewhere and this slice borrows none of it.
type ChainDimensionsRecord struct {
	Steps            int                         `json:"steps"`
	Observations     []ChainDimensionObservation `json:"observations"`
	EnforcementPlane string                      `json:"enforcement_plane"`
}

// BuildChainDimensions validates the recorded steps and computes the
// three computable dimensions as pure projections over them. Every
// rejection fires before any value is applied: an empty chain, a
// wild or duplicate or out-of-order seq, or any step with an action,
// trust level, or recovery class outside its closed set leaves no
// record behind. The computations never invent: a dimension whose
// backing data is absent stays unclassified, and observed findings
// list the offending step sequences so the row restates the chain
// instead of judging it.
func BuildChainDimensions(steps []ChainStep) (ChainDimensionsRecord, error) {
	if len(steps) == 0 {
		return ChainDimensionsRecord{}, errors.New("chain dimensions: empty chain")
	}
	for i, s := range steps {
		if err := s.Validate(); err != nil {
			return ChainDimensionsRecord{}, fmt.Errorf("chain dimensions: step %d: %w", i+1, err)
		}
		if s.Seq != i+1 {
			return ChainDimensionsRecord{}, fmt.Errorf("chain dimensions: steps must be numbered 1..%d in order, got seq %d at position %d", len(steps), s.Seq, i+1)
		}
	}
	return ChainDimensionsRecord{
		Steps: len(steps),
		Observations: []ChainDimensionObservation{
			observeIntentDeviation(steps),
			observeTrustDomainCrossing(steps),
			observeReversibilityReduction(steps),
		},
		EnforcementPlane: ChainDimensionEnforcementPlane,
	}, nil
}

// observeIntentDeviation records how many distinct intents the
// chain's steps claim to serve. A chain that carries intent data and
// shows more than one distinct intent is recorded as observed with
// every step beyond the first intent root listed as basis; a chain
// with intent data and one intent is not_observed; a chain whose
// steps carry no intent at all is unclassified - silent steps have
// recorded nothing, which is not the same as agreeing on one intent.
func observeIntentDeviation(steps []ChainStep) ChainDimensionObservation {
	root := ""
	seen := map[string]bool{}
	basis := []int{}
	recorded := 0
	for _, s := range steps {
		if strings.TrimSpace(s.Intent) == "" {
			continue
		}
		recorded++
		in := strings.TrimSpace(s.Intent)
		if root == "" {
			root = in
		}
		if !seen[in] {
			seen[in] = true
			if in != root {
				basis = append(basis, s.Seq)
			}
		}
	}
	obs := ChainDimensionObservation{Dimension: DimensionIntentDeviation}
	switch {
	case recorded == 0:
		obs.State = ChainStateUnclassified
	case len(basis) > 0:
		obs.State = ChainStateObserved
		obs.Basis = basis
	default:
		obs.State = ChainStateNotObserved
	}
	return obs
}

// observeTrustDomainCrossing records adjacent-step pairs in which
// both steps carry a trust level and the levels differ (either
// direction: the row records the crossing, the ranking story stays
// with section 16's projection discipline). A chain with at least
// one comparable adjacent pair and no difference is not_observed; a
// chain with fewer than two recorded levels in adjacency is
// unclassified, never a clean reading.
func observeTrustDomainCrossing(steps []ChainStep) ChainDimensionObservation {
	basis := []int{}
	comparable := 0
	for i := 1; i < len(steps); i++ {
		a, b := steps[i-1].Trust, steps[i].Trust
		if a == "" || b == "" {
			continue
		}
		comparable++
		if a != b {
			basis = append(basis, steps[i].Seq)
		}
	}
	obs := ChainDimensionObservation{Dimension: DimensionTrustDomainCrossing}
	switch {
	case comparable == 0:
		obs.State = ChainStateUnclassified
	case len(basis) > 0:
		obs.State = ChainStateObserved
		obs.Basis = basis
	default:
		obs.State = ChainStateNotObserved
	}
	return obs
}

// recoveryRank maps a recovery class onto the four-class
// truthfulness scale in its declared (most-to-least recoverable)
// order: a higher rank is worse. Absent classes have no rank - the
// W2.3 vocabulary is the only scale this file reads, per
// ChainDimensionReversibilitySourceRule.
func recoveryRank(c RecoveryClass) (int, bool) {
	for i, w := range AllRecoveryClasses() {
		if string(c) == w {
			return i, true
		}
	}
	return 0, false
}

// observeReversibilityReduction records whether the chain's recorded
// recovery classes get worse over time: a step ranked lower on the
// recoverable scale than the best-so-far is a reduction, with the
// worsening steps as basis. Improvement (later steps more
// recoverable) is not_observed, and so is a flat recorded scale;
// fewer than two recovery-class-bearing steps is unclassified.
func observeReversibilityReduction(steps []ChainStep) ChainDimensionObservation {
	basis := []int{}
	recorded := 0
	best := -1
	for _, s := range steps {
		if s.Recovery == "" {
			continue
		}
		recorded++
		r, ok := recoveryRank(s.Recovery)
		if !ok {
			continue // unreachable: Validate rejects wild classes first
		}
		if best >= 0 && r > best {
			basis = append(basis, s.Seq)
		}
		if best < 0 || r < best {
			best = r
		}
	}
	obs := ChainDimensionObservation{Dimension: DimensionReversibilityReduction}
	switch {
	case recorded < 2:
		obs.State = ChainStateUnclassified
	case len(basis) > 0:
		obs.State = ChainStateObserved
		obs.Basis = basis
	default:
		obs.State = ChainStateNotObserved
	}
	return obs
}
