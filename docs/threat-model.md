# Security Threat Model v0

Scope: Phase 0 of the 20131 Agent Security Runtime — a local,
observe-only agent-behaviour monitor (discovery + audit + judgement
layer). This document enumerates what we watch for, which surface
sees it, and which built-in rule (or explicit known gap) answers it.
It is the external counterpart of the built-in rule table
(`builtin-phase0`, 12 rules, embedded in the binary) and is
cross-checked against it in both directions by the gate.

Honest-boundary notes (read before trusting any line):

- Phase 0 **observes, warns, and audits — it never stops anything**.
  Every finding is recorded as `would_block` on a `policy.decision`
  audit line; "hard" marks a finding the user may not downgrade in
  the audit stream, not a prevention.
- A threat list is never complete. Items we know we cannot see are
  listed with an explicit `known_gap` line instead of silence.
- This document describes detection categories and shipped matcher
  shapes only. It deliberately contains no attack procedures,
  payloads, or evasion recipes (product red line: never ship
  how-to guides for the things we detect).
- Detection vocabulary stays out of marketing words: this product is
  described as agent security monitoring / runtime observation,
  including in Chinese-language copy (no border-device wording).

## Threat items

| id | threat (what an agent under compromise or misalignment could do) | detection surface | built-in rule(s) |
|----|------------------------------------------------------------------|-------------------|------------------|
| TM-01 | read or exfiltrate credentials (ssh keys, cloud credential stores, browser login vaults, dotenv secrets) | file/tool events via hook + mcp reserved slots, evaluated by the engine | `cred.ssh`, `cred.aws`, `cred.browser`, `cred.dotenv` |
| TM-02 | bulk destruction of data (recursive force deletes, filesystem reformat) | command shapes on hook/mcp surfaces + discovered process cmdlines | `destroy.rmrf`, `destroy.disk` |
| TM-03 | modification of privileged system paths (privilege configuration drop-ins) | file events via hook reserved slot | `path.sudoers` |
| TM-04 | outbound communication to domains outside the user's allow list | network intent events (hook/mcp reserved; not visible to process discovery) | `net.egress` |
| TM-05 | MCP tool injection: a tool exposing raw code-evaluation to model-supplied input | tool call events via mcp reserved slot | `mcp.eval` |
| TM-06 | fetch-and-run execution: piping remote scripts straight into a shell | command shapes on hook/mcp surfaces + discovered process cmdlines | `exec.remotepipe` |
| TM-07 | agent impersonation / relabel: a process claiming an agent family while executing from a world-writable directory | discovery snapshots (active today: exe paths of detected processes) | `agent.masquerade` |
| TM-08 | audit evasion: deleting or shredding the runtime's own audit trail | file events naming the audit path, from any source slot | `audit.tamper` |
| TM-09 | collector tampering: an attacker with the same user rights edits or disables the collector itself | `known_gap` — Phase 0 has no external attestation of its own binary; mitigations today: audit files at 0600, honest start lines, rotation; OS-integrity surfaces are later-phase scope | `known_gap` |
| TM-10 | process-observation blind spots: pre-existing/protected processes and unreadable cmdlines | `known_gap` (bounded, declared on every collector.start line as shipped coverage attrs) | `known_gap` |
| TM-11 | local resource exhaustion (fork bombs, runaway loops) | `known_gap` — v0 rules stay in the pure-match fast path; resource telemetry is later-phase scope | `known_gap` |
| TM-12 | clearing platform-native logs outside our audit file (OS event logs) | `known_gap` on non-unix shapes — path rules can be added to the user override surface today; a built-in ships once event surfaces land | `known_gap` |

Every built-in rule answers at least one threat above, and every
threat row names either at least one rule or an explicit `known_gap`
line: the gate greps both directions and fails on drift in either.

## Merge-surface limitation (stated, not hidden)

User override documents may add rules and whitelists but may not
alter or exactly shadow a hard built-in rule — that refusal is
mechanical. Refusing a *broader* allow matcher that happens to
overlap a hard matcher cannot be decided in general at merge time;
until enforcement phases exist (where the decision is only advisory
in the first place), this stays a documented limitation with its own
unit-test control.

## Evaluation (frozen golden set, 40 cases)

The judgement layer is benchmarked against `testdata/golden/`: 20
normal lines taken from real overnight self-monitoring harvest
(redacted) and 20 synthetic danger shapes. Pinned thresholds:
detection ≥ 80 %, per-class false positives ≤ 5 %, credential-class
≤ 2 %, and zero-disturbance ≤ 1 alert across the 20 normals. The
current shipped table's measured result on this set (kept green by
the gate on every run): detection 18/20 = 90 % — the two misses are
TM-11 and TM-12 by design, and the false-positive count on normals
is 0. A red eval blocks the release slice; a release cannot ship
with a sick judgement layer.

## What changes when enforcement arrives

Later phases turn `would_block` into prevented actions at the bus
and proxy execution points. When that happens this document gains a
"response" column, the hard annotation gains teeth, and TM-09 moves
from `known_gap` toward integrity-attested surfaces. Until then,
anything in this repo claiming prevention is a bug the gate
vocabulary greps already hunt for.
