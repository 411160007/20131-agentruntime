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
| source_class | enum            | no       | see `source_classes`; ABSENT MEANS unclassified legacy (additive v1 extension, observation-only) |
| attrs      | map[string]string | no       | keys ≤64B, values ≤1024B; reserved keys below |

Reserved attrs keys:

- `cap` — capability token (see `caps`), valid vocabulary or the line is
  rejected by every validator and by the audit writer (zero bytes move).
- `res_class` — resource sensitivity (see `res_classes`); empty means
  unclassified, wild values rejected.
- `hard` — carried on `policy.decision` lines: `"true"` marks a
  non-downgradable built-in finding (the annotated decision is
  `would_block` with severity critical; nothing is enforced in
  Phase 0).
- `kind`, `rule`, `pid`, `ppid`, `name`, `exe`, `cmdline`, `identity`,
  `locator`, `state`, `run`, `observed`, `mode` — discovery annotation
  surface (strings only). `rule` additionally names the matched
  built-in/user rule id on `policy.decision` lines.

## Vocabularies (dual-source: Go enums == this list == validator)

```contract
stages: proposed, evaluated, enforcement, action, observation
```

```contract
types: command.proposed, tool.call, file.access, network.intent, policy.decision, enforce.action, collector.start, collector.stop, agent.detected, agent.scan, session.start, turn.stop
```

Additive compatibility note (platform adapter surface): `session.start`
and `turn.stop` extend the vocabulary without renaming or re-grammaring
anything frozen above. They are lifecycle observations recorded by the
hook receiver (an agent session beginning / a turn ending); every
existing consumer keeps validating lines by the unchanged field rules,
and a pre-extension reader that filters on the older ten types simply
ignores the two new ones. No prior meaning changed.

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
source_classes: native_os, runtime, tool_mcp, agent_meta, agent_self, llm_interpretation
```

Source-class semantics (additive field, evidence trust order): the six
values name WHERE an event's evidence was collected, listed in
 descending trust — `native_os` (direct OS-level observation, the
highest class), `runtime` (in-process collector events), `tool_mcp`
(observational metadata from a tool/MCP relay), `agent_meta` (adapter
lifecycle facts about an agent), `agent_self` (the payload describing
its own actions), `llm_interpretation` (an external model's derived
reading, the lowest class). The class of a record is a property of the
collection mount, never of the payload: self-description cannot raise
its own class. Absent means unclassified legacy and is never defaulted
upward or downward. OBSERVATION-ONLY: no judgement, rule, or decision
path consumes this field in the current phase; the zero-consumption
state is machine-asserted structurally by gate-d7 (mount wiring and
field-shaped trust-order judgements land with later slices).

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
grep in the gate (the built-in set carries cap references on every
rule; the gate asserts the pairing both ways).

## Policy rule grammar (additive v1 extension of rule documents)

Rule documents (the built-in set and user overrides) match one event
field with one operator:

```contract
rule_fields: agent_id, type, tool, path, domain, cmdline, exe
```

```contract
rule_ops: equals, prefix, suffix, contains
```

`tool`, `path`, `domain`, `cmdline`, and `exe` read the matching attrs
key; absent attrs never match. Additive members of the Rule object:

- `hard` (bool, default false) — non-downgradable annotation. A hard
  rule MUST carry `effect=would_block` and `severity=4` (critical);
  the schema grammar rejects any other shape, and the user-override
  merge refuses documents that change a built-in rule or exactly
  shadow a hard one with an allow rule.
- `caps` ([]string, default empty) — capability tokens guarded by the
  rule; every token must be a member of `caps` above.

The built-in Phase 0 set is exactly 12 rules
(`builtin-phase0`, embedded in the binary): cred.ssh, cred.aws,
cred.browser, cred.dotenv, destroy.rmrf, destroy.disk,
exec.remotepipe, net.egress, mcp.eval, agent.masquerade, path.sudoers,
audit.tamper. Decisions are additive: matched rules record
`would_block` audit lines; nothing is ever enforced (see decisions
note above). The id ↔ threat mapping is `docs/threat-model.md` and is
cross-checked in both directions by the gate.

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
| `timeline`  | `--out --agent --since` | plain-text event timeline: header `timeline: <N> event(s)`, then one row per event `ts  decision  type  agent_id  summary` (columns left-aligned, single-space gap at least two); `--agent` exact id filter, `--since` RFC3339 lower bound; validates every line it reads; corrupt input ⇒ non-zero exit |
| `hook`      | `--out`        | adapter receiver: one hook payload on stdin → appended audit line(s); ALWAYS exits 0 with ZERO stdout bytes whatever it receives (an unreadable payload records nothing and still disturbs nothing); unparseable/oversized/unknown-event input ⇒ zero bytes recorded |
| `mcp`       | `--out --server` + positional server args | stdio JSON-RPC relay: forwards every byte both ways untouched while auditing completed `tools/call` round trips (hash + redacted excerpt + latency/sizes); judgement annotations are emitted only, the relay never consults them; exits non-zero only on transport failure |
| `integrate` | `--target --dir --bin` | config generator: `claude-code`/`openclaw` merge recorder wiring into an existing `--dir` file (idempotent; unparseable existing config ⇒ refusal, never overwrite); `codex` writes NOTHING and prints the honest coverage gap; unknown target or missing directory ⇒ non-zero exit, zero writes |

- Every flag exists as `-flag` and `--flag` (Go flag grammar).
- `--mode` accepts `observe` only (the complete Phase 0 mode set).
- Control subcommands never write; adapter subcommands write only
  audit lines: `hook`/`mcp` append exclusively to the JSONL audit
  stream (0600), `integrate` writes only inside its explicit `--dir`.
  The scan loop remains a pure audit emitter. Nothing in this binary
  performs network I/O: the relay speaks over child-process pipes only.
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

Engine delivery: tier L2/L3 lines are handed, in order, to every
attached rule-engine consumer (the built-in judgement layer renders
the `policy.decision` line described above; the consumer never
mutates the source line and never re-feeds decision lines to the
engine).
