# Security Time Machine v0 — documentation register (spec §250, single-column position)

> Nature: **register-only document slice**. Zero code, zero new state layer, zero
> enforcement. The "time machine" is defined here as the *read-only assembly over
> already-shipped record families* — not a new engine. current company freeze posture
> respected: this document adds nothing that runs.
>
> Owning clause: OWNER-SPEC-V2 §250 "Security Time Machine" (13-link causal
> chain + six supported operations). Gap carried from the V2 gap matrix
> (P3 row: "Time Machine = registered design row, no single-column position")
> and closed by this document. Prior consolidated mapping lives in
> docs/recovery-transaction-design.md (spec section 250 row); this file is the
> dedicated column, so the consolidated row stays untouched as the inherited
> cross-reference.

## 1. Causal chain — link by link (13 links, spec order)

| # | Chain link (verbatim from §250) | Where it is recorded today | Honest status |
|---|---|---|---|
| 1 | User | no dedicated user-principal record family in Phase 0 | known_gap (registered) |
| 2 | Intent | intent contract record family (schema-v2 §4 intent schema; §14 inflow; §12 alignment) | shipped (record form) |
| 3 | Authority | authority schema (schema-v2 W2.2) + UNTRUSTED consumption (§13); enforcement of authority = Phase 1 | shipped (record form) |
| 4 | Agent | identity family: agent_id stable fingerprint, known/observed/trusted tri-state | shipped |
| 5 | Plan | hook-payload plan fields flow into intent records (expected-actions subset); no first-class plan object | partial (known_gap for standalone plan model) |
| 6 | Action | event family (discovery / hook / mcpproxy / integrate) with source_class trust order | shipped |
| 7 | Capability | capability vocabulary on events and rules (res_class bearing 11-entry lexicon + nine data-action verbs) | shipped |
| 8 | Resource | res_class → trust domain mapping (schema-v2 §16 S0-S4 × run domains) | shipped |
| 9 | Policy | decision rows carry rule and policy annotations (hard flags, merge priority); policy_version on trace fields | shipped |
| 10 | Risk | severity / risk-factor fields on decision rows | shipped |
| 11 | Decision | Decision Trace field table (schema-v2 §23, W7.1); decision value closed set {allow, would_block} in Phase 0 | shipped (record form) |
| 12 | Outcome | task record family (task_status closed vocabulary, docs/task-primitive-design.md) + delegation outcome field (schema-v2 §20) | shipped (record form) |
| 13 | Recovery | recovery schema four-grade enum (schema-v2 §7) + docs/recovery-transaction-design.md; actual recovery execution = Phase 1 ⏸ | shipped (design + record form); execution registered, not implemented |

## 2. Supported operations — six rows (spec list, verbatim names)

| Operation (§250 support list) | Shipped substrate in this repo | Honest status |
|---|---|---|
| Historical Replay | internal/replay (W9.1): candidate rules replayed over audit corpus + golden set, byte-identical determinism pinned by gate-w9 | shipped (read-only) |
| Decision Replay | replay corpus evaluates the decision path per event; decision trace fields make per-event verdicts inspectable; report/evidence CLI (schema-v2 §25) exposes them | shipped (read-only assembly) |
| Policy Replay | same replay engine, policy-face verdicts (W9.1/W9.2 three-gate reports) | shipped (read-only) |
| Policy Diff | internal/replay/diff (W9.3): rule-set diff form + promotion record row (observation) | shipped (record form); enforce gating = Phase 1 |
| Recovery History | no execution history exists because no enforcement/recovery executed in Phase 0; recovery design rows (docs/recovery-transaction-design.md; §7 contract) are the substrate it will attach to | registered design row — not implemented |
| Incident Reconstruction | assembly of the six record families through the read-only CLI family (status / audit-tail / timeline / report / evidence) is the v0 shape; a dedicated reconstruction command is a registered design row | partial (CLI assembly shipped; dedicated reconstruction not implemented) |

## 3. Phase 0 posture and fences

- Zero new runtime: this document adds no package, no state store, no binary
  surface. `go diff face` for this slice is docs-only.
- Decision value closed set {allow, would_block} unchanged; Phase 0 stays
  observe-only (the Phase 0 wedge ruling). The time machine never mutates: every read path
  it uses is the existing read-only family.
- Source-trust order applies to reconstruction: agent self-description alone
  never constitutes a high-trust chain link (source_class six-tier, §229
  trust order inherited from W1).
- Honesty grading: recovery-related rows follow docs/recovery-honesty-grading.md
  (no unconditional full-reversibility marketing claim anywhere in this document or its dependents).

## 4. Clause reverse-lookup table (§250 → this document)

Every verbatim item of spec §250 maps to a row above; 13 chain links (§1 rows
1-13) + 6 supported operations (§2 rows 1-6) = 19 clause cells, each filled,
none silently blank.

| §250 item (verbatim) | Cell | Section here |
|---|---|---|
| User | §1 row 1 | causal chain |
| Intent | §1 row 2 | causal chain |
| Authority | §1 row 3 | causal chain |
| Agent | §1 row 4 | causal chain |
| Plan | §1 row 5 | causal chain |
| Action | §1 row 6 | causal chain |
| Capability | §1 row 7 | causal chain |
| Resource | §1 row 8 | causal chain |
| Policy | §1 row 9 | causal chain |
| Risk | §1 row 10 | causal chain |
| Decision | §1 row 11 | causal chain |
| Outcome | §1 row 12 | causal chain |
| Recovery | §1 row 13 | causal chain |
| Historical Replay | §2 row 1 | supported operations |
| Decision Replay | §2 row 2 | supported operations |
| Policy Replay | §2 row 3 | supported operations |
| Policy Diff | §2 row 4 | supported operations |
| Recovery History | §2 row 5 | supported operations |
| Incident Reconstruction | §2 row 6 | supported operations |

## 5. Known gaps (honest zero list)

1. No user-principal record family (chain link 1) — registered known_gap.
2. No first-class plan object (chain link 5) — plan fields ride intent records.
3. Recovery History and full Incident Reconstruction are registered design
   rows, not implemented; enforcement-side recovery = Phase 1 ⏸ (staged by the company decision ledger).
4. Signed evidence bundle (hash+manifest envelope shipped unsigned) remains a
   staged row; time-machine exports inherit that limitation.
