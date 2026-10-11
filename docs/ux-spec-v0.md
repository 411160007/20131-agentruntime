# UX Specification v0 — register skeleton, mechanically derived (documentation only)

Version lock: this is a **documentation slice**. It pairs with
`docs/prd-v0.md` and changes no machine: no script, no schema, no
runtime path, no decision vocabulary.

Status declaration (read this first): **register skeleton — not
implemented**. Sources are exactly §221 item 16, §298 P5 (the platform
cells, including the "UX" cell itself), and §307 (the acceptance
clauses a user must be able to verify). **Zero new semantics**: no
screens, flows, journeys, typography, copy decks, or visual language
are invented here — inventing them without source citations would be
exactly the failure this v0 refuses to commit.

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

## 2. Binding acceptance contract (§307, 15 clauses)

For the UX layer, each §307 clause binds as an *outcome the user must
be able to verify*. This table registers the obligation without
designing the surface. The UX-surface column is a constant by design:
all fifteen are unknowns in v0, and claiming otherwise would be
invention.

| # | Clause (verbatim) | Probe | Register (in-repo discussion) | UX surface |
|---|---|---|---|---|
| 1 | Unknown Agent 不会因未知身份而被粗暴全部阻断。 | `unknown` | in-repo mention lines: 29 | unspecified (known_gap) |
| 2 | 高风险敏感行为能够尽可能事前控制。 | `pre-action` | in-repo mention lines: 3 | unspecified (known_gap) |
| 3 | Agent 换 Tool / Child Process / MCP 不能轻易绕过 Capability。 | `bypass` | in-repo mention lines: 10 | unspecified (known_gap) |
| 4 | User Hard Deny 无法被 Agent / LLM / Learning 覆盖。 | `hard deny` | in-repo mention lines: 6 | unspecified (known_gap) |
| 5 | READ 不会自动等于 EXPORT。 | `export` | in-repo mention lines: 61 | unspecified (known_gap) |
| 6 | Prompt Injection 后 Native Boundary 仍然有效。 | `prompt injection` | in-repo mention lines: 5 | unspecified (known_gap) |
| 7 | Agent-to-Agent 权限不能自动继承。 | `inherit` | in-repo mention lines: 28 | unspecified (known_gap) |
| 8 | Recovery 对本地可恢复变化有效。 | `recovery` | in-repo mention lines: 129 | unspecified (known_gap) |
| 9 | 不可逆外部效果被准确标记为不可撤销或需补偿。 | `irreversible` | honest zero (gap) | unspecified (known_gap) |
| 10 | 日志与 Recovery 不会无限增长。 | `unbounded` | honest zero (gap) | unspecified (known_gap) |
| 11 | Cloud / LLM 失效不会关闭核心安全。 | `offline` | in-repo mention lines: 1 | unspecified (known_gap) |
| 12 | UI / Analytics / Behavior AI 故障不会关闭 Security Core。 | `degrade` | in-repo mention lines: 8 | unspecified (known_gap) |
| 13 | 正常 Agent Golden Tasks 仍保持高成功率。 | `golden` | in-repo mention lines: 37 | unspecified (known_gap) |
| 14 | 关键安全事件能产生可复现 Regression Test。 | `regression` | in-repo mention lines: 7 | unspecified (known_gap) |
| 15 | Enterprise 能导出完整 Decision / Evidence。 | `evidence` | in-repo mention lines: 164 | unspecified (known_gap) |

## 3. Platform and autonomy cells (§298 P5, 8 cells)

Source: §298 "P5 — Platform / Product". These are the cells where a
future UX Specification body must land. Register column uses the same
computed probe; the contract column is constant for the same honest
reason as §2 above.

| Cell (verbatim) | Register | UX contract |
|---|---|---|
| Windows | in-repo mention lines: 27 | registered — not designed in v0 |
| macOS | in-repo mention lines: 9 | registered — not designed in v0 |
| Linux | in-repo mention lines: 19 | registered — not designed in v0 |
| HarmonyOS | in-repo mention lines: 4 | registered — not designed in v0 |
| UX | in-repo mention lines: 21 | registered — not designed in v0 |
| Compatibility Safe Mode | honest zero (gap) | registered — not designed in v0 |
| Max Autonomy | honest zero (gap) | registered — not designed in v0 |
| Emergency Lock | honest zero (gap) | registered — not designed in v0 |

## 4. Cross-cutting surface constraints (inherited, not invented)

- **Observation-mode phrasing discipline.** The decision vocabulary
  stays the closed set {allow, would_block}; user-facing wording must
  never claim enforcement that does not exist. This is inherited from
  the shipped state described in `docs/distribution.md` and
  `docs/release-plan-v0.md` §1, not created here.
- **Failure posture.** §307 clauses 11 and 12 bind the UX/analytics
  layer to degrade without taking the security core down with it;
  registered as requirement, no flows designed.
- **i18n dependency.** Every user-visible string inherits the
  19-language architecture requirement (§221 item 17, §298 P6 "19
  Languages"); registered, not built. No copy in this file pretends
  translation coverage exists.

## 5. Known gaps (honest zero list)

- No wireframes, no interaction specs, no copy decks, no usability
  metrics, no accessibility statement: all of these are the body of a
  full UX Specification, and filling them requires source-cited
  revisions under the §1 expansion rule.
- User research inputs: zero in-repo as of this slice; none invented.
- Platform-specific surfaces (P5 cells Windows/macOS/Linux/HarmonyOS)
  carry no platform UX promises here; §298 lists them as priority
  cells, and that is all this file asserts.

## 6. Clause reverse-lookup table (closed set)

| Source row | Lands in |
|---|---|
| §221 item 16 "UX Specification" | this file, register form (§1, §5) |
| §298 P5 (8 cells) | §3 |
| §307 (15 clauses) | §2 (and §4 for the two failure-posture rows) |
| §221 item 15 (sibling slot) | `docs/prd-v0.md` — not this file |

Closed set: every source row above is placed exactly once; no row of
any source section is placed elsewhere or dropped.
