# Compatibility Matrix v0 — what each platform gives this runtime, against the shipped code

Version lock: this table, `docs/red-team-plan.md`,
`testdata/golden/compatibility-matrix.json` and
`scripts/compat-redteam-check.mjs` MUST evolve in one PR; the checker
re-derives platform facts from shipped source code (build tags, scan
files, schema enums) on every run and fails on any drift — never against
this file alone.

Scope: the platform-capability contract of the V2 security-coverage
specification. This matrix answers one question per cell: **does the
platform give the runtime a chance to collect this surface at all**,
tiered against what the shipped code actually does with the chance. It is
NOT the coverage-truthfulness matrix (`docs/coverage-truthfulness.md`),
which answers "what was actually collected" on the collection-surface
axis; the same row id never appears in both. One platform's chance, one
runtime's use — named per cell.

## The four tiers (closed set, shared with the coverage matrix — no second dialect)

| tier | meaning on this axis |
|---|---|
| FULL | the platform gives the chance and the shipped code names no degradation path for it |
| LIMITED | the platform gives a chance to collect, but the shipped code names a concrete degradation path — what comes back can be empty, weaker, or fallback-derived |
| MONITOR ONLY | the surface reaches the runtime only as declared, completed records; Phase 0 expresses and audits, it never decides, blocks, or stops |
| UNAVAILABLE | no shipped path to this surface on this platform (a compile-time or code-absence fact), not a promise of future coverage |

Nothing outside these four words is a legal tier value. The machine form
mirrors this table cell-for-cell
(`testdata/golden/compatibility-matrix.json`).

## Honesty rules of this matrix

- Every cell names at least one evidence pointer that the checker
  re-resolves against the shipped tree; an unresolvable pointer fails the
  gate, so no cell can be hollow.
- The inverse ban matters just as much: **no cell here may borrow a
  collection cell from the coverage matrix to impersonate enforcement
  capability.** The specification's File Pre-Action example is UNAVAILABLE
  everywhere because no shipped interception path exists — a match is
  recorded as would_block but never enforced.
- CI three-platform build+test jobs carry a partial contract/API shape
  only. Presenting them as real-machine OS-update regression is forbidden;
  those rows stay "declared, not yet verified", following the
  evals-coverage-v0 known_gap counting posture (an honest miss beats a
  silent pass).
- The Agent Adapter generates configuration; **it is not a security
  boundary** (a shipped specification rule restated on this axis). Hook
  rows are therefore LIMITED at best: adoption lives on the agent side,
  and the platform does not force it.
- Blanket promises that smooth per-platform mechanisms into one
  undifferentiated guarantee are forbidden in every carrier. Naming why
  each cell is not FULL is the product.

## The matrix

| surface | class | linux | darwin | windows | why not FULL / platform difference named |
|---|---|---|---|---|---|
| `compat.process-enumeration` | os | FULL | FULL | FULL | three platform channels (proc pid directories / full listing / toolhelp snapshot); the enumeration itself needs no privilege anywhere |
| `compat.process-commandline` | os | LIMITED | LIMITED | LIMITED | the platform gives a cmdline channel everywhere but names its own hiding: other users' processes read back empty on linux, kernel privacy defeats the args probe on darwin, protected processes cannot be opened on windows |
| `compat.network-observation` | network | MONITOR ONLY | MONITOR ONLY | MONITOR ONLY | no shipped socket enumerator anywhere; network reaches the runtime only as agent-declared network.intent records on the hook/MCP inflow slots, observed after the fact |
| `compat.file-access-observation` | file | MONITOR ONLY | MONITOR ONLY | MONITOR ONLY | the file.access type ships in the closed schema enum, fed only by agent-reported inflow; no OS-level file watcher ships on any platform |
| `compat.pre-action-interception` | enforcement | UNAVAILABLE | UNAVAILABLE | UNAVAILABLE | no shipped interception path on any platform; matches are recorded as would_block but never enforced — this row must never be dressed up with coverage-matrix collection cells |
| `compat.agent-hook-channel` | agent | LIMITED | LIMITED | LIMITED | the shipped adapter renders hook configuration for known agent shapes; whether the channel exists depends on the agent adopting the generated config, the platform does not force it, and the adapter is not a security boundary |
| `compat.mcp-observation` | mcp | MONITOR ONLY | MONITOR ONLY | MONITOR ONLY | the relay forwards first and observes completed records; a nil observer degrades it to pure relay, declared in the shipped field comment |
| `compat.identity-binding` | identity | FULL | LIMITED | LIMITED | linux ships dedicated machine-id files the fingerprint chain reads first; darwin/windows have none, so the chain ends at the hostname fallback the shipped code itself names as weaker |
| `compat.policy-decision-surface` | policy | MONITOR ONLY | MONITOR ONLY | MONITOR ONLY | the deterministic evaluator ships everywhere the collector ships, but while the enforcement-plane token reads none-in-observation-phase every match is an audit-stream observation, never a stop |
| `compat.audit-replay` | audit | MONITOR ONLY | MONITOR ONLY | MONITOR ONLY | persistence and rotation of the audit trail ship on every supported platform; the trail observes, warns and audits — it never stops anything |
| `compat.os-update-regression-suite` | compat | UNAVAILABLE | UNAVAILABLE | UNAVAILABLE | nothing shipped is triggered by an OS update (Contract / API Compat / Enforcement / Performance / Recovery); CI build+test carries a partial contract/API shape only and must not be presented as real-machine regression — declared, not yet verified |

## Unsupported platforms (compile-time fact)

On any GOOS outside linux, darwin, and windows the whole compatibility
surface is UNAVAILABLE: the shipped snapshot returns an explicit
"unsupported on this platform" error instead of pretending coverage.

### gd-compat-harmonyos — declared gap, not approximation

The specification names HarmonyOS with its official open capabilities as
the implementation path. The shipped tree has **zero implementation files
for it** — the checker re-derives this absence on every run. Every row of
this matrix is UNAVAILABLE there, recorded as declared, not yet verified.
Never approximate a HarmonyOS cell from another platform's row.

### gd-compat-osupdate — declared gap on the five regression families

The per-OS-update automatic five-test contract has no shipped trigger.
The only partial carrier is the CI three-platform build+test job, and it
carries a contract/API shape only. Enforcement, Performance and Recovery
regression on real machines after OS updates are all pending; the gap
matrix's real-device verification series covers this face as declared,
not yet verified — the honest-miss posture, again.

## Consumption obligation

Every consumer display (UI when it ships, enterprise reports when they
ship) must carry these tiers verbatim per platform, keep the two axes
(collection surface vs platform capability) visibly separate, and drop
nothing that says *why* a cell is not FULL. This matrix drives display
and documentation only: while the enforcement-plane token reads
none-in-observation-phase it feeds no decision, no rule engine, no policy
evaluator, and no audit judgement.
