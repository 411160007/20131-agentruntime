# Coverage Truthfulness Matrix v0 — collection surfaces × platforms against the shipped code

Version lock: this table, `testdata/golden/coverage-truthfulness.json` and
`scripts/coverage-truth-check.mjs` MUST evolve in one PR; the checker
re-derives counters, vocabularies and platform facts from shipped source
code on every run and fails on any drift — never against this file alone.

Scope: the coverage-truthfulness contract of the V2 security-coverage
specification. Every shipped collection surface carries one of four honest
tiers on each supported platform, and every cell names at least one
evidence pointer that the checker re-resolves against the tree — a cell
whose pointer does not resolve fails the gate, so no cell can be hollow.

## The four tiers (closed set)

| tier | meaning |
|---|---|
| FULL | the surface is collected on this platform and the shipped code declares no degradation path for it |
| LIMITED | the surface is collected, but the shipped code names a concrete degradation path — fields can come back empty or unreadable |
| MONITOR ONLY | the surface is observed and recorded only; Phase 0 expresses and audits, it never decides, blocks, or stops |
| UNAVAILABLE | the platform does not implement this surface at all (a compile-time fact) |

Nothing outside these four words is a legal tier value. The machine form
mirrors this table cell-for-cell (`testdata/golden/coverage-truthfulness.json`).

## Honesty rules of this matrix

- Platform differences are named per row and per cell. Blanket promises
  that smooth the per-OS hiding mechanisms into one undifferentiated
  guarantee are forbidden in every carrier — this document, the UI surface
  when it ships, and the enterprise report surface when it ships. Naming
  why each cell is not FULL is the product; the matrix exists precisely to
  force that naming.
- The counters behind the accounting rows are read from the shipped source
  at build time. A declared-but-unpopulated counter is recorded as
  declared-unset and is never rendered as live coverage.
- Every detection feeding this matrix is a would_block record on the audit
  stream; Phase 0 observes, warns and audits — it never stops anything
  (see threat-model.md). While the enforcement-plane token reads
  `none-in-observation-phase` this matrix drives display and documentation
  only; it feeds no decision, no rule engine, no policy evaluator, and no
  audit judgement.
- On any platform other than linux, darwin, and windows the whole
  collection surface is UNAVAILABLE: the snapshot returns an explicit
  "unsupported on this platform" error instead of pretending coverage.

## Consumption obligation (UI / Docs / Enterprise Report)

Every consumer display must carry these tiers verbatim per platform. None
may smooth a LIMITED or UNAVAILABLE cell into an unconditional promise;
none may drop the per-platform difference notes that say *why* a cell is
not FULL.

## The matrix

| surface | class | linux | darwin | windows | platform difference named |
|---|---|---|---|---|---|
| `os.process-snapshot` | os | FULL | FULL | FULL | three different enumerators (proc pid directories / full ps listing / toolhelp snapshot); none needs privilege for the enumeration itself |
| `os.cmdline-visibility` | os | LIMITED | LIMITED | LIMITED | degradation mechanisms differ: other users' processes read back empty on linux, kernel-level privacy defeats the ps args probe on darwin, protected processes cannot be opened for the limited query rights on windows |
| `os.hidden-process-accounting` | os | FULL | FULL | FULL | the per-cycle hidden counter is wired through the collector identically in code; each platform ships its own hidden predicate (empty or failing proc read / ps probe failure / OpenProcess failure) |
| `os.no-cmdline-accounting` | os | LIMITED | LIMITED | LIMITED | the empty-cmdline count ships everywhere; its hidden-by-OS sibling field is declared-unset (see counters below), so the accounting is honestly not complete |
| `agent.claude-code-detection` | agent | LIMITED | LIMITED | LIMITED | one shared fingerprint table (cli + inspector rules, windows launcher suffixes included); detection degrades wherever cmdline visibility degrades, per the OS-specific mechanism above |
| `agent.codex-detection` | agent | LIMITED | LIMITED | LIMITED | one shared fingerprint table (cli rule incl. command-runner/exec shapes); degradation follows the same per-OS paths |
| `agent.openclaw-detection` | agent | LIMITED | LIMITED | LIMITED | one shared fingerprint table (product name, npm package, gateway launch form; a bare "gateway" token is far too broad to key on); degradation follows the same per-OS paths |
| `mcp-server-detection` | mcp | LIMITED | LIMITED | LIMITED | direct binary fingerprints plus the parent-chain rule that needs lineage; orphaned-server accounting rides the same snapshot — all bounded by the rows above |
| `tool.call-observation` | tool | MONITOR ONLY | MONITOR ONLY | MONITOR ONLY | the local relay forwards first and observes completed records on every platform; a nil observer degrades it to pure relay, declared in the shipped field comment; nothing here decides |
| `capability.classification` | capability | MONITOR ONLY | MONITOR ONLY | MONITOR ONLY | one closed vocabulary table on every platform; it expresses capability use, does not enforce grants, and pins transitivity off — input fields degrade with the detection rows |

Per-cell evidence pointers (file + token the checker re-resolves) live in
the machine form; pointer anchors per row:
`internal/discovery/` (snapshot, fingerprints, stats, hidden predicates),
`cmd/agent-collector/run.go` (counter wiring and event attributes),
`internal/mcpproxy/proxy.go` (relay/observer), `internal/schema/capability.go`
(vocabulary and Phase 0 boundary).

## Counters behind the accounting rows

| counter | source | event attribute | state |
|---|---|---|---|
| no_cmdline | `ScanStats.NoCmdline` (incremented at scan) | `no_cmdline` | populated |
| hidden | `discovery.HiddenFromUs` per platform predicate | `hidden` | populated |
| no_cmdline_at | `ScanStats.NoCmdlineAt` declaration | — | declared-unset |

The third row is the honest seed: the field exists, no production path
increments it yet, and the checker censuses non-test occurrences expecting
exactly the declaration. The day real code populates it without upgrading
this matrix, the census fires red — a silent half-claim is exactly what
this slice forbids.
