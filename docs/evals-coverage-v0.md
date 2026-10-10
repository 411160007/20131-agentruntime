# Evaluation Coverage Map v0 — security classes × normal task classes against the golden corpus

Version lock: this table, the built-in rule vocabulary (internal/rules/rules.go) and testdata/golden/labels.json MUST evolve in one PR; gate-w0 re-counts corpus, labels, map and docs table from independent sources and fails on any drift.

Scope: the evaluation coverage contract for the shipped golden corpus
(`testdata/golden/normal.jsonl` 76 + `danger.jsonl` 64 = 140 cases,
ten evaluation dimensions, labels in `labels.json`). This document and
`testdata/golden/coverage-map.json` are the two required forms (human table +
machine-readable) and MUST stay in lockstep; `scripts/gate-w0.sh` re-counts
both against the corpus and the built-in rule table and fails on drift.

Honesty rules of this map:

- A pending class does NOT invalidate any shipped case: all previously shipped golden cases keep their current labels and thresholds unchanged.
- Pending rows name the primitive rules (and their real matching cases) that catch parts of the class today, plus the build-order slices that carry the missing semantics.
- Every detection here is a would_block record on the audit stream; Phase 0 observes, warns and audits — it never stops anything (see threat-model.md).

## Security class coverage (sixteen classes)

| class | status | built-in rule(s) | golden cases (direct / via primitives) | semantics carry point |
|---|---|---|---|---|
| Prompt Injection | semantics pending | — (primitives: exec.remotepipe, cred.dotenv, mcp.eval, net.egress, destroy.rmrf) | 0 / 5 | W1 + W3 |
| Goal Hijacking | semantics pending | — (primitives: cred.ssh) | 0 / 1 | W3 + W5 |
| Tool Abuse | covered | mcp.eval, exec.remotepipe | 13 / — | in service |
| MCP Abuse | covered | mcp.eval | 8 / — | in service |
| Skill Abuse | semantics pending | — (primitives: mcp.eval) | 0 / 7 | W1.2 + W4 |
| Credential Access | covered | cred.ssh, cred.aws, cred.browser, cred.dotenv | 19 / — | in service |
| Privilege Escalation | covered | path.sudoers | 5 / — | in service |
| Memory Poisoning | semantics pending | — | 0 / 0 | W2 + W12 |
| Agent-to-Agent Abuse | semantics pending | — | 0 / 0 | W5/W6 carrier slice + W6 |
| Supply Chain Attack | semantics pending | — (primitives: exec.remotepipe, cred.ssh) | 0 / 5 | W11 + W1.2 |
| Mass Deletion | covered | destroy.rmrf, destroy.disk | 11 / — | in service |
| Policy Bypass | covered | audit.tamper | 7 / — | in service |
| Capability Escalation | semantics pending | — (primitives: agent.masquerade, path.sudoers) | 0 / 8 | W6 + W4 |
| Data Exfiltration | semantics pending | — (primitives: cred.ssh, cred.aws, cred.browser, cred.dotenv, destroy.rmrf, destroy.disk, exec.remotepipe) | 0 / 17 | W4 + W7 |
| Network Exfiltration | semantics pending | — (primitives: net.egress) | 0 / 4 | W4 + W10 |
| Child Process Pivot | covered | agent.masquerade | 5 / — | in service |

Per-row notes:

- **Prompt Injection** — No dedicated injection-semantics matcher ships today. The five injection-chain cases are caught downstream by primitive rules (piped installer, dotenv read, eval tool, egress intent, rm -rf), which is exactly the Phase 0 honest shape: injection is observed through its effects, not through intent-reading.
- **Goal Hijacking** — One corpus case (task drift into an agent ssh key) is caught downstream by credential primitives; goal-semantics detection awaits the intent slices.
- **Tool Abuse** — Eval-exposing tools and shell-piped command shapes on the hook/mcp surfaces.
- **MCP Abuse** — Covered shape = tool-exposing-raw-eval (threat-model TM-05) via the mcp reserved slot. Deeper metadata-plane abuse (tool description poisoning, confused-deputy delegation) is graded later: W1.2 assigns Tool/MCP collection tier, policy semantics later phases.
- **Skill Abuse** — No skill-vocabulary matcher ships; eval-named tools are the closest primitive today.
- **Credential Access** — Four hard-priority credential-store rules plus the dotenv suffix rule; danger-side class detection 19/19 in the runner table.
- **Privilege Escalation** — Privilege configuration drop-ins (sudoers, sudoers.d variants, macOS prefixes). The sudo-prefixed destructive case lands in Mass Deletion; full token/session escalation semantics are later-phase scope (see threat-model known_gap wording on OS-integrity surfaces).
- **Memory Poisoning** — Nothing in today's corpus demonstrates this class; no existing case is invalidated — the class is simply not yet observable by shipped vocabulary.
- **Agent-to-Agent Abuse** — Delegation shapes exist on the normal side of the corpus (delegated subtask session, webhook-bootstrapped agent detected as allow); no abuse-side golden case ships yet.
- **Supply Chain Attack** — Fetch-and-run piping is caught today as a primitive (remote script straight into a shell); poisoned-dependency and package-integrity semantics are not observed by shipped rules.
- **Mass Deletion** — Recursive force deletes and filesystem reformat shapes, incl. localized and piped variants.
- **Policy Bypass** — Covered shape = attack on the audit/policy sink itself (TM-08) plus the merge-surface hard-rule no-downgrade gate (three negative-control fixtures in the runner battery). The winevt-clearing case gd-20 stays a declared known_gap (TM-12), honestly counted as a miss in the ledger, never silently passed.
- **Capability Escalation** — Sudoers writes and world-writable masquerade runs are caught as primitives; the escalation-as-trajectory semantics (capability growth over time) awaits the least-agency slice.
- **Data Exfiltration** — Credential-store reads (the typical exfiltration source step) are caught as primitives; payload-crossing-boundary semantics need the data-action vocabulary.
- **Network Exfiltration** — Outbound network intent is flagged as a class today (all four network-shaped dangers detected 4/4) but as generic egress visibility, not as confirmed data leaving with a payload; the egress semantics arrive with the boundary slice.
- **Child Process Pivot** — Covered shape = relabel/masquerade execution from world-writable directories (TM-07, incl. the miner-disguised-as-agent variant) plus proc.spawn capability tagging on piped-exec rules. Full parent-child chain re-adjudication (pivot-through-children as trajectory) is graded later with the behavior-chain upgrade.

Pending-class carry points, in full:

- Prompt Injection:
  - W1 — evidence trust order: content of untrusted provenance gets a declared source_class (low tier), never decides alone (machine-judged trust battery already shipped in the runner)
  - W3 — intent contract: intent mutations sourced from untrusted content are recorded as UNTRUSTED-marked observations (observe-only in Phase 0)
- Goal Hijacking:
  - W3 — intent contract v0 with the four-way action taxonomy (DIRECT/INFERRED/UNCERTAIN/UNRELATED) records task-goal drift
  - W5 — behavior-chain Intent Deviation dimension upgrades drift from narrative to structured observation
- Skill Abuse:
  - W1.2 — collection-mount grading: tool/skill invocations enter as Tool/MCP metadata tier (self-described fields never raise the tier)
  - W4 — nine data-action vocabulary re-adjudicates EXECUTE/SHARE per action, independent of skill naming
- Memory Poisoning:
  - W2 — persistence/profile schema family (section 289 contracts) creates the typed surface a poisoning observation needs
  - W12 — memory-security and learning-safety design slice (sections 262-265) defines detection semantics before any enforcement claim
- Agent-to-Agent Abuse:
  - W5/W6 carrier slice — delegation becomes a first-class observation record (parent/child/reason/scope/TTL, observe-only): delegation never inherits trust automatically until enforcement lands in later phases
  - W6 — least-agency step-count and parallelism counters record fan-out patterns typical of A2A abuse
- Supply Chain Attack:
  - W11 — supply-chain observation v0 (documentation surface + coverage truthfulness matrix) defines what is and is not seen
  - W1.2 — fetch-and-run events grade by collection mount, keeping remote-payload provenance in the record
- Capability Escalation:
  - W6 — least-agency guard records capability growth per session/step (rate, steps, parallelism counters; observation only, no kill-switch in Phase 0)
  - W4 — capability vocabulary migrates from the shipped 11-word set toward the nine data-action classes, making escalation diffs machine-readable
- Data Exfiltration:
  - W4 — data-boundary engine: READ-allowed never implies EXPORT-allowed; EXPORT is an independently re-adjudicated action class
  - W7 — evidence export envelope (manifest + hashes) keeps exfiltration observations themselves complete and tamper-evident
- Network Exfiltration:
  - W4 — trust-domain mapping for net.outbound: egress intent events grade against the nine-class action vocabulary (EXPORT/SHARE)
  - W10 — external-intelligence contract (zero-wired by default) could later enrich destination reputation without ever gating the deterministic core

## Normal task class coverage (twelve classes, 75 allow-side cases)

| class | cases | golden case ids |
|---|---|---|
| Coding | 5 | g7-c01, g7-f01, g7-f04, g7-p05, g7-u02 |
| Research | 3 | g7-c06, g7-f07, g7-q02 |
| Browsing | 2 | g7-u07, g7-x02 |
| Git | 4 | g7-f03, g7-u05, g7-x04, g8-n02 |
| NPM | 3 | g7-b02, g7-l04, g7-u04 |
| Docker | 2 | g7-f05, g8-n01 |
| Database | 0 | — (honest zero, see note) |
| Deployment | 1 | g7-u01 |
| Automation | 5 | g7-b01, g7-b07, g7-c03, g7-q01, g7-x01 |
| File Management | 12 | g7-c02, g7-c05, g7-l01, g7-l02, g7-l03, g7-l05, g7-r04, g7-u03, g7-u06, g7-u08, g7-u09, g7-x03 |
| Testing | 3 | g7-b05, g7-f02, g8-n03 |
| Multi-Step Tasks | 3 | g7-b03, g7-b06, g7-b08 |
| Observation infrastructure (residual bucket, not a task class) | 33 | (collector lifecycle + discovery lines plus the typed intent hint carried inside the scripted session; full ids in coverage-map.json) |

Classification is by the task shape the event demonstrates (command line,
path or tool attr), not by the narrative summary — summaries are agent
self-description and the shipped trust-order assertions keep them off the
decision surface entirely.

Per-class notes:

- **NPM** — Package and dependency operations; pip/brew/apt package installs are recorded under this class (same task shape, the spec names the ecosystem flagship).
- **Database** — Honest zero: the shipped normal-side corpus (dogfood harvest + expanded benign work vocabulary) carries no database-task event shape today. Forcing one would fabricate coverage; the fill path is the dogfood collector harvest plus W-series collection expansion, not synthetic invention.
- **Multi-Step Tasks** — Delegated-step shapes (step cache, summary output, delegated subtask session).
- **Deployment** — Local dev-server bring-up shape; the docker compose stack start is recorded under Docker (its named tool).

## Threshold ledger (v1.1 ledger form, W0.2)

Machine form: `testdata/golden/thresholds.json`. The four pinned readings
below were parsed from `go test ./internal/rules -run TestGoldenEvals`
output by the build slice that produced this ledger (never hand-copied);
gate-w0 re-runs the eval suite and cross-checks every value.

| reading | pinned value | threshold |
|---|---|---|
| detection | 62/64 (96.9%) | >= 80% |
| worst per-class false positive | 0.0000 | <= 0.05 |
| credential-class false positive | 0.0000 | <= 0.02 |
| disturbance (alerts on normal cases) | 0/75 | <= 1 |

Dimension floors: every one of the ten dimensions carries >= 8 real cases
(current counts: agent_behavior 10, compatibility 8, functional 28, localization 8, performance 8, security_regression 12, recovery 8, security 37, ux 10, untrusted 10); zero empty shells.

Known-gap carry: **gd-19, gd-20** are two deliberately
undetected shapes kept as declared blind spots (local resource exhaustion —
threat model TM-11; OS-native log clearing outside our audit file — TM-12).
They count against detection honestly; they are never presented as covered.
