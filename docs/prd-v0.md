# PRD v0 — Product Requirements Document, register skeleton, mechanically derived (documentation only)

Version lock: this is a **documentation slice**. It ships next to
`docs/release-plan-v0.md` (the release/canary/rollback pre-plans) and
changes no machine: no script, no schema, no runtime path, no decision
vocabulary. If a future slice implements any machinery referenced here,
that slice — not this document — owns the new gate.

Status declaration (read this first): everything below is a **register
skeleton — not implemented**. The rows are mechanically exported from
exactly three sections of the V2 master directive (dated 2026-09-28): §298 (V2.0
priority override), §221 items 15/16 (the deliverable slots this file
and `docs/ux-spec-v0.md` land), and §307 (final acceptance criteria).
**Zero new semantics**: no requirement appears here that is not present
in those sources; no later phase is pre-spent; no external marketing
language is carried.

## 1. Derivation method (auditable)

- Cell/clause text is quoted **verbatim** from the source sections — nothing paraphrased into new meaning.
- The "register" column is machine-derived, not hand-written. Probe command (as of this slice):
  `grep -ri -e "<probe>" --exclude=prd-v0.md --exclude=ux-spec-v0.md docs README.md | wc -l` — a case-insensitive line count over the repository documentation surface (`docs/` + `README.md`), excluding these two register files themselves so no probe ever counts its own rows.
- Register values use exactly two honest states:
  - `in-repo mention lines: N` (N > 0, computed) — the concept is discussed somewhere in-repo. This does **not** mean implemented.
  - `honest zero (gap)` (N = 0) — nothing in-repo mentions it yet.
- Zero-silence rule: every source cell/row gets exactly one register row; nothing skipped, merged, or left blank.
- Expansion rule for future revisions: every new or changed requirement line must cite its source section id (the V2 master directive §N). Uncited lines are not allowed in.
- Numbers are snapshots: future revisions must re-run the declared probe instead of hand-editing counts.

## 2. Priority-layer requirements matrix (§298, 57 cells)

Source: §298 keeps the older phases but overrides current execution
priority as P0–P6. The product requirement hierarchy is therefore
exactly the tables below — nothing added, nothing dropped
(57 cells).

### P0 — Security Foundation

| Cell (verbatim) | Register |
|---|---|
| Core API | in-repo mention lines: 3 |
| Event Schema | in-repo mention lines: 3 |
| Identity | in-repo mention lines: 29 |
| Capability | in-repo mention lines: 51 |
| Policy | in-repo mention lines: 117 |
| Hard Deny | in-repo mention lines: 6 |
| Pre-Action | in-repo mention lines: 3 |
| Native Enforcement | in-repo mention lines: 2 |
| Recovery Boundary | in-repo mention lines: 1 |
| Anti-Tamper | in-repo mention lines: 6 |

### P1 — Agent Understanding

| Cell (verbatim) | Register |
|---|---|
| Intent Contract | in-repo mention lines: 3 |
| Plan Analysis | in-repo mention lines: 1 |
| Intent Alignment | in-repo mention lines: 2 |
| Action Necessity | in-repo mention lines: 3 |
| Contextual Trust | honest zero (gap) |
| Process Lineage | honest zero (gap) |
| Behavior Chain | in-repo mention lines: 3 |
| Impact | in-repo mention lines: 65 |
| Blast Radius | in-repo mention lines: 4 |

### P2 — Adaptive Security

| Cell (verbatim) | Register |
|---|---|
| Adaptive Learning | in-repo mention lines: 2 |
| External Intelligence Adapter | honest zero (gap) |
| Shadow Policy | honest zero (gap) |
| Policy Replay | in-repo mention lines: 1 |
| Policy Simulator | in-repo mention lines: 2 |
| Learning Quarantine | in-repo mention lines: 1 |
| Trust Freeze | in-repo mention lines: 2 |
| Decision Cache | in-repo mention lines: 2 |

### P3 — Recovery & Evidence

| Cell (verbatim) | Register |
|---|---|
| Transaction Runtime | in-repo mention lines: 2 |
| Task-Level Recovery | in-repo mention lines: 4 |
| Security Time Machine | in-repo mention lines: 1 |
| Decision Trace | in-repo mention lines: 5 |
| Enterprise Evidence System | in-repo mention lines: 1 |
| Policy Diff | in-repo mention lines: 2 |

### P4 — Data / Multi-Agent / Supply Chain

| Cell (verbatim) | Register |
|---|---|
| Data Boundary | in-repo mention lines: 4 |
| Data Egress | honest zero (gap) |
| Memory Security | in-repo mention lines: 2 |
| Prompt Injection | in-repo mention lines: 5 |
| Goal Hijacking | in-repo mention lines: 5 |
| Agent-to-Agent | in-repo mention lines: 3 |
| Supply Chain | in-repo mention lines: 3 |
| Cost Guard | in-repo mention lines: 4 |

### P5 — Platform / Product

| Cell (verbatim) | Register |
|---|---|
| Windows | in-repo mention lines: 27 |
| macOS | in-repo mention lines: 9 |
| Linux | in-repo mention lines: 19 |
| HarmonyOS | in-repo mention lines: 4 |
| UX | in-repo mention lines: 21 |
| Compatibility Safe Mode | honest zero (gap) |
| Max Autonomy | honest zero (gap) |
| Emergency Lock | honest zero (gap) |

### P6 — Commercial / Cloud / Ecosystem

| Cell (verbatim) | Register |
|---|---|
| Website | in-repo mention lines: 1 |
| 19 Languages | honest zero (gap) |
| Docs | in-repo mention lines: 73 |
| Billing | in-repo mention lines: 2 |
| Cloud Control Plane | honest zero (gap) |
| Enterprise | in-repo mention lines: 10 |
| SDK | honest zero (gap) |
| Marketplace | honest zero (gap) |

## 3. Acceptance requirements (§307, 15 clauses)

Source: §307 — before a Beta/Commercial release, these fifteen must be
proven. Each clause is a product requirement in acceptance form. The
probe term in column three is taken verbatim from the clause text
itself.

| # | Clause (verbatim) | Probe | Register |
|---|---|---|---|
| 1 | Unknown Agent 不会因未知身份而被粗暴全部阻断。 | `unknown` | in-repo mention lines: 29 |
| 2 | 高风险敏感行为能够尽可能事前控制。 | `pre-action` | in-repo mention lines: 3 |
| 3 | Agent 换 Tool / Child Process / MCP 不能轻易绕过 Capability。 | `bypass` | in-repo mention lines: 10 |
| 4 | User Hard Deny 无法被 Agent / LLM / Learning 覆盖。 | `hard deny` | in-repo mention lines: 6 |
| 5 | READ 不会自动等于 EXPORT。 | `export` | in-repo mention lines: 61 |
| 6 | Prompt Injection 后 Native Boundary 仍然有效。 | `prompt injection` | in-repo mention lines: 5 |
| 7 | Agent-to-Agent 权限不能自动继承。 | `inherit` | in-repo mention lines: 28 |
| 8 | Recovery 对本地可恢复变化有效。 | `recovery` | in-repo mention lines: 129 |
| 9 | 不可逆外部效果被准确标记为不可撤销或需补偿。 | `irreversible` | honest zero (gap) |
| 10 | 日志与 Recovery 不会无限增长。 | `unbounded` | honest zero (gap) |
| 11 | Cloud / LLM 失效不会关闭核心安全。 | `offline` | in-repo mention lines: 1 |
| 12 | UI / Analytics / Behavior AI 故障不会关闭 Security Core。 | `degrade` | in-repo mention lines: 8 |
| 13 | 正常 Agent Golden Tasks 仍保持高成功率。 | `golden` | in-repo mention lines: 37 |
| 14 | 关键安全事件能产生可复现 Regression Test。 | `regression` | in-repo mention lines: 7 |
| 15 | Enterprise 能导出完整 Decision / Evidence。 | `evidence` | in-repo mention lines: 164 |

## 4. Deliverable placement (self-reference)

- §221 item 15 "Product Requirements Document" → this file
  (`docs/prd-v0.md`), register form.
- §221 item 16 "UX Specification" → `docs/ux-spec-v0.md`, register
  form (its own derivation, same rules).
- Product definition and principles are **not restated here**. See
  the V2 master directive §308/§309/§310 as the single authority; copying
  positioning language into repo docs would create a second source of
  truth, and this slice refuses to do that.
- Adjacent in-repo design docs are pointers, not requirements:
  `docs/architecture.md`, `docs/api-v0.md`, `docs/schema-v2.md`,
  `docs/threat-model.md`, `docs/evals-coverage-v0.md`,
  `docs/release-plan-v0.md`, `docs/distribution.md`,
  `docs/task-primitive-design.md`.

## 5. Non-goals (inherited, re-asserted)

- **No shipped-state claims.** Register rows describe discussion
  coverage in documentation, not shipping status. The shipped product
  surface remains exactly what `docs/distribution.md` and the
  repository gates describe today.
- **No Phase 1+ pre-spending.** Billing, Cloud Control Plane,
  Marketplace (P6 cells) are registered as hierarchy rows only; this
  document commits to none of them, per the §298 ordering they are
  last by construction.
- **Decision vocabulary unchanged.** The decision values in this
  product remain the closed observation set {allow, would_block};
  nothing here adds, renames, or implies enforcement verdicts.
- **Local-first / no telemetry.** No requirement row in this file
  licenses a network dependency in the core path or any usage
  reporting; these ground rules are inherited from
  `docs/distribution.md` and `docs/release-plan-v0.md`.

## 6. Known gaps (honest zero list)

- Requirement bodies (user-facing why/what statements per cell) are
  not written in v0. v0 turns these slots from "missing document" into
  "registered, sourced, auditable" — the bodies are follow-up work for
  source-cited revisions.
- Dependencies between the 57 cells are not asserted; that inference
  would be new semantics.
- Ordering inside a layer is not asserted; §298 lists each layer as a
  set, and this file keeps them as sets.
- Measurement/definition-of-done per cell is deferred to the §307
  acceptance rows, which themselves reference pending verification
  surfaces (see `docs/evals-coverage-v0.md`).
