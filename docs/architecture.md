# Architecture notes (D1 skeleton)

## The pipeline

Every supervised action flows through four stages, mirrored one-to-one by
`schema.Stage`:

```
Agent Proposed    ->  Policy Evaluated    ->  Native Enforcement  ->  Action
(proposed)            (evaluated)             (enforcement)          (action)
```

plus an `observation` stage for collector lifecycle records
(start/stop/heartbeat).

## Components

### internal/schema
The data model. Three record types, each with a strict `Validate()`:

- `Agent` — identity of a supervised process family (claude-code, codex,
  openclaw, mcp-server, system, unknown) on a supported platform
  (linux, darwin, windows).
- `Event` — one JSONL audit record (`v`, `ts` RFC3339 UTC, `id`,
  `agent_id`, `stage`, `type`, `decision`, `severity`, `summary`,
  optional flat `attrs`). Version 1 is the only supported wire version;
  validation rejects unknown enums, out-of-range severities, zero/absurd
  timestamps, oversized text fields, and malformed IDs.
- `Policy` / `Rule` — declarative matchers over `agent_id`, `type`, and
  the `tool` / `path` / `domain` attributes. Matching operators are
  `equals`, `prefix`, `suffix` (byte-exact, case-sensitive; producers
  normalize before writing attrs). Effects: `allow`, `ask`,
  `would_block` — the Phase 0 vocabulary deliberately contains no
  "block" effect.

### internal/policy
Deterministic evaluator: rules sorted by priority descending, stable on
ties (declaration order), first match wins, otherwise the policy default.
Evaluation never mutates the event.

### internal/auditlog
The single persistence exit of the collector: append-only JSONL, one file,
created `0600`, invalid events rejected before any bytes are written.
This package imports only the filesystem — no socket, dial, or HTTP types
exist anywhere in the collector code path, which is what makes the
"audit data never leaves the machine" claim structural rather than
aspirational. `scripts/tripwire.sh`-adjacent check: see the network-import
grep gate in `scripts/gate-d1.sh`.

### cmd/hello-collector
Single binary wiring the above: constructs the demo policy, registers
itself as the observed agent, produces one full pipeline chain as
validated JSONL events, and exits. `--version` prints name, version,
GOOS/GOARCH and the Go toolchain version.

## Roadmap boundary (what D1 is NOT)

Real process discovery, OS-level hooks, log rotation policy, UI, and any
enforcement are later milestones. The skeleton exists so those can land
as additional event *producers* against a frozen, tested event model.
