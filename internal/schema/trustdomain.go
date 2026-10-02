package schema

// Trust domain mapping (spec vNext section 239): the five trust levels
// S0-S4, the seven run domains, and the migration mapping between the
// legacy three-tier resource sensitivity classes (res_class low /
// medium / high) and the five-level semantics.
//
// This is a DATA BOUNDARY vocabulary and mapping contract, not an
// enforcement surface. It is deliberately NOT one of the fifteen
// schema names of section 289, so the section 11 master table and its
// twenty-eight cells stay untouched (the census doctrine forbids
// private rows beside the table).
//
// Direction discipline, pinned as contract constants so the rule is
// machine-visible instead of folklore:
//
//   - Projection (five to three) is total and exact: every trust
//     level names exactly one legacy res_class it would collapse to.
//     trustToLegacyClass is that table, and it doubles as the
//     migration annotation lines for the three-tier to five-tier
//     move: an old low/medium/high line is NOT silently re-labelled
//     into an S level; it keeps its meaning, and only explicitly
//     recorded trust levels project downward into the legacy class.
//   - Lift (three to five) is a window, never a point: low lifts to
//     {s0_normal} only, but medium lifts to {s1_private, s2_sensitive}
//     and high lifts to {s3_credential, s4_security_boundary}. There
//     is deliberately no single-valued lift helper in this package:
//     guessing which of the window an old line "really" meant would
//     fabricate sensitivity history, so LiftCandidates returns the
//     whole candidate set or nothing.
//   - Cross-domain data movement must pass through a Data Boundary
//     (section 239 verbatim obligation); recording it is vocabulary
//     for later boundary slices, borrowed by no decision path now.
//
// Phase 0 boundary: record-only. No file in the decision planes
// (policy, rules, bus, auditlog) and no file in the command tree may
// reference the symbols below while trust_domain_enforcement_plane
// reads none-in-observation-phase; scripts/schema-v2-check.mjs scans
// for that leak every run and the Go test in this package pins the
// docs contract. The absence of a trust level or run domain means
// unclassified legacy and is never inferred into a default.

// TrustLevel names one of the five sensitivity trust domains of spec
// vNext section 239.
type TrustLevel string

// The five trust levels in normative order (S0 through S4).
// Declaration order is the contract order; the Node checker predicate
// and the docs/schema-v2.md section 16 block mirror it exactly.
const (
	LevelNormal           TrustLevel = "s0_normal"
	LevelPrivate          TrustLevel = "s1_private"
	LevelSensitive        TrustLevel = "s2_sensitive"
	LevelCredential       TrustLevel = "s3_credential"
	LevelSecurityBoundary TrustLevel = "s4_security_boundary"
)

// AllTrustLevels lists the five levels in normative order.
func AllTrustLevels() []string {
	return []string{
		string(LevelNormal), string(LevelPrivate), string(LevelSensitive),
		string(LevelCredential), string(LevelSecurityBoundary),
	}
}

// Valid reports whether l is a member of the vocabulary. The empty
// string is valid and means unclassified legacy: absent trust levels
// are never inferred into a default (see TrustDomainAbsentDefault).
func (l TrustLevel) Valid() bool {
	switch l {
	case "", LevelNormal, LevelPrivate, LevelSensitive, LevelCredential, LevelSecurityBoundary:
		return true
	}
	return false
}

// RunDomain names one of the seven run domains of spec vNext section
// 239. Data crossing between domains is the shape the later Data
// Boundary slices classify; here the vocabulary only exists to be
// recorded and validated, never to decide.
type RunDomain string

// The seven run domains in spec order.
const (
	DomainUser         RunDomain = "user"
	DomainAgent        RunDomain = "agent"
	DomainTool         RunDomain = "tool"
	DomainSandbox      RunDomain = "sandbox"
	DomainRecovery     RunDomain = "recovery"
	DomainSecurityCore RunDomain = "security_core"
	DomainExternal     RunDomain = "external"
)

// AllRunDomains lists the seven domains in normative order.
func AllRunDomains() []string {
	return []string{
		string(DomainUser), string(DomainAgent), string(DomainTool),
		string(DomainSandbox), string(DomainRecovery),
		string(DomainSecurityCore), string(DomainExternal),
	}
}

// Valid reports whether d is a member of the vocabulary. The empty
// string is valid and means unclassified legacy, never a default.
func (d RunDomain) Valid() bool {
	switch d {
	case "", DomainUser, DomainAgent, DomainTool, DomainSandbox,
		DomainRecovery, DomainSecurityCore, DomainExternal:
		return true
	}
	return false
}

// trustToLegacyClass is the migration mapping table from the legacy
// three-tier res_class vocabulary onto the five-level trust semantics:
// each trust level is annotated with the single legacy class it
// projects down to. The docs section 16 line
// trust_domain_legacy_projection mirrors these five entries verbatim,
// and the Node checker pins them pair by pair against this map.
var trustToLegacyClass = map[TrustLevel]ResClass{
	LevelNormal:           ResLow,
	LevelPrivate:          ResMedium,
	LevelSensitive:        ResMedium,
	LevelCredential:       ResHigh,
	LevelSecurityBoundary: ResHigh,
}

// LegacyClassOf projects a trust level down to the legacy res_class
// it collapses to. It is total over the vocabulary: every recorded
// trust level names exactly one legacy class. Unknown levels, and the
// absent (empty) level, have no projection and return ok=false - an
// unclassified record is never upgraded or downgraded by arithmetic.
func LegacyClassOf(l TrustLevel) (ResClass, bool) {
	c, ok := trustToLegacyClass[l]
	return c, ok
}

// LiftCandidates returns the whole candidate window a legacy res_class
// lifts into: low to one level, medium and high each to two, and the
// absent or unknown class to none. The return is a set on purpose:
// there is no API in this package that selects a single trust level
// out of a legacy class, because picking one would fabricate which
// of private-or-sensitive, credential-or-security-boundary an old
// line "really" meant. Consumers must carry the ambiguity forward or
// record an explicit trust level; they must not guess.
func LiftCandidates(r ResClass) []TrustLevel {
	switch r {
	case ResLow:
		return []TrustLevel{LevelNormal}
	case ResMedium:
		return []TrustLevel{LevelPrivate, LevelSensitive}
	case ResHigh:
		return []TrustLevel{LevelCredential, LevelSecurityBoundary}
	}
	return nil
}

// Rule constants pinned verbatim against the six-key machine block in
// docs/schema-v2.md section 16.
const (
	// TrustDomainLegacyProjectionRule records that the mapping is a
	// downward projection only: five levels collapse to three classes
	// exactly, and the reverse lift is a candidate window, never a
	// forced upgrade.
	TrustDomainLegacyProjectionRule = "five-levels-project-down-to-three-never-lift-by-guess"
	// TrustDomainCrossingRule pins the section 239 verbatim
	// obligation: cross-domain data movement must pass through a
	// Data Boundary.
	TrustDomainCrossingRule = "cross-domain-data-movement-requires-data-boundary"
	// TrustDomainAbsentDefault pins the honest absence shape: a
	// record without a trust level or run domain is unclassified
	// legacy, and no reader may fill it in by guessing.
	TrustDomainAbsentDefault = "absent-means-unclassified-legacy-never-inferred"
	// TrustDomainEnforcementPlane pins the Phase 0 stance: mapping
	// table only, borrowed by no decision plane.
	TrustDomainEnforcementPlane = "none-in-observation-phase"
)
