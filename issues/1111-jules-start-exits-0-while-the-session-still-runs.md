# 1111 — Jules start exits 0 while the session still runs

**Status**: Open
**Priority**: P3 (Low)
**Severity**: Minor
**Category**: Feature
**Related**:

---

## Problem
Reported 2026-11-03 by the voxi, lmcoder and loom sessions.

`harnez agent start/resume --model jules` exits 0 when its wait runs out and the session is still IN_PROGRESS ("stopped waiting"). Background jobs then look done. The skill documents it, but all three sessions misread it once.

## Goal
/goal Decide and implement a distinct exit code for "still running" (or document why not); stop for an owner decision because scripts may rely on exit 0.

## Done when
- Owner picks the exit code (needs a decision: other providers and existing scripts).
- Start and resume return it on a wait timeout; tests cover it; skill updated.
