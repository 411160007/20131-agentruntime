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
one template, one fully instantiated schema, six honestly pending slots.

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
slots carry the full contract in section 3 or 4 of this file; `pending`
slots carry a pointer to the build slice that will fill them.

```schemav2
schema: decision
status: complete
```

```schemav2
schema: intent
status: pending
planned_slice: W2.2
```

```schemav2
schema: authority
status: pending
planned_slice: W2.2
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

## 4. Pending slots

The six `pending` slots above are not placeholders in prose: each is a
named build-slice obligation, and the wave closes only when all seven
slots read `status: complete` with four substantive element sections each.
Until then this file is honest about what it does not yet contract.
