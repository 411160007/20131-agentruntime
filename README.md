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
| `cmd/agent-collector` | discovery collector: polls the process table, identifies agent families, writes rotating JSONL audit (observe-only) |
| `internal/schema` | core data model: `Agent`, `Event`, `Policy` with strict validation |
| `internal/policy` | deterministic policy evaluator |
| `internal/discovery` | process-tree agent fingerprints (Claude Code, Codex, OpenClaw, MCP servers) + per-platform snapshot sources |
| `internal/auditlog` | local JSONL audit writer with size and UTC day-cut rotation |
| `scripts/` | build, gate batteries, license gate, artifact verification, negative-scan tripwire |
| `docs/` | architecture notes and the dependency license register |

## Running the collector

```sh
./agent-collector --once --out agent-audit.jsonl      # single scan
./agent-collector --interval 30s                       # keep watching until Ctrl-C
./agent-collector --rotate-bytes 4194304 --keep-history 8
```

Detections appear as `agent.detected` lines (kind, rule, pid lineage,
redacted command line) and each pass as one `agent.scan` coverage line.

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
go vet ./... && go test ./...
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
