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
one template, three fully instantiated schemas, four honestly pending
slots.

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
slots carry the full contract in section 3, 4, or 5 of this file;
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
status: pending
planned_slice: W2.3
```

```schemav2
schema: recovery
status: pending
planned_slice: W2.3
```

```schemav2
schema: evidence
status: pending
planned_slice: W2.4
```

```schemav2
schema: profile
status: pending
planned_slice: W2.4
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

## 6. Pending slots

The four `pending` slots above are not placeholders in prose: each is a
named build-slice obligation, and the wave closes only when all seven
slots read `status: complete` with four substantive element sections each.
Until then this file is honest about what it does not yet contract.
