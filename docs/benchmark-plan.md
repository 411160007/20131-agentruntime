# Performance Benchmark Plan v0 (slice W11.4)

Status declarations (three pins, binding on this whole document): this is
a **planning only** measurement contract - the benchmark battery is not
built and **contains no results**; every number below is either a
pre-pinned engineering goal or a recorded seed with an explicit
verification state, never a marketing promise. Engineering goals must not
be presented as verified claims before a real benchmark run proves them.
Emission plane: `none-in-observation-phase` - Phase 0 measures the cost
of observing and evaluating, it never intercepts, blocks, or stops.

Contract: `docs/schema-v2.md` §39. Machine mirror:
`testdata/golden/benchmark-plan.json`. Independent dual-source check:
`scripts/benchplan-check.mjs` (runs on every platform build through the
Go test mount). This file, the mirror, the checker, and the schema
section evolve in one PR - the checker re-derives instrumentation facts
from shipped source, never from this document alone.

## 1. Metric set (§73 continuous-measurement list, closed census)

Axis statement: this table answers "how is each performance-tax metric
going to be instrumented and judged" on the measurement axis. It is not
the capability matrix (does the platform allow collecting) and not the
coverage-truthfulness matrix (what was actually collected); row ids are
machine-checked disjoint from both.

Status vocabulary (closed set, no second dialect): `PARTIAL` - a shipped
carrier exists that measures a named slice of this metric; `PLANNED` -
the metric has a named future carrier design in this plan but no shipped
instrument yet; `ABSENT` - no shipped instrument and no named carrier
on this platform class, recorded as a declared gap.

| id | status | pre-pinned acceptance target | carrier / gap note |
|---|---|---|---|
| `bp.cpu-idle` | PLANNED | idle CPU as low as possible, below 0.5-1 percent | CI build+test wall-clock shape only; a dedicated idle-window sampler is designed but not built |
| `bp.resident-memory` | PLANNED | resident memory in the 100-200MB class | process RSS sampler in the battery, run under the shipped collector binary; not built yet |
| `bp.disk-io` | PARTIAL | measure and publish; no numeric ceiling pinned yet (an honestly open goal, not an invented number) | shipped audit-log byte census `TotalBytes` and rotation accounting give the write-side volume directly |
| `bp.network-overhead` | PARTIAL | measure and publish; no numeric ceiling pinned yet | shipped MCP proxy byte counters `ReqBytes` / `RespBytes` cover the proxy surface only; nothing else claims a network measurement |
| `bp.decision-latency` | PARTIAL | fast path low-millisecond or better (contract number; recorded seed in §6 below) | shipped proxy latency accounting (`latency`), honest only for first-seen requests as the code names; the end-to-end decision bench carrier now ships as `scripts/bench-replay-e2e.mjs` (startup-inclusive replay-corpus timing; readings stay in the `recorded` posture of §6 and must be re-measured by the CI benchmark battery before citation) |
| `bp.battery-impact` | ABSENT | no target pinned before instrumentation exists | no shipped battery reader; gap `gd-perf-battery-thermal` - CI runners are not battery-powered, so CI can never fake this row |
| `bp.temperature-impact` | ABSENT | no target pinned before instrumentation exists | no shipped thermal reader; same gap row, declared not yet verified |

Evidence pointers per row live in the machine mirror; the checker
re-resolves every pointer against the tree, so no cell can be hollow.

## 2. Lightweight constraints on the measurement design (§70)

The core stays extremely light. The benchmark battery must prove the
architecture adopts **Event Driven**, **Lazy Load**, **Cache**, **Fast
Path**, and **Async Slow Path** - and must prove the six banned
postures never appear: no continuous full-disk scanning, no reading all
file contents by default, no per-action LLM call, no permanent retention
of every log, no always-resident modules, no always-resident agent
adapters. Measurement design rule: an instrument may not itself violate
a banned posture (a sampler that full-scans the disk to measure disk IO
fails the gate on design review). The shipped tree already carries the
permanence bound on the logging side through rotation and quota
accounting (`internal/auditlog/governance.go`).

## 3. Resource-manager signals (§71)

The resource manager is designed to read CPU, RAM, Disk, Network,
**Battery**, **Thermal**, **Gaming**, **Compile**, and **Local AI**
signals, tune analysis frequency from them, and auto-degrade
non-critical background work - with one floor that is a contract, not a
tuning knob: **core safety must never degrade to ineffective**.
Shipped state: `ABSENT` - zero resource-manager symbols in the tree
(machine re-derivation below); recorded as gap `gd-perf-resource-manager`.
The benchmark plan pins what the future battery must assert: every
degrade path keeps the core observation path alive, and the degrade
decision itself is audited.

## 4. Performance modes (§72)

Three named modes - `Balanced` (default), `Performance` (fewest
disturbances), `Security` (more analysis) - with core safety present in
all three, permanently. Shipped state: `ABSENT` (gap
`gd-perf-modes`); until modes exist, every measurement in this plan runs
in the single shipped observation shape and must be labeled as such.

## 5. Environments and carriers

- Continuous carrier: the existing three-platform CI jobs
  (`ubuntu-latest`, `macos-latest`, `windows-latest` from
  `.github/workflows/build.yml`) run the mount on every PR. This
  reuses existing jobs - **zero new paid runners, zero new hardware**.
- CI honesty: the CI jobs carry a partial contract/API shape plus the
  mount; presenting them as real-machine performance evidence is
  forbidden. The battery, thermal, and per-mode rows prove nothing in
  CI, and the idle/resident numbers on shared runners are shapes, not
  promises.
- Real-machine and real-network follow-up: §7 below, batched.

## 6. Fast Path recorded seed

The fast-path decision cost carries one recorded seed: **1.467µs/event**
(about 1/680 of the one-millisecond budget), measured during the
judgement-layer development phase and registered in the internal
delivery ledger; the internal id is deliberately not reproduced in this
public tree. Verification state: `recorded`, meaning **estimation is not
verified fact** - the value was produced by a development-phase
measurement, not by the future CI benchmark battery, and it must be
re-measured by that battery before it may be cited anywhere outside
engineering documents. The contract number stays the pre-pinned target
in §1 (`bp.decision-latency`).

Recorded end-to-end reading through the replay carrier (same honesty
posture as the seed above - state `recorded`, estimation is not verified
fact): median 9.73 ms wall for 129 golden-corpus events judged end to end
by the shipped collector binary, startup-inclusive, about 75.4 µs per
event on a linux/amd64 development container (one build, one run of 15
repetitions, 2026-10-09). Reproduce with
`node scripts/bench-replay-e2e.mjs --binary <collector> --policy <candidate.json> --reps 15`;
the reading is engineering-plan material only, never a published promise.

## 7. Real-machine follow-up batch (four scenarios, closed census 4/4)

These four scenarios were opened by the pre-release research ledger
(their internal ids stay outside this tree) and are batched into this
benchmark plan so a single real-machine pass can retire them. Status
for all four: `declared, not yet verified` - honest miss over silent
pass, following the evals known_gap counting posture.

| id | scenario | carrier needed | what the benchmark must output |
|---|---|---|---|
| `bp-rm-1` | Apple Silicon and Rosetta execution behavior of the unsigned amd64 binary | one arm64 mac | exit-shape and timing of the shipped collector under Rosetta, versus the ad-hoc-signed arm64 build; the CI macos job is only a partial carrier for the build and signature-check shape |
| `bp-rm-2` | current-macos Gatekeeper user paths (right-click open, quarantine attribute inspection) | one mac on the current supported OS | first-launch and second-launch interaction cost, measured against the shipped guidance wording |
| `bp-rm-3` | quarantine-attribute propagation across download-and-unpack paths (browser, curl, wget, Finder) | one mac | which shipped byte paths preserve or drop the attribute, with per-path wall-clock cost |
| `bp-rm-4` | direct download experience of release artifacts from mainland networks | real network probe | fetch latency and success shape of the public artifact hosts, and of the self-hosted fallback path, before any mirror weighting decision |

Batching rule: these rows join the evals ledger with the same
honest-miss accounting as every other gap; until retired, no document
may claim real-machine verification of distribution behavior.

## 8. Honest-miss accounting

Gap census: `gd-perf-resource-manager`, `gd-perf-modes`,
`gd-perf-battery-thermal`, `gd-perf-realmachine-batch` (machine mirror
carries each with a re-resolvable pointer). Every ABSENT or PLANNED row above is a declared
gap or a declared design, never an inferred pass. Numbers appear in
this plan only when they are pre-pinned contract targets or explicitly
state-labeled seeds; "no numeric ceiling pinned yet" is a valid cell and
an invented number is a gate failure.

## 9. Lockstep

This document, `testdata/golden/benchmark-plan.json`,
`scripts/benchplan-check.mjs`, the Go test mount, and `docs/schema-v2.md`
§39 must evolve together in one PR; the checker re-derives the
shipped-symbol facts (battery/thermal readers, resource-manager
symbols, mode names) from the Go tree on every run and fails on drift
between "declared absent" here and any symbol that appears there.
