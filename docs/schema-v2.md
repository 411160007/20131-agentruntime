# Schema Contracts - Stability Wave v0

Purpose: lift the core record shapes of the audit surface out of implicit
code geometry and into explicit, versioned, machine-checked contracts. A
schema here is a promise to future readers and to every consumer of the
JSONL audit stream. Code may evolve underneath a contract; the contract
itself only evolves through the rules written in this file.

Scope of the wave: seven record schemas - `decision`, `intent`,
`authority`, `impact`, `recovery`, `evidence`, `profile`. The Event
envelope and its vocabularies already live as the frozen external
contract in `docs/api-v0.md`; this file adds the stability layer on top:
one template, seven fully instantiated schemas, zero pending slots.

Non-goal (observation-phase red line): nothing in this wave changes
runtime decision semantics. The evaluator, the rule engine, the bus, and
the audit writer are byte-identical before and after. Schemas describe
what is recorded; they never add judgement, enforcement, or new inputs to
decisions.

## 1. The four-element template

Every schema contract block must carry all four elements below. Each
element is an entity section with concrete lines - a heading alone, or a
heading plus a slogan, does not satisfy the template. Two independent
checkers machine-assert completeness on every run: the Node structural
check (`scripts/schema-v2-check.mjs`) and the Go sync tests in
`internal/schema`.

```schemav2
template_element: Version
requires: schema version constant, vocabulary listing authority, what counts as a versioned change
```

```schemav2
template_element: Compatibility
requires: additive evolution precedent, old-reader behavior, banned mutations
```

```schemav2
template_element: Migration
requires: obligation on existing records (or an explicit none plus why), phase gate for reserved values, major-bump requirements
```

```schemav2
template_element: Validation
requires: every enforcement site checking this schema, independent-source count
```

Element completeness predicate (shared definition, dual implementation):
an element section is substantive when its body carries at least three
lines, at least one `- ` list item, at least two inline code spans, and
at least one hundred twenty non-whitespace characters.

Slot lifecycle: a `pending` slot flips to `complete` exactly when its
four element sections are added to this file and its `planned_slice`
pointer line is removed. A `complete` slot without substantive elements
is red; a `pending` slot without a pointer is red. There is no third
state and no silent gap.

## 2. Wave slot map (seven schemas, zero silent gaps)

Each slot is declared exactly once in a `schemav2` block. `complete`
slots carry the full contract in sections 3 through 7 of this file;
`pending` slots carry a pointer to the build slice that will fill them.

```schemav2
schema: decision
status: complete
```

```schemav2
schema: intent
status: complete
```

```schemav2
schema: authority
status: complete
```

```schemav2
schema: impact
status: complete
```

```schemav2
schema: recovery
status: complete
```

```schemav2
schema: evidence
status: complete
```

```schemav2
schema: profile
status: complete
```

## 3. Decision schema - complete contract

A decision is the recorded outcome of a policy evaluation: every audit
line carries exactly one decision value in its `decision` field. This
section lifts that semantics out of the audit-row shape and states it as
a standalone contract, so later schemas can reference it instead of
re-deriving it from code.

Machine-readable core (mirrored by both checkers against every
independent source - the Go enum, the Node validator, and the frozen
external contract line in `docs/api-v0.md`):

```schemav2
decision_vocabulary: allow, ask, would_block
decision_phase0_runtime_set: allow, would_block
decision_carrier_field: decision
decision_effect_mapping: allow->allow; ask->ask; would_block->would_block
```

Semantics (frozen): a decision names what the policy layer concluded
about an event, never what the runtime did about it. In the observation
phase the runtime records only: `would_block` is recorded and never
enforced; `allow` grants and revokes nothing by itself; `ask` is reserved
vocabulary for a later interaction phase and is never emitted by any
observation-phase code path. No field of an audit record - including the
evidence-provenance class - is an input to a decision beyond the matched
policy rules: the provenance field is carried, never consumed, by the
decision surface, and that zero-consumption property is machine-asserted
by the gates listed under Validation below.

#### Version

- Event schema version constant: `SchemaVersion = 1` (wire field `v`).
  The decision vocabulary is at generation one: no token has ever been
  added, removed, or re-spelled since the first audit line.
- Listing authority: `AllDecisions()` in `internal/schema/event.go`
  declares the tokens in normative order; the contract line above, the
  Node validator list, and the external contract line mirror that exact
  order. Set equality alone is not enough - the sync tests compare the
  joined strings verbatim.
- A new decision token is a vocabulary change and must land in all four
  sources in the same change, with this section updated first. A change
  to what a token means at runtime is a major change: it requires a new
  contract generation and a migration note, never a silent redefinition.

#### Compatibility

- The `decision` field is not additive: it has been a required carrier on
  every line since the schema's first generation, so there is no legacy
  shape without it and no absent-key interpretation to define.
- Additive evolution inside generation one works through optional keys
  with absent-key defaults, proven twice by `tier` (absent means `L1`)
  and `source_class` (absent means unclassified legacy): pre-extension
  lines keep validating and keep routing byte-identically. Any future
  decision-adjacent field follows the same shape.
- Banned by this contract: renaming the `decision` field, re-casing or
  re-spelling a token, repurposing `ask` for a different meaning, or
  wiring the provenance class (or any summary/self-described field) into
  a decision input. External consumers pin the frozen list in
  `docs/api-v0.md`; it must keep matching the vocabulary line above.

#### Migration

- Existing records: none owed. Every historical JSONL line already
  satisfies this contract unchanged; the golden fixtures and the corpus
  under `testdata` re-validate inside every gate run with byte-identity
  comparisons, so a migration-requiring change would surface as red
  rather than as surprise.
- Reserved-value gate: enabling `ask` in the runtime set is a later-phase
  semantic change gated by the product phase rules, not a schema break -
  the vocabulary already contains the token, so no record migration is
  needed when an interactive consumer ships. That change must retire the
  observation-phase emission constraint `MustPhase0Decision` deliberately
  in the same change, not leave it half-applied.
- A future major bump (`v: 2`) requires a migration subsection here with
  three entity lines: the old-to-new field mapping, the reader
  compatibility window, and a golden re-validation assertion. None exists
  today because none is owed.

#### Validation

- Go authority: `Decision.Valid()`, `AllDecisions()`,
  `Phase0RuntimeDecisions()`, `MustPhase0Decision()`, and `DecisionFor()`
  in `internal/schema/event.go`; every emission site passes through
  `MustPhase0Decision` before a line can reach the writer.
- Independent validator: `scripts/validate-jsonl.mjs` keeps its own
  `DECISIONS` list and rejects any line whose `decision` is outside it;
  the audit writer refuses invalid events on a zero-byte path before
  touching the file, machine-tested in `internal/auditlog`.
- Three-source plus sync tests in this package: `Go` enum versus Node
  validator (`TestEnumsStayInSyncWithValidator`), Go versus the external
  contract doc (`TestAPIDocContractMatchesGoEnums`), and this file
  against all of them plus the reflection-based carrier-shape test
  (`TestDecisionContractSourcesAgree`,
  `TestDecisionCarrierFieldShape`, `TestDecisionEffectMappingContract`,
  `TestSchemaV2SlotCompleteness`). Any divergence fails the build.
- Structural gates: the gate chain greps emission paths for the reserved
  `ask` literal (closed-set enforcement at the file level), and the
  mount-decorating gate machine-asserts that no decision-plane file
  references the provenance field. `scripts/schema-v2-check.mjs` asserts
  this file's own structure: slot census, template census, vocabulary
  mirrors, and the substantive-element predicate of section 1.

## 4. Intent schema - complete contract

An intent record is the structured statement of what one user task is
meant to achieve. It is a record contract only: in the observation
phase nothing emits it and nothing in a decision path consumes it. The
ten fields below are lifted verbatim from the owner specification's
intent-contract section; the anchor block is the in-repo authority for
back-checking every wire token against its spec spelling.

Machine-readable core:

```schemav2
intent_field_vocabulary: goal, scope, expected_outcome, expected_actions, allowed_resources, sensitive_resources, forbidden_scope, authority, duration, constraints
intent_field_count: 10
intent_absent_semantics: known-gap-never-fabricated
intent_spec_anchor: intent-spec-anchor
```

Verbatim anchor (the ten field names exactly as the specification
states them, one per line, in normative order):

```intent-spec-anchor
Goal
Scope
Expected Outcome
Expected Actions
Allowed Resources
Sensitive Resources
Forbidden Scope
Authority
Duration
Constraints
```

Back-check rule: each anchor line, lower-cased with spaces replaced by
single underscores, must equal the corresponding wire token of
`intent_field_vocabulary`, in order - ten mechanical comparisons, no
hand-counting. The Node structural checker and the Go sync tests both
recompute the full mapping and the field count on every run.

Field table (wire token, JSON type, honest absent default, anchor
pointer):

| wire token | JSON type | absent means | anchor line |
|---|---|---|---|
| `goal` | string | not reported - known gap | line 1 `Goal` |
| `scope` | string | not reported - known gap | line 2 `Scope` |
| `expected_outcome` | string | not reported - known gap | line 3 `Expected Outcome` |
| `expected_actions` | list of strings | not reported - known gap | line 4 `Expected Actions` |
| `allowed_resources` | list of strings | not reported - known gap | line 5 `Allowed Resources` |
| `sensitive_resources` | list of strings | not reported - known gap | line 6 `Sensitive Resources` |
| `forbidden_scope` | string | not reported - known gap | line 7 `Forbidden Scope` |
| `authority` | `authority` record (section 5) | not reported - known gap | line 8 `Authority` |
| `duration` | string (free-form span) | not reported - known gap | line 9 `Duration` |
| `constraints` | list of strings | not reported - known gap | line 10 `Constraints` |

Semantics (frozen): every field is optional and absence carries exactly
one meaning - not reported. An agent that does not self-report a field
leaves it absent; a later slice may derive an inferred shape from
observed behavior and record that as an inference, but no code path may
fabricate a value for an unreported field. Intent modifications
originating from untrusted content are recorded through the `authority`
field with the sticky untrusted mark (section 5): the specification's
rule that such content "must be treated as UNTRUSTED and may not
auto-escalate user authority" is carried here in record form only - the
code that acts on the mark, and every posture question about acting on
it, belong to a later phase and are deliberately not decided in this
file.

#### Version

- Listing authority: `AllIntentFields()` in
  `internal/schema/intentauthority.go` declares the ten wire tokens in
  normative order; the vocabulary line above, the anchor block, and the
  Go mirror are compared as joined strings, order-sensitive. Set
  equality alone is not enough.
- The intent record has no wire generation yet (zero emitters): Version
  here names contract generation one of a not-yet-shipped record, and
  the first slice that emits intent records inherits this vocabulary
  unchanged or evolves it through the rules below.
- A new intent field is a vocabulary change that must land in the spec
  anchor, this section, and the Go list in the same change; adding an
  eleventh token without the anchor moving is banned.

#### Compatibility

- Fully additive to everything shipped: no existing event field, JSONL
  line, or validator rule changes shape because of this contract; the
  golden and corpus fixtures re-validate byte-identically inside every
  gate run before and after.
- Absent-key semantics: every field of `IntentRecord` is `omitempty`
  with the single honest default "not reported"; there is no legacy
  record shape to reinterpret and no fabricated filler value.
- Banned by this contract: renaming a wire token while the anchor line
  stays put, treating an absent field as permission (or as denial),
  consuming any intent field as an input to a decision in the
  observation phase, and silently escalating authority through a chain
  link. The closed set `{allow, would_block}` for decisions is
  untouched by anything in this section.

#### Migration

- Existing records: none owed. No `intent` record has ever been
  written, so there is nothing to migrate; the obligation transfers to
  the first emitting slice, which must satisfy this contract from its
  first line.
- Reserved-plane gate: enabling any consumer of `authority` marks, or
  any enforcement posture over them, is a later-phase semantic change
  gated by the product phase rules - not a schema break, because the
  vocabulary and the sticky rule are already written here in full.
- A future major bump of `IntentRecord` requires a migration
  subsection with three entity lines (old-to-new field mapping, reader
  compatibility window, re-validation assertion) before any emitter
  ships. None exists today because none is owed.

#### Validation

- Go authority: `AllIntentFields()`, `ParseIntentRecord()` (rejects any
  field name outside the closed ten-token vocabulary before trusting a
  value, nil record on error), and `EncodeChecked()` (validates the
  authority chain before serializing; rejected records produce no
  bytes at all) in `internal/schema/intentauthority.go`.
- Independent Node source: `scripts/schema-v2-check.mjs` parses this
  file's anchor block and vocabulary, recomputes the ten mechanical
  snake-case comparisons, cross-checks the Go declaration by reading
  its source text, and carries mutation self-tests for each of these
  checks.
- Fixtures: `testdata/intentauthority/good.jsonl` (three round-trip
  records) and `testdata/intentauthority/wild.jsonl` (seven rejected
  shapes: typo field names, spec-spelling keys, wild and
  wrong-cased origin tokens, a broken propagation mark, a type wild) -
  counts asserted programmatically, never by hand.
- Structural: the decision plane stays a zero-consumption surface - no
  policy, rule-engine, bus, or audit-writer file references these
  symbols; scripts/schema-v2-check.mjs greps the decision-plane
directories for these symbols on every run, and the d1..d7 + wave
  gates re-run green with byte-identity on every old fixture.

## 5. Authority schema - complete contract

An authority record is the chain of how a grant (or an intent
modification) entered the system. Like the intent schema it is a record
contract only: no transport surface, no enforcement, no decision
consumption in the observation phase. The five grant origins mirror the
five provider forms the specification lists for how an intent contract
can arrive.

Machine-readable core:

```schemav2
authority_origin_vocabulary: user_direct, agent_provided, ui_generated, external_model_interpreted, runtime_inferred
authority_untrusted_carrier: untrusted
authority_propagation_rule: untrusted-sticky-never-auto-escalate
authority_enforcement_plane: none-in-observation-phase
```

Origin table (wire token, meaning, spec provider form):

| wire token | meaning | provider form |
|---|---|---|
| `user_direct` | grant stated directly by the user | user-provided intent |
| `agent_provided` | task description the agent itself supplied | agent-described task |
| `ui_generated` | recorded shape produced through UI interaction | UI-generated intent |
| `external_model_interpreted` | derived by an external model's reading of content | model-assisted interpretation |
| `runtime_inferred` | derived by the product from observed behavior | runtime behavior inference |

Semantics (frozen): a chain is an ordered list of links, each naming
one origin; a link's `origin` is required (a link without a named
source is malformed, not "absent"), and the chain-level `untrusted`
mark is sticky - any untrusted link forces the chain mark set in record
form, which `AuthorityChain.Validate` machine-rejects when cleared
against a set link. The propagation rule above states the recorded
promise: an untrusted mark never auto-escalates user authority.
Enforcement is `none-in-observation-phase`: nothing acts on the mark
yet, and the open posture questions about future acting (including how
a constrained environment should fail) are not pre-decided by this
file - the material stays with its owner.

#### Version

- Listing authority: `AllGrantOrigins()` in
  `internal/schema/intentauthority.go` declares the five origin tokens
  in normative order; this file, the Go list, and the Node checker
  compare joined strings verbatim, order-sensitive.
- `AuthorityPropagationRule` and `AuthorityEnforcementPlane` are
  versioned contract strings: the Go constants and the machine lines
  above must stay byte-identical, asserted in the sync tests.
- A new origin token is a vocabulary change landing in all three
  sources in the same change; changing what the sticky rule means at
  runtime is a major change requiring a new contract generation, never
  a silent redefinition.

#### Compatibility

- Fully additive: no existing record, event field, or validator rule
  references the authority chain yet; old JSONL keeps validating
  byte-identically because this schema touches no carrier in the
  stream.
- Absent-key semantics: `links` and the chain-level `untrusted` are
  `omitempty` - an absent chain means "no recorded grant path", a
  known gap, never "trusted" and never "denied".
- Banned by this contract: inventing origin tokens outside the closed
  five, clearing the chain mark while an untrusted link stands (the
  machine-rejected direction above), and wiring any origin or mark
  into a decision input during the observation phase.

#### Migration

- Existing records: none owed; nothing has emitted an `AuthorityChain`.
  The first emitting slice (intent inflow, a later wave) inherits this
  contract whole or evolves it through the Version rules.
- Reserved-plane gate: `authority_enforcement_plane` names `none` while
  in observation phase; any later phase that ships an enforcement
  consumer must update this line deliberately in the same change - a
  half-applied enablement is the failure mode this gate exists to
  catch.
- A future major bump requires three entity lines (mapping, reader
  window, re-validation assertion) before shipping; none owed today.

#### Validation

- Go authority: `GrantOrigin.Valid()`, `AuthorityLink.Validate()`,
  `AuthorityChain.Validate()` (closed-set rejection plus the sticky
  propagation check) in `internal/schema/intentauthority.go`; wild
  origins fail before any record is usable, and `EncodeChecked`
  returns zero bytes on every rejection.
- Independent Node source: `scripts/schema-v2-check.mjs` mirrors the
  origin vocabulary and both contract strings against the Go source
  text and carries mutation self-tests (token drift, rule drift).
- Fixtures: shared with the intent schema under
  `testdata/intentauthority/` - every good chain round-trips, every
  wild shape (including the two origin wils and the propagation
  break) is rejected with byte counts unmoved.
- Structural: zero consumption by the decision plane, machine-grepped
  by the wave structural checker, exactly as the provenance class is
  by the gate chain; the closed
  set `{allow, would_block}` and every emission path are byte-identical
  through this slice.

## 6. Impact schema - complete contract

An impact record is the estimation surface for one important action:
the specification's impact-analysis section requires the system to
estimate, as well as it can, the seven dimensions below - a surface
change to one file does not mean only one file is affected. Like the
intent and authority schemas this is a record contract only: in the
observation phase nothing emits it and nothing in a decision path
consumes it. The "as well as it can" wording is honored by the honest
absent semantics below, never by fabricated estimates.

Machine-readable core (mirrored by both checkers against the Go
declarations and the fixture set):

```schemav2
impact_field_vocabulary: direct_impact, indirect_impact, propagation_impact, blast_radius, reversibility, dependency_impact, production_impact
impact_field_count: 7
impact_absent_semantics: not-estimated-known-gap
impact_spec_anchor: impact-spec-anchor
blast_radius_scope_vocabulary: file_scope, project_scope, network_scope, process_scope, database_scope, credential_scope, device_scope, subagent_scope
blast_radius_scope_count: 8
impact_enforcement_plane: none-in-observation-phase
```

Verbatim anchor (the seven dimension names exactly as the specification
lists them, one per line, in normative order):

```impact-spec-anchor
Direct Impact
Indirect Impact
Propagation Impact
Blast Radius
Reversibility
Dependency Impact
Production Impact
```

Back-check rule: each anchor line, lower-cased with spaces replaced by
single underscores, must equal the corresponding wire token of
`impact_field_vocabulary`, in order - seven mechanical comparisons, no
hand-counting. The Node structural checker and the Go sync tests both
recompute the full mapping and the field count on every run.

Field table (wire token, JSON type, honest absent default, anchor
pointer):

| wire token | JSON type | absent means | anchor line |
|---|---|---|---|
| `direct_impact` | string | not estimated - known gap | line 1 `Direct Impact` |
| `indirect_impact` | string | not estimated - known gap | line 2 `Indirect Impact` |
| `propagation_impact` | string | not estimated - known gap | line 3 `Propagation Impact` |
| `blast_radius` | scope estimate object (table below) | not estimated - known gap | line 4 `Blast Radius` |
| `reversibility` | recovery class (section 7) | not estimated - known gap | line 5 `Reversibility` |
| `dependency_impact` | string | not estimated - known gap | line 6 `Dependency Impact` |
| `production_impact` | string | not estimated - known gap | line 7 `Production Impact` |

Blast-radius scope table (wire token, meaning, spec source row). The
specification lists its eight scope dimensions as Chinese bullet lines
in its blast-radius-control section; the source lines carry no ASCII
tokens to anchor against, so the back-check degrades honestly here to
a row-order pin plus the count key `blast_radius_scope_count: 8` - the
eight rows below stand in the specification's declared order, the wire
tokens are in-repo naming assigned by this slice, and both checkers
recount the rows against the vocabulary line and the Go list:

| wire token | meaning | spec bullet |
|---|---|---|
| `file_scope` | file reach | line 1 |
| `project_scope` | whole-project reach | line 2 |
| `network_scope` | network reach | line 3 |
| `process_scope` | process reach | line 4 |
| `database_scope` | database reach | line 5 |
| `credential_scope` | credential reach | line 6 |
| `device_scope` | device reach | line 7 |
| `subagent_scope` | sub-agent reach | line 8 |

Semantics (frozen): every field is optional and absence carries exactly
one meaning - not estimated, a known gap. `blast_radius.scopes` is a
subset of the closed eight scope tokens (an unknown token is malformed,
not "absent"), and `extent` is free-form recorded prose. The
`reversibility` value is constrained to the closed four-class recovery
vocabulary of section 7: one token set across the wave, never a forked
synonym set. That field is the recorded front-end for two named future
consumers - the behavior-chain reversibility-reduction dimension
(slice W5.2) and the harm side of the product's north-star metric
definition - and in this slice no consumer exists. No impact field is
an input to any decision in the observation phase; the wave structural
checker greps the decision-plane directories for the Go symbols and
greps those directories and the command tree for recovery-execution
vocabulary (rollback, revert, undo, compensate), both with a zero-hit
baseline machine-asserted on every run.

#### Version

- Listing authority: `AllImpactFields()` in
  `internal/schema/impactrecovery.go` declares the seven wire tokens in
  normative order and `AllBlastScopes()` declares the eight scope
  tokens; the vocabulary lines above, the anchor block, the scope
  table, and the Go lists are compared as joined strings,
  order-sensitive. Set equality alone is not enough.
- The impact record has no wire generation yet (zero emitters): Version
  here names contract generation one of a not-yet-shipped record; the
  first slice that emits impact records inherits these vocabularies
  unchanged or evolves them through the rules below.
- A new impact field is a vocabulary change that must land in the spec
  anchor, this section, and the Go list in the same change; adding an
  eighth dimension token while the anchor stays at seven lines is
  banned. `ImpactEnforcementPlane` is a versioned contract string,
  mirrored byte-identically between Go and the machine line above.

#### Compatibility

- Additive and nullable by construction: the record attaches to nothing
  that ships, so every existing event line, the golden fixtures, and
  the corpus under `testdata` keep validating byte-identically inside
  every gate run before and after this contract.
- Absent-key semantics: every field of `ImpactRecord` is `omitempty`
  with the single honest default "not estimated"; there is no legacy
  record shape to reinterpret and no filler value - "estimate as well
  as you can" never becomes "invent a number".
- Banned by this contract: renaming a wire token while its anchor line
  stays put, adding a ninth blast scope while the specification still
  lists eight, treating an absent estimate as zero blast (or as full
  blast), consuming any impact field as an input to a decision in the
  observation phase, and spelling `reversibility` with any value
  outside the closed four-class set - execution-verb coinages such as
  `auto_rollback` are outside the set by construction and rejected
  before any bytes exist.

#### Migration

- Existing records: none owed. No `impact` record has ever been
  written, so there is nothing to migrate; the obligation transfers to
  the first emitting slice, which must satisfy this contract from its
  first line.
- Reserved-plane gate: enabling any consumer of the `reversibility`
  observation (W5.2 is the named future one) is a later-slice change
  through these rules - the vocabulary and the four-class constraint
  are already written here in full, so no record migration is owed
  when a consumer ships.
- A future major bump of `ImpactRecord` requires a migration
  subsection with three entity lines (old-to-new field mapping, reader
  compatibility window, re-validation assertion) before any emitter
  ships. None exists today because none is owed.

#### Validation

- Go authority: `AllImpactFields()`, `AllBlastScopes()`,
  `BlastScope.Valid()`, `ImpactRecord.Validate()`,
  `ParseImpactRecord()` (rejects any field name outside the closed
  seven-token vocabulary before trusting a value, nil record on error),
  and `EncodeImpactChecked()` (validate before serialize; rejected
  records produce no bytes at all) in
  `internal/schema/impactrecovery.go`.
- Independent Node source: `scripts/schema-v2-check.mjs` parses this
  file's anchor block, vocabulary lines, and scope table, recomputes
  the seven mechanical snake-case comparisons and the eight-row order
  pin, cross-checks both Go declarations by reading its source text,
  and carries mutation self-tests for each of these checks.
- Fixtures: `testdata/impactrecovery/good_impact.jsonl` (three
  round-trip records: full seven-field shape, sparse honest-absent
  shape, multi-scope non-reversible shape) and
  `testdata/impactrecovery/wild.jsonl` (seven rejected shapes shared
  with section 7, including the execution-verb `reversibility`
  pairing and the bare-noun scope token) - counts asserted
  programmatically, never by hand.
- Structural: the decision plane stays a zero-consumption surface - no
  policy, rule-engine, bus, or audit-writer file references these
  symbols, and no recovery-execution vocabulary appears in those
  directories or in the command tree; both greps run inside
  `scripts/schema-v2-check.mjs` on every gate run, and the d1..d7 +
  wave gates re-run green with byte-identity on every old fixture.

## 7. Recovery schema - complete contract

A recovery record is the honest classification of how far one recorded
change can be walked back. The specification's recovery-truthfulness
section requires exactly four classes and forbids the product from
advertising that every action can be undone: network sends, third-party
API submissions, external messages, and some payments or third-party
state changes may well be `non_reversible` or need
`external_compensation`. This section contracts the classification
plane only. Recovery execution - code that actually undoes, partially
restores, or compensates - is a later-phase surface (Phase 2), and no
such plane exists behind these records today.

Machine-readable core:

```schemav2
recovery_class_vocabulary: local_reversible, local_partial, external_compensation, non_reversible
recovery_class_count: 4
recovery_spec_anchor: recovery-spec-anchor
recovery_unclassified_semantics: absent-record-means-unknown-never-imply-reversible
recovery_execution_plane: none-in-observation-phase
recovery_truthfulness_rule: never-claim-fully-reversible
```

Verbatim anchor (the four classes exactly as the specification states
them, one per line, in normative order):

```recovery-spec-anchor
LOCAL REVERSIBLE
LOCAL PARTIAL
EXTERNAL COMPENSATION
NON-REVERSIBLE
```

Back-check rule: each anchor line, lower-cased with spaces and hyphens
replaced by single underscores, must equal the corresponding wire token
of `recovery_class_vocabulary`, in order - four mechanical comparisons.
This rule is one step wider than the intent and impact rules (hyphens
as well as spaces) precisely because the specification spells the last
class `NON-REVERSIBLE`; the display spelling is never a wire token, and
both checkers recompute the mapping on every run.

Class table (wire token, meaning, spec guidance):

| wire token | meaning | spec guidance |
|---|---|---|
| `local_reversible` | this product can walk the change back locally, alone | line 1 `LOCAL REVERSIBLE` |
| `local_partial` | local walk-back covers only part of the change | line 2 `LOCAL PARTIAL` |
| `external_compensation` | undoing needs a compensating action outside this product | line 3 `EXTERNAL COMPENSATION` |
| `non_reversible` | the change cannot be walked back | line 4 `NON-REVERSIBLE` |

Semantics (frozen): `class` is required - a recovery record without a
class is malformed, not "unknown". Honest absence lives at the record
level: no record written means not classified, a known gap that must
never be read back as reversible. The `transaction` field correlates
the record to the group of changes one task produced; it is a record
bit only - associating a task's changes for later whole-task recovery
is a later-phase obligation, not an execution promise of this
contract. `recovery_execution_plane` names `none`, and
`recovery_truthfulness_rule` is the recorded advertising ban: no
user-facing claim that all actions are reversible, and no wording that
packages after-the-fact recording as before-the-fact prevention.
Nothing in this section acts, executes, or enforces.

#### Version

- Listing authority: `AllRecoveryClasses()` in
  `internal/schema/impactrecovery.go` declares the four class tokens
  in normative declaration order; this file's vocabulary line, anchor
  block, class table, and the Go list are compared as joined strings,
  order-sensitive.
- `RecoveryClass.Valid()` is the closed-set predicate behind every
  check site; the four-value census is machine-recomputed on every run
  and pinned to `recovery_class_count`.
- `RecoveryExecutionPlane`, `RecoveryTruthfulnessRule`, and
  `ImpactEnforcementPlane` are versioned contract strings: the Go
  constants and the machine lines above must stay byte-identical,
  asserted in the sync tests. A fifth class is a vocabulary change
  landing in all sources in the same change; changing what a class
  means at runtime (for example shipping an execution plane) is a
  major change requiring a new contract generation, never a silent
  redefinition.

#### Compatibility

- Fully additive: no existing record, event field, or validator rule
  references a recovery class yet; old JSONL keeps validating
  byte-identically because this schema touches no carrier in the
  stream. The `reversibility` observation of section 6 reuses this
  vocabulary rather than forking a synonym set - one closed set,
  checked twice.
- Absent-record semantics: `recovery_unclassified_semantics` is the
  single interpretation - absent means unknown, never reversible and
  never denied. There is no implicit default and no legacy shape.
- Banned by this contract: inventing class tokens outside the closed
  four (including the spec display spellings leaking to the wire),
  defaulting an absent classification to `local_reversible`, writing
  recovery claims such as "fully undoable" into product copy, and
  executing or scheduling any rollback or compensation from a class
  value during the observation phase.

#### Migration

- Existing records: none owed; nothing has emitted a `RecoveryRecord`.
  The first emitting slice (and the W5.2 chain dimension that consumes
  the `reversibility` observation) inherits this contract whole or
  evolves it through the Version rules.
- Reserved-plane gate: `recovery_execution_plane` names `none` while
  in observation phase; any later phase that ships recovery execution
  must retire this line deliberately in the same change - machine
  block, this section, and the Go constant together. A half-applied
  enablement is the failure mode this gate exists to catch.
- A future major bump requires three entity lines (mapping, reader
  window, re-validation assertion) before shipping; none owed today.

#### Validation

- Go authority: `RecoveryClass.Valid()`, `RecoveryRecord.Validate()`
  (closed set plus the required-class rule - the empty token is
  rejected), `ParseRecoveryRecord()` (unknown field names rejected
  before any value is trusted), and `EncodeRecoveryChecked()` (zero
  bytes on every rejection) in `internal/schema/impactrecovery.go`.
- Independent Node source: `scripts/schema-v2-check.mjs` mirrors the
  class vocabulary, the hyphen-aware anchor back-check, and the three
  contract strings against the Go source text; it also runs the
  execution-vocabulary grep (rollback, revert, undo, compensate) over
  the decision-plane directories and the command tree - the red-shape
  assertion promised by this contract - and carries mutation
  self-tests (class drift, execution-plane pre-borrow).
- Fixtures: `testdata/impactrecovery/good_recovery.jsonl` (four
  records, one per class - the discrimination control proving each
  member is accepted and each non-member rejected) and the shared
  `testdata/impactrecovery/wild.jsonl` (spec-spelling class,
  execution-verb class coinages, cleared required class, type wild) -
  counts asserted programmatically.
- Structural: `recovery_execution_plane: none-in-observation-phase` is
  machine-pinned from both sides (Node checker and Go sync test, each
  asserting docs key equals Go constant equals the none value), and
  the execution-vocabulary grep runs on every gate; the closed set
  `{allow, would_block}` and every emission path are byte-identical
  through this slice.

## 8. Evidence schema - complete contract

An evidence record is the integrity surface for one key piece of
evidence: the specification's evidence-integrity section says key
evidence must carry seven named fields, and forbids deleting key
context just to make exporting easier. This slice contracts that field
set and its requiredness only. The conditional signature clause
("establish signatures or verifiable evidence envelopes when needed")
is recorded as an honest phasing note - no `signature` or `seal` field
exists in this contract, and the envelope arrives with its own later
slice, never as a silent extra key here.

Machine-readable core (mirrored by both checkers against the Go
declarations and the fixture set):

```schemav2
evidence_field_vocabulary: timestamp, source, integrity_hash, policy_version, decision_id, event_correlation_id, actor_agent_identity
evidence_field_count: 7
evidence_required_semantics: all-seven-required-for-key-evidence
evidence_spec_anchor: evidence-spec-anchor
evidence_enforcement_plane: none-in-observation-phase
evidence_context_retention_rule: must-not-drop-key-context-for-export
evidence_signature_envelope_phase: deferred-later-phase
```

Verbatim anchor (the seven requirements exactly as the specification
lists them, one per line, in normative order):

```evidence-spec-anchor
Timestamp
Source
Integrity Hash
Policy Version
Decision ID
Event Correlation ID
Actor / Agent Identity
```

Back-check rule: each anchor line, lower-cased with runs of spaces and
slashes collapsed to single underscores, must equal the corresponding
wire token of `evidence_field_vocabulary`, in order - seven mechanical
comparisons, no hand-counting. This rule is one step wider than the
intent and impact rules (slashes as well as spaces) precisely because
the specification spells the last requirement `Actor / Agent
Identity`; both checkers recompute the full mapping and the field
count on every run. The correlation field stays a per-event pointer;
transaction-level grouping, when a later slice needs it, reuses the
recovery contract's `transaction` token rather than forking a synonym
here (one-vocabulary-per-wave, the recovery-slice precedent).

Field table (wire token, JSON type, presence rule, anchor pointer):

| wire token | JSON type | presence | anchor line |
|---|---|---|---|
| `timestamp` | string | required, non-empty | line 1 `Timestamp` |
| `source` | string | required, non-empty | line 2 `Source` |
| `integrity_hash` | string | required, non-empty | line 3 `Integrity Hash` |
| `policy_version` | string | required, non-empty | line 4 `Policy Version` |
| `decision_id` | string | required, non-empty | line 5 `Decision ID` |
| `event_correlation_id` | string | required, non-empty | line 6 `Event Correlation ID` |
| `actor_agent_identity` | string | required, non-empty | line 7 `Actor / Agent Identity` |

Presence honesty: unlike the nullable wave records, every field here
is required, because "key evidence must carry" leaves no honest
partial form - a record missing one of the seven is malformed and
produces no bytes, and a hash whose `source` is never named is exactly
the fabricated-completeness shape the fixtures reject. Provenance
verification of the hash itself is a later-phase surface; this slice
requires the fields to exist, not to be trusted.

#### Version

- Schema version constant: contracts ride on `SchemaVersion = 1` audit
  envelope discipline; this record shape itself is versioned by the
  wave, first instantiation is slice W2.4.
- Vocabulary listing authority: the `evidence-spec-anchor` block above
  is the only normative source for the seven names; the wire tokens
  are its mechanical derivation, mirrored in
  `internal/schema/evidenceprofile.go` (`evidenceFieldWireNames`).
- A versioned change is any addition, removal, or rename inside the
  seven-token set, or any flip of the presence rule - each one needs
  its own slice, contract edit, and both-checker green in the same
  change; adding a `signature` field later is a versioned change of
  exactly this kind.

#### Compatibility

- Additive evolution precedent: like every wave schema this contract
  is additive - no existing audit line, fixture corpus, or golden file
  gains or loses fields through it, and old readers keep reading
  exactly what they read before.
- Old-reader behavior: nothing in the runtime emits or consumes
  evidence records yet (`evidence_enforcement_plane:
  none-in-observation-phase`), so reader compatibility is vacuous by
  construction until the first emitter slice (`ParseEvidenceRecord` is
  today the only decoder in the tree).
- Banned mutations: renaming a wire token, splitting `actor_agent_identity`
  into display spellings, introducing an eighth "convenience" field, or
  weakening the requiredness without retiring the spec anchor - all are
  contract events, not edits.

#### Migration

- Obligation on existing records: none. No evidence records were ever
  emitted under a previous shape, and the audit line's `v: 1` stream is
  untouched; writing that sentence is the migration, honestly.
- Phase gate for reserved values: the signature/seal envelope is the
  one reserved surface named in the spec; it stays out of the closed
  seven set until its own slice flips `evidence_signature_envelope_phase`.
- Major-bump requirements: a required-to-optional flip or field removal
  is a major change requiring retired-pointer notes in this file plus
  migration lines here; no silent byte-level reshaping.

#### Validation

- Enforcement sites: `ParseEvidenceRecord` / `EncodeEvidenceChecked`
  in `internal/schema/evidenceprofile.go` (unknown-field rejection
  before value trust, non-empty requiredness, zero bytes on rejection),
  the Node structural checker `scripts/schema-v2-check.mjs` (vocabulary,
  count, anchor back-check with the slash rule, Go mirror, plane and
  phasing strings), and the Go sync tests pinning docs to code verbatim.
- Independent-source count: three (Go declarations, Node checker keys,
  this contract file) recomputed on every gate run; the fixture pair set
  in `testdata/evidenceprofile/` carries good and wild shapes so the
  gates demonstrably have teeth.
- Structural: the decision-plane directories stay free of the new
  record symbols (planeLeak needle extended with this slice's symbols
  in the same change), and the closed decision set `{allow,
  would_block}` is byte-identical through this slice.

## 9. Profile schema - complete contract

A profile record is the accumulating observation plane the
specification asks for in two sections: a user-level long-term
security profile (twelve named areas) and a per-agent behavior profile
(eleven named accumulators). The contract is a record-form minimal
skeleton: field vocabularies, honest absence, and the
specification's own hard limit - the profile may serve as risk and
compatibility input but may never replace a hard security boundary -
pinned as constants. No judgement, scoring arithmetic, or consumer
lives behind these records in this slice, and the memory-security
principle (Memory is never Authorization) is recorded here only as a
boundary note for later consumers, with zero pre-borrowed semantics.

Machine-readable core:

```schemav2
profile_scope_vocabulary: personal, agent
profile_scope_count: 2
profile_personal_field_vocabulary: common_agents, common_tools, common_mcps, common_skills, projects, servers, domains, normal_workflows, sensitive_assets, hard_deny, preferred_security_mode, compatibility_notes
profile_personal_field_count: 12
profile_agent_field_vocabulary: common_tasks, common_tools, common_processes, common_files, common_network, normal_sequences, known_deviations, compatibility_issues, validated_fixes, confidence, trust_decay
profile_agent_field_count: 11
profile_spec_anchors: profile-personal-spec-anchor, profile-agent-spec-anchor
profile_absent_semantics: not-yet-observed-known-gap
profile_number_semantics: recorded-text-no-numeric-score
profile_enforcement_plane: none-in-observation-phase
profile_hard_boundary_rule: never-replaces-hard-security-boundaries
```

Verbatim anchors (the field names exactly as the specification lists
them - the personal set from its text block, the agent set from its
bullet list - one per line, in normative order):

```profile-personal-spec-anchor
Common Agents
Common Tools
Common MCPs
Common Skills
Projects
Servers
Domains
Normal Workflows
Sensitive Assets
Hard Deny
Preferred Security Mode
Compatibility Notes
```

```profile-agent-spec-anchor
Common Tasks
Common Tools
Common Processes
Common Files
Common Network
Normal Sequences
Known Deviations
Compatibility Issues
Validated Fixes
Confidence
Trust Decay
```

Back-check rule: each anchor line, lower-cased with spaces collapsed
to single underscores, must equal the corresponding wire token of its
scope's vocabulary, in order - twelve plus eleven mechanical
comparisons. `common_tools` is deliberately one token shared by both
scopes: the wave forbids forking synonymous vocabularies.

Scope table (row order pins the specification's section order; the
two scope tokens are in-repo naming assigned by this slice, since the
spec spells the kinds as section titles - the same honest degradation
already recorded for the blast-radius scope table):

| `personal` | user-level long-term security profile | spec personal-profile section, row 1 |
|---|---|---|
| `agent` | per-agent automatically accumulated behavior profile | spec agent-behavior section, row 2 |

All fields are optional at the record level: a profile accumulates
over time, so an absent field means not yet observed (a known gap),
never guessed content and never an implicit permissive default.
`confidence` and `trust_decay` stay free-form recorded text -
numeric scoring would pre-borrow judgement semantics this wave
refuses to carry.

#### Version

- Schema version constant: same wave discipline as the evidence
  contract (`SchemaVersion = 1` envelope, first instantiation slice
  W2.4); the two vocabularies are versioned together because one
  record kind names both scopes.
- Vocabulary listing authority: the two `profile-*-spec-anchor` blocks
  above; wire tokens live in `internal/schema/evidenceprofile.go`
  (`profilePersonalFieldWireNames`, `profileAgentFieldWireNames`,
  `profileScopeWireNames`), mirrored mechanically.
- A versioned change is any membership edit to the twelve, the eleven,
  or the two scope tokens, or a flip of the optional/absent rules -
  each needs its own slice with both checkers green in the same change.

#### Compatibility

- Additive evolution precedent: nothing existing changes shape; the
  profile contracts are new keys on a new record kind, and the audit
  stream, goldens, and corpus stay byte-identical.
- Old-reader behavior: no emitter and no consumer exist yet
  (`profile_enforcement_plane: none-in-observation-phase`), so old
  readers are unaffected by construction; the first consumer slice
  inherits the hard-boundary rule as a precondition, not as a surprise.
- Banned mutations: promoting `hard_deny` or `trust_decay` into a
  decision input in a docs edit, forking a second tools token per
  scope, or turning the scalar observations into numeric fields
  without retiring `profile_number_semantics` in the same change.

#### Migration

- Obligation on existing records: none - no profile records were
  written under any prior shape (`ParsePersonalProfile` and
  `ParseAgentProfile` are today's only profile decoders), and stating
  that is the honest migration line; no silent backfilling of
  "defaults".
- Phase gate for reserved values: judgement-feeding use of any profile
  field is reserved to a later phase and gated behind the
  hard-boundary rule plus the memory-is-not-authorization principle;
  neither is exercised by this slice.
- Major-bump requirements: making any field required, or merging the
  two scope vocabularies into one, is a major change requiring this
  section's rewrite and the row-order pin retired with notes.

#### Validation

- Enforcement sites: `ParsePersonalProfile` / `ParseAgentProfile` /
  the two checked encoders in `internal/schema/evidenceprofile.go`
  (unknown-field rejection before value trust, zero bytes on
  rejection), the Node structural checker (two vocabularies, counts,
  anchor back-checks, scope mirror, plane/hard-boundary/absent/number
  strings), and the Go sync tests pinning docs to code verbatim.
- Independent-source count: three (Go declarations, Node checker keys,
  this contract file) recomputed per run; `testdata/profile/` pairs
  good and wild fixtures (display-spelling keys, unknown tokens,
  wrong-type scalars) so the teeth are demonstrated, per the fixture
  discipline inherited from the intent and impact slices.
- Structural: this slice's symbols join the planeLeak needle set in
  the same PR, keeping decision-plane directories at zero references,
  and the closed set `{allow, would_block}` is byte-identical again.

## 10. Pending slots

None. The wave slot map above reads `status: complete` for all seven
schemas; the slot lifecycle in section 1 forbids a third state, and
this section exists so the census stays explicit rather than implied.

## 11. Wave master table - seven schemas x four elements

The stability demand of owner spec section 289 enumerates schema names and
requires Version, Compatibility, Migration, and Validation for each. This
section is the census for the seven schemas instantiated by the W2 wave:
seven times four, twenty-eight cells, one machine-readable block per
cell, zero blanks and zero slogans. Each cell carries exactly one state
from a closed set of three:

- `preexisting` - the cell substance shipped before this wave and lives
  on the frozen external contract surface; its evidence line must name
  `docs/api-v0.md` as the carrying contract.
- `this_wave` - the cell was written inside the W2 wave; its evidence
  line names the creating slice (`W2.1` through `W2.4`), and the element
  section it points at must pass the shared sufficiency predicate of
  section 1 on every re-run.
- `planned_build` - the cell is honestly unwritten; its evidence line
  must name the owing build slice in `W<n>.<m>` form. A slot whose census
  status reads `complete` may never carry this state; the state exists so
  future waves can extend the census without silent gaps.

Two independent implementations re-derive the whole table on every run:
the Node structural check `scripts/schema-v2-check.mjs` (cell census,
closed states, evidence rules, per-cell pointer resolution into
substantive element sections) and the Go second implementation
`internal/schema/schemamaster_sync_test.go` (same thresholds, plus a
creator-slice map that fails if the docs and the wave history disagree).
A docs-only rewrite cannot satisfy both by accident.

### 11.1 Coverage against the fifteen schema names of section 289

Section 289 lists fifteen names as the stability-contract scope. The
seven wave schemas are covered cell by cell in section 11.2. The
remaining eight, stated honestly, none dropped:

- `Event` - preexisting. The frozen line format, its `schema_version`
  additive-evolution rules, and its validation sites are contracted in
  `docs/api-v0.md`; duplicating them here would create a second writer,
  which the drift doctrine forbids.
- `Identity` - shipped as code enums and the agent tri-state since the
  core-completion slice; no four-element stability contract is written
  yet. Owing hook: the W5.3 delegation observation surface owes
  parent/child identity lines and must first write this census form for
  the identity record shape.
- `Capability` - shipped vocabulary since the core-completion slice.
  Owing hook: W4.1 lands the nine-action data vocabulary additively on
  this surface and must carry the four-element contract in the same PR.
- `Policy` - shipped rule engine and policy enums since the first
  slice. Owing hook: the W9 policy-simulator series must formalize the
  four elements before any rule-shape change ships.
- `Risk` - not built. Risk surfaces only as contract prohibitions (the
  W10.3 chain from analysis to risk to policy to core) and as the W11.1
  blast-radius estimation record; the four-element contract owes those
  slices, and the fail-open posture stays a pending owner ruling.
- `Agent Control API` - the minimum read-only surface (`status`,
  `audit-tail`, `timeline`) is contracted in `docs/api-v0.md` as the
  preexisting observation plane; pause, lock, and grant control verbs
  belong to Phase 1 and must not receive a contract before their
  enforcement ruling.
- `Platform Adapter API` - observation inflow exists (hook and MCP proxy
  event sources); the adapter API contract itself is the deferred
  adapter slice of the build order, registered there rather than
  silently omitted.
- `External Intelligence API` - zero integration by design in the
  observation phase (local-core iron rule); W10.1 through W10.3 are pure
  contract slices and will instantiate the four elements there.

No name above is dropped: each is either contracted, or has a named owing
slice. The census stays explicit rather than implied.

### 11.2 The twenty-eight cells

```schemav2
master_cell: decision/version
state: this_wave
evidence: created by W2.1
```

```schemav2
master_cell: decision/compatibility
state: this_wave
evidence: created by W2.1
```

```schemav2
master_cell: decision/migration
state: this_wave
evidence: created by W2.1
```

```schemav2
master_cell: decision/validation
state: this_wave
evidence: created by W2.1
```

```schemav2
master_cell: intent/version
state: this_wave
evidence: created by W2.2
```

```schemav2
master_cell: intent/compatibility
state: this_wave
evidence: created by W2.2
```

```schemav2
master_cell: intent/migration
state: this_wave
evidence: created by W2.2
```

```schemav2
master_cell: intent/validation
state: this_wave
evidence: created by W2.2
```

```schemav2
master_cell: authority/version
state: this_wave
evidence: created by W2.2
```

```schemav2
master_cell: authority/compatibility
state: this_wave
evidence: created by W2.2
```

```schemav2
master_cell: authority/migration
state: this_wave
evidence: created by W2.2
```

```schemav2
master_cell: authority/validation
state: this_wave
evidence: created by W2.2
```

```schemav2
master_cell: impact/version
state: this_wave
evidence: created by W2.3
```

```schemav2
master_cell: impact/compatibility
state: this_wave
evidence: created by W2.3
```

```schemav2
master_cell: impact/migration
state: this_wave
evidence: created by W2.3
```

```schemav2
master_cell: impact/validation
state: this_wave
evidence: created by W2.3
```

```schemav2
master_cell: recovery/version
state: this_wave
evidence: created by W2.3
```

```schemav2
master_cell: recovery/compatibility
state: this_wave
evidence: created by W2.3
```

```schemav2
master_cell: recovery/migration
state: this_wave
evidence: created by W2.3
```

```schemav2
master_cell: recovery/validation
state: this_wave
evidence: created by W2.3
```

```schemav2
master_cell: evidence/version
state: this_wave
evidence: created by W2.4
```

```schemav2
master_cell: evidence/compatibility
state: this_wave
evidence: created by W2.4
```

```schemav2
master_cell: evidence/migration
state: this_wave
evidence: created by W2.4
```

```schemav2
master_cell: evidence/validation
state: this_wave
evidence: created by W2.4
```

```schemav2
master_cell: profile/version
state: this_wave
evidence: created by W2.4
```

```schemav2
master_cell: profile/compatibility
state: this_wave
evidence: created by W2.4
```

```schemav2
master_cell: profile/migration
state: this_wave
evidence: created by W2.4
```

```schemav2
master_cell: profile/validation
state: this_wave
evidence: created by W2.4
```

The slot lifecycle of section 1 keeps its meaning here: a cell is green
only while its element section stays substantive under the shared
thresholds, so later waves that gut a section break this table before
they break any consumer.

## 12. Intent alignment record contract (slice W3.2)

Owner spec section 232 defines the intent alignment engine: a
continuous comparison of User Intent against Agent Plan against Actual
Action, with every action classified into a closed four-token set and
two honesty equations stated alongside - UNCERTAIN is not MALICIOUS,
UNRELATED is not MALICIOUS - plus a closing escalation precondition
(unrelated or uncertain combined with high impact, high sensitivity,
or irreversibility must raise the control level).

This section contracts the RECORD shape of that classification for the
observation phase only. It is deliberately not a four-element wave
slot: the fifteen schema names of section 289 enumerate the wave
census, and the action classification vocabulary is not among them, so
section 11 and its twenty-eight cells stay untouched and this section
adds no master_cell block (the census doctrine forbids private rows
beside the table). The classification itself is a record bit carried
by slice code; consuming it to move a control level is the enforcement
plane, contracted as none below, and the escalation precondition
travels as a recorded string, never as wired behavior.

### 12.1 Closed class vocabulary

The spec spells the four classes in upper case; the wire tokens are
derived mechanically (lower case) and the anchor back-check below
recomputes the mapping on every run - nobody recounts or retypes it.

```alignment-spec-anchor
DIRECT
INFERRED
UNCERTAIN
UNRELATED
```

```schemav2
alignment_class_vocabulary: direct, inferred, uncertain, unrelated
alignment_class_count: 4
alignment_absent_semantics: no-record-means-never-compared-never-implied-direct
alignment_malicious_rule: uncertain-is-not-malicious-unrelated-is-not-malicious
alignment_escalation_precondition: recorded-not-enforced
alignment_enforcement_plane: none-in-observation-phase
```

Every line of the anchor block maps to the same-position wire token
via the lower-case rule; the joined vocabulary string, the Go list in
`AllActionClasses`, and the block above must agree verbatim and in
order, mirrored again by `scripts/schema-v2-check.mjs`. The absent
semantics line is the single honest reading of a missing record: the
action was never compared (a known gap) - never an implicit direct
classification, because silently upgrading every unexamined action to
"on plan" would be the exact fabrication this wave forbids.

The two honesty equations are structural, not editorial: `malicious`
and any malice-flavoured coinage are outside the closed set, so a
suspicion-shaped value in the class field is a rejection before bytes,
not a stored judgement. The class field answers "does this action
correspond to the recorded intent or plan", and nothing else.

### 12.2 Record shape and field census

`AlignmentRecord` in `internal/schema/actionalignment.go` carries
exactly these five wire fields, pinned by row order and count against
the Go list `AllAlignmentRecordFields`:

| field | required | meaning |
| --- | --- | --- |
| `class` | yes | one of the four closed alignment tokens |
| `action_ref` | no | correlation handle to the classified action record |
| `intent_ref` | no | correlation handle to the user intent record compared |
| `plan_ref` | no | correlation handle to the agent plan record compared |
| `basis` | no | human-readable comparison rationale line, record bit only |

Class is required because an alignment record without a class is
malformed rather than "unknown": honest absence lives at the record
level (no record written means never compared), never at the class
level. A record failing any gate - unknown field name, wild class,
missing class - is rejected by `ParseAlignmentRecord` with a nil
result and produces no encoded bytes through `EncodeAlignmentChecked`.

### 12.3 Zero-consumption discipline

The record symbols (`ActionClass`, `AlignmentRecord`,
`alignmentClassWireNames`, `alignmentRecordFieldWireNames`) join the
planeLeak needle set in `scripts/schema-v2-check.mjs` in this same
PR: no policy, rules, bus, or auditlog file may reference them while
the enforcement plane reads none-in-observation-phase. Decision values
stay inside the Phase 0 closed pair `{allow, would_block}` unchanged
by this slice; the four alignment classes are not decision inputs, and
the reserved token `ask` stays where section 3 put it. This is the
carried-never-consumed promise of section 232's "continuous
comparison" reduced to record form: comparison outcomes are recorded
as data, and the control-level consequence the spec asks for waits in
the open enforcement ruling like every other enforcement surface.

---

## 13. UNTRUSTED consumption contract (slice W3.3)

Owner spec section 230 closes with one normative sentence: intent
modifications arriving from untrusted content must be treated as
UNTRUSTED and must never auto-raise user authority. Section 5
(W2.2) contracted the RECORD shape of the mark - the sticky chain
propagation that an untrusted link forces the chain mark. This
section contracts the RECORD-FORM CONSUMER of that mark: a pure
in-package function that folds one intent modification into one
intent record under five machine-pinned rules.

Like section 12 this is deliberately not a four-element wave slot:
the fifteen schema names of section 289 do not enumerate a
"modification" schema, so section 11 and its twenty-eight cells
stay untouched and this section adds no master_cell block (the
census doctrine forbids private rows beside the table). The
consumption here is record-time only; no runtime path calls it in
the observation phase, and wiring it into a decision or transport
path - including the open fail-versus-closed posture question that
the pending decision sheet leaves unresolved - is a later-phase

### 13.1 The five consumption rules

```schemav2
untrusted_consume_rule: untrusted-source-modifications-are-recorded-marked-never-auto-escalate
authority_preservation_rule: original-chain-links-preserved-identical
origin_forge_rule: untrusted-source-never-claims-user_direct
sticky_clearance_rule: consumption-never-clears-an-existing-untrusted-mark
consume_enforcement_plane: none-in-observation-phase
```

Each line is pinned verbatim against its Go constant in
`internal/schema/untrustedconsume.go` (`UntrustedConsumeRule`,
`AuthorityPreservationRule`, `OriginForgeRule`,
`StickyClearanceRule`, `ConsumeEnforcementPlane`) by the sync test
in that package and by the Node checker predicate; drift in either
direction is a red build.

The five rules read as one story. A modification names exactly one
of the ten closed intent wire tokens and carries the grant-origin
label plus the trust bit of the channel that produced it. A
modification naming `authority` is rejected outright: grants travel
as chain links appended in time order and there is no field-
overwrite path to audit, which is what "the original authority
value is preserved, not overwritten" means structurally. An
untrusted-source modification claiming `user_direct` origin is the
forgery shape and is rejected before any value is applied - the
spec's "never auto-raise user authority" reduced to a red fixture.
Everything else is applied to a fresh copy of the record (scalar
fields replaced, list fields appended behind fresh allocations,
the base record never mutated) while the pre-existing chain links
are copied through byte-identically in their original order and
positions: a lower layer never alone moves the user's boundary,
and the only link the consumption path can add is one that carries
its own origin label and, if untrusted, its own mark. The chain-
level mark is set by an untrusted source and is never cleared by a
trusted one - a later clean write does not launder an earlier
untrusted one; clearing is a user-side action outside this path.

### 13.2 Plane discipline

`ApplyIntentModification`, `IntentModification`, and the five rule
constants join the planeLeak needle set in
`scripts/schema-v2-check.mjs` in this same PR: no policy, rules,
bus, or auditlog file may reference them while
`consume_enforcement_plane` reads none-in-observation-phase, and
the command tree is scanned for the same symbols by
`scripts/gate-w3.sh` with a planted-shape control (planting a
reference must fire, removing it must clear - the gate proves its
own teeth before believing any green). Decision values stay inside
the Phase 0 closed pair `{allow, would_block}` unchanged by this
slice; the consumption point is a record function over record
types, called by tests and by nothing else. This is section 230's
closing sentence reduced from prose to a callable contract that
observes the same carried-never-consumed promise every earlier
record in this file keeps.

## 14. Intent inflow contract (slice W3.1)

Owner spec section 231 states the two-sided shape of plan analysis:
an exposed plan is analyzed against the stated intent, an unexposed
one is inferred from observed behavior. The record side of "stated"
needs one thing before either half exists: the channels through
which a stated intent enters the record, and the single unambiguous
meaning of "not stated". Section 5 (W2.2) contracted the record
shape itself; this section contracts the inflow face that feeds it,
and the inference half stays explicitly unbuilt here (building it
would be the fabrication this slice exists to make impossible).

Like sections 12 and 13 this is deliberately not a four-element
wave slot: the fifteen schema names of section 289 do not enumerate
an "inflow" schema, so section 11 and its twenty-eight cells stay
untouched and this section adds no master_cell block (the census
doctrine forbids private rows beside the table).

### 14.1 The five inflow keys

```schemav2
inflow_channel_vocabulary: cli_file,hook_task,absent_not_reported
hook_task_mapping_rule: task-field-maps-to-goal-and-nothing-else
inflow_absent_default: not-reported-is-known-gap-never-fabricated
inflow_forge_rule: inflow-channel-never-escalates-authority
inflow_enforcement_plane: none-in-observation-phase
```

The first line is the closed channel vocabulary in normative order,
pinned verbatim against `AllIntentInflowSources` in
`internal/schema/intentinflow.go`; the other four are pinned
verbatim against the rule constants (`HookTaskMappingRule`,
`InflowAbsentDefault`, `InflowForgeRule`, `InflowEnforcementPlane`)
by the sync test in that package and by the Node checker predicate;
drift in either direction is a red build.

The keys read as one story. The CLI/file channel is a thin pass-
through to the existing parser: every rejection the parser already
makes (unknown wire names, malformed chains, broken propagation) is
this channel's rejection, and a rejection yields no record at all.
The hook-payload channel is narrower by construction: its only
accepted key is `task`, and the task string flows into the goal
field and nothing else - there is no hook-channel write path to any
other intent field, so the authority chain is not policed here but
structurally unreachable. A payload smuggling `authority`, `goal`,
or any other wire name through the hook channel is the escalation
shape and is rejected before any value is applied. When neither
channel reports, the result is a positive record of absence: the
empty record, every field unreported, `known gap` as its only
meaning. An agent that does not self-report is recorded as
not-self-reported; no code path fills the hole with an invented
plan, goal, or grant.

### 14.2 Plane discipline

`IntentInflow`, `IntentInflowSource`, `AllIntentInflowSources`,
`BuildIntentFromFile`-shaped inflow constructors
(`InflowFromFile`, `InflowFromHookPayload`), `AbsentIntentRecord`,
and the four rule constants join the planeLeak needle set in
`scripts/schema-v2-check.mjs` in this same PR: no policy, rules,
bus, or auditlog file may reference them while
`inflow_enforcement_plane` reads none-in-observation-phase, and the
command tree is scanned for the same symbols by the Go sync test
with a planted-shape control (the positive control fires against
this package's own shipped source, so the walk proves its own teeth
before believing any green). Decision values stay inside the
Phase 0 closed pair `{allow, would_block}` untouched by this slice;
the inflow face is a record function over record types, called by
tests and by nothing else. The `network.intent` event type of the
event schema is a different object (network-connection intent
observation) and this contract neither consumes nor renames it.

## 15. Data action vocabulary contract (slice W4.1)

Owner spec section 237 names nine data action classes - DISCOVER,
READ, WRITE, MODIFY, DELETE, EXECUTE, EXPORT, SHARE, PERSIST - and
attaches three non-equivalences that are the point of the
classification: reading is not exporting, reading is not sharing,
writing is not executing. Section 238 adds the packaging rule:
wrapping data (compress, encode, encrypt, archive) never lowers the
sensitivity of what leaves, so EXPORT always requires an independent
re-judgement. This section contracts the RECORD shape of that
vocabulary as an optional additive Event field; it is deliberately
not a capability token and not a four-element wave slot.

Like sections 12 through 14 this contract is deliberately not one of
the fifteen schema names of section 289: the wave enumerates no "data
action" schema, so section 11 and its twenty-eight cells stay
untouched and this section adds no master_cell block (the census
doctrine forbids private rows beside the table).

### 15.1 The five data action keys

```schemav2
data_action_vocabulary: discover,read,write,modify,delete,execute,export,share,persist
data_action_nonequivalence_rule: read-not-export-read-not-share-write-not-execute
data_action_export_rejudgement_rule: export-requires-independent-rejudgement
data_action_absent_default: absent-means-unclassified-legacy-never-inferred
data_action_enforcement_plane: none-in-observation-phase
```

The first line is the closed vocabulary in normative order, pinned
verbatim against `AllDataActions` in `internal/schema/dataaction.go`
and against the `data_actions` contract line in docs/api-v0.md; the
other four are pinned verbatim against the rule constants
(`DataActionNonequivalenceRule`, `DataActionExportRejudgementRule`,
`DataActionAbsentDefault`, `DataActionEnforcementPlane`) by the sync
test in that package and by the Node checker predicate; drift in
either direction is a red build.

The keys read as one story. The nine words classify what kind of
data-facing effect a line records, and the non-equivalence rule is
the classification's load-bearing content: a consumer that collapses
`read` into `export`, `share`, or `execute` has deleted the
distinction this vocabulary exists to keep. `export` is judged on its
own, every time, and the packaging word families of section 238 are
annotation material for that re-judgement (landed with the next
slice), never a shortcut around it. Absent means unclassified legacy:
the field is optional, old lines carry no key, and no reader infers
a class into the hole. The capability table stays byte-identical, so
every legacy line referencing `net.outbound` keeps its exact meaning
under `schema_version` 1 - the additivity is structural, not
promised.

### 15.2 Plane discipline

`DataAction`, `AllDataActions`, and the four rule constants join the
planeLeak needle set in `scripts/schema-v2-check.mjs` in this same
PR: no policy, rules, bus, or auditlog file may reference them while
`data_action_enforcement_plane` reads none-in-observation-phase, and
the command tree is scanned for the same symbols by the Go sync test
with a planted-shape control (the positive control fires against
this package's own shipped source, so the walk proves its own teeth
before believing any green). Decision values stay inside the Phase 0
closed pair `{allow, would_block}` untouched by this slice; the
vocabulary is consumed by validators, docs, and tests and by nothing
else.

## 16. Trust domain mapping contract (slice W4.2)

Section 239 of the owner spec defines five trust levels (S0 Normal,
S1 Private, S2 Sensitive, S3 Credential, S4 Security Boundary) and
seven run domains (User, Agent, Tool, Sandbox, Recovery, Security
Core, External), and states that cross-domain data movement must
pass through a Data Boundary. Slice W4.2 lands the mapping between
the legacy three-tier resource sensitivity classes (`res_class`
low / medium / high, the shipped legacy surface) and that five-level semantics,
plus the run-domain vocabulary, as a record-only machine contract.
This section is deliberately not one of the fifteen schema names of
section 289: the wave enumerates no "trust domain" schema, so
section 11 and its twenty-eight cells stay untouched and this
section adds no master_cell block (the census doctrine forbids
private rows beside the table).

### 16.1 The six trust domain keys

```schemav2
trust_level_vocabulary: s0_normal,s1_private,s2_sensitive,s3_credential,s4_security_boundary
run_domain_vocabulary: user,agent,tool,sandbox,recovery,security_core,external
trust_domain_legacy_projection: s0_normal=low,s1_private=medium,s2_sensitive=medium,s3_credential=high,s4_security_boundary=high
trust_domain_crossing_rule: cross-domain-data-movement-requires-data-boundary
trust_domain_absent_default: absent-means-unclassified-legacy-never-inferred
trust_domain_enforcement_plane: none-in-observation-phase
```

The first two lines are the closed vocabularies in normative order,
pinned verbatim against `AllTrustLevels` and `AllRunDomains` in
`internal/schema/trustdomain.go`; the third is the migration
annotation table - the five pairs pinned against
`trustToLegacyClass` pair by pair - and the last three are pinned
verbatim against the rule constants (`TrustDomainCrossingRule`,
`TrustDomainAbsentDefault`, `TrustDomainEnforcementPlane`; the
projection rule constant `TrustDomainLegacyProjectionRule` carries
the direction discipline of the table below). Drift in either
direction is a red build, asserted twice: by the Node checker
predicate and by the Go sync test in the same package.

### 16.2 Migration direction: projection is a function, lift is a window

The legacy three-tier to five-tier move is annotated, never automated:

- **Five to three (projection) is total and exact.** Every trust
  level names exactly one legacy class it collapses to: s0_normal to
  low; s1_private and s2_sensitive each to medium; s3_credential and
  s4_security_boundary each to high. `LegacyClassOf` is the only
  direction the mapping runs as a function.
- **Three to five (lift) is a window, never a point.** Low lifts to
  the one-member window {s0_normal}; medium lifts to {s1_private,
  s2_sensitive}; high lifts to {s3_credential, s4_security_boundary}.
  Selecting one member of a window would fabricate which distinction
  an old line "really" drew, so the shipped API returns the whole
  candidate set (`LiftCandidates`) or nothing, and no single-valued
  lift helper exists in the package at all. Consumers carry the
  ambiguity forward or record an explicit trust level.
- **Legacy lines keep their exact meaning.** No old low/medium/high
  record is re-labelled into an S level by any tool in this wave;
  the projection pairs above are the migration annotation lines,
  present so a later boundary slice can interpret both vocabularies
  side by side without inventing history.
- **Absent means unclassified.** A record with no trust level and no
  run domain is valid, unclassified legacy, and never inferred into
  s0_normal or any default. Empty tokens have no projection and lift
  to nothing.

### 16.3 Plane discipline

`TrustLevel`, `RunDomain`, `AllTrustLevels`, `AllRunDomains`,
`LegacyClassOf`, `LiftCandidates`, `trustToLegacyClass`, and the
rule constants join the planeLeak needle set in
`scripts/schema-v2-check.mjs` in this same PR: no policy, rules,
bus, or auditlog file may reference them while
`trust_domain_enforcement_plane` reads none-in-observation-phase,
and the command tree is scanned for the same symbols by the Go test
with a planted-shape control (the positive control fires against
this package's own shipped source, so the walk proves its own teeth
before believing any green). The cross-domain obligation of section
239 is a recording requirement for the later Data Boundary slices
(W4.3 onward); nothing in the current phase enforces or judges on
it. Decision values stay inside the Phase 0 closed pair `{allow,
would_block}` untouched by this slice.

## 17. EXPORT re-judgement and packaging annotation contract (slice W4.3)

Section 238 of the owner spec names the packaging word families -
compress, encode, encrypt, archive, copy, upload, send, share - and
attaches the rule that wrapping never lowers the sensitivity of what
leaves, so EXPORT always requires an independent re-judgement. This
section lands the record shape of that annotation next to the section
15 vocabulary it serves: it names the closed eight, pins the rules as
constants, and contracts one pure constructor that emits an
observation row carrying no decision.

Like sections 12 through 16 this contract is deliberately not one of
the fifteen schema names of section 289: the wave enumerates no
"export wrap" schema, so section 11 and its twenty-eight cells stay
untouched and this section adds no master_cell block (the census
doctrine forbids private rows beside the table).

### 17.1 The five export re-judgement keys

```schemav2
export_packaging_word_vocabulary: compress,encode,encrypt,archive,copy,upload,send,share
export_packaging_sensitivity_rule: packaging-never-lowers-sensitivity
export_rejudgement_independence_rule: read-allow-never-carries-to-export
export_rejudgement_absent_default: absent-means-no-packaging-observed-never-inferred
export_rejudgement_enforcement_plane: none-in-observation-phase
```

The first line is the closed family in normative order, pinned
verbatim against `AllPackagingWords` in
`internal/schema/exportwrap.go`; the other four are pinned verbatim
against the rule constants (`ExportPackagingSensitivityRule`,
`ExportRejudgementIndependenceRule`, `ExportRejudgementAbsentDefault`,
`ExportRejudgementEnforcementPlane`) by the sync predicate in
`scripts/schema-v2-check.mjs` and the Go test in that package; drift
in either direction is a red build.

The keys read as one story. `share` is deliberately a member of both
this family and the nine data actions of section 15: as a data
action it classifies the movement itself, as a packaging word it
annotates a wrapping operation the re-judgement must see - the two
closed sets stay separate tables with separate meanings. The
independence rule is the load-bearing content: an allow recorded
beside a READ of a target is never an allow for an EXPORT of that
target, and the golden shape of this contract is the two-row facet -
same target, a read row and an export row recorded side by side,
each a record, neither carrying a decision field of any shape (the
Go test reflects over the struct to keep it that way). Packaging
words annotate; they never exempt: an export row is flagged for
independent re-judgement whether its packaging list is empty or
full, and `SensitivityPreserved` is surfaced per row so a consumer
reads the rule with the data instead of from folklore. Absent means
no packaging observed: no wrapper is inferred into the hole and no
absence of wrapping is ever read as a lowered sensitivity.

### 17.2 Plane discipline

`PackagingWord`, `AllPackagingWords`, `ExportRejudgementRecord`,
`BuildExportRejudgement`, and the four rule constants join the
planeLeak needle set in `scripts/schema-v2-check.mjs` in this same
PR: no policy, rules, bus, or auditlog file may reference them while
`export_rejudgement_enforcement_plane` reads none-in-observation-phase,
and the command tree is scanned for the same symbols by the Go test
with a planted-shape control (the positive control fires against
this package's own shipped source, so the walk proves its own teeth
before believing any green). Decision values stay inside the Phase 0
closed pair `{allow, would_block}` untouched by this slice; the
constructor is called by tests and by nothing else. The wire
contract of docs/api-v0.md gains nothing here: no event field is
added, the capability table stays byte-identical, and this is
annotation material for the Data Boundary recording obligations of
section 16 - not their enforcement.

## 18. Action necessity record contract (slice W5.1)

Section 233 of the owner spec refuses a risk judgement that asks
only whether an action is dangerous: the same command may be a
perfectly reasonable step of the task the user named, or it may read
a credential no part of that task ever asked for. The spec closes
with a verbatim obligation - the system must record Action
Necessity Evidence - and this section lands the record shape of that
obligation: three closed questions (reasonable, necessary,
replaceable), three closed answers, one evidence string per recorded
answer, and one pure constructor emitting a row that carries no
decision.

Like sections 12 through 17 this contract is deliberately not one of
the fifteen schema names of section 289: the wave enumerates no
"action necessity" schema, so section 11 and its twenty-eight cells
stay untouched and this section adds no master_cell block (the
census doctrine forbids private rows beside the table).

### 18.1 The six action necessity keys

```schemav2
action_necessity_question_vocabulary: reasonable_step,necessary_step,substitutable_step
action_necessity_answer_vocabulary: supports,refutes,no_material
action_necessity_danger_only_rule: danger-judgement-alone-never-suffices
action_necessity_evidence_rule: every-recorded-answer-requires-nonempty-evidence
action_necessity_absent_default: absent-means-unassessed-never-inferred
action_necessity_enforcement_plane: none-in-observation-phase
```

The first two lines are the closed vocabularies in normative order,
pinned verbatim against `AllNecessityQuestions` and
`AllNecessityAnswers` in `internal/schema/actionnecessity.go`; the
other four are pinned verbatim against the rule constants
(`ActionNecessityDangerOnlyRule`, `ActionNecessityEvidenceRule`,
`ActionNecessityAbsentDefault`, `ActionNecessityEnforcementPlane`)
by the sync predicate in `scripts/schema-v2-check.mjs` and the Go
test in that package; drift in either direction is a red build.

The keys read as one story. The constructor never judges: answers
and evidence arrive from the recorder, and the row stores what was
judged, never a judge - the section 233 shape examples (a deploy
chain answering "deploy the site" versus reading an unrelated SSH
key for the same intent) exist as fixtures, not as rules. The
evidence rule is load-bearing exactly as the spec words it: an
answer with empty or whitespace evidence is refused before any value
is applied, so a row without evidence cannot exist, and the
`Assessed` and `RefutedPresent` fields are pure projections over the
recorded entries - they restate the row, they add no finding.
Absence is single-meaning: a row with no entries records that
nothing was assessed, no answer is ever inferred into the hole, and
the absence of a refutation flag on an unassessed row is never read
as a necessity finding. `no_material` is an explicit looked-and-
found-nothing answer, distinct from absence by construction.

### 18.2 Plane discipline

`NecessityQuestion`, `NecessityAnswer`, `AllNecessityQuestions`,
`AllNecessityAnswers`, `ActionNecessityRecord`,
`BuildActionNecessity`, and the four rule constants join the
planeLeak needle set in `scripts/schema-v2-check.mjs` in this same
PR: no policy, rules, bus, or auditlog file may reference them while
`action_necessity_enforcement_plane` reads none-in-observation-phase,
and the command tree is scanned for the same symbols by the Go test
with a planted-shape control (the positive control fires against
this package's own shipped source, so the walk proves its own teeth
before believing any green). Decision values stay inside the Phase 0
closed pair `{allow, would_block}` untouched by this slice; the
constructor is called by tests and by nothing else. The wire
contract of docs/api-v0.md gains nothing here: no event field is
added, the nine data actions of section 15 and the trust domains of
section 16 stay byte-identical, and this is recording material for
the behavior-chain dimensions of section 242 - not their
enforcement.

## 19. Behavior chain dimension record contract (slice W5.2)

Section 242 of the owner spec refuses to let a behavior chain be
judged step by step alone: it names eight dimensions the chain
analysis must compute beyond sequence risk, and shows a sequence of
individually plausible steps (read a credential, archive, encode,
send out) that is dangerous as a whole. This section lands the
honest first move: the closed eight-dimension vocabulary, an 8/8
coverage registration in which every non-computable dimension names
a real later-slice anchor or stands explicitly pending, and pure
constructors computing the first three computable dimensions from
recorded chain steps - Intent Deviation, Trust Domain Crossing, and
Reversibility Reduction, the last consuming only the recovery
classes of slice W2.3.

Like sections 12 through 18 this contract is deliberately not one of
the fifteen schema names of section 289: the wave enumerates no
"chain dimension" schema, so section 11 and its twenty-eight cells
stay untouched and this section adds no master_cell block (the
census doctrine forbids private rows beside the table).

### 19.1 The seven behavior chain keys

```schemav2
chain_dimension_vocabulary: intent_deviation,capability_escalation,data_sensitivity_escalation,trust_domain_crossing,reversibility_reduction,blast_radius_growth,destination_change,delegation_chain
chain_dimension_computable_v0: intent_deviation,trust_domain_crossing,reversibility_reduction
chain_dimension_observation_states: observed,not_observed,unclassified
chain_dimension_coverage_rule: eight-of-eight-rows-registered-never-dangling
chain_dimension_unclassified_rule: missing-step-data-yields-unclassified-never-not-observed
chain_dimension_reversibility_source_rule: reversibility-reduction-reads-recovery-classes-only
chain_dimension_enforcement_plane: none-in-observation-phase
```

The first three lines are closed vocabularies in normative order,
pinned verbatim against `AllChainDimensions`,
`ComputableChainDimensions`, and `AllChainObservationStates` in
`internal/schema/chaindimensions.go`; the last four are pinned
against the rule constants (`ChainDimensionCoverageRule`,
`ChainDimensionUnclassifiedRule`,
`ChainDimensionReversibilitySourceRule`,
`ChainDimensionEnforcementPlane`) by the sync predicate in
`scripts/schema-v2-check.mjs` and the Go test in this package;
drift in either direction is a red build.

The constructors never judge: a chain step records its position, one
of the section 15 nine, and the optional intent text, trust level,
and recovery class it claims, and the three observations restate
what those records already say. Absence is single-meaning - a
dimension whose backing data was never recorded reads
`unclassified`, never `not_observed`, because "nothing recorded" is
not evidence that nothing happened, and no reader may turn silence
into either a finding or a clean bill of health.

### 19.2 The section 242 anchor and the 8/8 coverage registration

The eight bullet lines below mirror the owner specification's eight
dimension names verbatim; the Node checker derives the wire tokens
mechanically (lower case, spaces to single underscores) and pins
them row by row, in order, against `AllChainDimensions`:

* Intent Deviation
* Capability Escalation
* Data Sensitivity Escalation
* Trust Domain Crossing
* Reversibility Reduction
* Blast Radius Growth
* Destination Change
* Delegation Chain

The registration table gives every one of the eight a standing, so
the computable subset is a declared stage of a complete map rather
than a convenient slice:

| `intent_deviation` | computable_v0 | - |
| `capability_escalation` | anchored | W6 |
| `data_sensitivity_escalation` | anchored | W6 |
| `trust_domain_crossing` | computable_v0 | - |
| `reversibility_reduction` | computable_v0 | - |
| `blast_radius_growth` | anchored | W11 |
| `destination_change` | pending | - |
| `delegation_chain` | anchored | W5.3 |

Anchored rows name the later slice that will compute them (W5.3
lands the delegation observation fields, W6 the least-agency
counting, W11 the blast-radius estimation); the pending row says
out loud that destination change has no owner slice yet. A ninth
row, a missing row, a reordered row, or a computable row faking an
anchor is a red build in both directions.

### 19.3 Plane discipline

`ChainDimension`, `ChainStep`, `ChainObservationState`,
`BuildChainDimensions`, `ComputableChainDimensions`,
`AllChainDimensionCoverage`, and the four rule constants join the
planeLeak needle set in `scripts/schema-v2-check.mjs` in this same
PR: no policy, rules, bus, or auditlog file may reference them while
`chain_dimension_enforcement_plane` reads none-in-observation-phase,
and the command tree is scanned for the same symbols by the Go test
with a planted-shape positive control. Decision values stay inside
the Phase 0 closed pair `{allow, would_block}` untouched by this
slice: the record, the step, and the observation carry no decision,
score, or severity field (the reflective test pins the field sets
at 3, 5, and 3), and the constructors are called by tests and by
nothing else. The wire contract of docs/api-v0.md gains nothing
here: no event field is added, the nine data actions of section 15,
the trust domains of section 16, and the recovery classes of section
12's W2.3 contract stay byte-identical - this slice only reads them.

## 20. Delegation observation record contract (slice W5.3)

Section 236 of the owner spec draws the delegation inequality -
Agent A Capability is not Agent B Capability - and rules that A's
authority never transfers automatically to B, extending the same
discipline beyond agents to skills, MCP servers, tools, child
processes, and plugins, with seven objects that must be
re-evaluated whenever a delegation happens. Section 283 names the
eight fields every delegation must record and closes with the
requirement that any child agent must revalidate its own
permissions. This slice lands the observation record both sections
demand: a pure restatement, no enforcement borrowed from Phase 1.

Like sections 12 through 19 this contract is deliberately not one
of the fifteen schema names of section 289: the wave enumerates no
"delegation" schema, so section 11 and its twenty-eight cells stay
untouched and this section adds no master_cell block (the census
doctrine forbids private rows beside the table).

### 20.1 The six delegation keys

```schemav2
delegation_subject_vocabulary: skill,mcp,tool,child_process,plugin
delegation_revalidation_states: revalidated,not_revalidated,unrecorded
delegation_no_auto_inheritance_rule: parent-authority-never-auto-transfers-to-child
delegation_reevaluation_registry: identity,authority,capability,scope,data,risk,ttl
delegation_revalidation_obligation_rule: every-child-must-revalidate-own-permissions
delegation_enforcement_plane: none-in-observation-phase
```

The first two lines are closed vocabularies in normative order,
pinned verbatim against `AllDelegationSubjects` and
`AllDelegationRevalidationStates` in
`internal/schema/delegationobservation.go`; the last four are
pinned against the rule constants
(`DelegationNoAutoInheritanceRule`, `DelegationReevaluationRegistry`,
`DelegationRevalidationObligationRule`, `DelegationEnforcementPlane`)
by the sync predicate in `scripts/schema-v2-check.mjs` and the Go
test in this package; drift in either direction is a red build.

The record is deliberately not a checker. `delegated_capability`
and `data_scope` are recorded as free text and never matched
against live capability tables: matching would pre-borrow the
Phase 1 enforcement posture the taskbook freezes ("enforcement not
inherited = Phase 1"), and a record that quietly gains checking
power would itself break the no-inheritance rule it exists to
restate. Absence is single-meaning: the eight must-record fields
never accept an empty or whitespace-only value (the whole record
fails, no half record survives), and the revalidation obligation is
only admissibly silent when the silence is itself stated as
`unrecorded` - never defaulted to `revalidated`, because "nothing
recorded" is not evidence that the child did revalidate.

### 20.2 The section 283 and section 236 anchor bullets

The eight bullet lines below mirror the owner specification's
section 283 must-record list verbatim; the Node checker derives the
first eight record field tokens mechanically (lower case, spaces to
single underscores) and pins them, in order, against
`DelegationRecordFieldWireNames`:

* Parent Agent
* Child Agent
* Delegation Reason
* Delegated Capability
* Data Scope
* Authority Source
* TTL
* Outcome

The five subject bullets below mirror section 236's closing
extension line (Skill, MCP, Tool, Child Process, Plugin) and derive
mechanically to the subject vocabulary in the same order:

* Skill
* MCP
* Tool
* Child Process
* Plugin

The taskbook's seven-field shorthand for this slice
(parent/child/reason/delegated-cap/scope/TTL/outcome) is the first
seven lines above; the spec's own eighth line, Authority Source,
completes the record and is exactly what section 236's Authority
re-evaluation object records - the shorthand is not a subtraction.

### 20.3 The twelve-field record census

| `parent_agent` | recorder | section 283 line 1 |
| `child_agent` | recorder | section 283 line 2 |
| `delegation_reason` | recorder | section 283 line 3 |
| `delegated_capability` | recorder | section 283 line 4, free text, never matched |
| `data_scope` | recorder | section 283 line 5, free text, never matched |
| `authority_source` | recorder | section 283 line 6, completes the section 236 Authority object |
| `ttl` | recorder | section 283 line 7, recorded as stated, no expiry arithmetic |
| `outcome` | recorder | section 283 line 8 |
| `subject` | recorder | closed five of section 236 |
| `child_revalidation` | recorder | closed three; silence only via explicit unrecorded |
| `authority_not_inherited` | constructor-pinned | restates section 236 verbatim, uneditable |
| `enforcement_plane` | constructor-pinned | restates the Phase 0 stance, uneditable |

A thirteenth row, a reordered row, or a recorder field faking a
pinned constant is a red build in both directions (Go reflection
test and Node field table pin).

### 20.4 Plane discipline

`DelegationSubject`, `DelegationRevalidationState`,
`DelegationObservation`, `DelegationObservationInput`,
`BuildDelegationObservation`, `AllDelegationSubjects`,
`AllDelegationRevalidationStates`, `DelegationRecordFieldWireNames`,
and the four rule constants join the planeLeak needle set in
`scripts/schema-v2-check.mjs` in this same PR: no policy, rules,
bus, or auditlog file may reference them while
`delegation_enforcement_plane` reads none-in-observation-phase, and
the command tree is scanned for the same symbols by the Go test
with a planted-shape positive control. Decision values stay inside
the Phase 0 closed pair `{allow, would_block}` untouched by this
slice: the record and its input carry no decision, score, or
severity field (the reflective test pins the field sets at 12 and
10), and the constructor is called by tests and by nothing else.
The wire contract of docs/api-v0.md gains nothing here: no event
field is added, and the section 19 delegation-chain coverage row
stays anchored - this slice lands the record shape that the future
chain computation will consume, it does not compute the dimension
itself. Section 19's reversibility note keeps holding: the
recovery classes stay W2.3's, untouched here.

## 21. Agency guard observation record contract (slice W6.1)

Section 234 of the owner spec refuses to stop at Least Privilege:
Least Agency bounds how long an agent may run, how many tools it
may call, how many things it may do in parallel, how many children
it may spawn, and what it may spend. Section 261 names ten hard
bound fields per agent and per task and shows the escalation
ladder LIMIT, HOLD, CANCEL/RECOVER for the moment a bound trips.
This section lands the observation half only: three pure counters
(event rate, step count, parallelism) over per-agent or per-task
streams, a limit record whose hit state is computed and whose only
admissible response is record-only-no-action, and a 10/10 coverage
registration in which every non-counted bound field names either a
real later-slice anchor or stands explicitly pending - never a
silent hole.

Like sections 12 through 20 this contract is deliberately not one
of the fifteen schema names of section 289: the wave enumerates no
"agency guard" schema, so section 11 and its twenty-eight cells
stay untouched and this section adds no master_cell block (the
census doctrine forbids private rows beside the table).

### 21.1 The seven agency guard keys

```schemav2
agency_guard_field_vocabulary: max_steps,max_runtime,max_tool_calls,max_network_requests,max_parallelism,max_cpu,max_memory,max_storage,max_api_cost,max_child_agents
agency_counter_vocabulary: event_rate,step_count,parallelism
agency_counter_scope_vocabulary: agent,task
agency_limit_state_vocabulary: below_ceiling,at_ceiling,above_ceiling
agency_count_conservation_rule: sequences-one-to-N-events-never-lost-or-duplicated
agency_action_enum_gate: limit-hold-cancel-recover-never-recorded-as-response
agency_enforcement_plane: none-in-observation-phase
```

The first four lines are closed vocabularies in normative order,
pinned verbatim against `AllAgencyGuardFields`,
`AllAgencyCounterKinds`, `AllAgencyCounterScopes`, and
`AllAgencyLimitStates` in `internal/schema/agencyguard.go`; the
last three are pinned against the rule constants
(`AgencyCountConservationRule`, `AgencyActionEnumGate`,
`AgencyEnforcementPlane`) - and `AgencyGuardResponseRule` plus
`AgencyLadderRegistry` against their record uses - by the sync
predicate in `scripts/schema-v2-check.mjs` and the Go test in this
package; drift in either direction is a red build.

The constructors never halt and never kill: the aggregate input
carries no ceiling field at all (a ceiling that was never declared
cannot silently shape a count), the limit record computes its
state from two stated numbers instead of accepting a stated
verdict, and the response line is constructor-pinned, not caller
editable. The enum gate is mechanical: every free-text identity
line a recorder may state (`scope_id`, `event_class`) is rejected
before construction if it equals a Phase 1 ladder action token, so
LIMIT, HOLD, CANCEL, and RECOVER can never be written into a
product-path record value by any input shape. Count conservation
is mechanical too: the sequence must be the gap-free run 1..N, the
recorded `event_count` equals the number of accepted events, step
and rate events state exactly one count each, and the parallelism
observed value is the pure maximum of stated concurrency (a stated
zero is idle activity, never inferred silence). Absence is
single-meaning: an empty stream builds nothing, and the
taskbook's shared-collection-source stance with slice W8.2 is the
storage quota anchor row in the table below.

### 21.2 The section 261 anchor and the 10/10 coverage registration

The ten bullet lines below mirror the owner specification's ten
bound-field names verbatim; the Node checker derives the wire
tokens mechanically (lower case, spaces to single underscores) and
pins them row by row, in order, against `AllAgencyGuardFields`:

* Max Steps
* Max Runtime
* Max Tool Calls
* Max Network Requests
* Max Parallelism
* Max CPU
* Max Memory
* Max Storage
* Max API Cost
* Max Child Agents

The registration table gives every one of the ten a standing, so
the counted subset is a declared stage of a complete map rather
than a convenient slice:

| `max_steps` | counted_v0 | - |
| `max_runtime` | anchored | W6.2 |
| `max_tool_calls` | counted_v0 | - |
| `max_network_requests` | counted_v0 | - |
| `max_parallelism` | counted_v0 | - |
| `max_cpu` | pending | - |
| `max_memory` | pending | - |
| `max_storage` | anchored | W8.2 |
| `max_api_cost` | anchored | W6.2 |
| `max_child_agents` | counted_v0 | - |

Counted rows are this slice's three counters: steps and
parallelism are counted directly, and tool calls, network
requests, and child-agent creations are counted by the event-rate
counter over the class the recorder states (the class is free
text on purpose - matching it against a live vocabulary would
pre-borrow enforcement). Anchored rows name the later slice that
will cover them (W6.2 lands the runtime and cost proxy fields,
W8.2 the per-agent/per-task storage quota from the same
collection source); the two pending rows say out loud that CPU
and memory have no collector owner yet. An eleventh row, a
missing row, a reordered row, or a counted row faking an anchor
is a red build in both directions.

### 21.3 Plane discipline

`AgencyGuardField`, `AgencyCounterKind`, `AgencyCounterScope`,
`AgencyLimitState`, `AgencyCounterEvent`, `AgencyCounterAggregate`,
`AgencyLimitRecord`, `BuildAgencyCounterAggregate`,
`BuildAgencyLimitRecord`, `AllAgencyGuardCoverage`,
`AgencyForbiddenActionTokens`, and the five rule constants join
the planeLeak needle set in `scripts/gate-w5.sh` and the Go scan
in this package in this same PR: no policy, rules, bus, or
auditlog file may reference them while `agency_enforcement_plane`
reads none-in-observation-phase, and the command tree is scanned
for the same symbols with a planted-shape positive control. The
section 261 ladder tokens themselves stay outside every record
value (the enum gate rejects them as inputs and the reflective
test pins them absent from every output field). Decision values
stay inside the Phase 0 closed pair `{allow, would_block}`
untouched by this slice, and LIMIT and HOLD stay Phase 1 action
values with zero writes into any product path. The wire contract
of docs/api-v0.md gains nothing here: no event field is added;
the counters read recorded streams and write records only.


## 22. Cost guard observation record contract (slice W6.2)

Section 261 bounds agents and tasks in fields that are, in the
end, either counts or durations - requests, calls, children, wall
time, cpu time, memory held over time - with exactly one
exception: Max API Cost names money, and money lives on an
external billing plane this system does not speak to. The true
cost is therefore structurally absent, and this section lands the
honest shape for that absence: consumption is recorded through
five proxy fields in count/duration form, a value line without its
source is a known_gap whose wire shape carries no value key at
all, and the cost-truth record has no value entrance to fabricate
through at any caller. The zero-LLM fact rides the same rail: this
runtime's decision path calls no LLM at all, so the
llm_invocation_count zero is admitted through exactly one named
constructor whose pinned evidence line -
`zero-llm-calls-in-decision-path-proven-by-gate` - is the
decision-path gate itself: the stdlib-only import surface of this
contract, the outbound-closure stance of the module, and the
planeLeak scans this same PR extends. An explicitly stated zero with its source
and an honest absence are two different, both-admissible lines
(the W5.1 two-shapes stance restated for consumption); inventing a
third - a zero with no source - is precisely what the absence rule
rejects (the no-inventory doctrine).

Like sections 12 through 21 this contract is deliberately not one
of the fifteen schema names of section 289: the wave enumerates no
"cost guard" schema, so section 11 and its twenty-eight cells stay
untouched and this section adds no master_cell block (the census
doctrine forbids private rows beside the table).

### 22.1 The eight cost guard keys

```schemav2
cost_proxy_field_vocabulary: runtime_duration_millis,billable_request_count,cpu_time_millis,memory_mib_millis,llm_invocation_count
cost_proxy_shape_vocabulary: count,duration
cost_value_state_vocabulary: stated_observed,known_gap,gate_evidenced_zero
cost_collector_standing_vocabulary: stated_by_recorder,no_collector_known_gap,gate_evidenced_zero_line
cost_field_shape_mapping: runtime_duration_millis=duration,billable_request_count=count,cpu_time_millis=duration,memory_mib_millis=duration,llm_invocation_count=count
cost_absence_rule: no-source-is-known-gap-never-fabricated-zero
cost_truth_stance: cost-truth-lives-on-external-billing-plane-structurally-absent
cost_enforcement_plane: none-in-observation-phase
```

The first four lines are closed vocabularies in normative order,
pinned verbatim against `AllCostProxyFields`, `AllCostProxyShapes`,
`AllCostValueStates`, and `AllCostCollectorStandings` in
`internal/schema/costguard.go`; the mapping line is not a sixth
vocabulary but a derivation - it must equal the mapping produced
by the coverage registration below, row for row; the last three
are pinned against the rule constants (`CostAbsenceRule`,
`CostTruthStance`, `CostGuardEnforcementPlane`) - and
`CostGuardResponseRule` plus `CostZeroLLMStatement` against their
record uses - by the sync predicate in
`scripts/schema-v2-check.mjs` and the Go tests in this package;
drift in either direction is a red build.

The constructors never halt and never bill: the proxy input offers
no state field at all (a recorder states numbers and a source, the
state is derived), the two half shapes - a value with no source, a
source with no value - are rejected before construction leaves a
record behind, and the response line is constructor-pinned, not
caller editable. The truth record is stricter still: its struct
has no value field, so the external-plane absence is mechanical,
not a convention a future caller can forget. The gate-evidenced
zero has one door only (`BuildCostLLMZeroRecord`, pinned to
`llm_invocation_count`): no other proxy can borrow a proven zero,
and the normal path cannot reach the state at all.

### 22.2 The five proxy fields and the 5/3 coverage registration

The registration table gives every proxy field its shape and its
collector standing, so the observation surface is a declared stage
of a complete map rather than a convenient slice:

| `runtime_duration_millis` | duration | stated_by_recorder |
| `billable_request_count` | count | stated_by_recorder |
| `cpu_time_millis` | duration | no_collector_known_gap |
| `memory_mib_millis` | duration | no_collector_known_gap |
| `llm_invocation_count` | count | gate_evidenced_zero_line |

The first two rows fulfil W6.1's anchored pair (max_runtime and
max_api_cost to W6.2): the runtime bound is watched by the
duration proxy, and the cost bound is watched by the billable
request-count proxy while its truth stays the known_gap line of
section 22.3. The two gap rows carry W6.1's pending stance
forward unchanged: cpu and memory get a field shape today and a
number only once a collector owner exists; until then the only
admissible sourceless record for those fields is the known_gap
line, which is what "says out loud that there is no collector
yet" means in record form. The fifth row is the zero the gate
proves, not a zero counted from a stream. W6.1's ten-row
registration stays byte-stable behind this slice as its own
snapshot - this section adds a contract, not a revision, and the
two tables are pinned in independent directions. A sixth row, a
missing row, a reordered row, or a gap row upgrading itself to
stated is a red build in both directions.

### 22.3 Plane discipline

`CostProxyField`, `CostProxyShape`, `CostValueState`,
`CostCollectorStanding`, `CostProxyCoverageRow`,
`CostProxyRecordInput`, `CostProxyRecord`, `CostTruthRecord`,
`BuildCostProxyRecord`, `BuildCostLLMZeroRecord`,
`BuildCostTruthRecord`, `AllCostProxyFields`,
`AllCostProxyCoverage`, and the five rule constants join the
planeLeak needle set in `scripts/gate-w5.sh` and the Go scan in
this package in this same PR: no policy, rules, bus, or auditlog
file may reference them while `cost_enforcement_plane` reads
none-in-observation-phase, and the command tree is scanned for the
same symbols with the planted-shape positive control. The shared
section 261 enum gate stays in force here: `scope_id` and `source`
are rejected before construction if they equal a Phase 1 ladder
action token, so LIMIT, HOLD, CANCEL, and RECOVER keep zero writes
into any product-path record value. The truth stance restated as a
record line - `record-only-no-action`, `known_gap`, and the
`gate_evidenced_zero` door - changes nothing upstream: decision
values stay inside the Phase 0 closed pair `{allow, would_block}`
untouched by this slice, and no aggregate reads a cost record. The
wire contract of docs/api-v0.md gains nothing here: no event field
is added; the proxies read stated lines and write records only.

## 23. Decision trace field table (slice W7.1)

Spec vNext section 251 requires every important security decision
to keep a machine-readable Decision Trace and lists fifteen field
names. This section is the reverse-lookup: it reads the audit line
the product already writes (`rules.DecisionEvent` renders the
policy.decision row, its attrs carrying `rule`, `hard`, `cap`, and
`res_class`) against the fifteen-name table, and states for every
row how the field stands today - nothing more, nothing less. No
event field is added, no emitter changes, and nothing upstream of
the audit line moves: the trace is a derived read-side record
whose values are copied, never re-derived.

```schemav2
decision_trace_field_vocabulary: decision_id,timestamp,agent,task,intent,action,resource,capability,risk_factors,policy_version,policy_rule,decision,enforcement_mode,outcome,recovery_state
decision_trace_stance_vocabulary: carried_on_audit_line,carried_via_attrs,additive_field,known_gap_absent
decision_trace_absence_rule: field-without-value-is-absent-not-zero
decision_trace_correlation_rule: trace-pins-origin-event-id-not-equal-decision-id
decision_trace_enforcement_mode: record_only_phase0
decision_trace_enforcement_plane: none-in-observation-phase
decision_trace_default_applied_rule: empty-rule-means-default-applied
```

The first two lines are closed vocabularies in normative order,
pinned verbatim against `AllTraceFields`, `AllTraceStances`, and
the fifteen-row `AllTraceCoverage` table in
`internal/schema/decisiontrace.go`; the five rule lines are pinned
against `TraceAbsenceRule`, `TraceCorrelationRule`,
`TraceEnforcementModeValue`, `TraceEnforcementPlane`, and
`TraceDefaultAppliedRule` - drift in either direction is a red
build in both the Go test and the sync predicate in
`scripts/schema-v2-check.mjs`.

### 23.1 The fifteen-row reverse-lookup

| `decision_id` | carried_on_audit_line | id |
| `timestamp` | carried_on_audit_line | ts |
| `agent` | carried_on_audit_line | agent_id |
| `task` | known_gap_absent | |
| `intent` | known_gap_absent | |
| `action` | carried_on_audit_line | type |
| `resource` | carried_via_attrs | res_class |
| `capability` | carried_via_attrs | cap |
| `risk_factors` | carried_on_audit_line | severity |
| `policy_version` | additive_field | policy_version |
| `policy_rule` | carried_via_attrs | rule |
| `decision` | carried_on_audit_line | decision |
| `enforcement_mode` | additive_field | enforcement_mode |
| `outcome` | known_gap_absent | |
| `recovery_state` | known_gap_absent | |

Six fields ride the audit line itself, three ride its attrs, two
arrive additively with this slice (the recorder-stated
`policy_version` and the constructor-pinned `enforcement_mode`),
and four stay structurally absent at Phase 0: `task`, `intent`,
`outcome`, and `recovery_state`. The gap rows are named, never
filled: their keys are omitted from the wire shape and listed in
`known_gap_fields`, while an attrs-backed field whose attr was
never written is listed in `unrecorded_fields` and omitted too -
absence is literally absence, never a zero, an empty string, or a
borrowed recovery verdict in disguise. A missing matched-rule
restates as `default_applied` (the documented DecisionEvent
literal for "no rule matched, default applied"), not as silence.

### 23.2 Plane discipline

`TraceField`, `AllTraceFields`, `TraceStance`, `AllTraceStances`,
`TraceCoverageRow`, `AllTraceCoverage`, `TraceStanceOf`,
`DecisionTrace`, `BuildDecisionTrace`, `TraceAbsenceRule`,
`TraceCorrelationRule`, `TraceEnforcementModeValue`,
`TraceEnforcementPlane`, `TraceDefaultApplied`,
`EncodeTraceChecked`, and `TraceDefaultAppliedRule` join the
planeLeak needle set in `scripts/gate-w5.sh` and `scripts/gate-w6.sh`
and the Go scan in this package in this same PR: no policy, rules,
bus, or auditlog file may reference them while
`decision_trace_enforcement_plane` reads none-in-observation-phase,
and the command tree is scanned for the same symbols with the
planted-shape positive control. Construction rejects, before any
record exists: non-policy.decision lines, decisions outside the
Phase 0 runtime pair, missing id/agent/timestamp/hard annotation,
an origin event id that is empty or equal to the decision id (no
self-correlation), a wild or unstated `policy_version`, and any
copied attr value colliding with the Phase 1 ladder action tokens.
`EncodeTraceChecked` refuses hand-edited gap lists that drifted
from the table. The trace carries no verdict field of its own
beyond the copied decision value, no score, and no severity: the
census of its wire keys is pinned at fourteen.

## 24. Evidence export v0 contract (slice W7.2)

Spec vNext section 252 asks the Enterprise Evidence System to
support at least CSV, JSON, JSONL, PDF, and a Signed Evidence
Bundle, recommends a bundle tree of manifest.json, events.jsonl,
decisions.jsonl, agents.json, policies.json, recovery.json, and a
hashes/ directory; section 253 then requires key evidence to carry
seven integrity attributes and forbids dropping context for export
convenience. This slice is export **v0**: what genuinely has a
source today is delivered, everything else is named as an honest
gap row - never a fabricated placeholder file.

The exports read the W7.1 Decision Trace records and nothing else:
no emitter, no audit writer, and no decision plane moves. Byte
determinism is the machine gate - the same input exported twice
leaves identical bytes - which is precisely why the manifest
carries no wall-clock: the envelope timestamp is derived from the
latest trace timestamp, and with no traces it is omitted (absent,
not zero). An empty source label is rejected outright; a guessed
source would itself be a fabricated integrity field.

```schemav2
evidence_export_format_vocabulary: jsonl,csv,json,pdf,signed_bundle
evidence_format_stance_vocabulary: delivered_by_export_v0,delivered_via_manifest_inline,known_gap_absent
evidence_bundle_member_vocabulary: manifest.json,events.jsonl,decisions.jsonl,agents.json,policies.json,recovery.json,hashes
evidence_integrity_field_vocabulary: timestamp,source,integrity_hash,policy_version,decision_id,event_correlation_id,actor_identity
evidence_timestamp_rule: envelope-timestamp-derived-from-evidence-never-wall-clock
evidence_absent_cell_rule: csv-absent-cell-is-literal-absent-token-never-empty
evidence_writeback_rule: export-never-writes-back-to-source
evidence_context_omission_rule: no-section251-field-dropped-from-export-for-convenience
evidence_hash_rule: sha256-of-file-bytes-mismatch-is-red-never-repaired
evidence_bundle_enforcement_plane: none-in-observation-phase
```

The four vocabulary lines are closed sets in registration order,
pinned verbatim against `AllEvidenceFormats`,
`AllEvidenceFormatStances`, `AllEvidenceMembers`, and
`AllEvidenceIntegrityFields`; the six rule lines are pinned
against `EvidenceTimestampRule`, `EvidenceAbsentCellRule`,
`EvidenceWriteBackRule`, `EvidenceContextRule`, `EvidenceHashRule`,
and `EvidenceBundleEnforcementPlane` in `internal/schema/
evidenceexport.go`. Drift either direction is a red build in the
Go test and in the sync predicate of `scripts/schema-v2-check.mjs`.

### 24.1 Format coverage (section 252 "at least")

| `jsonl` | delivered_by_export_v0 | ExportTracesJSONL |
| `csv` | delivered_by_export_v0 | ExportTracesCSV |
| `json` | delivered_by_export_v0 | ExportTracesJSON |
| `pdf` | known_gap_absent | rendering stack deferred |
| `signed_bundle` | known_gap_absent | signing infrastructure deferred |

CSV renders the sixteen closed columns (the fifteen section 251
fields in spec order plus the correlation column); every uncarried
cell takes the single literal token `absent` - never an empty
cell, never a disguised zero. JSONL keeps input order so the
exported stream preserves audit sequence.

### 24.2 Bundle member coverage (section 252 tree)

| `manifest.json` | delivered_by_export_v0 | BuildEvidenceManifest |
| `events.jsonl` | known_gap_absent | no event-stream source wired into v0 |
| `decisions.jsonl` | delivered_by_export_v0 | ExportTracesJSONL body |
| `agents.json` | known_gap_absent | agent registry export not in this slice |
| `policies.json` | known_gap_absent | policy snapshot export not in this slice |
| `recovery.json` | known_gap_absent | waits for the upstream recovery field |
| `hashes` | delivered_via_manifest_inline | per-file sha256 inline in manifest files[] |

### 24.3 Integrity attributes (section 253)

| `timestamp` | manifest source_timestamp derived from traces |
| `source` | manifest source, non-empty enforced |
| `integrity_hash` | per-file sha256 pin, mismatch is red |
| `policy_version` | copied trace field on every row |
| `decision_id` | copied trace field on every row |
| `event_correlation_id` | copied trace origin_event_id |
| `actor_identity` | copied trace agent field |

`VerifyEvidenceBundle` checks manifest-to-files correspondence
one-to-one: a pinned file missing, a supplied file the manifest
never named, or a single flipped byte is a red verdict - reported,
never repaired. The exports hold no write path to any source: the
paired test pins the audit file's sha256 and mtime across a full
bundle build (the `export-never-writes-back-to-source` line above
is the contract; the test is the proof). The signed bundle stays
the taskbook's own phasing note - signing infrastructure is not
this slice, and no fake signature field is smuggled into the
manifest to look complete.


## 25. Read-only CLI surface: report + evidence (slice W7.3)

Section 307 item 15 ("enterprise can export complete Decision /
Evidence") gets its Phase 0 realisation form here: the collector's
status/audit-tail/timeline family gains two user-visible read-only
subcommands. `report` aggregates counts over validated audit lines
and prints to stdout only. `evidence` turns policy.decision lines
into W7.1 traces, envelopes them with the W7.2 integrity bundle, and
writes the verified bundle **only** into an explicit existing
operator directory. Honest skipping is bucketed and counted: a
decision with no preceding same-agent event is never self-correlated,
an un-stated policy version is a recorder error refused up front, and
a rejected trace never leaks a partial record.

```schemav2
evidence_cli_skip_bucket_vocabulary: no_prior_origin,self_correlation,trace_rejected
evidence_cli_origin_rule: origin-is-preceding-same-agent-event-never-self-correlation
evidence_cli_version_rule: policy-version-must-be-operator-stated-never-defaulted
evidence_cli_write_rule: evidence-writes-only-into-existing-explicit-dir-never-overwrite
report_cli_zero_write_rule: report-subcommand-performs-filesystem-writes-zero
cli_surface_enforcement_plane: none-in-observation-phase
```

The five CLI contract lines are pinned against the Go sources:
the skip-bucket list derives from `evidenceSkipBuckets` in
`cmd/agent-collector/evidence.go` in registration order, the write
rule against the refusal-before-any-`WriteFile` path, the report
zero-write rule against the absence of any write API in
`cmd/agent-collector/report.go`, and the enforcement plane line is
`none-in-observation-phase` like every Phase 0 surface. Determinism
is unchanged from section 24: the same audit input exported twice
leaves byte-identical bundle members, and the golden snapshot test
pins `renderReport` output verbatim.

### 25.1 Skip-bucket coverage

| `no_prior_origin` | counted-skip | leading decision with no preceding same-agent line |
| `self_correlation` | counted-skip | origin would equal the decision id (blocked upstream too) |
| `trace_rejected` | counted-skip | BuildDecisionTrace rejection, full line skipped not patched |


## 26. Storage governance V2: time + capacity dual limit (slice W8.1)

The rotation base (size policy, UTC day-cut, KeepHistory) ships since
D1 and stays untouched; this section adds the second axis of section
268: audit history is bounded by BOTH wall-clock retention per
severity class AND a total-capacity hard cap (section 269), so
neither axis may grow without bound. The section 270 pressure ladder
is landed as the observation it is contracted to be in this phase -
five closed bands computed from usage against the configured quota
and recorded, with every band response pinned
`record-only-no-action`: no compression, aggregation, or eviction
policy is wired into the write path, and a Critical security event
can never be dropped or blocked by pressure (section 270 last line),
which the write-path tests prove by landing events far over quota.

Pruning itself is a governance pass (`Log.Enforce`), never a write
side effect: the oldest rotated segments go first, a segment expires
only under the retention window of the highest severity class it
holds, and capacity eviction skips any segment holding Critical
events (positive control: critical conservation counts are equal
before and after an over-pressure pass). When the exempt history
alone still busts the cap the governor reports the honest watermark
line - `when-only-held-segments-remain-report-over-quota-never-drop-critical` -
and BytesAfter stays truthfully above quota instead of faking
compliance with a silent drop.

The live head file inherits the 0953ac1 rotation-gate exemption: a
rename-on-threshold legitimately leaves a freshly recreated empty
head until the next event, so governance never prunes the live
segment, the empty head is exempt from content validation while
keeping its 0600 mode gate, and rotated archives still validate
fully. A segment whose lines cannot be parsed is held, never
guessed-pruned (fail-closed on data loss); reads for governance
decisions touch only per-line `ts` and `severity` metadata, so
Logging Privacy (section 272) holds by construction.

### 26.1 The storage governance keys

```schemav2
storage_severity_retention_mapping: info=7d,low=7d,medium=30d,high=90d,critical=180d
storage_pressure_band_vocabulary: normal,compress_aggregate,strong_aggregation,evict_old_normal,retain_critical_only
storage_govern_action_vocabulary: prune_expired,evict_for_capacity,hold_critical,hold_live_head,hold_unreadable,over_quota_critical_only
storage_dual_limit_rule: time-and-capacity-both-enforced-neither-infinite-growth
storage_critical_hold_rule: critical-classified-segments-never-pruned-by-capacity-pressure
storage_pressure_response_rule: record-only-no-action
storage_overquota_honesty_rule: when-only-held-segments-remain-report-over-quota-never-drop-critical
storage_live_head_rule: governance-never-prunes-the-live-segment-empty-head-stays-valid
storage_write_block_rule: pressure-never-blocks-writes-in-observation-phase
storage_unreadable_rule: unreadable-segment-is-held-never-guessed-pruned
storage_quota_stance: tier-defaults-are-initial-engineering-defaults-not-permanent-commercial-promise
```

`storage_severity_retention_mapping` mirrors the section 268 defaults
(info/low = runtime event detailed 7d, medium = security decision
30d, high = agent/task audit 90d, critical = critical incident 180d)
and is derived mechanically from the `StorageRetention*` constants in
`internal/auditlog/governance.go`; Enterprise may widen windows via
`Governance.RetainOverrides` (section 268 "Enterprise 可以配置更长周
期"), never silently narrow below the shipped defaults' documented
meaning. `storage_pressure_band_vocabulary` and the band table below
derive from `storageBandCoverage()`; the six governance actions are
the closed `GovernAction` enum. Rule lines mirror the eight Go
constants one for one.

| severity | window |
| `info` | 7d |
| `low` | 7d |
| `medium` | 30d |
| `high` | 90d |
| `critical` | 180d |

| band | usage_edge | response |
| `normal` | below_60 | record_only |
| `compress_aggregate` | 60_to_80 | record_only |
| `strong_aggregation` | 80_to_90 | record_only |
| `evict_old_normal` | 90_to_95 | record_only |
| `retain_critical_only` | 95_to_100 | record_only |

### 26.2 Honest gaps this slice does not pretend to close

* Quota TIER numbers (section 269 Free 512MB / Pro 1-2GB / Recovery
  1-10GB) are initial engineering defaults carried as the
  `storage_quota_stance` line only; no tier auto-configuration is
  wired, and remote configuration of quotas is a known_gap here.
* The band responses themselves (compress, aggregate, evict-old,
  retain-critical-only) are recorded, not executed: real
  compression/aggregation pipelines are not built by this slice
  (`record-only-no-action`).
* per-agent / per-task quotas and rate fields are section 271 =
  slice W8.2; the user-visible `storage` subcommand reading
  `TotalBytes` (the du-source pinned by the row-census tests) is
  section 273 = slice W8.3.
* Like sections 22 through 25 this contract is deliberately not one
  of the fifteen schema names of section 289: section 11 and its
  twenty-eight cells stay untouched and no master_cell block is
  added.

## 27. Per-agent / per-task storage quota observation (slice W8.2)

Section 271 bounds what a single agent or task may consume: one
anomalous agent must never eat the whole disk with its own audit
lines, and critical security events keep being retained no matter
what a quota says. Section 26 lands the global dual limit; this
section lands the per-scope lens on top of it, inside the auditlog
package (writer.go and rotate.go untouched, byte for byte).

```schemav2
quota_kind_vocabulary: event_rate,log_bytes
quota_scope_pair_rule: per-agent-and-per-task-quoted-separately-never-averaged
quota_shared_source_rule: quota-recounts-stored-audit-lines-never-a-second-collector
quota_critical_exempt_rule: critical-events-never-quota-dropped-report-watermark
quota_unattributed_rule: line-without-stated-id-unattributed-never-defaulted
quota_absent_ceiling_rule: undeclared-ceiling-not-computed-stated-known-gap
quota_response_rule: record-only-no-action
quota_watermark_token: over_quota_critical_only
quota_state_mirror: below_ceiling,at_ceiling,above_ceiling
quota_task_attr_key: task_id
```

* Shared source: `ObserveQuota` re-counts the bytes already written
  to the segment family (rotated history plus the live head). It
  starts no second collector, keeps no parallel stream, and
  references no agency-guard symbol — the §21 vocabularies are mirrored by
  literals and pinned from the schema side by
  `TestQuotaVocabulariesStaySyncedWithSchema`, so drift in either
  direction is red on both planes.
* Attribution: the agent axis reads `agent_id` (the event contract
  makes it mandatory); the task axis is attributed only from a
  stated `attrs.task_id`. A line without it is unattributed on the
  task axis and the report states the gap once — never a default
  task, never a guessed bucket (the W7.1 trace doctrine: absence
  named, not fabricated).
* Record-only (observation-phase stance): every ceiling emits records
  with the shipped response token `record-only-no-action`; the
  write path never consults the observation and a triggered quota
  blocks, drops, delays, or prunes nothing.
* Critical exemption: a triggered scope that still holds Critical
  lines carries `critical_held_events` (conservation count) and the
  honest watermark `over_quota_critical_only` — the W8.1 token
  reused, not a second dialect of the same stance.
* An undeclared ceiling is a stated `known_gaps` line, never a zero
  and never a computed state (P06, inherited from the cost guard);
  an event-rate ceiling without a stated window is likewise not
  computed. A segment with any malformed line is held whole and
  reported unattributed (fail-closed, never guessed onto a scope).
* Conservation pins: `events_seen` = attributed lines counted,
  `critical_seen_events` totals all Critical lines read,
  `bytes_attributed + bytes_unattributed` equals the readable plus
  held bytes scanned. Nothing is lost, nothing is double-counted,
  nothing is invented.
* This observation is the per-scope half of the data the future
  `storage` subcommand (section 273, slice W8.3) surfaces read-only
  beside `TotalBytes`.
* Like sections 22 through 26 this contract is deliberately not one
  of the fifteen schema names of section 289: section 11 and its
  twenty-eight cells stay untouched and no master_cell block is
  added.

## 28. Storage occupancy read-only surface (slice W8.3)

Section 273 requires that a user or an enterprise can find out how much
disk 20131 actually holds, and that no design lets logs grow silently
until the disk is full. Section 26 landed the dual limits and section 27
landed the per-scope lens; this section lands the user-visible read-only
surface over both: the collector's `storage` subcommand prints the
occupancy census for the eight members the specification names, beside
the status / audit-tail / timeline / report / evidence family.

```schemav2
storagecli_member_vocabulary: runtime_logs,audit,recovery,evidence,total,quota,retention,pressure_status
storagecli_read_only_rule: storage-census-opens-audit-read-only-never-creates-never-advances
storagecli_absence_rule: member-without-source-reported-absent-never-rendered-as-zero
storagecli_shared_source_rule: storage-census-opens-audit-read-only-never-creates-never-advances;quota-recounts-stored-audit-lines-never-a-second-collector
storagecli_ceiling_rule: undeclared-ceiling-not-computed-stated-known-gap
storagecli_retention_alias: renders-section-26-retention-mapping-never-a-second-window-table
storagecli_band_alias: renders-section-26-pressure-band-vocabulary-never-a-second-ladder
storagecli_enforcement_plane: none-in-observation-phase
```

* The member list is one closed registration
  (`storageViewMembers` in `internal/auditlog/storageview.go`), emitted in
  specification order and fail-closed: the census refuses to render if a
  member is added, dropped, or reordered.
* `audit` and `total` come from the shipped du source (`TotalBytes` over
  the live segment plus the rotated family). The cross-check is against
  an independently walked disk sum, never a hand-transcribed number.
* `runtime_logs`, `recovery`, and `evidence` have no separately measured
  store in this release: the audit file is the collector's only
  persistence exit, recovery records are a classification plane carried
  on audit lines, and evidence bundles land only in an operator-declared
  existing directory. All three are reported with the absent token - not
  as zero, not as an estimate.
* `retention` is derived from the shipped per-class constants and renders
  the section 26 mapping token for token; `pressure_status` renders the
  section 26 band vocabulary and is not computed at all when no capacity
  ceiling was declared. Both lines are aliases of shipped vocabularies,
  never a second window table or a second ladder.
* Per-agent and per-task lines re-count already-durable audit lines
  through the shipped observation path. The two dimensions are displayed
  separately, never averaged into each other, and an undeclared ceiling
  is stated as a known gap.
* The census opens the live segment read-only (no create flag): asking
  how much is stored never advances the audit plane, never materialises
  an absent file, and never disturbs a concurrent append handle. No line
  in this slice can block, delete, or trim anything - the enforcement
  plane stays `none-in-observation-phase` like every other Phase 0
  surface, and the write path does not consult the census.

## 29. Historical replay and the three-gate report (slices W9.1-W9.2)

Section 254 of the master directive orders the policy simulator as
Candidate Policy -> Historical Replay -> Security Evaluation ->
False Positive Analysis -> Agent Productivity Analysis -> Performance
Analysis -> Recovery Impact Analysis -> Promotion Decision, with new
rules preferring shadow before promotion. Slice W9.1 lands the replay
step (`internal/replay`, `agent-collector replay --policy <candidate>
[--labels <table>] <corpora...>`); slice W9.2 lands the middle
evaluation faces as a machine-readable three-gate report
(`agent-collector replay --gates ...`). Recovery impact and the
promotion decision record are separate slices and appear nowhere
here.

Isolation is the contract for both surfaces. The packages read a
candidate document, audit corpora, and an optional label table; they
register nothing, construct no engine, open no writer, and import
nothing that reaches the runtime decision plane. Every blocking
reading is the Phase 0 observation word would_block - reserved
vocabulary effects (an ask-shaped effect has no Phase 0 decision line)
are held, never emitted, exactly as in the runtime recorder. The
verdict report and the gate report are byte-deterministic: two runs
over identical inputs render identical bytes.

```gates
gate_sections: security_evaluation,false_positive_analysis,agent_productivity_analysis,performance_analysis
gate_status_pair: measured,known_gap
gate_absent_rule: section-without-source-states-known-gap-never-defaulted-zero
gate_label_denominator_rule: fp-rate-over-labels-expecting-allow; missed-rate-over-labels-expecting-would_block
gate_label_without_event_rule: label-with-no-replayed-event-counted-never-dropped
gate_timing_rule: wall-clock-outside-byte-determinism-contract-structural-counts-only
gate_per_source_rule: per-source-boundaries-not-stored-declared-gap-no-invented-split
gate_response_rule: record-only-no-promotion-decision-in-this-report
```

* Verdict report fields: `seq`, `id`, `type`, `agent_id`, `decision`,
  `severity`, `matched_rule` (omitted when the default was applied -
  `default_applied` records that honestly), plus totals with the
  fixed-order `by_rule` tally (count descending, id ascending) and
  stable `mismatches` lines. Held counts both unparseable corpus
  lines and verdicts the Phase 0 emission discipline refuses to
  render; nothing is guessed and nothing is silently dropped.
* Gate report fields: `security_evaluation` mirrors the replay totals
  and adds `severity_would_block` banding (ascending, would_block
  only); `false_positive_analysis` is measured only with a label table
  and at least one replayed verdict, with the rate definitions pinned
  above, otherwise it is a stated known gap that emits no metric keys;
  `agent_productivity_analysis` lists per-agent exposure (events,
  would_block, blocked share) ordered by agent id;
  `performance_analysis` is structural only (evaluated events, held
  lines, candidate rule count) and states both absence stances for
  wall-clock timings and per-source boundaries.
* Machine checks: determinism double-run on the golden corpus, an
  oracle recount of the false-positive arithmetic taken from the raw
  label file and a generic-decode of the rendered verdicts (never
  hand-copied), known-gap negative controls (missing label table and
  empty corpora must leak no metric keys), reserved-vocabulary scans,
  and input-immutability pins (report, candidate, and labels render
  identically before and after a gate build).

## 30. Policy diff form and the promotion decision record (slice W9.3)

Section 254 ends the policy simulator pipeline at a Promotion
Decision: after replay, evaluation, and the analysis faces, a human
review decides whether a candidate leaves shadow observation, and
that decision belongs to the ledger, not to the runtime. Slice W9.3
lands the two remaining record surfaces: the rule-set diff form
(`internal/replay.Diff`, rendered next to the replay and gate
reports) and the promotion decision record shape
(`schema.PolicyPromotionRecord`), with the section 254 discipline -
absent a promoted record, no enforcement plane is admissible -
pre-pinned as the record's own restatement and wired nowhere. This
slice builds the recording surface only; the discipline gate itself
is Phase 1 semantics.

Both surfaces inherit the isolation contract: no registry, no
writer, no engine, no runtime decision plane, no merge, no apply.
The diff renders the effect fields of the documents it compares as
data and introduces no decision vocabulary of its own; the
promotion record carries no effect, no severity, no pointer. Both
render byte-deterministically.

```diff
diff_states: added,removed,modified
diff_changed_field_order: priority,field,op,value,effect,severity,hard,caps
diff_default_change_rule: stated-only-on-disagreement-absent-means-unchanged
diff_empty_list_rule: added-removed-modified-render-as-empty-arrays-never-null
diff_identical_rule: identical-true-only-when-no-add-no-remove-no-modify-no-default-change
diff_input_gate: both-documents-must-pass-policy-grammar-nil-or-invalid-rejects-with-no-half-diff
diff_response_rule: record-only-zero-enforcement-plane
```

* Diff fields: two citation heads (`base`, `candidate`: id, name,
  version, rule count), `added`/`removed` entries carrying the full
  rule and its `doc_side`, `modified` entries naming the edited
  fields in the normative order above plus both full rule views, an
  optional `default_effect_change` pair, `identical`, and the pinned
  stance line. Rule lists and union ids are ordered by rule id; two
  runs over the same pair serialize to identical bytes.

```promotion
promotion_decision_vocabulary: shadow,promoted,rejected,deferred
promotion_vocabulary_order_rule: declaration-order-is-normative-shadow-precedes-promotion-per-section-254
promotion_absent_rule: unstated-gates-citation-renders-explicit-absent-token-never-empty-string
promotion_replay_cite_rule: record-must-cite-64-hex-replay-digest-a-decision-without-evidence-is-not-recordable
promotion_no_half_record_rule: every-rejection-returns-zero-value-never-partial-record
promotion_pins_rule: stance-and-enforcement-restatements-constructor-pinned-hand-mutation-fails-validation
promotion_enforcement_plane: none-in-observation-phase
```

* Promotion record fields: `kind` (fixed `policy.promotion`),
  `candidate_id`/`candidate_version`, `base_policy_id`/
  `base_policy_version`, `decision` (the closed four above),
  `rationale` (required text - a decision without a stated reason is
  not recordable), `decided_by` (id grammar), `decided_at` (RFC3339),
  `replay_digest` (64 lowercase hex, required citation),
  `gates_digest` (64 lowercase hex or the explicit `absent` token),
  and the two constructor-pinned restatements (`stance`,
  `promotion_enforcement_plane`). The record is not an EventType;
  while `promotion_enforcement_plane` reads the shipped `none-in-observation-phase` token,
  no decision-plane file (policy, rules, bus, auditlog) may
  reference these symbols.
* Machine checks: diff golden byte pin (one canonical edited pair),
  added/removed/modified/default classification with normative field
  order, identical-pair empty-array rendering, determinism double
  run, invalid/nil input rejection with no half diff, closed
  vocabulary round-trip, digest/timestamp/version grammar rejection
  table (twelve shapes), pinned-restatement tamper tests, and the
  reserved-enforcement-vocabulary scan over both rendered surfaces.
  The wave-end gate for this slice family (```gates/```diff/
  ```promotion/```cache word census and self-assertion) lands with
  the W9 closing gate.

## 31. Decision cache observation shape (slice W9.4)

Section 267 bounds any future decision cache: the key carries the
context (agent, task, resource, capability, policy version,
context, TTL, confidence), one grant may never be reused
indefinitely across unrelated scenes, and a change of the key
context must invalidate the entry. Phase 0 has no cache and no
decisions to cache - every decision is recomputed - so slice W9.4
lands the recording surface only: the eight-constraint key shape
plus the taskbook's `context_epoch` invalidation slot
(`schema.DecisionCacheKey`) and an entry-count / hit-rate
observation record (`schema.DecisionCacheObservation`) that reuses
the shipped section 261 counter source from slice W6.1 (the
event-rate kind and the per-agent / per-task scope pair) instead of
starting a second collector. The section 267 disciplines - finite
TTL mandatory, context-change and epoch invalidation, Phase 0 hits
identically zero - are pre-pinned as the record's own restatements
and wired nowhere.

The shape never consults a store, returns a verdict, short-circuits
a computation, or holds runtime state; the invalidation helpers are
pure comparisons over stated values. A nonzero hit sighting is not
recordable in this phase: a hit would assert that a decision was
answered from memory, which Phase 0 declares structurally
impossible (positive-control pin). The rate line follows the shipped
honest-absence discipline - with no consultations it renders the
explicit `absent` token, never a fabricated zero.

```cache
decision_cache_key_vocabulary: agent,task,resource,capability,policy_version,context,ttl_seconds,confidence_percent
decision_cache_key_order_rule: declaration-order-is-normative-eight-spec-constraints-plus-ninth-context-epoch-slot
decision_cache_finite_ttl_rule: ttl-must-be-positive-inside-the-record-bound-indefinite-reuse-is-not-recordable
decision_cache_context_invalidation_rule: changed-context-dimension-or-any-epoch-move-invalidates-equality-is-the-only-fresh-answer
decision_cache_phase_zero_hits: hits-recorded-zero-always-every-decision-recomputed-nonzero-rejects-construction
decision_cache_absent_rule: no-consultations-renders-explicit-absent-token-never-fabricated-zero
decision_cache_counter_source: event-rate-kind-and-agent-task-scope-reused-from-slice-w6-1-no-second-collector
decision_cache_no_half_record_rule: every-rejection-returns-zero-value-never-partial-key-or-record
decision_cache_pins_rule: stance-and-enforcement-restatements-constructor-pinned-hand-mutation-fails-validation
decision_cache_enforcement_plane: none-in-observation-phase
```

* Key fields: the eight spec constraints in spec order (`agent`,
  `task`, `resource`, `capability` id grammar; `policy_version`
  >= 1; non-blank `context`; finite `ttl_seconds` inside the record
  bound; `confidence_percent` 0..100) plus the ninth construction
  field `context_epoch` (>= 1; zero is the unset shape). Two keys
  bind the same context when the identity dimensions, the context
  statement, the policy version, and the epoch all match; TTL and
  confidence are entry properties, not context.
* Observation fields: `kind` (fixed `decision.cache`), the reused
  counter source (`counter_kind` pinned to `event_rate`, `scope`
  from the shipped pair), `subject_id` (id grammar), the key shape,
  `entries_observed`, `consultations_observed`, `hits_recorded`
  (Phase 0 pin: zero), `invalidated_by_context_change`, the derived
  `hit_rate_percent` line, and the two constructor-pinned
  restatements (`stance`, `decision_cache_enforcement_plane`). The
  record is not an EventType; while
  `decision_cache_enforcement_plane` reads the shipped
  `none-in-observation-phase` token, no decision-plane file (policy,
  rules, bus, auditlog) may reference these symbols.
* Machine tests: normative wire-order pin over the nine fields,
  byte-deterministic key encoding, indefinite-reuse TTL rejection
  table (zero, negative, over-bound), key rejection table with
  zero-value guarantee, context-binding and epoch invalidation
  matrix (seven breaking mutations, two non-context properties),
  Phase 0 nonzero-hit positive control, record rejection table
  (second-collector name, unshipped scope, negative counts, hits
  over consultations, half key), fabricated-rate tamper test,
  pinned-restatement tests, and the reserved-vocabulary scan over
  the emitted record. The ```cache word family joins the wave-end
  census with its siblings.

## 32. External intelligence function contract (slice W10.1)

Section 290 defines the standard external-intelligence interface as
six named functions over structured data, with one prohibition: free
text may never become policy directly. Slice W10.1 lands the contract
surface only - six signatures plus their input/output schema shape,
written into this document and wired nowhere. There is no
implementation, no import, and no call site: the default wiring state
of the whole W10 family is zero-connection, and the failure-fallback
semantics (section 291) and the poisoning guard (section 292) land as
their own contract slices (W10.2, W10.3) ahead of the wave-end gate.

The isolation contract applies verbatim: this section adds no
registry, no writer, no engine, no runtime decision plane, no merge,
no apply. The intelligence path is never a required path for any file
operation (section 266); intelligence answers are untrusted auxiliary
input that may enter analysis only - never straight into policy,
never enforcement.

Signatures (closed set, six names exactly, per section 290):

* `analyze_intent(input: IntentObservation) -> Analysis`
* `analyze_plan(input: PlanObservation) -> Analysis`
* `analyze_context(input: ContextObservation) -> Analysis`
* `analyze_behavior(input: BehaviorObservation) -> Analysis`
* `explain_decision(input: DecisionTraceRef) -> Explanation`
* `recommend_policy(input: PolicyContext) -> Recommendation`

```intel
intel_function_closed_set: analyze_intent,analyze_plan,analyze_context,analyze_behavior,explain_decision,recommend_policy
intel_function_count_pin: exactly-six-names-any-add-or-drop-fails-the-contract
intel_input_gate: one-structured-observation-envelope-per-call-never-free-text-only
intel_output_closed_forms: analysis,explanation,recommendation-three-structured-shapes-never-raw-text
intel_analysis_field_order: analysis,confidence_percent,evidence_references,uncertainty,recommendation
intel_absent_confidence_rule: unstated-confidence-renders-explicit-absent-token-never-invented-zero
intel_evidence_citation_rule: evidence-references-cite-existing-artifact-digests-64-hex-shape-never-coined-strings
intel_free_text_policy_ban: free-text-output-never-becomes-policy-directly
intel_required_path_ban: intelligence-path-is-never-a-required-path-per-section-266
intel_untrusted_input_rule: answers-enter-analysis-only-never-policy-or-enforcement-directly
intel_failure_forward: unavailable-or-malformed-answers-change-no-decision-and-the-fallback-contract-continues-in-slice-w10-2
intel_wiring_state: contract-docs-only-zero-implementation-zero-import-zero-call-site
intel_enforcement_plane: none-in-observation-phase
```

* Contract fields: each of the six functions takes exactly one
  structured observation envelope (`IntentObservation`,
  `PlanObservation`, `ContextObservation`, `BehaviorObservation`,
  `DecisionTraceRef`, `PolicyContext`) and returns one of three
  closed structured shapes - `Analysis`, `Explanation`,
  `Recommendation` - never free text. The `Analysis` shape carries
  the five section-290 fields in normative order (analysis statement,
  `confidence_percent` 0..100 or the explicit absent token - an
  unstated confidence is never an invented zero, evidence references
  as a list of 64-hex artifact digests already present in the ledger,
  an `uncertainty` statement, and a `recommendation`). The
  `Explanation` shape binds a decision id to rule citations from the
  audit record; the `Recommendation` shape is a candidate statement
  under the same citation gate, admissible only as input to a policy
  evaluation, never as policy itself. While `intel_enforcement_plane`
  reads the shipped `none-in-observation-phase` token, no
  decision-plane file (policy, rules, bus, auditlog) may reference
  these symbols; the contract is documentation-only by construction.
* Machine checks: six-function closed-set census over this file -
  exactly six signature lines, the same six names on the
  `intel_function_closed_set` line, cross-checked against the section
  290 function block (count reverse-lookup equals six; an add or a
  drop is RED); the schema docs machine check stays VALID over the
  amended file; the slice diff touches documentation only (file list
  filtered to docs/ with zero code files). The ```intel word family
  joins the wave-end census with its siblings, thirteen keys pinned
  from disk.

## 33. External intelligence fallback contract (slice W10.2)

Section 291 names what can go wrong on the external-intelligence path and
what must happen when it does. The failure surface is a closed set of six
modes - timeout, 429, network failure, malformed output, hallucination
signal, unsafe recommendation. When any of them fires, 20131 must fall
back to deterministic local security: the local core keeps deciding with
zero intelligence input, exactly as it decides today while the whole W10
family stays wired nowhere. Two degenerate forms are prohibited verbatim:
"no model, therefore allow everything" is banned, and "no model,
therefore block everything" is banned. What remains admissible is graded
handling over the four section-291 axes in normative order - action risk,
sensitivity, impact, reversibility - which the deterministic policy
engine already evaluates without asking the intelligence path for help.

This slice lands the contract surface only: no implementation, no
import, no call site, no detector, no wiring. It inherits the section-32
isolation contract unchanged (no registry, no writer, no engine, no
runtime decision plane) and consumes the forward-reference line
`intel_failure_forward` that slice W10.1 pre-pinned, so the W10.1
thirteen keys keep their pinned meaning with zero amendment. Detection
and handling of hallucination signals and unsafe recommendations as
content - the validation chain that screens what intelligence may
influence - is the section-292 poisoning guard, which lands as its own
contract slice (W10.3) and is not borrowed here.

Fallback rules (closed set, six modes exactly, per section 291):

* A timeout, a 429, or a network failure classifies the answer as
  unavailable for the decision at hand; the deterministic core answer
  stands untouched, and no retry, queue, or wait slot is defined by this
  contract (availability posture is future enforcement-phase work).
* A malformed answer - anything failing the section-32 structured-shape
  gate - is treated as unavailable, never partially parsed: there is no
  rule in this contract under which a broken payload influences any
  decision, because a partially trusted structure is how untrusted data
  smuggles itself into trusted code.
* An answer carrying a hallucination signal or an unsafe recommendation
  is likewise treated as unavailable for that decision; the guard that
  recognizes such signals is the W10.3 poisoning contract, and until any
  of it is built and wired, the honest current state is that the product
  consumes no intelligence answer at all (zero-import, zero-call-site),
  so every fallback line here describes an already-permanent runtime
  condition.
* Graded handling over action risk, sensitivity, impact, and
  reversibility is the only admissible response shape; uniform allow-all
  or block-all reactions to intelligence absence are contract violations
  by construction, and the Phase 0 observation plane records would_block
  outcomes either way without enforcing them.

```intel
intel_fallback_mode_closed_set: timeout,429,network-failure,malformed-output,hallucination-signal,unsafe-recommendation
intel_fallback_mode_count_pin: exactly-six-modes-any-add-or-drop-fails-the-contract
intel_fallback_target: deterministic-local-security-core-continues-with-zero-intelligence-input
intel_fallback_ban_allow_all: intelligence-absent-never-means-allow-all
intel_fallback_ban_block_all: intelligence-absent-never-means-block-all
intel_fallback_graded_axes: action-risk,sensitivity,impact,reversibility-four-axes-in-normative-order
intel_fallback_malformed_equals_unavailable: structurally-invalid-answer-classifies-as-unavailable-never-partially-parsed
intel_fallback_hallucination_forward: hallucination-and-unsafe-answer-guard-lands-in-slice-w10-3-not-borrowed-here
intel_fallback_decision_plane_zero_llm_import: no-decision-plane-file-imports-or-embeds-any-model-client-or-inference-transport
intel_fallback_decision_plane_zero_network: decision-plane-dependency-closure-contains-no-network-package
intel_fallback_wiring_state: contract-docs-only-zero-implementation-zero-import-zero-call-site
intel_fallback_enforcement_plane: none-in-observation-phase
```

* Clause coverage (contract line ↔ section 291 clause, every clause of
  the source text answered exactly once): the six-mode enumeration maps
  to `intel_fallback_mode_closed_set` plus its count pin; "必须回退
  Deterministic Local Security" maps to `intel_fallback_target`; the two
  quoted prohibitions map to `intel_fallback_ban_allow_all` and
  `intel_fallback_ban_block_all`; "必须基于 Action Risk / Sensitivity /
  Impact / Reversibility 进行分级处理" maps to
  `intel_fallback_graded_axes`; malformed output handling maps to
  `intel_fallback_malformed_equals_unavailable`; the remaining two modes
  map to `intel_fallback_hallucination_forward`, which states the
  deferral honestly instead of inventing guard mechanics.
* Machine checks: fallback-key census re-taken from disk (twelve
  `intel_fallback_` keys pinned; the family grows thirteen to twenty-five
  and any wave-end count is recomputed mechanically, never hand-copied);
  decision-plane zero-LLM-import grep over policy, rules, bus, auditlog
  and the runtime command with plant/remove teeth controls; zero-network
  closure via the dependency listing of the four decision-plane packages
  (zero hits on net, net/http, crypto/tls; go.mod stays with zero
  requires); the schema docs machine check stays VALID over the amended
  file; the slice diff touches documentation only.

## 34. AI output poisoning guard contract (slice W10.3)

Section 292 names the surfaces that intelligence output must never touch
directly and the chain every AI suggestion must walk through before it
can influence anything. The untouchable surface is a closed set of five
boundaries - Hard Deny, Root Boundary, Credential Boundary, Recovery
Boundary, Anti-Tamper. The mandatory chain is a closed set of four steps
in normative order - Schema Validation, Source Validation, Risk
Evaluation, Policy Evaluation. Section 228 adds the role-side mirror of
the same wall: third-party models may serve as intent interpreter, plan
interpreter, context analyzer, behavior explainer, threat-analysis
assistant, or recommendation engine, and exactly four direct actions are
forbidden to them - grant Root, modify User Hard Deny, modify Security
Boundary, release Anti-Tamper. Deterministic policy evaluation is never
a step an answer can skip: bypass of the deterministic policy engine is
banned on the same footing, so the admissible sink for any intelligence
output is the earliest stage of the analysis chain - it may enter at
Analysis, it may inform Risk, it is weighed at Policy, and it never
arrives at Core as a boundary mutation.

This slice lands the contract surface only: no implementation, no
import, no call site, no validator, no wiring. It inherits the
section-32 isolation contract and the section-33 fallback contract
unchanged, and it consumes the `intel_fallback_hallucination_forward`
deferral exactly as pinned: the guard that recognizes hallucination
signals and unsafe recommendations as content is this contract, and its
recognition mechanics stay ahead of any wiring. The honest current-state
line continues: the product consumes no intelligence answer at all, the
sixth evidence-trust tier (external LLM interpretation, the
`llm_interpretation` source class) exists in the schema closed set as a
declared rank with no shipped producer anywhere in the collectors, and
therefore every prohibition here guards an empty inflow today - which is
precisely why the chain and the boundary set can be frozen now, before
any source exists to be tempted. Per section 229, a low-trust source can
never by itself move a higher-trust security boundary; this contract
pins that sentence as an enforceable stance rather than prose.

Guard rules (closed sets, verbatim against sections 292 and 228):

* The five untouchable boundaries form a closed set. An intelligence
  answer that requests, implies, or encodes a change to any of them is
  untrusted content for analysis at most; it produces no write, no
  mutation, no policy delta, and no enforcement action - and once any
  enforcement phase exists, mutation verbs from this path are expected
  to be mechanically rejected at the chain's last step, not reviewed
  into permission.
* The four-step validation chain is mandatory and ordered. An answer
  that fails schema validation never reaches source validation; one
  from a disallowed source never reaches risk evaluation; one whose
  risk evaluation is pending or failed never reaches policy
  evaluation; and none of the four steps can be bypassed by
  transport-level claims (speed, confidence, provenance headers, or
  the model asserting its own trust level are all payload
  self-description, which the section-229 trust order ranks below the
  boundaries it is trying to move).
* The four section-228 must-nots map one-to-one onto the guarded
  boundaries and are stated as contract-level bans: grant Root is
  banned; modify User Hard Deny is banned; modify Security Boundary -
  read across the credential and recovery boundaries - is banned;
  release Anti-Tamper is banned. The ban is on direct action by model
  output at every plane, in observation phase and, by pre-pinned
  stance, in every later phase.
* Bypass of deterministic policy evaluation is banned as the fifth
  stance line: the policy engine's answer is computed by embedded
  rules and hard-coded defaults regardless of intelligence presence or
  absence; an intelligence output may only be an input that policy
  evaluation itself consumes through the frozen chain, never a
  substitute for running it.
* Future enforcement-phase machine-check positions are pre-pinned now,
  declared and not implemented: a guard-family census re-taken from
  disk at wave gates; the must-not set reverse-checked line-by-line
  against contract clauses; a zero-producer grep for the
  `llm_interpretation` source class outside the schema closed set with
  plant/remove teeth; and a sink-stage denylist check when (not if) any
  consumer is ever proposed for wiring review.

```intel
intel_guard_boundaries_closed_set: hard-deny,root-boundary,credential-boundary,recovery-boundary,anti-tamper
intel_guard_boundary_count_pin: exactly-five-boundaries-any-add-or-drop-fails-the-contract
intel_guard_validation_chain_closed_set: schema-validation,source-validation,risk-evaluation,policy-evaluation-four-steps-in-normative-order
intel_guard_chain_count_pin: exactly-four-steps-any-add-or-drop-fails-the-contract
intel_guard_admissible_sink_first_stage: intelligence-output-enters-at-analysis-stage-only-never-at-policy-or-core-as-boundary-mutation
intel_guard_must_not_closed_set: grant-root,modify-user-hard-deny,modify-security-boundary,release-anti-tamper
intel_guard_must_not_count_pin: exactly-four-prohibitions-mapped-one-to-one-onto-guarded-boundaries
intel_guard_policy_bypass_banned: intelligence-answer-never-bypasses-deterministic-policy-evaluation
intel_guard_low_trust_cannot_move_high_trust: sixth-tier-evidence-never-unilaterally-moves-a-higher-tier-security-boundary
intel_guard_sixth_tier_structural_absence: no-shipped-producer-emits-llm-interpretation-source-class-zero-llm-current-state-continuation-proof
intel_guard_enforcement_slots_prepinned: guard-census-mustnot-reverse-lookup-zero-producer-grep-sink-stage-denylist-declared-not-implemented
intel_guard_wiring_state: contract-docs-only-zero-implementation-zero-import-zero-call-site
intel_guard_enforcement_plane: none-in-observation-phase
```

* Clause coverage (contract line ↔ source clause, every clause of the
  two source texts answered exactly once): the five bullets of section
  292's "不能直接修改" list map to
  `intel_guard_boundaries_closed_set` plus its count pin; section
  292's four-step chain maps to
  `intel_guard_validation_chain_closed_set` plus its count pin and,
  for the ordering semantics, the ordered-bypass bullet above; the six
  allowed roles of section 228 are already pinned by the section-32
  function contract and are not re-traded here; the four "绝不能直接"
  items of section 228 map to `intel_guard_must_not_closed_set` with
  its one-to-one count pin; "低可信来源不能单独改变高优先级安全边界"
  of section 229 maps to `intel_guard_low_trust_cannot_move_high_trust`;
  the taskbook's sink formulation maps to
  `intel_guard_admissible_sink_first_stage`; the "绕 Deterministic
  Policy" ban maps to `intel_guard_policy_bypass_banned`; the
  pre-pinned check positions map to `intel_guard_enforcement_slots_prepinned`;
  and the structural-absence continuation proof maps to
  `intel_guard_sixth_tier_structural_absence`.
* Machine checks: guard-key census re-taken from disk (thirteen
  `intel_guard_` keys pinned; family count recomputed mechanically at
  the wave-end gate, never hand-copied); §228 must-not reverse lookup
  four-for-four and §292 boundaries five-for-five and chain
  four-for-four against the fence above; zero-producer grep for
  `llm_interpretation` outside the schema declaration surface with
  plant/remove teeth controls; the section-33 decision-plane
  zero-LLM-import and zero-network closure checks re-run and stay
  clean; the schema docs machine check stays VALID over the amended
  file; the slice diff touches documentation only.

## 35. Impact / blast-radius estimation record shape (slice W11.1)

Impact analysis asks every important action to be estimated as far as
possible; blast-radius control names eight scope dimensions to compute
per agent or task. This slice lands the estimation record surface for
the part that is honestly computable ahead of any enforcement engine:
from the raw JSON argument bytes of one tool call, two of the eight
dimensions are derivable as a closed syntactic subset - `file_scope`
(argument strings shaped as POSIX absolute, "./" or "../" relative,
"~/"-home-relative, or Windows drive-prefixed paths) and `network_scope`
(argument strings shaped as URLs whose scheme is one of the six shipped
transport schemes, recorded as lower-cased bare host or bracketed IPv6
literal; userinfo is stripped before anything is recorded, because a
credential-bearing authority would leak the credential scope). Bare
names, scheme-less host:port text, and non-shipped schemes are
deliberately not claimed: guessing would turn an estimate into a
fabrication.

The remaining six dimensions are never silently absent. Every record
carries the declared-gap list in normative closed order -
`project_scope, process_scope, database_scope, credential_scope,
device_scope, subagent_scope` - so the record can never read as a
complete blast-radius picture. Three honesty fields are
constructor-pinned and re-checked by validation: the estimate basis
token `estimated-from-argument-text-only-not-observed-fact` (the scope
lists state what the argument text says, never an observed fact about
the world), the outward-phrasing separation
`never-renders-as-blocked-or-contained` (an estimation speaks about
predicted reach, never about prevention), and the record-only stance
with the enforcement-plane token `none-in-observation-phase` - the one
shipped spelling every earlier wave record already uses, so no second
dialect can enter.

Grammar rules: the record binds the exact argument bytes it estimated
through a SHA-256 digest; the three scope lists must be present and are
sorted and deduplicated (an empty list states "nothing claimable in the
argument text", null states nothing and is not admissible); every
entry is re-checked against the closed grammar it came from; a rejected
build returns no record at all, never a half-filled estimate. Like the
promotion record, restatements are pinned so a hand-built record must
carry the identical values or validation rejects it.

Wiring boundary: nothing. The record is not an event type, enters no
decision-plane file, and is consulted by no evaluator, rule engine,
bus, or audit writer; while the enforcement-plane token reads
`none-in-observation-phase` the shape carries no effect, severity, or
decision input. Machine assertions for this slice live in the schema
package tests: constructor-pinned tokens, positive and negative
fixtures for each derivable dimension, declared-gap closed-vocabulary
mirrors, digest binding, null-list and userinfo refusals, and the
single-plane-token spelling check across every wave record.

## 36. Coverage truthfulness matrix (slice W11.2)

The V2 security-coverage contract demands that every Agent, OS, Tool,
MCP, and Capability surface shows its real coverage state, never a
smoothed promise. This slice lands the matrix in its two required forms -
the human table (`docs/coverage-truthfulness.md`) and the machine-readable
mirror (`testdata/golden/coverage-truthfulness.json`) - under a four-tier
closed vocabulary: `FULL`, `LIMITED`, `MONITOR ONLY`, `UNAVAILABLE`. No
fifth tier exists; no cell may be empty. FULL means the shipped code
declares no degradation path; LIMITED means the shipped code names a
concrete one; MONITOR ONLY means observation and record only, which is
every shipped surface today; UNAVAILABLE means the platform does not
implement the surface at all (and on any GOOS outside linux, darwin, and
windows the whole collection surface is UNAVAILABLE by compile-time fact).

Every cell carries at least one evidence pointer - a shipped file plus a
token that must resolve inside it - and the machine check
(`scripts/coverage-truth-check.mjs`) re-resolves every pointer against
the real tree on every run, so a hollow cell is structurally impossible.
The two forms are lockstep-bound: same rows, same order, same tiers,
checked from independent parses of both carriers.

Counters are in this contract by census, not by prose. The shipped
per-cycle attributes `no_cmdline` and `hidden` are recorded as populated
and re-found in the collector wiring at build time. The unix
hidden-by-OS seed field is recorded as declared-unset: it exists in the
statistics shape, no production path increments it, and the check counts
non-test occurrences expecting exactly the declaration. The day code
populates it without upgrading the matrix in the same PR, the census
goes red - a silent half-claim is exactly what this contract forbids.

Outward obligation: every consumer display - this document, the UI
surface when it ships, the enterprise report surface when it ships -
must carry the tiers verbatim per platform together with the per-platform
difference notes that say why a cell is not FULL. Blanket promises that
smooth the per-OS hiding mechanisms into one undifferentiated guarantee
are forbidden in every carrier; the checker fails the exact marketing
shapes, and naming the difference is the product.

Wiring boundary: nothing. The matrix is display and documentation only -
not an event type, not a decision input. While the enforcement-plane
token reads `none-in-observation-phase` (the one shipped spelling; the
check rejects any second dialect) the matrix reaches no evaluator, rule
engine, bus, policy file, or collector: the name coverage-truthfulness
appearing in any decision-plane source is a gate failure. Machine
assertions for this slice live in the Node check and its selftest
controls, run over the real tree by the discovery package tests.
