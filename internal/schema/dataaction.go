package schema

// DataAction vocabulary (spec vNext section 237): the closed set of nine
// data action classes an event may record through its optional
// data_action field. This is a DATA BOUNDARY classification dimension,
// not a capability token: the capability table in capability.go is
// untouched by it (the eleven tokens stay byte-identical, so every
// legacy line referencing net.outbound keeps its exact meaning), and
// events without the field remain perfectly valid records.
//
// Section 237 semantic pins, recorded as contract constants so the
// non-equivalences are machine-visible instead of folklore:
//   - READ is not EXPORT and not SHARE: looking at a resource never
//     moves it anywhere.
//   - WRITE is not EXECUTE: storing bytes is not running code.
//   - EXPORT always requires an independent re-judgement: a decision
//     that allowed a READ or WRITE of a target never carries over to an
//     EXPORT of that same target, and packaging (compressing, encoding,
//     encrypting) does not lower the sensitivity of what leaves.
//
// Phase 0 boundary: data_action is a record-only annotation. No file in
// the decision planes (policy, rules, bus, auditlog) may reference the
// symbols below while DataActionEnforcementPlane reads
// none-in-observation-phase; scripts/schema-v2-check.mjs scans for that
// leak every run, and the Go sync test pins the docs contract. The
// absence of the key means unclassified legacy - it is never inferred
// into a default class.

// DataAction names one of the nine recorded data action classes.
type DataAction string

// The nine vocabulary members in normative order (spec vNext section
// 237). Declaration order is the contract order; the Node validator,
// docs/api-v0.md, and the docs/schema-v2.md section 15 block all mirror
// it exactly.
const (
	DataActionDiscover DataAction = "discover"
	DataActionRead     DataAction = "read"
	DataActionWrite    DataAction = "write"
	DataActionModify   DataAction = "modify"
	DataActionDelete   DataAction = "delete"
	DataActionExecute  DataAction = "execute"
	DataActionExport   DataAction = "export"
	DataActionShare    DataAction = "share"
	DataActionPersist  DataAction = "persist"
)

// Rule constants pinned verbatim against the five-key machine block in
// docs/schema-v2.md section 15.
const (
	// DataActionNonequivalenceRule records that the three headline
	// non-equivalences of section 237 are part of the vocabulary's
	// meaning, not suggestions.
	DataActionNonequivalenceRule = "read-not-export-read-not-share-write-not-execute"
	// DataActionExportRejudgementRule pins the one rule the packaging
	// word families must never erode: EXPORT is always judged anew.
	DataActionExportRejudgementRule = "export-requires-independent-rejudgement"
	// DataActionAbsentDefault pins the honest absence shape: a line
	// without data_action is unclassified legacy, and no reader may
	// fill it in by guessing.
	DataActionAbsentDefault = "absent-means-unclassified-legacy-never-inferred"
	// DataActionEnforcementPlane pins the Phase 0 stance: vocabulary
	// only, enforcement borrowed by no plane.
	DataActionEnforcementPlane = "none-in-observation-phase"
)

// AllDataActions lists the nine vocabulary members in normative order.
// scripts/validate-jsonl.mjs mirrors this list as the independent second
// source of truth and docs/api-v0.md as the third; the sync tests fail
// if any two diverge.
func AllDataActions() []string {
	return []string{
		string(DataActionDiscover), string(DataActionRead), string(DataActionWrite),
		string(DataActionModify), string(DataActionDelete), string(DataActionExecute),
		string(DataActionExport), string(DataActionShare), string(DataActionPersist),
	}
}

// Valid reports whether d is a member of the vocabulary. The empty
// string is valid and means the field is absent: unclassified legacy,
// never a default class. Anything else outside the nine is rejected.
func (d DataAction) Valid() bool {
	if d == "" {
		return true
	}
	for _, w := range AllDataActions() {
		if string(d) == w {
			return true
		}
	}
	return false
}
