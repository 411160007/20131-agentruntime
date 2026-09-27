# 20131 Agent Security Runtime

**Give AI autonomy. Keep control.**

The 20131 Agent Security Runtime observes what your AI agents actually do
on your machine — the commands they propose, the files they touch, the
tools they call — records everything into a local, append-only JSONL audit
log, and evaluates each action against explicit policies you control.

The core model is:

```
Agent Proposed  ->  Policy Evaluated  ->  Native Enforcement  ->  Action
```

AI may act autonomously within what is authorized; the security boundary
is decided by policy, never by the agent itself.

## Local-first guarantees

- **No cloud, no accounts, no telemetry.** The runtime is a single local
  binary; it performs no network I/O of any kind. The audit file on your
  disk is the only output.
- **Audit honesty.** Phase 0 observes and records; it does not block.
  Policy outcomes that *would* block are recorded as `would_block`.
- **Privacy by construction.** Audit files are created with owner-only
  permissions (`0600`).

## Status

Early skeleton. Current contents:

| Path | What it is |
|---|---|
| `cmd/hello-collector` | single-binary skeleton: writes a demo policy-evaluation pipeline to local JSONL |
| `cmd/agent-collector` | discovery collector + read-only control surface (`status`, `audit-tail`); observe-only |
| `internal/schema` | core data model: `Agent`, `Event` (+ tier routing, capability attrs), `Policy`, capability vocabulary; strict validation |
| `internal/policy` | deterministic policy evaluator |
| `internal/discovery` | process-tree agent fingerprints (Claude Code, Codex, OpenClaw, MCP servers) + per-platform snapshot sources |
| `internal/bus` | event bus: multi-source fan-in with tier routing into the single audit pipe; reserved adapter slots |
| `internal/identity` | stable agent ids (machine fingerprint + normalized exe locator) + known/observed/trusted passport states |
| `internal/auditlog` | local JSONL audit writer with size and UTC day-cut rotation |
| `scripts/` | build, gate batteries, license gate, artifact verification, API contract cross-check, negative-scan tripwire |
| `docs/` | architecture notes, the frozen Core API contract (`docs/api-v0.md`), and the dependency license register |
| `testdata/` | pre-tier legacy JSONL fixtures pinning additive zero-breakage of the schema contract |

## Running the collector

```sh
./agent-collector --once --out agent-audit.jsonl      # single scan
./agent-collector --mode observe --interval 60s --rotate-bytes 1048576
```

Detections appear as `agent.detected` lines (kind, rule, pid lineage,
redacted command line) and each pass as one `agent.scan` coverage line.

## Identity and the control surface (read-only)

Discovery sightings carry STABLE ids: `agi-` + a sha256 over the machine
fingerprint and the normalized executable locator (exe path, else
command line, else name), so the same agent keeps its id across scans
and across collector runs. Passport states run
`known -> observed -> trusted`; promotion to `trusted` only ever happens
from an explicit user trust file — never automatically, and the
observation machinery has structurally no transition edge into trusted.

```sh
./agent-collector status --out agent-audit.jsonl [--trust ~/.config/20131/trust.txt]
./agent-collector audit-tail --out agent-audit.jsonl --n 20
```

`--mode observe` is the ONLY mode this release accepts: the runtime
observes, alerts, and audits; it never blocks. `would_block` decisions
are recorded, not enforced. The full frozen field/command contract lives
in [docs/api-v0.md](docs/api-v0.md); machine gates cross-check it
against both implementations.

## Build & run

Requires Go 1.27+.

```sh
go build ./cmd/hello-collector
./hello-collector --version
./hello-collector --out ./agent-events.jsonl   # writes local JSONL events
```

Cross-compile all release targets:

```sh
bash scripts/build-dist.sh
node scripts/verify-cross.mjs
```

## Quality gates

```sh
bash scripts/gate-d1.sh   # skeleton + core model
bash scripts/gate-d2.sh   # discovery slice + rotation
bash scripts/gate-d3.sh   # core completion: bus, capability, identity, control surface, frozen contract
node scripts/licenses-check.mjs --selftest && node scripts/licenses-check.mjs
bash scripts/tripwire.sh --selftest && bash scripts/tripwire.sh
```

CI (`.github/workflows/build.yml`) runs the test suite plus the gates above
and builds + executes `hello-collector --version` on Linux, macOS, and
Windows.

## License

Apache License 2.0 — see [LICENSE](LICENSE) and [NOTICE](NOTICE).
Dependency license policy and the current register:
[docs/licenses.md](docs/licenses.md).
