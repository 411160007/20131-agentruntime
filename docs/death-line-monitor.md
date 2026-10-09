# Death-Line Monitor — survival criteria, reading sources, weekly ledger

Status: canonical for the observation phase. The table in §2 is the single
source of truth for the survival decision; `scripts/deathline-readings.mjs`
consumes the constants below and `scripts/deathline-check.mjs` enforces that
this document and those constants stay in sync (doc↔code gate, machine-checked
in every slice). No number in the ledger is ever hand-typed.

## 1. Clock

T0 = the first public release stamp **v0.5.0**, published **2026-09-28**.
Machine line (parsed by the checker, keep shape): T0 = 2026-09-28
Machine-verifiable source: the releases API `published_at` field of the
v0.5.0 tag. The survival clock counts calendar days from T0.

## 2. Criteria table (machine-parsed — keep row shapes stable)

| window   | deadline   | metric rule 1     | metric rule 2    | metric rule 3   | continuation rule |
|----------|------------|-------------------|------------------|-----------------|-------------------|
| window A | deadline 2026-10-28 | downloads >= 1000 | uv >= 5000 | stars >= 100 | hits >= 2 |
| window B | deadline 2026-11-27 | stars >= 300 | engagement >= 15 | intent >= 50 | hits >= 1 |

- **Window A (T0+30)**: any two of the three metric rules met at the deadline
  ⇒ the project continues; fewer than two ⇒ the project stops active
  development and drops to maintenance mode (the repository stays public;
  maintenance carries no new build slices).
- **Window B (T0+60)**: at least one rule met ⇒ the paid-surface slices may
  open. Window B is never evaluated before its deadline.
- `engagement` = external issues + external pull requests + community
  mentions counted on public surfaces; `intent` = signup/waitlist-to-paid
  intent signals recorded in the external funnel ledger. Neither is collected
  by the repository tooling yet; both are reported as UNAVAILABLE until a
  machine source exists.

## 3. Metric sources (read-only, free, no credentials required)

| metric    | source                                             | notes |
|-----------|----------------------------------------------------|-------|
| downloads | GitHub releases API, per-asset `download_count` summed over all releases | the only free machine source for download truth; asset-level, never proxy counts |
| stars     | GitHub repository API `stargazers_count`            | point-in-time snapshot |
| uv        | external site traffic ledger, injected via `--uv`   | if no machine source exists, the row records `UNAVAILABLE` + `NA` — an honest absence, never a fabricated zero |

## 4. Weekly reading ledger

File: `docs/death-line-ledger.csv`, append-only, one row per reading date:

```text
date,downloads,stars,uv,uv_status,hits_a,verdict_a
```

- `date` — UTC day of the reading (YYYY-MM-DD), strictly ascending, unique.
- `downloads` / `stars` — live API readings at that moment.
- `uv` / `uv_status` — either an integer with `MEASURED`, or `NA` with
  `UNAVAILABLE`. The pairing is enforced by the checker.
- `hits_a` — how many of the three window-A rules the reading meets; derived
  by the tool from the constants in §2, never entered by hand.
- `verdict_a` — `PASS` when `hits_a >= 2` (on track at that reading), `FAIL`
  otherwise. Provisional: the binding decision is made on the deadline day
  from the last reading taken on or before it.

Collect a reading (requires network, read-only):

```sh
node scripts/deathline-readings.mjs --repo <owner>/<name> [--uv <int>] [--date YYYY-MM-DD] [--dry-run]
```

Verify the whole surface (works offline, runs in every slice gate):

```sh
node scripts/deathline-check.mjs            # doc↔code sync + ledger consistency
node scripts/deathline-check.mjs --selftest # injected-corruption controls
node scripts/deathline-readings.mjs --selftest
```

## 5. Honesty rules (binding)

1. A metric with no machine source is recorded as UNAVAILABLE; it never
   counts as a hit and never counts as a zero.
2. The ledger may only grow or be re-read the same day with an explicit
   `--force` replace; back-dating and silent edits are checker failures.
3. Readings are snapshots, not averages; the deadline verdict uses the last
   row on or before the deadline day.
4. The weekly cadence rides on existing check-in shifts; this slice adds no
   new scheduling surface.
