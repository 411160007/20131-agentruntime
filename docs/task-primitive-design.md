# Task Primitive Design — turn→task mapping v0 (schema proposal wave)

Purpose: give the North Star metric a denominator. `section 295` of the
owner spec names the product North Star **Safe Agent Task Success Rate**
and lists **Task Recovery Success Rate** among the V2 KPIs. Both are
ratios over *tasks*, and the shipped system has no task object: the
event vocabulary is session/turn granularity (`session.start`,
`turn.stop`) and nothing between a turn and a task is recorded. Until a
task exists as a contract, every "success rate" number is unverifiable
by construction — there is no denominator to divide by. This file is
the version-zero contract proposal that closes that gap at the
documentation/schema level only.

This file is machine-checked by `scripts/task-primitive-check.mjs`
(vocabulary reverse-lookup, closed-set censuses, shipped-vs-proposed
isolation, diff-range purity). The checker and this file evolve in the
same PR — a vocabulary change landing in only one of the two is red.

## 1. Non-goals (observation-phase red lines)

- No runtime semantics change. The evaluator, rule engine, bus, and
  audit writer are byte-identical before and after this wave; this file
  emits nothing and forbids nothing.
- No new `EventType` tokens, no edits to the frozen external contract in
  `docs/api-v0.md`. Additive field shapes proposed here are **reserved
  vocabulary** — they describe a future recorder, not the current one.
- No claim that task records ship today. The only honest present-tense
  statement about tasks in this repository is section 8's gap table.
- The Phase 0 decision vocabulary remains the closed set
  `{allow, would_block}` for emitted decisions (`ask` stays reserved
  contract vocabulary); nothing in this file grants `block`/`deny`
  existence to any plane.

## 2. Shipped ground truth this design stands on

Every token marked `[shipped]` below is re-parsed from its source of
truth on each checker run (parsed, not copied):

| Token | Source of truth |
|---|---|
| `session.start` `[shipped]` | `internal/schema/event.go` `AllEventTypes()`, mirrored by `scripts/validate-jsonl.mjs` and `docs/api-v0.md` |
| `turn.stop` `[shipped]` | same three-source set |
| `policy.decision` `[shipped]` | same three-source set |
| `goal` `[shipped]` | `schema-v2.md` `intent_field_vocabulary` first token |
| `allow` / `ask` / `would_block` `[shipped]` | decision vocabulary trio in `internal/schema/event.go` |
| lease auto-revocation on task completion `[shipped spec]` | owner spec `section 24` (Capability Lease: Task 完成 → 自动撤销) |

The `session.start` / `turn.stop` pair is the additive-evolution
precedent this proposal follows: new lifecycle vocabulary was added to
the event enum, the validator mirror, and the contract doc together,
and every older JSONL line still parses unchanged. A future task
field must land the same way — additive under `schema_version`, zero
breakage for old readers, three-source sync tests extended first.

## 3. Task model v0 (closed vocabularies)

A **task** is the unit of agent work the owner spec writes about
(`section 24` leases, `section 120` Task Registry, `section 248`
Task-Level Recovery): one user-intended outcome, bounded in time,
scoped by leases. v0 defines it as a derived object, not an emitted
record (see section 5).

```taskv0
task_status_vocabulary: open, active, completed, failed, abandoned, blocked, unknown
```

Exactly seven terminal-or-live states. No synonyms, no invented extra.

- `open` — boundary evidence exists that work started; no turn closed yet.
- `active` — at least one `turn.stop` `[shipped]` falls inside the interval.
- `completed` — closing evidence present: an explicit terminal hint, or a
  lease revocation `section 24` observed after work, with no unresolved
  finding.
- `failed` — completed control flow whose outcome evidence contradicts
  the recorded `goal` `[shipped]` intent line.
- `abandoned` — session ended (`turn.stop` run ends at stream end)
  without terminal evidence.
- `blocked` — a `would_block` `[shipped]` decision is the last
  in-scope decision for the interval and no later allow resolves it.
- `unknown` — the default when evidence is insufficient to choose any
  other state. Honesty rule: `unknown` is never silently promoted.

```taskv0
boundary_signal_vocabulary: session_start_edge, turn_stop_sequence, goal_hint, lease_expiry_edge
```

Exactly four boundary signals. First two are `[shipped]` event tokens
(`session.start`, `turn.stop`) observed directly; `goal_hint` keys on
the shipped intent-record first field; `lease_expiry_edge` is
`[proposed]` — reserved vocabulary for the future recorder, asserted
absent from every shipped source by the checker, and never emitted.

## 4. Turn→task mapping v0 (deterministic, replayable, derive-only)

Five rules. They define one function from an existing JSONL stream to
task intervals **computed at read time** — nothing here writes to the
stream, so old files keep their exact bytes and meaning.

- `MR-1` (interval seeding): each `session.start` `[shipped]` opens one
  task interval in state `open`; intervals are per-session, ordered by
  stream position.
- `MR-2` (turn folding): each `turn.stop` `[shipped]` inside an open
  interval moves it to `active` and appends its identity to the
  interval's turn list (the turn list is the join key for later
  evidence: decisions, recovery records, and eval lines all carry the
  same event references).
- `MR-3` (terminal hints): an in-scope `goal` `[shipped]` intent line
  whose text carries an explicit completion statement, or an observed
  lease-revocation record `section 24`, closes the interval as
  `completed` candidate; a candidate is confirmed only with outcome
  evidence, else it stays `unknown` — completion is earned, not
  assumed. Outcome evidence has two sources: `later_turn` (a turn
  appended after the candidate position) and `eval_assertion` (one or
  more in-scope turns covered by assertion rows whose expectation
  equals the observed decision, with zero conflicts). A conflicting
  assertion never promotes. Both sources are view-layer readings over
  shipped records; the provenance tag is emitted by the mapper only,
  never into any stream.
- `MR-4` (safety tail): if the last in-scope `policy.decision`
  `[shipped]` is `would_block` `[shipped]` and no later decision in the
  interval resolves it, the interval is `blocked`. `allow` decisions
  never move a task out of `blocked` by themselves (Phase 0 records no
  enforcement, so absence of a block is not proof of success).
- `MR-5` (stream end): a session whose last event is a `turn.stop`
  `[shipped]` with no terminal evidence maps to `abandoned`; a session
  opened but never turned maps to `unknown`, not to zero.

Determinism contract: replaying the same stream through `MR-1`..`MR-5`
yields byte-identical task intervals. Task identity is positional:
`task_id = session_id + ":" + ordinal` — no content hashes of
mutable text, so replay never renumbers.

## 5. Event-layer additive field shapes (reserved, not emitted)

The future recorder (a later slice, gated by this contract) would add
per-event additive fields under the same `schema_version` discipline
as the `session.start` / `turn.stop` precedent:

```taskv0
task_field_vocabulary: task_id, task_status, task_boundary, task_refs
```

Exactly four reserved field names, all `[proposed]`:

- `task_id` — the `MR` positional identity of section 4.
- `task_status` — one of `task_status_vocabulary`, seven tokens only.
- `task_boundary` — which `boundary_signal_vocabulary` token justified
  the state transition.
- `task_refs` — the turn-list join key (opaque reference list).

Old-reader compatibility predicate (the W1.2 precedent restated as a
rule, not a slogan): an event line missing all four fields must
validate, evaluate, and replay exactly as before; a line carrying them
must validate under the same grammar with additive-only acceptance; no
existing token is renamed or re-grammared. Until the recorder slice
lands, the correct way to obtain task views is the derive-only mapper
of section 4 over the shipped stream.

## 6. Denominator and North Star reading

`Safe Agent Task Success Rate` v0 reading, stated as a contract:

```taskv0
denominator_rule: every task interval reaching any terminal state counts once in the denominator, including terminal unknown
```

- `DR-1`: denominator = terminal intervals (`completed`, `failed`,
  `abandoned`, `blocked`, `unknown`) over all mapped intervals per
  replay window; live intervals (`open`, `active`) are excluded and
  their count is reported beside the ratio, never merged into it.
- `DR-2`: numerator = `completed` intervals only, and only those with
  outcome evidence — the ratio's ceiling equals the `unknown` share
  until evidence coverage grows, and that ceiling is reported.
- `DR-3`: the per-window reading ships with its honesty envelope:
  total intervals, live count, unknown count, blocked count. A ratio
  printed without the envelope is treated as an unverifiable number
  (same posture as the golden-eval summary lines).

`Task Recovery Success Rate` reuses the same denominator discipline:
a task with a recovery record whose terminal state is `completed`
counts recovered; recovery never promotes `unknown` by itself.

## 7. Known gaps (what the next slices must build)

| Gap | Why v0 does not close it | Landing shape |
|---|---|---|
| boundary detection beyond session/turn | `lease_expiry_edge` is reserved; no shipped token exists to key on | recorder slice after this contract |
| outcome evidence sources | join v0 landed in the derive-only mapper: later-turn and eval-assertion evidence both promote, conflicts never; coverage grew with the shipped join corpus (testdata/join/hint-pair.jsonl with its own label ledger): asserted turns now meet hinted intervals on on-disk data and both evidence sources promote from disk, machine-pinned in the mapper selftest; the intent recorder slice landed the schema side of widened hint capture (typed `intent.record` event, additive v1 vocabulary, goal required by the validator on both mirrors); the corpus migration landed: the normal rules-corpus stream carries a typed `intent.record` hint inside its scripted session and the five-source census families moved in one lockstep PR, so the rules-corpus printed ratio earns a non-zero numerator with its envelope (machine-pinned in the mapper selftest); the danger stream holds no session intervals and stays hint-free by construction | corpus migration (five sources in one PR, W3 lockstep precedent) — landed |
| intent↔task binding strength | `goal_hint` keys on intent first field; free-text completion statements are weak evidence; the typed `intent.record` line gives the recorder an emittable form and the normal rules-corpus stream now carries one on disk; the binding stays positional-v0 (a hint binds the last-opened interval of the same stream) | binding upgrade reserved for the recorder slice |
| recovery join | `section 248` task-level recovery needs `task_refs` emitted | Phase 1 enforcement wedge plan |
| multi-agent task sharing | one lease per agent per task assumed; shared tasks unmodeled | deferred, no v0 pretense |

## 8. Version lock and gap registration (machine-pinned anchors)

- version lock: this document and `scripts/task-primitive-check.mjs`
  are one contract; both carry this exact sentence.
- known gap registration (verbatim posture): the North Star ratio
  denominator does not exist in shipped code today; this file is the
  contract proposal that makes it buildable, not an implementation.
- no second system: task identity uses the shipped session/turn
  identifiers; nothing here invents a parallel session model.
- vocabulary census v0: `task_status_vocabulary` seven tokens,
  `boundary_signal_vocabulary` four tokens, `task_field_vocabulary`
  four tokens, mapping rules `MR-1`..`MR-5` five rules, denominator
  rules `DR-1`..`DR-3` three rules. Any census drift is red.
- evidence provenance tags (`later_turn`, `eval_assertion`, `none`) are
  view-layer metadata of the mapper output, not a fifth vocabulary;
  they are never written into any stream and no second copy of an
  assertion corpus may live in tool source.
