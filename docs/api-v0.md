# Core API Contract v0 (frozen)

This document is the frozen public contract of the core API surface:
the JSONL event line, the shared vocabularies, the collector CLI control
commands, and the version line shape. Evolution is **additive only**:
fields may gain optional members; vocabularies may gain values only
together with a schema-version bump or an explicit compatibility note
here; nothing listed here may be renamed, removed, or re-grammared
without a new major version of this contract.

A machine gate cross-checks this document against both implementations
(Go enums, the Node JSONL validator) and the real binaries:
`node scripts/apicontract-check.mjs` plus
`TestAPIDocContractMatchesGoEnums`. The contract lists are the fenced
`key: a, b, c` blocks below; the checkers parse them verbatim.

## JSONL event line

Exactly one JSON object per line; the line ends with `\n`. Field names
and grammar are fixed:

| field      | type              | required | notes |
|------------|-------------------|----------|-------|
| v          | int               | yes      | schema version; currently 1 |
| ts         | string (RFC3339Nano, UTC) | yes | year 2020..2100 |
| id         | string            | yes      | `[A-Za-z0-9][A-Za-z0-9._-]{0,63}`; unique per collector run |
| agent_id   | string            | yes      | same grammar; stable ids use `agi-` + 40 hex |
| stage      | enum              | yes      | see `stages` |
| type       | enum              | yes      | see `types` |
| decision   | enum              | yes      | see `decisions`; Phase 0 runtime emits only `allow`/`would_block` |
| severity   | int 0..4          | yes      | 0 info, 1 low, 2 medium, 3 high, 4 critical |
| summary    | string 1..512B    | yes      | human text, redacted by producers |
| tier       | enum              | no       | see `tiers`; ABSENT MEANS L1 (additive v1 extension) |
| attrs      | map[string]string | no       | keys ≤64B, values ≤1024B; reserved keys below |

Reserved attrs keys:

- `cap` — capability token (see `caps`), valid vocabulary or the line is
  rejected by every validator and by the audit writer (zero bytes move).
- `res_class` — resource sensitivity (see `res_classes`); empty means
  unclassified, wild values rejected.
- `kind`, `rule`, `pid`, `ppid`, `name`, `exe`, `cmdline`, `identity`,
  `locator`, `state`, `run`, `observed`, `mode` — discovery annotation
  surface (strings only).

## Vocabularies (dual-source: Go enums == this list == validator)

```contract
stages: proposed, evaluated, enforcement, action, observation
```

```contract
types: command.proposed, tool.call, file.access, network.intent, policy.decision, enforce.action, collector.start, collector.stop, agent.detected, agent.scan
```

```contract
decisions: allow, ask, would_block
```

`ask` is reserved contract vocabulary for a later phase. Non-obvious
constraint (Phase 0): every runtime decision value MUST be inside
`{allow, would_block}`; `would_block` is recorded, never enforced. The
event schema (`MustPhase0Decision`), the bus, unit tests, and the
gate-d3 literal grep enforce this at four independent levels.

```contract
tiers: L0, L1, L2, L3
```

Tier semantics (additive field): L0 dropped at the bus and only
counted; L1 persisted directly (the default when the key is absent);
L2/L3 persisted AND delivered in order to rule-engine consumers.

```contract
caps: file.read, file.write, file.delete, net.outbound, net.listen, proc.spawn, shell.exec, mcp.tool, credential.access, env.read, process.inspect
```

```contract
res_classes: low, medium, high
```

Capability grants are NON-TRANSITIVE: a child process never inherits a
parent's grants (annotation only in Phase 0; nothing enforces grants
yet). Rule documents referencing capabilities must reference tokens in
`caps` — machine-checked by the rules table vs capability table cross
grep in the gate (currently zero cap-carrying rules; the check
strengthens automatically as rules arrive).

## Agent identity (stable ids)

```
agent_id = "agi-" + hex(sha256(machine_fingerprint || 0x00 || normalized_locator))[0:40]
```

- `machine_fingerprint`: `/etc/machine-id` (then
  `/var/lib/dbus/machine-id`, then hostname) — local sources only.
- `normalized_locator`: exe path, else cmdline, else name; separators
  unified to `/`, whitespace collapsed, case folded (folding can only
  merge spellings of one binary, never split one spelling).
- Stability: same executable shape ⇒ same id across scans AND across
  collector runs. Pids and run ids never enter the derivation.
- Passport states:

```contract
states: known, observed, trusted
```

  Promotion edges: known→observed (automatic on sighting);
  observed→known / trusted→observed (forget / revoke).
  **No transition graph edge enters `trusted`** — reaching trusted
  requires the explicit user trust file (`--trust`), which the
  collector only ever READS; an id absent from that file can never be
  auto-promoted.

## Collector CLI (read-only control surface)

Binary grammar: `agent-collector [subcommand] [flags...]`

| subcommand  | flags        | output contract |
|-------------|--------------|-----------------|
| (none)      | `--out --interval --once --max-cycles --rotate-bytes --rotate-daily --keep-history --mode --trust` | scan loop; writes only the JSONL audit stream |
| `status`    | `--out --trust` | line-oriented plain text: `status: <N> event lines`, then `by type:` / `by kind:` / `by state:` sections, keys byte-ascending, one `  <key> <count>` line each |
| `audit-tail`| `--out --n`    | the trailing N valid audit lines printed verbatim (validates every line it prints; corrupt input ⇒ non-zero exit) |

- Every flag exists as `-flag` and `--flag` (Go flag grammar).
- `--mode` accepts `observe` only (the complete Phase 0 mode set).
- Subcommands never write: audit file opened read-only; the only file
  sink in the program remains the scan loop's JSONL writer (0600).
- Unknown flags/subcommands exit 2.

## Version line

```contract
version_regex: ^agent-collector [^ ]+ [a-z0-9]+/[a-z0-9]+ \(go[0-9.]+\)$
```

Shape `<name> <version> <os>/<arch> (<goversion>)`; the hello binary
uses the identical shape with its own name. This line is frozen for the
packaging release (D5) naming contract and MUST NOT break.

## Bus source registry

Sources named at wiring time: `discovery-scan` (active), `hook`, `mcp`
(reserved slots: visible in the registry, publish-rejected and counted
until their slice ships). The collector.start line carries
`bus_sources` with this exact comma-joined set, so the wiring shape is
auditable from the file alone.
