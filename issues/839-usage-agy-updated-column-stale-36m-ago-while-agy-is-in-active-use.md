# 839 — usage: AGY Updated column stale (36m ago) while agy is in active use

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
