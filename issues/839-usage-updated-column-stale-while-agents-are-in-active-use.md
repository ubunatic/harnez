# 839 — usage: Updated column stale (AGY 36m, Claude 15m) while agents are in active use

**Status**: Open
**Priority**: P2 (Medium)
**Severity**: Minor
**Category**: Bug
**Related**: 838 (watch overlay freshness), 829-837 (collector as single writer), 112 (agy poll backoff), 104

---

/goal Make the "Antigravity (AGY)" row's Updated column reflect current agy activity while
agy is in use, verified by a test and a live check, or stop and report when blocked on a user
decision or denied permission.

## 1. Problem & Motivation
`harnez usage -w` shows "36m ago" in the Updated column for "Antigravity (AGY)" although the
user is actively working in an agy session (model argon). The row looks stale, so its quota
numbers cannot be trusted.

Same symptom for Claude: its row showed "15m ago" while several Claude sessions were running.
So the cause is likely shared (collector cadence or the timestamp the column reads), not
agy-specific; fix and verify both rows.

## 2. Technical Specification / Findings
Unverified candidate causes, check before designing:
- The collector (since 829-837) may poll agy rarely, or be in the auth/backoff state added in
  112 (`internal/usage/agy.go`), so no fresh `agy -p "/usage"` result lands.
- Agy hook rows (seen in 830's live check) may update activity without updating the
  timestamp the Updated column reads.
- Updated may show the last quota fetch rather than the last activity; decide which the
  column should mean.

## 3. Implementation & Verification Plan
- Re-verify live: collector logs/state for agy, last fetch time, backoff state.
- Fix the cause; add a test for the freshness source of the AGY row.
- Live check with the installed collector after `make install` during an agy session.

## 4. Searcher findings (2026-10-21, s839 luna:low)
- Updated renders `AgentUsage.LastRefreshed` (`internal/usage/usagetable.go:101`), set in
  `internal/usage/registry.go:140,266,279`. It means quota-fetch freshness, not activity; hook
  events only write passive windows (`internal/usage/collector.go:145,179`).
- Collector cadence is 15 min (`spec/usage.yaml:17`, `collector.go:28`), so "15m ago" is
  expected by design. Claude/AGY also reuse quota younger than `MinWatchInterval`
  (`claude.go:321`, `agy.go:423`); AGY has the 30 min auth backoff (`agy.go:129,443`), no
  marker present now.
- Real bug: systemd journal shows SQLite `disk I/O error (6410)` writing all provider
  snapshots at 01:47, 02:02, 02:17, 02:32 CEST (also after a restart). Stored snapshots stuck
  at ~00:44. Reported ages don't match these files exactly; verify the runtime read path.
- M1 root cause: SQLite code 6410 is `SQLITE_IOERR_GETTEMPPATH`; the collector unit's
  `ProtectSystem=strict` made `/tmp` read-only in its mount namespace, leaving SQLite no
  suitable temporary directory. The telemetry database and WAL/SHM were owned by `uwe` under
  the unit's writable `.local/share/harnez` path; the filesystem had 55 GB free.
- The unit first started at 00:46 CEST on 2026-10-21, and the first collection immediately
  logged 6410 for all three providers. It restarted at 00:57 and 01:02 with the same errors;
  01:47 was a later 15-minute tick, not when the failure began. `PrivateTmp=yes` is the fix.
- M1 verification (2026-10-21): the new unit's private `/tmp` is writable; after restart at
  02:46 CEST the SQLite database/WAL advanced and the journal showed no snapshot write errors.
  `harnez usage --offline` read a 0m-old overall snapshot. `make test-q1` passed after the
  unit-template regression assertion was added (the pre-fix run failed at that assertion), and
  `make install` succeeded. AGY quota data remained 12m old, which is M2 scope.
- M2 findings (2026-10-21): the collector started using its systemd unit at 00:46 CEST, when
  the first snapshot attempt failed; 01:47 was a later tick, not the start of the age increase.
  Reducing `collector_cadence` from 15m to 5m and the cache staleness multiplier from 2 to 1
  keeps automatic cache reuse within one tick. `MinWatchInterval` remains 30s, below the new
  cadence. AGY meter quota older than 5m now triggers a live refresh; the 30m meter regression
  test asserts this, while the issue 112 30m auth backoff test remains intact.
- M2 verification (2026-10-21): the pre-fix `make test-q1` failed on the new cadence, cache-age,
  and old-meter regressions; after the spec and expectations were updated, `make test-q1` passed.
  `make install` succeeded and the collector restarted at 02:54 CEST, logging a 5m0s interval.
  At 02:54:45 CEST, `harnez usage` showed Claude Code, OpenAI Codex, and Antigravity (AGY)
  updated “just now”; the journal showed the collector running without snapshot errors.

## Milestones
- **M1 (collector persistence I/O error) — resolved 2026-10-21:** enabled `PrivateTmp=yes`;
  regression assertion and live collector verification passed.
- **M2 (shorter cadence) — resolved 2026-10-21:** set the spec cadence to 5m, cache staleness
  multiplier to 1, and AGY meter max age to 5m. `MinWatchInterval` reuse is 30s, and the issue
  112 auth backoff remains 30m. Tests and live collector verification passed; see findings above.
