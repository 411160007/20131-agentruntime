# Red-Team Plan v0 — the ten attack families, planned against the shipped threat model

Version lock: this plan, `docs/compatibility-matrix.md`,
`testdata/golden/compatibility-matrix.json` and
`scripts/compat-redteam-check.mjs` MUST evolve in one PR; the checker
re-derives the threat-row census from `docs/threat-model.md` and
cross-checks this plan in both directions on every run.

Status declaration (read this first): this is **planning only — the
engine is not built**. The document carries zero interception, zero
decision intervention, and zero new attack surface. Like the threat model
it extends, it deliberately **contains no attack procedures**: no exploit
steps, no bypass recipes, no payloads — only which shipped detection
shapes a future exercise would aim at and what a pass or miss would mean.

## Ground rules

- Phase 0 boundary: every current detection is a would_block record; the
  runtime observes, warns and audits — it never stops anything. A red-team
  engine may only be built behind an explicit phase promotion, and this
  plan pre-pins that gate instead of crossing it.
- No marketing: a passing exercise is reported as coverage of the named
  rows only; it never upgrades the enforcement-plane token
  (none-in-observation-phase) and never generalizes into an
  all-platforms-equal promise. Results land in the evals ledger with
  honest miss accounting, following the declared known_gap posture of
  the evals-coverage document (a miss is recorded as a miss).
- Ten families closed set: the simulation families below mirror the
  specification's red-team family list one-for-one; adding an eleventh
  family means upgrading this document, the checker census, and the
  machine mirror in the same PR.

## Simulation subjects (the seven shapes each exercise should reproduce)

- SIM-1: a normal agent behaving legitimately (false-positive floor)
- SIM-2: a malicious agent with hostile intent from startup
- SIM-3: a hijacked agent whose control flow is turned by outside pressure
- SIM-4: an injected agent whose context carries embedded instructions
- SIM-5: a malicious skill offered to the agent
- SIM-6: a malicious MCP server on the relayed transport
- SIM-7: a compromised tool already trusted by the agent

Every family exercise must run against at least one malicious/hijacked/
injected subject shape and against SIM-1 as the negative control; a family
that only ever fires on SIM-1 is a false-positive finding, not a win.

## The ten families, each bound to shipped threat rows

### RT-01 Prompt Injection

Aim: instruction-bearing content steering an agent into shipped-detection
territory. Threat rows: TM-05 (a tool exposing raw code evaluation to
model-supplied input) plus, as of the threat-model v1 uplift, TM-14
(untrusted content treated as instruction) and TM-15 (injection
escalating into goal hijacking chain). Plus the injected subject shapes
(SIM-4/SIM-5).
Expected shipped signal: mcp.eval would_block records and inflow-source
narrowing marks on the audit stream. Pass means the record appears; miss
means the injection path reaches no named detection and is recorded as a
gap against this row, not smoothed.

### RT-02 Tool Abuse

Aim: legitimate tools driven to destructive or fetch-and-run shapes.
Threat rows: TM-02 (bulk destruction), TM-06 (fetch-and-run execution).
Expected signal: destroy.rmrf / destroy.disk / exec.remotepipe would_block
records from command shapes on hook/MCP slots and discovered cmdlines.
Known degradation: the cmdline-visibility limits named by both matrices
apply — an exercise result must state which axis it exercised.

### RT-03 MCP Abuse

Aim: the relay itself as the attack surface — malformed pairing, wedged
observer, server behavior outside its declared shape. Threat rows: TM-05
with the MCP-server detection limits named in the coverage matrix.
Expected signal: completed-record observation survives (pairing, wedged
observer declared); a lost record under attack is a miss recorded against
this row.

### RT-04 Credential Access

Aim: reads against ssh keys, cloud credential stores, browser vaults,
dotenv files. Threat rows: TM-01. Expected signal: cred.ssh / cred.aws /
cred.browser / cred.dotenv would_block records. The file-access axis is
MONITOR ONLY (agent-reported inflow only) — the plan states up front that
an exercise routed below the report line is unobservable by design and
belongs in the gap ledger.

### RT-05 Data Exfiltration

Aim: moving collected secrets out. Threat rows: TM-04 (outbound beyond
the allow list), TM-16 (egress re-check avoidance, threat-model v1
uplift), TM-01 upstream. Expected signal: net.egress would_block
records on declared network.intent events. Constraint named: no shipped
socket enumerator exists, so egress coverage rides declared intents only;
an exercise that routes through the OS without declaring intent is a
recorded structural gap, not a detection failure to hide.

### RT-06 Privilege Escalation

Aim: writes toward privileged configuration drop-ins. Threat rows: TM-03.
Expected signal: path.sudoers would_block records on hook-slot file
events. Full token/session escalation semantics stay on the threat
model's known_gap wording (later-phase scope) and the plan inherits that
honest boundary verbatim.

### RT-07 Process Pivoting

Aim: an agent family claimed while executing from a world-writable
directory; pre-existing or protected processes as blind spots. Threat
rows: TM-07 (masquerade), TM-10 (observation blind spots, bounded and
declared on every collector start line). Expected signal: agent.masquerade
records from exe-path checks; blind-spot subjects are exercised to
confirm the declared accounting (hidden/no-cmdline counters), and a blind
spot that is not counted is a miss.

### RT-08 Bypass Attempts

Aim: going after the runtime's own trail and the collector. Threat rows:
TM-08 (audit evasion), TM-09 (collector tampering, declared known_gap —
no external attestation of the collector binary ships in Phase 0), TM-12
(clearing platform-native logs; the windows event-log clearing case stays
a declared known_gap honestly counted as a miss). Expected signal:
audit.tamper records for our own path; OS-native log surfaces are
partially unobservable and the exercise reports that as gap coverage,
not silence.

### RT-09 Policy Confusion

Aim: contradictory or overlapping rule shapes that make the deterministic
evaluator pick the wrong reading. Threat rows: declared_known_gap — no
shipped threat row covers rule-conflict semantics yet; the nearest
shipped shapes are the reserved-slot split between declared intents
(hook/MCP) and process discovery behind TM-04, and the closed decision
vocabulary. First action for this family was not an exercise but a
proposed threat row: TM-13 shipped in the threat model at this wave
close (W11.5 uplift), recorded here without pretending current
detection coverage — the row is an explicit known_gap.

### RT-10 Capability Escalation

Aim: an agent growing its effective reach step by step within permitted
surfaces. Threat rows: TM-11 (local resource exhaustion, declared
known_gap — v0 rules stay in the pure-match fast path; resource telemetry
is later-phase scope), plus TM-17 (intent hijacking / intent deviation,
threat-model v1 uplift — same later-phase boundary). Expected signal today: the chain-dimension
observation vocabulary marks escalation shapes; nothing intercepts, and
the plan says so in every report template.

## Cadence and engine gate

- The specification's cadence (every version ships with an automatic red
  team pass) is a plan target, not a shipped fact: the engine does not
  exist yet. When a phase promotion authorizes it, the engine consumes
  exactly these ten families and the seven subject shapes, writes
  would_block-style records into the same audit stream, and its results
  join the evals ledger with the same honest-miss accounting.
- Until then, the shipped partial carriers are named honestly: the
  deterministic rule table's own test battery, the replay surface, and
  the CI three-platform build+test jobs. None of them is a red-team
  engine and none may be presented as one.
- Bidirectional gate: this plan references shipped threat rows and every
  shipped threat row (TM-01 through the current census) is referenced by
  at least one family; the checker fails the tree on either direction's
  drift, the same discipline the threat model itself enforces.
