# Dependency & license register (machine-gated)

Policy: only `MIT`, the BSD family, and `Apache-2.0` licenses may enter the
dependency tree. Copyleft licenses (GPL / AGPL / LGPL / MPL / CC-BY-SA) are
a hard veto for shipped binaries. `scripts/licenses-check.mjs` verifies
this file against `go list -m all` on every CI run.

## Project code

- own modules | license: Apache-2.0 (see LICENSE)

## Toolchain-provided (statically linked, not vendored)

- Go standard library | license: BSD-3-Clause (Go license)

## External module dependencies

(none as of the D1 skeleton — the collector builds with the Go standard
library only; every `- module:` line below must stay reconciled with
`go list -m all` by the license gate)
