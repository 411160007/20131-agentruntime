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
The single persistence exit of the collector: append-only JSONL, created
`0600`, invalid events rejected before any bytes are written. `OpenLog`
adds segment rotation on top of the same writer: size threshold (`MaxBytes`)
and UTC day-cut (`Daily`), both parameterized, with optional history
pruning that only ever removes files carrying a strict rotation stamp.
This package imports only the filesystem — no socket, dial, or HTTP types
exist anywhere in the collector code path, which is what makes the
"audit data never leaves the machine" claim structural rather than
aspirational. `scripts/tripwire.sh`-adjacent check: see the network-import
grep gate in `scripts/gate-d1.sh` and `scripts/gate-d2.sh`.

### internal/discovery
Agent discovery from process-tree snapshots, split so the *matching*
logic is pure and platform-independent while the *sources* are per-GOOS:

- `discovery.go` — fingerprint table (Claude Code, Codex, OpenClaw,
  MCP servers incl. a parent-chain rule for servers orphaned by an
  exited supervisor), ancestor attribution with pid<=1 and cycle-safe
  depth caps, and command-line redaction (`Redact`) applied before any
  OS-derived string reaches the audit file.
- `scan_linux.go` — `/proc` poll (comm, stat ppid, cmdline, exe).
- `scan_darwin.go` — `ps -axo` poll; same user-space KERN_PROC view as
  libproc without cgo. Recorded deviation: libproc native polling is
  the Phase 1 upgrade.
- `scan_windows.go` — Toolhelp32 snapshot + `QueryFullProcessImageNameW`
  + PEB command-line read via `ReadProcessMemory` (pure `syscall`,
  CGO stays off). Recorded deviation: ETW streaming is the Phase 1
  upgrade; polling gives the same view, less freshness.
- `ScanStats` coverage counters (unreadable command lines, attribution
  refusals) flow into every `agent.scan` line — audit honesty includes
  naming the blind spots.

### cmd/hello-collector
Single binary wiring the above: constructs the demo policy, registers
itself as the observed agent, produces one full pipeline chain as
validated JSONL events, and exits. `--version` prints name, version,
GOOS/GOARCH and the Go toolchain version.

### cmd/agent-collector
The real discovery loop: `discovery.Snapshot()` -> `Detector.Scan()` ->
validated `agent.detected` / `agent.scan` observation events ->
rotating `auditlog.Log`. Flags: `--once`, `--interval`, `--out`,
`--rotate-bytes`, `--rotate-daily`, `--keep-history`, `--max-cycles`.
SIGINT/SIGTERM write a `collector.stop` line before exit; every line is
schema-validated and observation-stage only — the Phase 0 vocabulary
discovery emits contains no blocking decision.

## Roadmap boundary (what D1-D2 are NOT)

Real OS event streams (ETW providers, libproc, auditd), hooks/proxies,
enforcement, and UI are later milestones. Discovery here is polling
user-space snapshots: it sees processes after launch and nothing before,
which the audit records honestly via coverage attrs.
