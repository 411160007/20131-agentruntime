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
