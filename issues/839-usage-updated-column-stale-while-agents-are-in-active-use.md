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

## Milestones
- **M1 (collector persistence I/O error):** reproduce and fix the SQLite disk I/O error in the
  collector (check systemd unit sandboxing, DB path/WAL, file locks); regression test.
- **M2 (column meaning):** pending user decision: keep Updated = last quota fetch (15 min
  cadence), or show last activity / shorten cadence.
