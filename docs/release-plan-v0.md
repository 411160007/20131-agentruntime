# Release Plan v0 — release, canary, and rollback pre-plans (documentation only)

Version lock: this is a **documentation slice**. It ships next to
`docs/distribution.md` (the shipped build/asset contract) and changes no
machine: no script, no schema, no runtime path, no decision vocabulary.
If a future slice implements any machinery described here, that slice —
not this document — owns the new gate.

Status declaration (read this first): everything below is **pre-plan —
not implemented**. There is no canary engine, no automatic rollback, and
no client-side update mechanism in the product. Releases to date follow
`docs/distribution.md` exactly; this document adds a plan shape around
them, honestly labelled per section.

## 1. Scope and ground rules

Two specification clauses drive this document:

- §221 item 21 ("Release Plan") — the first-batch deliverable this file
  lands as the documentation form.
- §225 ("Public Beta", "Canary", "Automatic Rollback") — later-batch
  items that this file pre-pins as plan rows without building them.

Ground rules the plan may never cross (inherited, re-asserted here):

- **Local-first.** A release must not add any network dependency to the
  core path; the shipped binary's closure stays as `docs/distribution.md`
  and the import-closure gate describe it today. Update and rollout
  channels are extensions, never core behavior.
- **No telemetry.** The product reports nothing about its users.
  Therefore every canary signal in §3 must be a *public, aggregate*
  signal (download counters, filed issues, external health probes) —
  the plan may not quietly invent a private signal source.
- **Observation-only runtime.** Phase 0 decisions remain `allow` /
  `would_block`; nothing in a rollout or rollback flow may change the
  decision set or intercept behavior.
- **Honest claims.** A release note may only state what the shipped
  gates actually check; "verified" claims need the gate name behind
  them. No "100% protection" phrasing, ever.

## 2. Release (train shape)

What today's release consists of (shipped facts, restated as a plan):

1. **Version bump**: the repo-root `VERSION` file is the single source;
   the tag `v<ver>`, CI ldflags, artifact names, and `SHA256SUMS.txt`
   lines are all derived from it (machine-checked; see
   `docs/distribution.md`).
2. **Pre-tag gates green**: full CI battery on the release commit —
   build matrix, cross-platform verification, the golden evals runner
   meeting its pinned thresholds, the tripwire negative scan, and every
   named doc↔code sync checker. A red gate stops the train; the train
   never ships with a known-red gate or an undeclared exception.
3. **Assembly**: `scripts/build-dist.sh` with the explicit Intel
   signing branch decision (A or B) from `docs/distribution.md`; only
   `dist/release/` bytes become assets, one `SHA256SUMS.txt` per release.
4. **Publication**: the GitHub release (prerelease flag per §3) plus
   the site download pages and `install.sh` checksum pins updated in the
   same wave; users who verify checksums get exactly the bytes listed.

Status: steps 1–4 are **shipped** as a manual, gate-guarded process.
What is *pre-planned*: nothing new here — this section mainly fixes the
discipline that any later automation must inherit.

## 3. Canary / staged rollout (pre-plan — not implemented)

Constraints first: with no client update channel and no telemetry, a
"canary" cannot mean pushing a build to a slice of installed users. The
only honest staging primitive available is the **release channel flag**:

- **Cohort model**: publish a release marked *prerelease* (the GitHub
  prerelease flag) as the canary stage. Installed users are unaffected
  (nothing auto-updates); early adopters who opt into the prerelease
  asset are the canary cohort by definition. Dogfood runs on the
  project's own machines accompany every stage, as they already do.
- **Stage ladder** (proposed): `prerelease → wait window → full
  release`. The wait window is a time-box (days, not hours — exact
  value to be set at first execution, recorded in the release note),
  during which the §3 signals below are read before promoting.
- **Promotion signals** (all public, zero telemetry):
  - per-asset `download_count` deltas from the GitHub Releases REST API
    (the existing KPI source; no new collection),
  - issue/mention rate on the public repository (a crash claim filed by
    anyone is a stop-the-line signal, triaged honestly),
  - external health of the download path (site pages, asset URLs
    answering, checksums matching — the same read-only probes the
    project already runs),
  - the evals/golden battery and every sync checker still green at the
    promoted commit.
- **Explicit non-goals**: no percentage rollout (impossible without an
  update channel), no server-side gating, no cohort identity. If a
  future hosted/cloud extension (§225 "Cloud Extension") ever ships an
  opt-in update channel, automatic staged rollout becomes *its* design
  problem with *its* honesty review — this plan pre-pins that the
  update channel is opt-in and never silently enabled.

Status: **pre-plan only**. No tooling, no flag automation, no wait
window has ever been executed. First execution requires a decision
record and the full §2 gate battery regardless.

## 4. Rollback (pre-plan — not implemented)

There is no automatic rollback in the product and none is implied by
the word "rollback" here. What this plan defines is a **publication
rollback**: withdrawing a shipped artifact set and repointing users to
the last known-good release.

- **Triggers** (any one is sufficient):
  - a checksum mismatch between a published asset and `SHA256SUMS.txt`,
  - a confirmed hard failure on a shipped platform (install crash,
    `install.sh` fail-closed on good bytes, corrupt archive),
  - a security report showing the release enables harm the threat model
    treats as in-scope,
  - an honest-claims violation discovered post-publication (a release
    note asserting something a gate does not check).
- **Procedure** (manual, CI-gated):
  1. edit the release to mark it *not latest* / annotate the defect
     honestly in the release body (never silently delete assets users
     may have pinned),
  2. repoint the site download pages and `install.sh` version pin to
     the previous tag, rebuild the checksum pins from that tag's
     `SHA256SUMS.txt`,
  3. move `VERSION` forward-only on the main line (a rolled-back
     publication does not rewrite history; the next train carries a
     higher version),
  4. record the trigger, the withdrawn version, and the defect in the
     next release note.
- **Schema-direction honesty**: audit line formats evolve additively
  with `schema_version` handling. A user who rolls their *binary* back
  to an older collector while keeping audit files written by a newer
  version may hit the reject-old-path/unknown-version guard. The plan
  states this as a known limitation: **publication rollback protects
  future installs; it does not promise older binaries read newer data.**
- **Non-goals**: no remote kill switch, no forced downgrade, no
  silent re-release of modified bytes under the same version string
  (a fixed artifact always ships as a new version).

Status: **pre-plan only**. No rollback has occurred to date.

## 5. Freeze windows and emergency path

- **Freeze**: during an announced freeze window no tag is cut and no
  site download pin is moved, except through the emergency path below.
  (This row exists so a future governance decision can point at a
  named mechanism instead of inventing one.)
- **Emergency security release**: a security fix may shorten the train
  (skip the §3 wait window) but may **not** skip any §2 gate — the CI
  battery, tripwire scan, and checksum contracts run in full at the
  fix commit. If gates cannot be green, the correct emergency action
  is the §4 publication rollback, not a rushed forward release.

## 6. Clause cross-reference table

| Clause | Where landed here | Implementation status |
|---|---|---|
| §221 item 21 — Release Plan | §1, §2 (train shape), §5 (freeze/emergency) | documentation form; process shipped, discipline now written down |
| §225 — Public Beta | §3 cohort model (prerelease-as-canary is the beta staging primitive) | pre-plan, not implemented |
| §225 — Canary | §3 whole section | pre-plan, not implemented |
| §225 — Automatic Rollback | §4 (defines publication rollback; *automatic* client rollback is structurally impossible under the no-update-channel/no-telemetry constraints and is therefore NOT promised) | pre-plan, not implemented |
| §225 — Self-Healing | out of scope here; recovery design rows live in `docs/recovery-transaction-design.md` and `docs/recovery-honesty-grading.md` | separate design track |

Every required element of a release plan — versioning source, artifact
contract, gate battery, staging ladder, signals, rollback triggers and
procedure, freeze and emergency rules — has exactly one home section
above, and every section above maps back to a named clause row.

## 7. Known gaps (declared, not hidden)

- Canary signal quality is low by design: public signals only, so a
  canary can miss silent breakage on users' machines. Stated openly
  instead of pretending the cohort model has telemetry-grade power.
- No update channel means rollback reach is future-install-only (§4).
- darwin/amd64 real-hardware verification is still pending
  (`docs/distribution.md`); the Intel branch decision in §2 step 3 is
  the current mitigation, and it inherits that pending state.
- The wait-window duration and first canary execution are undecided
  until an actual release train chooses to run them; this document
  deliberately carries no fabricated numbers.
