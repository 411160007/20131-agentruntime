package schema

import (
	"errors"
	"strings"
)

// Export re-judgement and packaging annotation vocabulary (spec vNext
// section 238, consumed by the section 237 EXPORT class): wrapping
// data - compressing, encoding, encrypting, archiving, copying,
// uploading, sending, sharing - never lowers the sensitivity of what
// leaves, so an EXPORT is always judged on its own and a decision
// that allowed a READ or WRITE of a target never carries over to an
// EXPORT of that same target.
//
// This file is the annotation half of that rule: it names the closed
// packaging word family, pins the rules as contract constants, and
// provides one pure record constructor that produces an
// ExportRejudgementRecord row. It is record-only. No file in the
// decision planes (policy, rules, bus, auditlog) may reference the
// symbols below while export_rejudgement_enforcement_plane reads
// none-in-observation-phase; scripts/schema-v2-check.mjs scans for
// that leak every run, and the Go sync test pins the docs contract.
//
// The record deliberately carries no decision field of any shape:
// the Phase 0 closed pair {allow, would_block} lives elsewhere and
// this slice borrows none of it. A golden facet test shows the two
// rows the rule is about - same target, a read row and an export
// row - and proves they are independent records, not one decision
// reused under a second label.

// PackagingWord names one of the eight section 238 packaging words.
// Note that "share" is deliberately a member of both this family and
// the nine data actions above: as a data action it classifies the
// movement itself; as a packaging word it annotates a wrapping
// operation that an independent EXPORT re-judgement must see. The
// two vocabularies stay separate tables with separate meanings.
type PackagingWord string

// The eight packaging words in normative order (spec vNext section
// 238). Declaration order is the contract order; docs/schema-v2.md
// section 17 mirrors it verbatim and the Node checker predicate pins
// both directions.
const (
	PackagingCompress PackagingWord = "compress"
	PackagingEncode   PackagingWord = "encode"
	PackagingEncrypt  PackagingWord = "encrypt"
	PackagingArchive  PackagingWord = "archive"
	PackagingCopy     PackagingWord = "copy"
	PackagingUpload   PackagingWord = "upload"
	PackagingSend     PackagingWord = "send"
	PackagingShare    PackagingWord = "share"
)

// Rule constants pinned verbatim against the five-key machine block
// in docs/schema-v2.md section 17.
const (
	// ExportPackagingSensitivityRule pins section 238 verbatim in
	// spirit: no wrapping lowers what is wrapped.
	ExportPackagingSensitivityRule = "packaging-never-lowers-sensitivity"
	// ExportRejudgementIndependenceRule pins the carry-over ban: an
	// allow recorded for a READ (or any other class) of a target is
	// never an allow for an EXPORT of that target.
	ExportRejudgementIndependenceRule = "read-allow-never-carries-to-export"
	// ExportRejudgementAbsentDefault pins the honest absence shape:
	// a line with no packaging words records that no wrapping was
	// observed - it never infers wrapping into the hole, and never
	// infers a re-judgement exemption either.
	ExportRejudgementAbsentDefault = "absent-means-no-packaging-observed-never-inferred"
	// ExportRejudgementEnforcementPlane pins the Phase 0 stance:
	// annotation only, enforcement borrowed by no plane.
	ExportRejudgementEnforcementPlane = "none-in-observation-phase"
)

// AllPackagingWords lists the eight vocabulary members in normative
// order. The docs section 17 block is pinned against this function
// by the Node checker predicate; drift in either direction is a red
// build.
func AllPackagingWords() []string {
	return []string{
		string(PackagingCompress), string(PackagingEncode), string(PackagingEncrypt),
		string(PackagingArchive), string(PackagingCopy), string(PackagingUpload),
		string(PackagingSend), string(PackagingShare),
	}
}

// Valid reports whether p is a member of the packaging vocabulary.
// The empty string is not a word: absence is recorded as an empty
// annotation, never as a wildcard member.
func (p PackagingWord) Valid() bool {
	if p == "" {
		return false
	}
	for _, w := range AllPackagingWords() {
		if string(p) == w {
			return true
		}
	}
	return false
}

// ExportRejudgementRecord is the annotation row produced for one
// data-facing observation of a target. It has exactly six fields,
// none of them a decision: the checker and the Go test both pin the
// field set so no later edit can quietly bolt an outcome onto the
// re-judgement seed.
type ExportRejudgementRecord struct {
	Target                 string
	Action                 DataAction
	Packaging              []PackagingWord
	IndependentRejudgement bool
	SensitivityPreserved   bool
	EnforcementPlane       string
}

// BuildExportRejudgement constructs the record row for a classified
// action on a target with the observed packaging words (possibly
// none). Every rejection fires before any value is applied: an
// empty target, an unclassified or wild action, or any word outside
// the closed eight leaves no partial record behind. Duplicate words
// collapse keeping first-occurrence order. IndependentRejudgement
// is true exactly for the export class; SensitivityPreserved is a
// contract constant surfaced per-row so a consumer reads the rule
// with the data, never from folklore.
func BuildExportRejudgement(target string, action DataAction, packaging []PackagingWord) (ExportRejudgementRecord, error) {
	if strings.TrimSpace(target) == "" {
		return ExportRejudgementRecord{}, errors.New("export rejudgement: empty target")
	}
	if !action.Valid() || action == "" {
		return ExportRejudgementRecord{}, errors.New("export rejudgement: action must be a classified member of the nine")
	}
	seen := make(map[PackagingWord]bool, len(packaging))
	words := make([]PackagingWord, 0, len(packaging))
	for _, p := range packaging {
		if !p.Valid() {
			return ExportRejudgementRecord{}, errors.New("export rejudgement: packaging word outside the closed eight")
		}
		if seen[p] {
			continue
		}
		seen[p] = true
		words = append(words, p)
	}
	return ExportRejudgementRecord{
		Target:                 target,
		Action:                 action,
		Packaging:              words,
		IndependentRejudgement: action == DataActionExport,
		SensitivityPreserved:   true,
		EnforcementPlane:       ExportRejudgementEnforcementPlane,
	}, nil
}
