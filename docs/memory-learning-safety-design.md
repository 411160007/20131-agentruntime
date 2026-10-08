# Memory Security / Learning Safety Design Surface v0 — design only, no execution plane

Version lock: this document and `scripts/w12-memsec-check.mjs` MUST evolve in
one PR; the checker re-parses the shipped grant-origin enum, the authority
propagation rule, and the enforcement-plane token from
`internal/schema/intentauthority.go`, the decision-cache key constraints from
`internal/schema/decisioncache.go`, and the observation-counter kinds and
scopes from `internal/schema/agencyguard.go` on every run — never against
this file alone.

Stance (Phase 0, inherited from the shipped observation-only charter): this is
a **design surface**. No memory store, no adaptive-learning pipeline, no
quarantine state machine, and no trust-freeze machinery ships behind it.
Enforcement stays at the shipped single spelling, parsed from source and
restated here: `none-in-observation-phase`. What does ship today is the
grounding this design stands on: the closed grant-origin enum and the sticky
untrusted propagation rule in `intentauthority.go`, the context-bound
decision-cache key shape in `decisioncache.go` (spec section 267), and the
three observation counters over the two scopes in `agencyguard.go`
(spec section 261, slice W6.1).

## 0. Gate-assertion source of truth (same-source citation, no second system)

The gate assertion for the learning face was fixed once, in the Phase 1 MCP
gateway proposal, section 4.1 (decision-chain stage map), LEARN row, verbatim:

> 观察学习入口=记录/统计面；**§235 禁学习结果升 Root + §262 记忆≠授权**两铁律入 gate 断言（学习面自动升权路径结构性禁止）

This document is the V12 design-surface landing of exactly that row. It
invents no second gate-assertion system: every check below is the same
two-iron-rule formulation (learning results must not reach Root; memory is not
authorization) expressed as design lines and machine-checkable censuses.

## 1. Memory Security (spec section 262)

The runtime must protect every one of the six memory surfaces — as a closed
set, not an exemplar list:

<!-- closed:surfaces -->
```text
- Conversation Memory
- Task Memory
- Long-term Memory
- Vector Store
- Scratchpad
- Cached Context
```

Every significant memory record carries the seven attributes below at minimum
(closed set):

<!-- closed:attributes -->
```text
- Origin
- Author
- Timestamp
- Trust
- Scope
- TTL
- Integrity
```

The most important principle, verbatim in spirit and pinned in form:
**Memory ≠ Authorization**. Even if a memory record asserts
「用户已经允许访问密码库」, the real Authority and the current Policy must be
re-verified at the moment of use; a quoted claim from memory is evidence of
intent, never a grant.

Grounding in shipped source (parsed, not copied, by the checker): the grant
origin enum is the closed five — `user_direct`, `agent_provided`,
`ui_generated`, `external_model_interpreted`, `runtime_inferred`. **None of
the five is "remembered" or "learned"**: no memory or learning result can be
the origin of a grant, because no such constructor exists in the enum. The
propagation rule `untrusted-sticky-never-auto-escalate` ships as a constant
no caller can edit: untrusted input is sticky and never auto-escalates
authority. Cached Context in particular reuses the section 267 decision-cache
key constraints (agent, task, resource, capability, policy_version, context,
ttl_seconds, confidence_percent) beside the context_epoch invalidation slot:
one grant is never reused indefinitely across unrelated scenes — the same
memory-≠-authorization rule expressed as a cache-key construction rule.

## 2. Learning Safety (spec section 263)

Adaptive Learning may learn exactly the six categories below (closed set):

<!-- closed:learnable -->
```text
- normal work patterns
- anomaly patterns
- compatibility
- user preferences
- workflows
- risk evidence
```

Adaptive Learning may never produce any of the five effects below (closed
set; these are outputs of the learning plane, not inputs it may reason
toward):

<!-- closed:forbidden-effects -->
```text
- modify Hard Deny
- automatically acquire Root
- release the S4 Boundary
- permanently close Anti-Tamper
- automatically delete a Recovery Boundary
```

Every high-risk learning conclusion must walk the explicit eight-stage
promotion pipeline, in order, ending at Promote (closed set; stage names are
title-case because the repo hygiene scanner reserves the all-caps forms of
the decision-chain words):

<!-- closed:promotion-pipeline -->
```text
1. Observe
2. Candidate
3. Probation
4. Shadow
5. Replay
6. Evaluate
7. Validate
8. Promote
```

Design line: promotion is always an explicit pipeline stage with recorded
evidence, never a side effect of observation.

## 3. Learning Quarantine V2 (spec section 264)

Any of the six triggers below moves the learning plane into Learning
Quarantine (closed set):

<!-- closed:quarantine-triggers -->
```text
- burst of many new behaviors in a short window
- mass automatic Allow decisions
- sudden behavior expansion after an Agent Hash change
- capability surge after an MCP / Skill change
- persistent conflict between the learning model and Native Evidence
- abnormal Policy Promotion
```

During Quarantine: **no new behavior may be upgraded directly to high trust.**

Grounding and honest registration: the shipped section 261 observation
substrate — counters `event_rate`, `step_count`, `parallelism` over the
`agent` and `task` scopes — already measures the burst-rate family of
triggers (new-behavior volume, decision-rate anomaly). The hash-change and
MCP/Skill-change detectors, the automatic-Allow volume census, and the
learning-model-versus-native-evidence conflict judge have **no shipped
collector yet** (known_gap, registered honestly here; they are design inputs
of the enforcement phases, not claims of this slice). No second counter may
start in this slice; quarantined observations reuse the W6.1 source only.

## 4. Trust Freeze V2 (spec section 265)

When a learning model is found abnormal, the response sequence is the closed
seven, in order:

<!-- closed:freeze-pipeline -->
```text
1. Freeze
2. Diagnostic
3. Historical Replay
4. Compare Native Evidence
5. Repair Model
6. Re-Evaluate
7. Resume
```

Design line, pinned verbatim: 不得因为学习错误而全局放宽安全边界。— a
learning error never globally relaxes the safety boundary; the blast radius
of a freeze is the learning model itself. "Compare Native Evidence" compares
against the shipped audit surface (the JSONL single-exit decision trace),
never against the learning model's own memory — the section 262 rule applied
to the repair loop.

## 5. Capability Lease V2 cross-reference (spec section 235)

Capability Lease stays the default authorization mechanism with the eleven
fields below (closed set):

<!-- closed:lease-fields -->
```text
- Task ID
- Agent ID
- Capability
- Scope
- Resource
- Conditions
- TTL
- Quota
- Revocable
- Reason
- Authority Source
```

The verbatim prohibition carried from section 235:
「禁止通过"学习结果"把短期 Lease 悄悄升级成永久 Root 权限。」
Design line: the *Authority Source* field of a Lease (or of any renewal)
cannot be filled by a learning output — renewals re-enter through the same
grant-origin enum as fresh leases, which contains no learned origin (§1).

## 6. Structural-absence assertion (the design face of the gate)

Design assertion MA-1: a promotion path from memory or learning results to
Authority or Root **does not structurally exist** — because (a) the grant
origin enum is closed and contains no memory/learning member, (b) leases and
renewals can only be issued through that enum, (c) the propagation rule is a
constructor-pinned constant that never auto-escalates, and (d) every
promotion must walk the explicit eight-stage pipeline whose non-terminal
stages grant no elevated trust. The gate for this surface checks the absence
by construction (census of the enum, of the pipeline, and of this document's
closed sets), not by scanning for bad behavior at runtime.

## 7. Coverage table (machine-checked)

| clause | disposition | design location |
|---|---|---|
| spec section 262 | covered | §1 memory surfaces, seven attributes, Memory ≠ Authorization, origin grounding |
| spec section 263 | covered | §2 learnable/forbidden censuses, eight-stage promotion pipeline |
| spec section 264 | covered | §3 six triggers, high-trust freeze-during-quarantine line, counter grounding with known_gap registered |
| spec section 265 | covered | §4 seven-step freeze pipeline, no-global-relaxation verbatim |
| spec section 235 (cross-reference) | registered | §5 Lease eleven fields, learning-result prohibition verbatim |

## 8. Handoff note to the wave gate (gate-w12 design form)

This slice is a pure design artifact: the wave gate must additionally assert
zero code-plane increment in this slice's diff range (docs/ and scripts/
only). The gate-w12 battery inherits from here: the four clause ↔ design
line reverse-check, MA-1 presence, the same-source proposal citation
(no second formulation), and the closed-set censuses above.
