#!/usr/bin/env bash
# gate-w8.sh - machine-assertion battery for the W8 storage governance wave.
#
# First edition. It pins what the wave has actually shipped and nothing more:
# slice W8.1 time + capacity dual limit (section 268/269/270: five-tier
# retention windows over a total-capacity hard ceiling, critical-classified
# segments held out of capacity eviction, the over_quota_critical_only honesty
# watermark, the five-band record-only pressure ladder with zero write-path
# wiring, the inherited empty-head exemption, and unreadable segments held
# rather than guessed) and slice W8.2 per-agent / per-task storage quota
# observation (section 271: shared-source recount of already-durable audit
# lines, per-scope ceilings never averaged, stated task ids only, undeclared
# ceilings reported as a known gap instead of a fabricated zero, critical
# conservation plus the aliased watermark token, and record-only responses
# that never block a write), plus slice W8.3 the user-visible read-only
# storage census (section 273: eight closed members in specification order,
# a read-only handle that never creates and never advances the audit plane,
# absent members stated instead of zeroed, retention and band lines aliased
# from the shipped section 26 vocabularies, per-scope lines kept in two
# separate dimensions, and no write API anywhere on the surface). The
# wave-close hard judgement line of the taskbook is gate-w8 rc=0, and the
# section 304 Storage Usage review line is carried in the leg report, not
# hung here (its collector dependency stays recorded as-is in the gap
# matrix).
#
# Fleet counts are read from the shipped checker at run time rather than
# copied into this file, so a mutation added by a later leg cannot pass a
# stale banner number.
#
# Every step runs against real repo artifacts and carries its own
# control. Any red exits non-zero.
set -euo pipefail
cd "$(dirname "$0")/.."

export TMPDIR="$(pwd)/.tmptest"
mkdir -p "$TMPDIR"
find "$TMPDIR" -mindepth 1 -maxdepth 1 -name 'go-build*' -exec rm -rf {} + 2>/dev/null || true
command -v go >/dev/null 2>&1 || export PATH="$HOME/.local/go/bin:$PATH"
LOGDIR=/home/node/gate-logs
mkdir -p "$LOGDIR"

step() { printf '\n=== %s ===\n' "$*"; }

step '00 preflight: W8.1, W8.2 and W8.3 surface repo-visible with eleven storage keys over section 26, ten quota keys over section 27, and eight storage-CLI keys over section 28'
test -f internal/auditlog/governance.go || { echo 'RED: W8.1 surface missing'; exit 1; }
test -f internal/auditlog/quota.go || { echo 'RED: W8.2 surface missing'; exit 1; }
test -f internal/auditlog/governance_test.go || { echo 'RED: W8.1 tests missing'; exit 1; }
test -f internal/auditlog/quota_test.go || { echo 'RED: W8.2 tests missing'; exit 1; }
test -f scripts/gate-w8.sh || { echo 'RED: this gate not on disk'; exit 1; }
test -f internal/auditlog/storageview.go || { echo 'RED: W8.3 census surface missing'; exit 1; }
test -f cmd/agent-collector/storage.go || { echo 'RED: W8.3 subcommand missing'; exit 1; }
test -f cmd/agent-collector/storage_test.go || { echo 'RED: W8.3 tests missing'; exit 1; }
grep -q '^## 26. Storage governance V2: time + capacity dual limit' docs/schema-v2.md \
  || { echo 'RED: section 26 header missing'; exit 1; }
grep -q '^## 27. Per-agent / per-task storage quota observation' docs/schema-v2.md \
  || { echo 'RED: section 27 header missing'; exit 1; }
grep -q '^## 28. Storage occupancy read-only surface' docs/schema-v2.md \
  || { echo 'RED: section 28 header missing'; exit 1; }
s=$(grep -cE '^storage_[a-z_]+: ' docs/schema-v2.md)
[ "$s" -eq 11 ] || { echo "RED: storage rule key census $s/11"; exit 1; }
q=$(grep -cE '^quota_[a-z_]+: ' docs/schema-v2.md)
[ "$q" -eq 10 ] || { echo "RED: quota rule key census $q/10"; exit 1; }
c8=$(grep -cE '^storagecli_[a-z_]+: ' docs/schema-v2.md)
[ "$c8" -eq 8 ] || { echo "RED: storage CLI rule key census $c8/8"; exit 1; }
# the honesty watermark is one token in two dialects: W8.1 emits the capacity
# line, W8.2 aliases it, and the structural pin forbids a second spelling.
grep -q '^storage_govern_action_vocabulary: .*over_quota_critical_only$' docs/schema-v2.md \
  || { echo 'RED: storage watermark token missing from the action vocabulary'; exit 1; }
grep -q '^quota_watermark_token: over_quota_critical_only$' docs/schema-v2.md \
  || { echo 'RED: quota watermark alias not pinned'; exit 1; }
echo "W8 SURFACE VISIBLE (governance + quota + storage census with tests, three headings, eleven plus ten plus eight keys, watermark aliased not re-spelled)"

step '01 v2-check full battery including the two W8 predicates'
node scripts/schema-v2-check.mjs > "$LOGDIR/gate-w8-v2check.out" 2>&1 \
  || { tail -40 "$LOGDIR/gate-w8-v2check.out"; echo RED: schema-v2-check; exit 1; }
grep -q 'SCHEMA-V2: ALL GREEN' "$LOGDIR/gate-w8-v2check.out" \
  || { echo 'RED: v2-check banner missing'; exit 1; }
grep -q 'function storageGovProblems' scripts/schema-v2-check.mjs \
  || { echo 'RED: storage predicate definition lost'; exit 1; }
grep -q 'function quotaProblems' scripts/schema-v2-check.mjs \
  || { echo 'RED: quota predicate definition lost'; exit 1; }
grep -q 'const sg = storageGovProblems(v, sgSrc)' scripts/schema-v2-check.mjs \
  || { echo 'RED: storage predicate not wired into the run'; exit 1; }
grep -q 'const qp = quotaProblems(v, quotaSrc)' scripts/schema-v2-check.mjs \
  || { echo 'RED: quota predicate not wired into the run'; exit 1; }
grep -q 'function storageViewProblems' scripts/schema-v2-check.mjs \
  || { echo 'RED: storage census predicate definition lost'; exit 1; }
grep -q 'const sv = storageViewProblems(v, svSrc, storCliSrc)' scripts/schema-v2-check.mjs \
  || { echo 'RED: storage census predicate not wired into the run'; exit 1; }
echo 'V2-CHECK FULL RUN GREEN (storage governance, quota, and storage census predicates defined and wired)'

step '02 selftest positive controls: every mutation caught, fleet size read from the shipped checker'
node scripts/schema-v2-check.mjs --selftest > "$LOGDIR/gate-w8-selftest.out" 2>&1 \
  || { tail -20 "$LOGDIR/gate-w8-selftest.out"; echo RED: selftest; exit 1; }
banner=$(grep -m1 'SELFTEST OK' "$LOGDIR/gate-w8-selftest.out" || true)
[ -n "$banner" ] || { echo 'RED: selftest banner missing'; tail -5 "$LOGDIR/gate-w8-selftest.out"; exit 1; }
fleet=$(printf '%s' "$banner" | tr -cd '0-9' | cut -c1-4)
# mechanical cross-check: the banner count must equal the case count in the
# shipped cases block, so this gate cannot be satisfied by a stale number.
cases=$(awk '/const cases = \[/,/^  \];/' scripts/schema-v2-check.mjs | grep -cE '^ *\[')
[ "$fleet" -eq "$cases" ] || { echo "RED: fleet banner $fleet != cases block $cases"; exit 1; }
[ "$fleet" -ge 69 ] || { echo "RED: fleet $fleet, want at least 69 (W8 floor)"; exit 1; }
MISSES=$(grep -c 'SELFTEST MISS' "$LOGDIR/gate-w8-selftest.out" || true)
[ "$MISSES" -eq 0 ] || { echo "RED: $MISSES selftest mutations missed"; exit 1; }
echo "SELFTEST $fleet/$cases CAUGHT (banner derived from the shipped cases block)"

step '03 Go: named W8.1 and W8.2 batteries (full-package regression rides the CI matrix)'
go test -count=1 ./internal/auditlog -run 'TestRetention|TestCapacity|TestPressure|TestObserveRecordOnly|TestTotalBytes|TestUnreadable|TestWriteNeverBlocked|TestEmptyHead' -v > "$LOGDIR/gate-w8-gov.out" 2>&1 \
  || { tail -30 "$LOGDIR/gate-w8-gov.out"; echo RED: W8.1 governance battery; exit 1; }
for t in TestRetentionPrunesExpiredWindowsOnly TestCapacityPressureHoldsCriticalPositiveControl TestPressureBandLadderBoundaries TestObserveRecordOnlyAndQuotaAbsence TestTotalBytesMatchesDisk TestUnreadableHeldAndForeignUntouched TestWriteNeverBlockedByPressure TestEmptyHeadExemptionInherited; do
  grep -q "^--- PASS: $t" "$LOGDIR/gate-w8-gov.out" || { echo "RED: $t did not pass"; exit 1; }
done
echo 'W8.1 GOVERNANCE BATTERY GREEN (retention prunes expired windows only, capacity pressure holds critical as positive control, five-band ladder boundaries, record-only observation, du source against disk, unreadable held and foreign untouched, writes never blocked, empty head never pruned)'

go test -count=1 ./internal/auditlog -run 'TestQuota' -v > "$LOGDIR/gate-w8-quota.out" 2>&1 \
  || { tail -30 "$LOGDIR/gate-w8-quota.out"; echo RED: W8.2 quota battery; exit 1; }
for t in TestQuotaVocabulariesStaySyncedWithSchema TestQuotaEventRatePositiveAndNegative TestQuotaTaskAttributesStatedIdOnly TestQuotaUndeclaredCeilingIsKnownGapNotZero TestQuotaWindowNotStatedRateNotComputed TestQuotaCriticalConservationAndWatermark TestQuotaMalformedSegmentHeldNotGuessed TestQuotaObservationNeverBlocksWrites; do
  grep -q "^--- PASS: $t" "$LOGDIR/gate-w8-quota.out" || { echo "RED: $t did not pass"; exit 1; }
done
echo 'W8.2 QUOTA BATTERY GREEN (vocabularies in sync, per-scope positive and negative controls, stated task ids only, undeclared ceiling is a known gap not a zero, rate omitted when the window is not stated, critical conservation with the watermark, malformed segments held, observation never blocks a write)'

go test -count=1 ./cmd/agent-collector -run 'TestStorage' -v > "$LOGDIR/gate-w8-storage.out" 2>&1 \
  || { tail -30 "$LOGDIR/gate-w8-storage.out"; echo RED: W8.3 storage battery; exit 1; }
for t in TestStorageCensusMatchesDiskSum TestStorageGoldenRenderedLines TestStorageCensusIsReadOnlyBeforeAndAfter TestStorageAbsentMembersStatedNotZeroed TestStorageUndeclaredCapacityLeavesBandUncomputed TestStorageScopeCensusKeepsDimensionsSeparate TestStorageMemberOrderLockedToRegistration; do
  grep -q "^--- PASS: $t" "$LOGDIR/gate-w8-storage.out" || { echo "RED: $t did not pass"; exit 1; }
done
echo 'W8.3 STORAGE CENSUS BATTERY GREEN (census equals an independently walked disk sum, golden display shape pinned verbatim, read-only proven by hash and mtime identity plus never creating an absent family, absent members stated not zeroed, undeclared ceiling leaves the band uncomputed, agent and task dimensions displayed separately, member order locked to the closed registration)'

step '04 decision-plane reachability grep: no decision-plane or command file consumes the W8 symbols (plant/remove control)'
LEAKRE8='Governance|QuotaRecord|QuotaReport|QuotaState|StoragePressureBand|StorageBand|GovernAction|GovernDecision|GovernReport|PressureReading|over_quota_critical_only'
probe8() {
  grep -rlE "$LEAKRE8" --include='*.go' cmd internal/policy internal/rules internal/bus 2>/dev/null | grep -v '_test\.go' || true
}
hits=$(probe8)
[ -z "$hits" ] || { echo "RED: W8 symbols reachable in decision/command files:"; echo "$hits"; exit 1; }
PLANT=cmd/agentruntime/w8-teeth-plant.go
if [ ! -d cmd/agentruntime ]; then
  d=$(find cmd -mindepth 1 -maxdepth 1 -type d | head -1)
  PLANT="$d/w8-teeth-plant.go"
fi
printf 'package main\n\n// probe\nvar _ = QuotaRecord{}\n' > "$PLANT"
hits=$(probe8)
rm -f "$PLANT"
[ -n "$hits" ] || { echo 'RED: planted reference did not fire (gate has no teeth)'; exit 1; }
hits=$(probe8)
[ -z "$hits" ] || { echo 'RED: removal did not clear the probe'; exit 1; }
echo 'DECISION-PLANE GREP ZERO HITS (planted shape fired, removal cleared)'

step '05 inherited chain: gate-w7 full battery (re-runs gate-w5, gate-w4, gate-w3, gate-w2, gate-w1, gate-w0, gate-d7 and gate-d1..d6 serially)'
bash scripts/gate-w7.sh > "$LOGDIR/gate-w7-inside-w8.out" 2>&1 \
  || { tail -30 "$LOGDIR/gate-w7-inside-w8.out"; echo RED: gate-w7 regression; exit 1; }
# shipped-residue note: the last banner line of gate-w7.sh reads GATE-W6 for
# historical reasons (proved in the W1 and W3 legs); the green signal is that
# line plus the absence of any RED marker in its log.
grep -q 'GATE-W6: ALL GREEN' "$LOGDIR/gate-w7-inside-w8.out" \
  || { echo 'RED: gate-w7 completion banner missing'; exit 1; }
grep -q 'DECISION-PLANE GREP ZERO HITS' "$LOGDIR/gate-w7-inside-w8.out" \
  || { echo 'RED: gate-w7 step 04 did not report'; exit 1; }
echo 'GATE-W7 GREEN INSIDE GATE-W8 (zero regression on the full inherited chain)'

step '06 section 304/307 observation-form review anchors for the storage wave (verbatim, zero promotion)'
pre8() {
  grep -qF "$1" docs/schema-v2.md || { echo "RED: storage anchor lost: $1"; exit 1; }
}
# dual limit is enforced on both rails, neither side may be traded away
pre8 'storage_dual_limit_rule: time-and-capacity-both-enforced-neither-infinite-growth'
# capacity pressure may never evict a critical segment
pre8 'storage_critical_hold_rule: critical-classified-segments-never-pruned-by-capacity-pressure'
# when only held segments remain the honest line is over quota, not a drop
pre8 'storage_overquota_honesty_rule: when-only-held-segments-remain-report-over-quota-never-drop-critical'
# the live head stays valid even when empty (inherited exemption)
pre8 'storage_live_head_rule: governance-never-prunes-the-live-segment-empty-head-stays-valid'
# both wave responses stay record-only
pre8 'storage_pressure_response_rule: record-only-no-action'
pre8 'quota_response_rule: record-only-no-action'
# quota never becomes a second collector and never averages the two scopes
pre8 'quota_shared_source_rule: quota-recounts-stored-audit-lines-never-a-second-collector'
pre8 'quota_scope_pair_rule: per-agent-and-per-task-quoted-separately-never-averaged'
# storage governance stays an observation stance: writes are never blocked by
# pressure, and the wave declares no enforcement plane at all (record-only).
pre8 'storage_write_block_rule: pressure-never-blocks-writes-in-observation-phase'
# the read-only census states absence and never a second dialect
pre8 'storagecli_read_only_rule: storage-census-opens-audit-read-only-never-creates-never-advances'
pre8 'storagecli_absence_rule: member-without-source-reported-absent-never-rendered-as-zero'
pre8 'storagecli_retention_alias: renders-section-26-retention-mapping-never-a-second-window-table'
pre8 'storagecli_band_alias: renders-section-26-pressure-band-vocabulary-never-a-second-ladder'
p=$(grep -cE '^(storage|quota)_[a-z_]*(enforcement|execution)_plane: ' docs/schema-v2.md || true)
[ "$p" -eq 0 ] || { echo "RED: the W8 wave grew an enforcement plane declaration ($p lines)"; exit 1; }
c=$(grep -cE '^[a-z_]+_(enforcement|execution)_plane: none-in-observation-phase$' docs/schema-v2.md)
t=$(grep -cE '^[a-z_]+_(enforcement|execution)_plane: ' docs/schema-v2.md)
[ "$c" -eq "$t" ] || { echo "RED: a declared plane was promoted out of observation stance ($c/$t)"; exit 1; }
[ "$c" -ge 16 ] || { echo "RED: plane census $c, want at least 16 none-lines"; exit 1; }
echo "STORAGE OBSERVATION ANCHORS GREEN (nine verbatim anchors, zero enforcement-plane declaration in the wave, plane census all-none: $c/$t, section 304 value not hung)"

step '07 tripwire standalone (internal-ledger negative scan)'
bash scripts/tripwire.sh > "$LOGDIR/gate-w8-tripwire.out" 2>&1 \
  || { cat "$LOGDIR/gate-w8-tripwire.out"; echo RED: tripwire; exit 1; }
grep -q 'TRIPWIRE CLEAN' "$LOGDIR/gate-w8-tripwire.out" || { echo RED: tripwire banner; exit 1; }

printf '\nGATE-W8: ALL GREEN\n'
