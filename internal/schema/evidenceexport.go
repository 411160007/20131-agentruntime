package schema

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"strings"
)

// Evidence export v0 (spec vNext sections 252 and 253, slice W7.2):
// the Decision Trace of W7.1 is a record shape; this slice turns
// already-built traces into exported bytes - JSONL, CSV, and JSON
// - wrapped in a manifest that pins a per-file integrity hash, and
// a verifier that reads the envelope back and goes red on any
// tamper. The bundle is a read-side derivation: inputs are COPIED,
// never mutated, and no exporter writes back to the audit stream
// the evidence came from (the zero-write-back rule is asserted by
// source hash and mtime in the paired test, not by intent alone).
//
// Scope honesty (section 252 asks for "at least" five formats and
// a seven-member bundle tree):
//
//   - jsonl, csv, and json are delivered here;
//   - pdf needs a rendering stack this stdlib-only binary refuses
//     to carry, and signed bundles need signing infrastructure -
//     both are named known_gap rows, never silently skipped, and
//     the signed-bundle deferral is the taskbook's own honest note,
//     not an oversight;
//   - of the recommended bundle members only manifest.json and
//     decisions.jsonl have real sources at Phase 0; events.jsonl,
//     agents.json, policies.json, and recovery.json are honest
//     known_gap rows - fabricating placeholder files to fill the
//     tree would be exactly the "delete key context for
//     convenience" failure section 253 forbids, in reverse.
//
// Determinism is the machine-gate: the same input exported twice
// leaves byte-identical output. That is why the manifest carries
// NO wall-clock timestamp - the envelope timestamp is derived from
// the evidence itself (the latest trace timestamp) and is absent
// (omitted, never zero) when no trace carries one.
//
// docs/schema-v2.md section 24 is the human-facing contract; the
// Node checker mirrors its vocabularies, rule lines, and the two
// coverage tables against this file; the decision plane keeps zero
// references to the symbols below while
// evidence_bundle_enforcement_plane reads none-in-observation-phase
// (the gate LEAKRE needle set scans for exactly that).

// EvidenceFormat is one member of the closed five-token format
// vocabulary of section 252 ("CSV, JSON, JSONL, PDF, Signed
// Evidence Bundle"), listed here in the coverage order.
type EvidenceFormat string

// The five format tokens in registration order.
const (
	FormatJSONL  EvidenceFormat = "jsonl"
	FormatCSV    EvidenceFormat = "csv"
	FormatJSON   EvidenceFormat = "json"
	FormatPDF    EvidenceFormat = "pdf"
	FormatSigned EvidenceFormat = "signed_bundle"
)

// EvidenceFormatStance classifies how one format or bundle member
// stands against this slice. Three stances, exhaustive and mutually
// exclusive: delivered by export v0, delivered only as an inline
// manifest field (the hashes/ directory form), or structurally
// absent and named.
type EvidenceFormatStance string

const (
	StanceDeliveredV0 EvidenceFormatStance = "delivered_by_export_v0"
	StanceInlineV0    EvidenceFormatStance = "delivered_via_manifest_inline"
	StanceGapAbsent   EvidenceFormatStance = "known_gap_absent"
)

// AllEvidenceFormats lists the five format tokens in registration
// order. The count and order are machine-asserted on every run.
func AllEvidenceFormats() []string {
	return []string{string(FormatJSONL), string(FormatCSV), string(FormatJSON), string(FormatPDF), string(FormatSigned)}
}

// AllEvidenceFormatStances lists the three stances in order.
func AllEvidenceFormatStances() []string {
	return []string{string(StanceDeliveredV0), string(StanceInlineV0), string(StanceGapAbsent)}
}

func (s EvidenceFormatStance) valid() bool {
	switch s {
	case StanceDeliveredV0, StanceInlineV0, StanceGapAbsent:
		return true
	}
	return false
}

// EvidenceBundleMember is one member of the recommended bundle tree
// of section 252, listed in the spec's own display order.
type EvidenceBundleMember string

const (
	MemberManifest  EvidenceBundleMember = "manifest.json"
	MemberEvents    EvidenceBundleMember = "events.jsonl"
	MemberDecisions EvidenceBundleMember = "decisions.jsonl"
	MemberAgents    EvidenceBundleMember = "agents.json"
	MemberPolicies  EvidenceBundleMember = "policies.json"
	MemberRecovery  EvidenceBundleMember = "recovery.json"
	MemberHashes    EvidenceBundleMember = "hashes"
)

// AllEvidenceMembers lists the seven bundle members in spec order.
func AllEvidenceMembers() []string {
	return []string{string(MemberManifest), string(MemberEvents), string(MemberDecisions), string(MemberAgents), string(MemberPolicies), string(MemberRecovery), string(MemberHashes)}
}

// EvidenceIntegrityField is one of the seven integrity attributes
// section 253 requires key evidence to carry, in the spec's own
// listing order.
type EvidenceIntegrityField string

const (
	IntegrityTimestamp   EvidenceIntegrityField = "timestamp"
	IntegritySource      EvidenceIntegrityField = "source"
	IntegrityHash        EvidenceIntegrityField = "integrity_hash"
	IntegrityPolicyVer   EvidenceIntegrityField = "policy_version"
	IntegrityDecisionID  EvidenceIntegrityField = "decision_id"
	IntegrityCorrelation EvidenceIntegrityField = "event_correlation_id"
	IntegrityActor       EvidenceIntegrityField = "actor_identity"
)

// AllEvidenceIntegrityFields lists the seven section 253 fields in
// spec order.
func AllEvidenceIntegrityFields() []string {
	return []string{string(IntegrityTimestamp), string(IntegritySource), string(IntegrityHash), string(IntegrityPolicyVer), string(IntegrityDecisionID), string(IntegrityCorrelation), string(IntegrityActor)}
}

// Rule lines, pinned verbatim against docs section 24.
const (
	EvidenceTimestampRule          = "envelope-timestamp-derived-from-evidence-never-wall-clock"
	EvidenceAbsentCellRule         = "csv-absent-cell-is-literal-absent-token-never-empty"
	EvidenceWriteBackRule          = "export-never-writes-back-to-source"
	EvidenceContextRule            = "no-section251-field-dropped-from-export-for-convenience"
	EvidenceHashRule               = "sha256-of-file-bytes-mismatch-is-red-never-repaired"
	EvidenceBundleEnforcementPlane = "none-in-observation-phase"
)

// EvidenceAbsentCellToken is the single literal shape an uncarried
// CSV cell takes. Never "", never "0", never "-".
const EvidenceAbsentCellToken = "absent"

// EvidenceFormatCoverageRow and EvidenceMemberCoverageRow are the
// two reverse-lookup tables: format and bundle member against what
// export v0 actually delivers. Notes name the source for delivered
// rows and the deferral reason for gaps; neither list is hand-
// typed downstream - docs rows and the Node census derive from
// these registrations.
type EvidenceFormatCoverageRow struct {
	Format EvidenceFormat
	Stance EvidenceFormatStance
	Note   string
}

type EvidenceMemberCoverageRow struct {
	Member EvidenceBundleMember
	Stance EvidenceFormatStance
	Note   string
}

var evidenceFormatCoverage = []EvidenceFormatCoverageRow{
	{FormatJSONL, StanceDeliveredV0, "ExportTracesJSONL"},
	{FormatCSV, StanceDeliveredV0, "ExportTracesCSV"},
	{FormatJSON, StanceDeliveredV0, "ExportTracesJSON"},
	{FormatPDF, StanceGapAbsent, "rendering stack deferred; stdlib-only binary"},
	{FormatSigned, StanceGapAbsent, "signing infrastructure deferred by the taskbook phasing note"},
}

var evidenceMemberCoverage = []EvidenceMemberCoverageRow{
	{MemberManifest, StanceDeliveredV0, "BuildEvidenceManifest"},
	{MemberEvents, StanceGapAbsent, "no event-stream source wired into export v0"},
	{MemberDecisions, StanceDeliveredV0, "decisions.jsonl body"},
	{MemberAgents, StanceGapAbsent, "agent registry export not in this slice"},
	{MemberPolicies, StanceGapAbsent, "policy snapshot export not in this slice"},
	{MemberRecovery, StanceGapAbsent, "recovery-state export waits for its upstream field"},
	{MemberHashes, StanceInlineV0, "per-file sha256 carried inline in manifest files[]"},
}

// AllEvidenceFormatCoverage / AllEvidenceMemberCoverage return
// copies of the registrations.
func AllEvidenceFormatCoverage() []EvidenceFormatCoverageRow {
	out := make([]EvidenceFormatCoverageRow, len(evidenceFormatCoverage))
	copy(out, evidenceFormatCoverage)
	return out
}

func AllEvidenceMemberCoverage() []EvidenceMemberCoverageRow {
	out := make([]EvidenceMemberCoverageRow, len(evidenceMemberCoverage))
	copy(out, evidenceMemberCoverage)
	return out
}

// FormatStanceOf / MemberStanceOf return the registered stance of
// one name, plus ok=false for anything outside the closed tables.
func FormatStanceOf(name string) (EvidenceFormatStance, bool) {
	for _, r := range evidenceFormatCoverage {
		if string(r.Format) == name {
			return r.Stance, true
		}
	}
	return "", false
}

func MemberStanceOf(name string) (EvidenceFormatStance, bool) {
	for _, r := range evidenceMemberCoverage {
		if string(r.Member) == name {
			return r.Stance, true
		}
	}
	return "", false
}

// TraceCellValue maps one section 251 field to its exported value
// for one trace: a copied string, or the single absent token. It
// reads only fields the W7.1 record already carries; it never
// re-derives, re-judges, or invents.
func TraceCellValue(t DecisionTrace, field string) string {
	if field == "origin_event_id" {
		if t.OriginEventID == "" {
			return EvidenceAbsentCellToken
		}
		return t.OriginEventID
	}
	var v string
	switch field {
	case "decision_id":
		v = t.DecisionID
	case "timestamp":
		v = t.Timestamp
	case "agent":
		v = t.Agent
	case "action":
		v = t.Action
	case "resource":
		v = t.Resource
	case "capability":
		v = t.Capability
	case "risk_factors":
		v = t.RiskFactors
	case "policy_version":
		v = t.PolicyVersion
	case "policy_rule":
		v = t.PolicyRule
	case "decision":
		v = t.Decision
	case "enforcement_mode":
		v = t.EnforcementMode
	case "task", "intent", "outcome", "recovery_state":
		v = "" // known_gap rows: absent stands as absent
	default:
		return "" // not a section 251 field (caller error)
	}
	if v == "" {
		return EvidenceAbsentCellToken
	}
	return v
}

// exportColumns is the closed CSV header: the fifteen section 251
// fields in spec order plus the correlation column. Dropping a
// column for convenience is what EvidenceContextRule forbids.
func exportColumns() []string {
	return append(AllTraceFields(), "origin_event_id")
}

func csvCell(s string) string {
	if strings.ContainsAny(s, ",\"\n\r") {
		return "\"" + strings.ReplaceAll(s, "\"", "\"\"") + "\""
	}
	return s
}

// ExportTracesJSONL renders one trace per line in input order.
func ExportTracesJSONL(traces []DecisionTrace) ([]byte, error) {
	var b strings.Builder
	for i := range traces {
		line, err := json.Marshal(&traces[i])
		if err != nil {
			return nil, fmt.Errorf("evidence export jsonl: %w", err)
		}
		b.Write(line)
		b.WriteByte('\n')
	}
	return []byte(b.String()), nil
}

// ExportTracesJSON renders the closed array form.
func ExportTracesJSON(traces []DecisionTrace) ([]byte, error) {
	if traces == nil {
		traces = []DecisionTrace{}
	}
	b, err := json.Marshal(traces)
	if err != nil {
		return nil, fmt.Errorf("evidence export json: %w", err)
	}
	return append(b, '\n'), nil
}

// ExportTracesCSV renders header plus rows over the sixteen
// closed columns; uncarried cells take the single absent token.
func ExportTracesCSV(traces []DecisionTrace) ([]byte, error) {
	cols := exportColumns()
	var b strings.Builder
	hdr := make([]string, len(cols))
	for i, c := range cols {
		hdr[i] = csvCell(c)
	}
	b.WriteString(strings.Join(hdr, ","))
	b.WriteByte('\n')
	for i := range traces {
		row := make([]string, len(cols))
		for j, c := range cols {
			v := TraceCellValue(traces[i], c)
			if v == "" {
				return nil, fmt.Errorf("evidence export csv: column %q is not a section 251 field", c)
			}
			row[j] = csvCell(v)
		}
		b.WriteString(strings.Join(row, ","))
		b.WriteByte('\n')
	}
	return []byte(b.String()), nil
}

// EvidenceFile is one named byte blob handed to the manifest.
type EvidenceFile struct {
	Name    string
	Content []byte
}

type evidenceManifestEntry struct {
	Name   string `json:"name"`
	SHA256 string `json:"sha256"`
	Bytes  int    `json:"bytes"`
	Lines  int    `json:"lines"`
}

type evidenceManifest struct {
	ManifestVersion string                  `json:"manifest_version"`
	Source          string                  `json:"source"`
	SourceTimestamp string                  `json:"source_timestamp,omitempty"`
	TimestampRule   string                  `json:"timestamp_rule"`
	HashRule        string                  `json:"hash_rule"`
	WriteBackRule   string                  `json:"writeback_rule"`
	Files           []evidenceManifestEntry `json:"files"`
}

func sha256Hex(b []byte) string {
	h := sha256.Sum256(b)
	return hex.EncodeToString(h[:])
}

// BuildEvidenceManifest pins name/sha256/bytes/lines for every
// file (sorted by name for byte-determinism) and derives the
// envelope timestamp from the traces - never from a clock. An
// empty source is rejected outright: a guessed source would be a
// fabricated integrity field.
func BuildEvidenceManifest(source string, traces []DecisionTrace, files []EvidenceFile) ([]byte, error) {
	if source == "" {
		return nil, fmt.Errorf("evidence manifest: source must be non-empty (never guessed)")
	}
	names := map[string]bool{}
	entries := make([]evidenceManifestEntry, 0, len(files))
	for _, f := range files {
		if f.Name == "" || f.Name == "manifest.json" {
			return nil, fmt.Errorf("evidence manifest: illegal file name %q", f.Name)
		}
		if names[f.Name] {
			return nil, fmt.Errorf("evidence manifest: duplicate file name %q", f.Name)
		}
		names[f.Name] = true
		lines := 0
		if f.Name != "manifest.json" {
			lines = strings.Count(string(f.Content), "\n")
		}
		entries = append(entries, evidenceManifestEntry{
			Name: f.Name, SHA256: sha256Hex(f.Content), Bytes: len(f.Content), Lines: lines,
		})
	}
	for i := 0; i < len(entries); i++ {
		for j := i + 1; j < len(entries); j++ {
			if entries[j].Name < entries[i].Name {
				entries[i], entries[j] = entries[j], entries[i]
			}
		}
	}
	ts := ""
	for i := range traces {
		if traces[i].Timestamp > ts {
			ts = traces[i].Timestamp
		}
	}
	m := evidenceManifest{
		ManifestVersion: "evidence-manifest-v0",
		Source:          source,
		SourceTimestamp: ts,
		TimestampRule:   EvidenceTimestampRule,
		HashRule:        EvidenceHashRule,
		WriteBackRule:   EvidenceWriteBackRule,
		Files:           entries,
	}
	b, err := json.MarshalIndent(m, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("evidence manifest: %w", err)
	}
	return append(b, '\n'), nil
}

// EvidenceBundle is the assembled read-side bundle: manifest plus
// the three delivered format bodies for one trace set.
type EvidenceBundle struct {
	Manifest       []byte
	DecisionsJSONL []byte
	DecisionsCSV   []byte
	DecisionsJSON  []byte
}

// Files lists the bundle contents as named blobs, manifest first.
func (eb EvidenceBundle) Files() []EvidenceFile {
	return []EvidenceFile{
		{Name: "manifest.json", Content: eb.Manifest},
		{Name: "decisions.jsonl", Content: eb.DecisionsJSONL},
		{Name: "decisions.csv", Content: eb.DecisionsCSV},
		{Name: "decisions.json", Content: eb.DecisionsJSON},
	}
}

// BuildEvidenceBundle exports the traces in every delivered format
// and seals them with a manifest. The traces slice is only read.
func BuildEvidenceBundle(traces []DecisionTrace, source string) (EvidenceBundle, error) {
	jsonl, err := ExportTracesJSONL(traces)
	if err != nil {
		return EvidenceBundle{}, err
	}
	csv, err := ExportTracesCSV(traces)
	if err != nil {
		return EvidenceBundle{}, err
	}
	js, err := ExportTracesJSON(traces)
	if err != nil {
		return EvidenceBundle{}, err
	}
	files := []EvidenceFile{
		{Name: "decisions.jsonl", Content: jsonl},
		{Name: "decisions.csv", Content: csv},
		{Name: "decisions.json", Content: js},
	}
	man, err := BuildEvidenceManifest(source, traces, files)
	if err != nil {
		return EvidenceBundle{}, err
	}
	return EvidenceBundle{Manifest: man, DecisionsJSONL: jsonl, DecisionsCSV: csv, DecisionsJSON: js}, nil
}

// VerifyEvidenceBundle reads a manifest back and checks every
// pinned hash and size against the supplied files. Manifest and
// files must correspond one-to-one: a missing file, an extra file
// the manifest never named, or a single flipped byte is a red
// verdict. Nothing here repairs or re-encodes anything - mismatch
// is reported, never fixed.
func VerifyEvidenceBundle(manifest []byte, files []EvidenceFile) []string {
	var m evidenceManifest
	if err := json.Unmarshal(manifest, &m); err != nil {
		return []string{"manifest unparsable: " + err.Error()}
	}
	if m.ManifestVersion != "evidence-manifest-v0" {
		return []string{"manifest version drifted: " + m.ManifestVersion}
	}
	if m.TimestampRule != EvidenceTimestampRule || m.HashRule != EvidenceHashRule || m.WriteBackRule != EvidenceWriteBackRule {
		return []string{"manifest rule line drifted"}
	}
	byName := map[string][]byte{}
	var problems []string
	for _, f := range files {
		if f.Name == "manifest.json" {
			continue
		}
		if _, dup := byName[f.Name]; dup {
			problems = append(problems, "duplicate file supplied: "+f.Name)
			continue
		}
		byName[f.Name] = f.Content
	}
	pinned := map[string]bool{}
	for _, e := range m.Files {
		pinned[e.Name] = true
		got, ok := byName[e.Name]
		if !ok {
			problems = append(problems, "file pinned in manifest but absent: "+e.Name)
			continue
		}
		if len(got) != e.Bytes {
			problems = append(problems, "byte count mismatch for "+e.Name)
			continue
		}
		if sha256Hex(got) != e.SHA256 {
			problems = append(problems, "integrity hash mismatch for "+e.Name)
		}
	}
	for name := range byName {
		if !pinned[name] {
			problems = append(problems, "file supplied but not pinned in manifest: "+name)
		}
	}
	return problems
}
