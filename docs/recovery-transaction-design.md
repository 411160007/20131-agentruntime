# Recovery / Transaction Design Surface v0 — design only, no execution plane

Version lock: this document and `scripts/w12-recovery-check.mjs` MUST evolve
in one PR; the checker re-parses the shipped recovery vocabulary from
`internal/schema/impactrecovery.go` and the shipped correlation family from
`internal/schema/decisiontrace.go` on every run — never against this file
alone.

Stance (Phase 0, inherited verbatim from the shipped recovery schema):
this is a **design surface**. Declaration records only. **No recovery
execution plane exists behind it** — code that actually undoes, partially
undoes, or compensates side effects is not shipped and not scheduled in
Phase 0. Recovery execution is gated to the enforcement phases by the
three-stage charter (owner ruling: Phase 0 = observe, never enforce;
snapshot-rollback machinery belongs to the final stage).

## 0. Honest registration of the "not this cycle" decision

The v1.0 first-phase table registered Recovery as deferred, verbatim:

> 本期不排（登记）｜理由：Recovery Core 语义=对已执行动作的恢复/回滚，服务强制层；
> Phase 0 观察不拦截=无已执行强制动作可恢复；归属=〔内部决策编号，产品面省略〕Phase 2（Linux 强制+快照回滚）。
> 预登记不悬空：决策/事件枚举的演进路径=加性（schema_version 机制已备），
> Phase 2 加 `recovery` 决策值不破坏 v1 消费端。

That reason has **not been overturned** — nothing shipped since adds an
enforcement plane, so there are still no enforced actions to recover from.
The deferral is only re-sequenced: the V2 master directive (spec sections
246–250) asks for the *design* of the transaction/recovery contract while
execution stays deferred. This document is that design surface. The actual
recovery machinery remains ⏸ Phase 1/2.

## 1. Transaction Runtime (spec section 246) — step pipeline, observed

Recovery is more than "backup + rollback". The canonical flow, as a closed
step set that future transaction records will enumerate. Step names are
the specification's closed seven (rendered case-normalized: the repo
hygiene scanner reserves the all-caps form of the middle token as a
decision-chain word; the checker enforces the census case-insensitively):

```text
Start
↓
Plan
↓
Recovery Point
↓
Authorize
↓
Action
↓
Verify
↓
Commit
```

On failure the flow enters:

```text
Rollback
```

Invariant carried verbatim from the specification: only a result that
passed Verify may enter Commit. In Phase 0 this invariant is a **design
rule asserted against record shapes**, not an executed gate — no step is
enforced, no action is rolled back.

Proposed record shape (additive schema evolution path, not yet wire fields):

| field | meaning | Phase 0 posture |
|---|---|---|
| `transaction_id` | groups every event/decision caused by one flow | design only, known_gap |
| `tx_step` | one of the closed seven steps above | design only, known_gap |
| `verify_outcome` | pass / fail / not-verified | design only, known_gap |
| `rollback_path` | which recovery class the rollback would need | design only, known_gap |
## 2. Transaction id ↔ audit correlation — design against shipped anchors

The audit stack already pins a correlation family: every decision trace
pins its origin event id (see `internal/schema/decisiontrace.go`, and the
evidence profile note that transaction-level grouping reuses the
recovery record shape per wave — `internal/schema/evidenceprofile.go`).
The transaction id is therefore **one grouping level above** the existing
origin-event correlation, not a replacement:

- events/decisions keep their origin-event correlation (shipped);
- a future additive `transaction_id` on those lines joins them into one
  flow; the evolution path is the `schema_version` additive mechanism, so
  v1 consumers are never broken;
- the evidence bundle already reserves the member `recovery.json` with an
  honest absent stance — the shipped verbatim token
  "recovery-state export waits for its upstream field"
  (`internal/schema/evidenceexport.go`); the transaction design names that
  upstream field set, it does not fake it.

## 3. Task-Level Recovery (spec section 248)

Multiple changes caused by one task must be associated, as far as
possible, with the same transaction. Canonical example chain from the
specification:

```text
Code Change → NPM Install → Docker Build → Config Change → Nginx Change → Deploy
```

On failure the system should recover the locally reversible changes of
the **whole task**, not just the last file. Phase 0 posture: the design
commits only to a **change-set census** — every recorded change inside one
transaction lists the targets it touched, so a later execution plane has
a complete, honest worklist. Executing that worklist is ⏸ Phase 1/2.

## 4. Recovery Fabric (spec section 249) — mechanisms and honest levels

The specification names six fabric mechanisms:

Copy-on-Write, Incremental Snapshot, Content Hash, Deduplication,
Versioning, Metadata.

Only Content Hash has shipped precedent (audit writer integrity hashing,
evidence bundle hashes); the other five are structurally absent in
Phase 0 — declared here as design rows with known_gap, never as claims.

Recovery levels, closed set (five, per spec section 249):

```text
Temporary
Normal
High-Value
Critical
System
```

Recovery storage must be governed by an **independent quota**. Design
note: the storage-governance wave already shipped per-agent/per-task
quota observation for audit storage (docs/schema-v2.md + storage CLI
surface); recovery storage gets
its own quota namespace when it exists — separate from the event log,
so recovery data can never starve evidence retention (or vice versa).

## 5. Clause coverage table (spec sections 246–250)

| clause | subject | disposition in this design surface |
|---|---|---|
| spec section 246 | Transaction Runtime | covered (§1: closed step set + verify/commit invariant as design rule) |
| spec section 247 | Recovery Truthfulness | deferred: the four-class closed vocabulary is already shipped in the recovery schema (docs/schema-v2.md section 7) — local_reversible, local_partial, external_compensation, non_reversible; the outward marketing-truthfulness document is its own follow-up slice |
| spec section 248 | Task-Level Recovery | covered (§3: change-set census design) |
| spec section 249 | Recovery Fabric | covered (§4: six mechanisms, five levels, independent quota as design rows) |
| spec section 250 | Security Time Machine | registered: the causal chain User→Intent→Authority→Agent→Plan→Action→Capability→Resource→Policy→Risk→Decision→Outcome→Recovery maps onto shipped record families (events, decisions, traces, profiles); historical/decision/policy replay build on the existing read-only CLI family (status, audit-tail, timeline, report, evidence); Policy Diff and full Incident Reconstruction are registered design rows, not shipped |

## 6. Honesty constraints (verbatim-bound)

- The specification forbids, verbatim, marketing that "所有动作都可以撤销"
  (every action can be undone) — 20131 must never advertise that.
  Network sends, third-party API submissions, external messages, partial
  payments and third-party state changes may belong to
  non_reversible / external_compensation.
- This document ships **zero execution guarantees**. Every table row
  marked known_gap or "design only" means the plane does not exist.
- Phase 0 red line, restated from the shipped schema stance: declaration
  plane only; no recovery or rollback execution plane exists behind any
  record shape named here; the decision value set remains
  {allow, would_block} and nothing in this design adds to it.
