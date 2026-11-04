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

## Audit 2026-11-04
Jules audit (session 6487310379501879876, against 70da579e, `reports/jules/1111.md`): still valid, references
checked. All three wait-timeout paths return the "stopped waiting" text with a nil error
(`internal/subagent/jules.go:724`, `:734`, `:776`; text built at `:836`). Still needs the owner's exit-code pick.
