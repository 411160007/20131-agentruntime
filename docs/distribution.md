# Distribution

How release artifacts for the 20131 Agent Security Runtime are built,
named, signed, shipped, and verified.

## Version stamp — one source

The repo-root `VERSION` file is the single source of the release stamp.
The git tag (`v<ver>`), the CI workflow, the `-ldflags -X main.version=...`
build flag, every artifact file name, and every line of the release
`SHA256SUMS.txt` are derived from that one string. CI fails if a `v*` tag
does not equal `v$(cat VERSION)`.

## Artifact name contract

All shipped binaries follow:

```
<name>-<version>-<os>-<arch>[.exe]
```

Release assets add one packaging cap on top of the same name:

- linux / darwin: `<name>-<version>-<os>-<arch>.tar.gz`
- windows: `<name>-<version>-windows-amd64.exe.zip`

Each archive embeds the binary (under its contract name), `LICENSE`,
`NOTICE`, and a `CHECKSUM.txt` holding the binary's SHA-256. Outside the
archives, one `SHA256SUMS.txt` per release lists every release asset.
The older bare form (binary name plus a lone OS suffix, no version and
no architecture) is retired; the release
name-contract gate greps every tracked file and asserts it has zero
residue — including in documentation prose about the retirement itself.

## Platform matrix

| Target | Status | Notes |
|---|---|---|
| linux/amd64 | shipped | |
| linux/arm64 | shipped | |
| darwin/arm64 (Apple Silicon) | shipped — primary mac target | Go's linker embeds an ad-hoc code signature automatically (machine-checked: `LC_CODE_SIGNATURE` present in the built binary) |
| darwin/amd64 (Intel) | shipped via the CI signing branch | Go does **not** sign amd64 cross builds; a macOS CI runner adds an ad-hoc `codesign -s -` signature. If that branch goes red the release omits Intel builds with an honest note. Intel compatibility on real hardware is still pending verification. |
| windows/amd64 | shipped | |

## Signing honesty (read this before first run)

We ship **ad-hoc signatures only**. There is no paid Developer ID and no
App Store notarization, so platform trust prompts apply:

- **macOS (Sequoia and later):** a freshly downloaded binary is quarantined
  and Gatekeeper will refuse the first launch. The supported path: open it
  once, then System Settings → Privacy & Security → scroll to the blocked
  item → **Open Anyway** → confirm. This creates a persistent exception.
  Running from Terminal after removing quarantine
  (`xattr -d com.apple.quarantine <file>`) is a community-reported path,
  not an official one; verify against your current OS version.
- **Windows:** unsigned builds may trigger SmartScreen ("More info" →
  "Run anyway"). Newer Windows versions with Smart App Control can block
  unsigned software outright, and download reputation can reset. In managed
  enterprise environments policy may block execution entirely.
- Ad-hoc signatures prove the bytes were not modified after signing; they
  vouch for **nothing** about the publisher. That is why every release ships
  a `SHA256SUMS.txt` — verify the hash of what you downloaded against this
  repository before running it.

## Verifying a download

```sh
sha256sum -c SHA256SUMS.txt        # linux; on macOS: shasum -a 256 -c
# windows:  certutil -hashfile <file> SHA256   (compare against SUMS)
tar -tzf <asset>.tar.gz            # expect: binary, LICENSE, NOTICE, CHECKSUM.txt
cd <extracted> && sha256sum -c CHECKSUM.txt
```

`scripts/macho-sig-check.mjs` is the in-repo detector used by the release
gate to machine-assert the signature load command on Mach-O artifacts.

## Cross-surface sync gate

The prose above, `VERSION`, `.github/workflows/build.yml`, and
`scripts/build-dist.sh` must never drift apart, and a published release's
asset names plus the website download page must match this contract
exactly. `scripts/releaseleg-check.mjs` machine-asserts all of it
(documented matrix == builder target set, CI matrix ⊆ documented matrix,
single-stamp consumption everywhere, name-contract tokens everywhere,
signing branch A/B shapes intact, live asset set == expected set with
the Intel pair all-or-none, download page links == live set). Run it
with `--assets <release.json>` (and optionally `--site <page.html>`)
after every release assembly; `--selftest` proves the gate catches
injected drift.
