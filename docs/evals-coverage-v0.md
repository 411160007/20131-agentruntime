# Evaluation Coverage Map v0 — security classes × normal task classes against the golden corpus

Version lock: this table, the built-in rule vocabulary (internal/rules/rules.go) and testdata/golden/labels.json MUST evolve in one PR; gate-w0 re-counts corpus, labels, map and docs table from independent sources and fails on any drift.

Scope: the evaluation coverage contract for the shipped golden corpus
(`testdata/golden/normal.jsonl` 80 + `danger.jsonl` 132 = 212 cases,
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
| Prompt Injection | semantics pending | — (primitives: exec.remotepipe, cred.dotenv, mcp.eval, net.egress, destroy.rmrf) | 0 / 18 | W1 + W3 |
| Goal Hijacking | semantics pending | — (primitives: cred.ssh) | 0 / 1 | W3 + W5 |
| Tool Abuse | covered | mcp.eval, exec.remotepipe | 36 / — | in service |
| MCP Abuse | covered | mcp.eval | 19 / — | in service |
| Skill Abuse | semantics pending | — (primitives: mcp.eval) | 0 / 16 | W1.2 + W4 |
| Credential Access | covered | cred.ssh, cred.aws, cred.browser, cred.dotenv | 39 / — | in service |
| Privilege Escalation | covered | path.sudoers | 11 / — | in service |
| Memory Poisoning | semantics pending | — | 0 / 0 | W2 + W12 |
| Agent-to-Agent Abuse | semantics pending | — | 0 / 0 | W5/W6 carrier slice + W6; explicit threat-model counterpart registered (threat-model.md, Agent-to-Agent abuse observation v0) |
| Supply Chain Attack | semantics pending | — (primitives: exec.remotepipe, cred.ssh) | 0 / 18 | W11 + W1.2 |
| Mass Deletion | covered | destroy.rmrf, destroy.disk | 18 / — | in service |
| Policy Bypass | covered | audit.tamper | 7 / — | in service |
| Capability Escalation | semantics pending | — (primitives: agent.masquerade, path.sudoers) | 0 / 20 | W6 + W4 |
| Data Exfiltration | semantics pending | — (primitives: cred.ssh, cred.aws, cred.browser, cred.dotenv, destroy.rmrf, destroy.disk, exec.remotepipe) | 0 / 37 | W4 + W7 |
| Network Exfiltration | semantics pending | egress.export (EXPORT-record judgement) + `net.egress` primitive | 4 / 5 | W4 + W10 |
| Child Process Pivot | covered | agent.masquerade | 11 / — | in service |

Per-row notes:

- **Prompt Injection** — No dedicated injection-semantics matcher ships today. The eighteen injection-chain cases are caught downstream by primitive rules (piped installer, dotenv read, eval tool, egress intent, rm -rf — the five direct attempts gd-25 to gd-29 mirror each shipped primitive effect and the second wave gd-61 to gd-64 mirrors four of the five shipped primitives with fresh injection-chain shapes, and the third wave gd-77 to gd-80 mirrors four more fresh injection-chain shapes across piped bootstrap, dotenv pull, eval-tool call and egress intent), which is exactly the Phase 0 honest shape: injection is observed through its effects, not through intent-reading.
- **Goal Hijacking** — One corpus case (task drift into an agent ssh key) is caught downstream by credential primitives; goal-semantics detection awaits the intent slices.
- **Tool Abuse** — Eval-exposing tools and shell-piped command shapes on the hook/mcp surfaces.
- **MCP Abuse** — Covered shape = tool-exposing-raw-eval (threat-model TM-05) via the mcp reserved slot. Deeper metadata-plane abuse (tool description poisoning, confused-deputy delegation) is graded later: W1.2 assigns Tool/MCP collection tier, policy semantics later phases.
- **Skill Abuse** — No skill-vocabulary matcher ships; eval-named tools are the closest primitive today. The corpus ticket landed: four direct attempts (gd-41 to gd-44) each mirror one shipped primitive effect (a third-party skill exposing a raw eval bridge tool, a skill hook invoking an eval passthrough adapter, an installed skill registering an eval exec helper, a productivity skill routing through an eval rpc router), caught downstream through their eval-exposing tool shapes — no skill-vocabulary matcher ships. The second wave landed: four further direct attempts (gd-53 to gd-56) mirror four more eval-exposing skill shapes (a form handler that evals the template it just wrote, a config poller piping remote config straight into an eval engine, a cron adapter evaluating a remote job-spec expression, a localization skill routing translated placeholders through an eval message formatter), again caught downstream through their eval-exposing tool shapes — still no skill-vocabulary matcher ships.
- **Credential Access** — Four hard-priority credential-store rules plus the dotenv suffix rule; danger-side class detection 31/31 in the runner table.
- **Privilege Escalation** — Privilege configuration drop-ins (sudoers, sudoers.d variants, macOS prefixes). The sudo-prefixed destructive case lands in Mass Deletion; full token/session escalation semantics are later-phase scope (see threat-model known_gap wording on OS-integrity surfaces).
- **Memory Poisoning** — Nothing in today's corpus demonstrates this class as a detection; no existing case is invalidated — the class remains not yet observable by shipped vocabulary. The normal side now carries memory-provenance records on the typed event wire (g7-m01 through g7-m04): a stated record validates as data, matches no shipped rule, and never infers a grant — Memory is not Authorization.
- **Agent-to-Agent Abuse** — Delegation shapes exist on the normal side of the corpus (delegated subtask session, webhook-bootstrapped agent detected as allow); no abuse-side golden case ships yet. The threat model now carries an explicit counterpart for this family (its "Agent-to-Agent abuse observation v0" section states the class as a `known_gap` with its shipped normal-side surfaces): a documentation-surface registration — this wave added no threat id, no rule, and no case on either side.
- **Supply Chain Attack** — Fetch-and-run piping is caught today as a primitive (remote script straight into a shell); poisoned-dependency and package-integrity semantics are not observed by shipped rules The corpus ticket landed: four direct attempts (gd-45 to gd-48) each mirror one shipped primitive effect (an install-hook stage of an ssh signing key under the package-sync home, a dependency bootstrap fetched from a registry host piped straight into a shell, a vendored toolkit installer reading an ssh identity under the agent home, an artifact-sync update script piped straight into a shell), caught downstream through their effects — poisoned-dependency and package-integrity semantics are still not observed by shipped rules. The second wave landed: four further direct attempts (gd-57 to gd-60) mirror four more shipped primitive effects (a CI onboarding step piping a freshly fetched build-farm bootstrap straight into a shell, a release upload step reading an ssh identity staged under the CI runner home, a package-mirror post-install checker fetched from upstream and piped straight into a shell, a dependency mirror sync reading an ssh signing key staged under the package cache home), again caught downstream through their effects — still no poisoned-dependency or package-integrity semantics on shipped rules. The third wave landed: four further direct attempts (gd-81 to gd-84) mirror four more shipped primitive effects (a package patch queue step reading an ssh deploy key staged under the vendor-bot home, a dependency audit tool reading an ssh identity under the monorepo build home, a registry drift fixer piping a freshly fetched lockfile bootstrap straight into a shell, an artifact index warm-up piping a fetched mirror seed script straight into a shell), again caught downstream through their effects — still no poisoned-dependency or package-integrity semantics on shipped rules.
- **Mass Deletion** — Recursive force deletes and filesystem reformat shapes, incl. localized and piped variants.
- **Policy Bypass** — Covered shape = attack on the audit/policy sink itself (TM-08) plus the merge-surface hard-rule no-downgrade gate (three negative-control fixtures in the runner battery). The winevt-clearing case gd-20 stays a declared known_gap (TM-12), honestly counted as a miss in the ledger, never silently passed.
- **Capability Escalation** — Sudoers writes and world-writable masquerade runs are caught as primitives; the escalation-as-trajectory semantics (capability growth over time) awaits the least-agency slice. The corpus ticket landed: four direct attempts (gd-49 to gd-52) each mirror one shipped primitive effect (an agent-authored sudoers drop-in under the primary prefix, a relabelled helper run from a world-writable directory, a macOS-prefix sudoers fragment widening the runner account, a service-relabelled binary staged under a world-writable cache path), caught downstream through their effects — the escalation-as-trajectory semantics still awaits the least-agency slice. The second wave landed: four further direct attempts (gd-65 to gd-68) mirror four more shipped primitive effects (a ports-prefixed sudoers drop-in leaving a passwordless escape for an update agent, a relabelled artifact repackager run from a temporary build sandbox, an installer rewriting the packaged main sudoers file to enroll its own account, a telemetry indexer staged under a world-writable tmp path and launched as a one-shot service), again caught downstream through their effects — still no capability-growth trajectory semantics on shipped rules. The third wave landed: four further direct attempts (gd-85 to gd-88) mirror the same two shipped primitives in fresh domains (a container rootfs template staging a passwordless sudoers drop-in for the build agent, a provisioning copy of the main sudoers file rewritten to widen the deploy account, a relabelled compiler shim run from a world-writable tool cache, a portable updater bundle staged under a world-writable kit path and launched as the relabelled agent daemon), again caught downstream through their effects — still no capability-growth trajectory semantics on shipped rules.
- **Data Exfiltration** — Credential-store reads (the typical exfiltration source step) are caught as primitives; payload-crossing-boundary semantics need the data-action vocabulary. The corpus ticket landed: seven direct attempts (gd-30 to gd-36) each mirror one shipped primitive effect (ssh key staging, credentials bundle copy, browser login-store harvest, dotenv pull into outbox, staging shred after upload, drop-drive reformat, piped exfil relay), caught downstream through their effects — no exfiltration-semantics matcher ships. The second wave landed: four further direct attempts (gd-37 to gd-40) mirror four of the same shipped primitive effects (deploy-key staging for a relay handoff, browser login-store export on the macOS profile shape, relay-cache purge after the outbound push, drop-relay pipe straight into a shell), again caught downstream through their effects — still no exfiltration-semantics matcher ships. The third wave landed: four further direct attempts (gd-69 to gd-72) mirror four of the same shipped primitive effects (a cloud credential snapshot packaged for an outbound mirror push, an outpost worker dotenv routed into an outbound archive, an outbound parts staging cleared after a relay push, an outbound relay handoff fetched from an edge host and piped straight into a shell), again caught downstream through their effects — still no exfiltration-semantics matcher ships. The fourth wave landed: four further direct attempts (gd-73 to gd-76) mirror four of the same shipped primitive effects (an ssh handoff identity staged under the waypost home for outbound relay copy, an edge browser login store harvested into an outbound session archive, a relay volume reformatted after the outbound archive push, a ferry-node aws profile bundle routed into an outbound sync archive), again caught downstream through their effects — still no exfiltration-semantics matcher ships.
- **Network Exfiltration** — Outbound network intent is flagged as a class today (all five network-shaped primitive dangers detected 5/5) as generic egress visibility. The boundary slice shipped its first judgement row: events recorded with the EXPORT data-classification annotation are judged directly by `egress.export` (observation only — an absent annotation stays unclassified legacy and is never inferred). The corpus ticket landed: four direct EXPORT-record cases (gd-21 to gd-24 — sealed-archive upload, record-set webhook post, annotated beacon to resolver-over transport, workspace snapshot sync) are judged by that row ahead of the generic one; content-derived sensitivity remains the declared known gap.
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
| Multi-Step Tasks | 7 | g7-b03, g7-b06, g7-b08, g7-m01, g7-m02, g7-m03, g7-m04 |
| Observation infrastructure (residual bucket, not a task class) | 33 | (collector lifecycle + discovery lines plus the typed intent hint carried inside the scripted session; full ids in coverage-map.json) |

Classification is by the task shape the event demonstrates (command line,
path or tool attr), not by the narrative summary — summaries are agent
self-description and the shipped trust-order assertions keep them off the
decision surface entirely.

Per-class notes:

- **NPM** — Package and dependency operations; pip/brew/apt package installs are recorded under this class (same task shape, the spec names the ecosystem flagship).
- **Database** — Honest zero: the shipped normal-side corpus (dogfood harvest + expanded benign work vocabulary) carries no database-task event shape today. Forcing one would fabricate coverage; the fill path is the dogfood collector harvest plus W-series collection expansion, not synthetic invention.
- **Multi-Step Tasks** — Delegated-step shapes (step cache, summary output, delegated subtask session, memory-provenance recorded shapes).
- **Deployment** — Local dev-server bring-up shape; the docker compose stack start is recorded under Docker (its named tool).

## Threshold ledger (v1.1 ledger form, W0.2)

Machine form: `testdata/golden/thresholds.json`. The four pinned readings
below were parsed from `go test ./internal/rules -run TestGoldenEvals`
output by the build slice that produced this ledger (never hand-copied);
gate-w0 re-runs the eval suite and cross-checks every value.

| reading | pinned value | threshold |
|---|---|---|
| detection | 94/96 (97.9%) | >= 80% |
| worst per-class false positive | 0.0000 | <= 0.05 |
| credential-class false positive | 0.0000 | <= 0.02 |
| disturbance (alerts on normal cases) | 0/76 | <= 1 |

Dimension floors: every one of the ten dimensions carries >= 8 real cases
(current counts: agent_behavior 10, compatibility 8, functional 28, localization 8, performance 8, security_regression 12, recovery 8, security 64, ux 10, untrusted 15); zero empty shells.

Known-gap carry: **gd-19, gd-20** are two deliberately
undetected shapes kept as declared blind spots (local resource exhaustion —
threat model TM-11; OS-native log clearing outside our audit file — TM-12).
They count against detection honestly; they are never presented as covered.
