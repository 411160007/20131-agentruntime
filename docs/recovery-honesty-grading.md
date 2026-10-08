# Recovery Honesty Grading (spec section 247 — outward truthfulness face)

Slice: W12.2 (design + documentation only; zero code change, zero
execution). This is the outward marketing-truthfulness document that the
transaction design coverage table registered as its own follow-up slice
for recovery truthfulness (see docs/recovery-transaction-design.md
section 5, "spec section 247" row, and docs/schema-v2.md section 7).

The four grades below are a **restatement, parsed from the shipped
schema source** (`internal/schema/impactrecovery.go`), not a second
invention. The machine checker re-parses the source and fails on drift.

## 1. The rule this document exists to enforce

The specification states, verbatim:

> 20131 不得宣传“所有动作都可以撤销”。

Translation of the normative sentence: the product must never advertise
that all actions can be undone. Network sends, third-party API
submissions, external messages, and some payments or third-party state
changes may well be `non_reversible` or need `external_compensation`.
The four-grade scale exists precisely because honesty is the point —
a grading that could be painted as "everything is undoable" would be a
grading with no information in it.

## 2. The four honest grades (closed set, normative order)

Display spellings exactly as the specification states them:

```recovery-class-display
LOCAL REVERSIBLE
LOCAL PARTIAL
EXTERNAL COMPENSATION
NON-REVERSIBLE
```

Wire tokens (mechanically derived: lower-cased, spaces and hyphens
replaced by single underscores, in order — same rule as the schema
contract):

| wire token | meaning (shipped source comment parity) |
|---|---|
| local_reversible | undone locally by this product alone |
| local_partial | undone locally only in part |
| external_compensation | needs a compensating external action |
| non_reversible | cannot be undone (network sends, third-party state changes, payments) |

## 3. Per-face grading (closed table; every row names exactly one grade)

Each row grades one observable face of what an agent might do while this
product watches and records. The grade says how far that face could
honestly be walked back **in principle**; it does **not** promise any
undoing machinery, because none ships today (section 6).

| face | grade | honest basis |
|---|---|---|
| Local file edit whose prior content is fully retained on local disk (content-hash copy) | local_reversible | The byte-for-byte predecessor exists locally; walking back needs no outside cooperation. Design intent only — no execution plane ships (section 6). |
| Append-only audit trail entries (JSONL records already written) | local_partial | Future entries can be rotated or truncated by policy, but records already written cannot be un-written without breaking the integrity chain that makes them trustworthy. |
| Outbound bytes already sent over the network (API calls, forwarded tool invocations) | non_reversible | Once bytes leave the machine, the remote side may have processed them; no local action can recall them. |
| A publish to a third-party registry (package/version made public) | external_compensation | A compensating public action (deprecate, retraction notice) is possible only through the third party's own mechanism, with a window in which consumers already saw the artifact. |
| A message delivered to a human (email, chat) | non_reversible | The recipient may have read or forwarded it; "unsend" does not exist as a truthfulness-gradeable action. |
| A payment or third-party account state change | non_reversible | Settlement and third-party state changes are owned by outsiders; at best a new compensating transaction follows, which is a different state, not an undo. |

## 4. Forbidden marketing clauses (closed list)

No outward copy (site, README, docs, blog, release notes, pitch) may
assert the following claims or their variants. The checker parses this
fenced block as the machine-readable prohibition list:

```forbidden-marketing-lines
all actions can be undone
every action is reversible
fully reversible
can undo anything
一键恢复任何操作
所有动作都可以撤销
全量可撤销
一键完全回滚
```

Rules attached to the list:

- The list is *additive-proof*: widening it later is a documentation
  change; removing an entry is a specification-level decision, not a
  marketing decision.
- Quoting a forbidden line inside a prohibition context (this document,
  the checker fixtures, the threat-model word-family scan) is legal and
  checked as such; standalone assertions are not.

## 5. Class-qualified claims rule

Any outward statement about recovery must be **class-qualified**: it
must name exactly one of the four wire tokens from section 2 and the
face it applies to (for example "audit records are graded
local_partial"), and must not read like an execution promise.
Unclassified recovery talk is treated as a violation of the
`never-claim-fully-reversible` rule, same as the forbidden lines.

## 6. Execution-plane honesty (declaration plane only)

- `recovery_execution_plane: none-in-observation-phase` — restated from
  the shipped source constant; no code in this repository undoes,
  partially restores, or compensates anything today.
- `recovery_truthfulness_rule: never-claim-fully-reversible` — the
  recorded promise lifted from the specification.
- Grades in this document classify recorded side effects; they are not
  a feature list. Recovery execution remains a later-phase surface.

## 7. Deferred decision, registered verbatim

观测型 recovery point 实装与否 = 另裁位保持。
(Whether to implement the observation-type recovery point remains with
the separate decision slot; this slice neither implements it nor
pre-empts that decision. The ordering rationale registered in the
transaction design — that the "not this cycle" reason was never
overturned, only re-sequenced — stands unchanged.)

## 8. Cross-references

- docs/schema-v2.md section 7 — the machine-readable four-class contract
  (sync tests pin source/docs verbatim parity).
- docs/recovery-transaction-design.md — transaction pipeline, task-level
  change-set census, recovery fabric levels; its coverage table defers
  the outward truthfulness face to this document.
- scripts/w12-grading-check.mjs — this document's machine checker:
  re-parses the grades from source, censuses the per-face table, and
  runs the forbidden-marketing word-family scan across the outward
  corpus.
