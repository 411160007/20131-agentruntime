package schema

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
)

// Impact / blast-radius estimation record (owner specification impact
// analysis and blast-radius control sections, slice W11.1 record
// surface). The impact record landed by the stability wave is the
// passive seven-field shape an operator or a later phase fills in.
// This slice lands the *estimation* surface the specification asks
// for ahead of that: for one action, derive what can be derived from
// the argument text alone, record it, and state - in the record
// itself - exactly how much of the eight-scope vocabulary was NOT
// derivable.
//
// Two of the eight scope dimensions are computable from a tool-call's
// arguments as a closed syntactic subset:
//
//   - file_scope: argument strings shaped as paths (POSIX absolute,
//     "./" or "../" relative, "~/"-home-relative, and Windows
//     drive-prefixed forms). Bare names with no path separator are
//     deliberately not claimed: they are indistinguishable from
//     ordinary prose, and guessing would turn an estimate into a
//     fabrication.
//   - network_scope: argument strings shaped as URLs whose scheme is
//     one of the six shipped transport schemes. Userinfo is stripped
//     before anything is recorded - a credential-bearing authority
//     would leak the credential scope, one of the six dimensions this
//     record always declares unestimated. Bare host:port text without
//     a scheme is likewise not claimed.
//
// The remaining six dimensions (project, process, database,
// credential, device, sub-agent reach) are pinned into the record as
// an always-present declared-gap list. The record therefore can never
// read as a complete blast-radius picture: absence of a dimension is
// structurally impossible, "not derivable from argument text" is
// stated instead. This is the honesty discipline of the estimation
// contract: the record is an estimate over argument text, never an
// observed fact about the world, and its claim boundary states that
// no estimation result renders as "blocked" or "contained".
//
// Like the promotion record, the restatements are constructor-pinned:
// the estimate basis, the claim boundary, the record-only stance, and
// the enforcement-plane token - pinned to the single shipped spelling
// every earlier wave record already uses (no second dialect). A
// hand-built record must carry the identical pinned values or
// Validate rejects it.
//
// What the record never carries: an effect, a severity, a decision
// input, a runtime pointer, or a policy body. Nothing in the
// evaluator, the rule engine, the bus, or the audit writer consults
// it; while impact_estimation_enforcement_plane reads the shipped
// observation token, the record is not an EventType and enters no
// decision-plane file. docs/schema-v2.md section 35 is the
// human-facing contract.

// ImpactEstimationBasis is the pinned estimation-versus-fact field:
// every scope list in the record was derived from argument text only.
const ImpactEstimationBasis = "estimated-from-argument-text-only-not-observed-fact"

// ImpactEstimationClaimBoundary is the pinned outward-phrasing
// separation: an estimation record speaks about predicted reach,
// never about prevention. Renderings of this record must not claim
// the estimated scopes were blocked or contained.
const ImpactEstimationClaimBoundary = "never-renders-as-blocked-or-contained"

// ImpactEstimationRecordStance is the pinned record-only restatement.
const ImpactEstimationRecordStance = "impact_estimation_record: record-only, zero enforcement plane; an estimation is an estimate over argument text, never an observed fact and never a decision input (pre-pinned discipline, not wired in this phase)"

// ImpactEstimationEnforcementPlane is pinned to the single shipped
// observation-phase token. It deliberately mirrors the value of the
// earlier waves' plane constants; a machine test asserts one shared
// spelling so no second dialect can enter.
const ImpactEstimationEnforcementPlane = "none-in-observation-phase"

// The closed syntactic grammars of the two derivable dimensions.
// Schemes are matched case-insensitively and recorded lower-cased.
var impactEstimationSchemes = []string{"http", "https", "ws", "wss", "socks4", "socks5"}

// ImpactEstimationRecord is one per-action estimation as recorded.
// Field order is the wire order and is part of the contract.
type ImpactEstimationRecord struct {
	Kind              string       `json:"kind"`
	ActionID          string       `json:"action_id"`
	ArgsDigest        string       `json:"args_digest"`
	FileScope         []string     `json:"file_scope"`
	NetworkScope      []string     `json:"network_scope"`
	UnestimatedScopes []BlastScope `json:"unestimated_scopes"`
	EstimateBasis     string       `json:"estimate_basis"`
	ClaimBoundary     string       `json:"claim_boundary"`
	Stance            string       `json:"stance"`
	EnforcementPlane  string       `json:"impact_estimation_enforcement_plane"`
}

// ImpactEstimationInput is what a caller supplies: the action the
// estimation is about and the raw JSON object bytes of its tool-call
// arguments (the same bytes the proxy carries alongside the tool
// name). The digest, the two derived scope lists, the declared-gap
// list, and every pinned restatement are produced by the builder,
// never supplied.
type ImpactEstimationInput struct {
	ActionID string
	ArgsRaw  []byte
}

// classifyArgumentString returns the file-path or network-authority
// reading of one argument string under the closed grammars, or ("",
// false) when the string is not claimable. Order of the checks is
// normative: a scheme-shaped string is judged only as network text,
// so "https://x/y" never double-lists as a path, and "C:/dir" (no
// "//") is not scheme-shaped and reads as a Windows drive path.
func classifyArgumentString(s string) (string, bool) {
	if s == "" {
		return "", false
	}
	if lower := strings.ToLower(s); strings.Contains(lower, "://") {
		i := strings.Index(lower, "://")
		scheme := lower[:i]
		rest := s[i+3:]
		for _, sc := range impactEstimationSchemes {
			if scheme == sc {
				host := extractAuthority(rest)
				if validAuthorityLiteral(host) && !strings.Contains(host, "@") {
					return strings.ToLower(host), true
				}
				return "", false
			}
		}
		return "", false // scheme present but outside the closed six
	}
	if strings.HasPrefix(s, "/") && len(s) > 1 {
		return s, true
	}
	if strings.HasPrefix(s, "./") || strings.HasPrefix(s, "../") || strings.HasPrefix(s, "~/") {
		return s, true
	}
	if len(s) >= 4 && s[1] == ':' && (s[2] == '/' || s[2] == '\\') &&
		((s[0] >= 'A' && s[0] <= 'Z') || (s[0] >= 'a' && s[0] <= 'z')) {
		return s, true
	}
	return "", false
}

// extractAuthority derives the literal host from the remainder after
// "://". Step order is normative: cut the first path/query/fragment
// boundary, then drop userinfo (before the port check, so
// "user:pass@host" never mis-cuts at the userinfo colon), then drop a
// trailing ":digits" port outside bracketed IPv6 literals.
func extractAuthority(rest string) string {
	for i := 0; i < len(rest); i++ {
		c := rest[i]
		if c == '/' || c == '?' || c == '#' {
			rest = rest[:i]
			break
		}
	}
	if strings.HasPrefix(rest, "[") {
		if end := strings.Index(rest, "]"); end >= 0 {
			return rest[:end+1]
		}
		return rest
	}
	if at := strings.LastIndex(rest, "@"); at >= 0 {
		rest = rest[at+1:]
	}
	if colon := strings.LastIndex(rest, ":"); colon >= 0 && allDigits(rest[colon+1:]) {
		return rest[:colon]
	}
	return rest
}

func allDigits(s string) bool {
	if s == "" {
		return false
	}
	for i := 0; i < len(s); i++ {
		if s[i] < '0' || s[i] > '9' {
			return false
		}
	}
	return true
}

// validAuthorityLiteral checks the host is non-empty and carries only
// literal characters: letters, digits, dot, hyphen, colon (IPv6
// inside brackets), and the brackets themselves.
func validAuthorityLiteral(host string) bool {
	if host == "" {
		return false
	}
	for i := 0; i < len(host); i++ {
		c := host[i]
		ok := c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z' || c >= '0' && c <= '9' ||
			c == '.' || c == '-' || c == ':' || c == '[' || c == ']'
		if !ok {
			return false
		}
	}
	return true
}

// walkArgumentValues visits every JSON string leaf of the decoded
// argument tree (object values, array elements, nested alike) and
// classifies it. Keys are not classified: the grammars describe what
// an argument *says*, not how a tool names its parameters.
func walkArgumentValues(v any, files, nets *[]string) {
	switch t := v.(type) {
	case string:
		if s, ok := classifyArgumentString(t); ok {
			lower := strings.ToLower(t)
			if strings.Contains(lower, "://") {
				*nets = append(*nets, s)
			} else {
				*files = append(*files, s)
			}
		}
	case map[string]any:
		for _, child := range t {
			walkArgumentValues(child, files, nets)
		}
	case []any:
		for _, child := range t {
			walkArgumentValues(child, files, nets)
		}
	}
}

func dedupSorted(in []string) []string {
	out := make([]string, 0, len(in))
	seen := map[string]bool{}
	for _, s := range in {
		if !seen[s] {
			seen[s] = true
			out = append(out, s)
		}
	}
	sort.Strings(out)
	return out
}

// ImpactEstimationUnestimatedScopes returns the six declared-gap
// dimensions in closed-vocabulary order: the two derivable dimensions
// minus the eight-scope vocabulary. It is derived from the shipped
// eight-token list so a vocabulary change can never silently shrink
// or reword the gap declaration.
func ImpactEstimationUnestimatedScopes() []BlastScope {
	out := make([]BlastScope, 0, 6)
	for _, s := range AllBlastScopes() {
		if s == string(ScopeFile) || s == string(ScopeNetwork) {
			continue
		}
		out = append(out, BlastScope(s))
	}
	return out
}

// Validate checks the record grammar. A hand-constructed record must
// pass the same gate a build produces: Build pins values, Validate
// does all the judging, including re-checking every scope entry
// against the closed grammars it came from.
func (r *ImpactEstimationRecord) Validate() error {
	if r.Kind != "impact.estimation" {
		return fmt.Errorf("impact estimation record: kind %q invalid", r.Kind)
	}
	if !validID(r.ActionID) {
		return fmt.Errorf("impact estimation record: action_id %q invalid", r.ActionID)
	}
	if !validDigest(r.ArgsDigest) {
		return fmt.Errorf("impact estimation record: args_digest must be 64 lowercase hex characters (the record must bind the exact argument text it estimated)")
	}
	if r.FileScope == nil || r.NetworkScope == nil || r.UnestimatedScopes == nil {
		return fmt.Errorf("impact estimation record: the three scope lists must be present, never null: an empty list states \"nothing claimable in the argument text\", null states nothing and is not admissible")
	}
	for _, p := range r.FileScope {
		if s, ok := classifyArgumentString(p); !ok || s != p || strings.Contains(strings.ToLower(p), "://") {
			return fmt.Errorf("impact estimation record: file_scope entry %q is outside the closed path grammar", p)
		}
	}
	if !sortedUnique(r.FileScope) {
		return fmt.Errorf("impact estimation record: file_scope must be sorted and deduplicated")
	}
	for _, h := range r.NetworkScope {
		if h != strings.ToLower(h) || !validAuthorityLiteral(h) || strings.Contains(h, "@") {
			return fmt.Errorf("impact estimation record: network_scope entry %q is outside the closed literal-host grammar (userinfo must never be recorded)", h)
		}
	}
	if !sortedUnique(r.NetworkScope) {
		return fmt.Errorf("impact estimation record: network_scope must be sorted and deduplicated")
	}
	want := ImpactEstimationUnestimatedScopes()
	if len(r.UnestimatedScopes) != len(want) {
		return fmt.Errorf("impact estimation record: unestimated_scopes must declare exactly the six non-derivable dimensions")
	}
	for i, s := range r.UnestimatedScopes {
		if s != want[i] {
			return fmt.Errorf("impact estimation record: unestimated_scopes must mirror the closed vocabulary order, got %q at position %d", string(s), i)
		}
	}
	if r.EstimateBasis != ImpactEstimationBasis {
		return fmt.Errorf("impact estimation record: estimate_basis must be the pinned estimation-not-fact line, got %q", r.EstimateBasis)
	}
	if r.ClaimBoundary != ImpactEstimationClaimBoundary {
		return fmt.Errorf("impact estimation record: claim_boundary must be the pinned outward-phrasing separation, got %q", r.ClaimBoundary)
	}
	if r.Stance != ImpactEstimationRecordStance {
		return fmt.Errorf("impact estimation record: stance must be the pinned record-only line, got %q", r.Stance)
	}
	if r.EnforcementPlane != ImpactEstimationEnforcementPlane {
		return fmt.Errorf("impact estimation record: impact_estimation_enforcement_plane must be the shipped none-in-observation-phase token, got %q", r.EnforcementPlane)
	}
	return nil
}

func sortedUnique(in []string) bool {
	for i := 1; i < len(in); i++ {
		if in[i-1] >= in[i] {
			return false
		}
	}
	return true
}

// BuildImpactEstimationRecord derives the two computable dimensions
// from the argument JSON and produces one validated record, or no
// record at all: every rejection returns the zero value, never a
// half-filled estimate.
func BuildImpactEstimationRecord(in ImpactEstimationInput) (ImpactEstimationRecord, error) {
	if len(in.ArgsRaw) == 0 {
		return ImpactEstimationRecord{}, fmt.Errorf("impact estimation record: argument bytes are required; an action with no argument surface has nothing to estimate and is recorded by absence, not by this shape")
	}
	var tree any
	dec := json.NewDecoder(strings.NewReader(string(in.ArgsRaw)))
	if err := dec.Decode(&tree); err != nil {
		return ImpactEstimationRecord{}, fmt.Errorf("impact estimation record: argument bytes are not valid JSON: %w", err)
	}
	if _, ok := tree.(map[string]any); !ok {
		return ImpactEstimationRecord{}, fmt.Errorf("impact estimation record: arguments must decode to a JSON object")
	}
	sum := sha256.Sum256(in.ArgsRaw)
	var files, nets []string
	walkArgumentValues(tree, &files, &nets)
	rec := ImpactEstimationRecord{
		Kind:              "impact.estimation",
		ActionID:          in.ActionID,
		ArgsDigest:        hex.EncodeToString(sum[:]),
		FileScope:         dedupSorted(files),
		NetworkScope:      dedupSorted(nets),
		UnestimatedScopes: ImpactEstimationUnestimatedScopes(),
		EstimateBasis:     ImpactEstimationBasis,
		ClaimBoundary:     ImpactEstimationClaimBoundary,
		Stance:            ImpactEstimationRecordStance,
		EnforcementPlane:  ImpactEstimationEnforcementPlane,
	}
	if err := rec.Validate(); err != nil {
		return ImpactEstimationRecord{}, err
	}
	return rec, nil
}
