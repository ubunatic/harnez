# 1107 — agent selftest refuses to run from a developer role

**Status**: Open
**Priority**: P2 (Medium)
**Severity**: Minor
**Category**: Bug
**Related**: taia (codeberg.org/harnez/taia), 1083 (grooming for taia focus)

---

/goal A developer-role worker can read the selftest steps, with a test. Stop and report when blocked on a user decision or denied permission; on a loop or no progress, clear the goal and write a failure report here.

## Problem

Reported by the taia session (2026-11-03): `harnez agent selftest` refuses to run from a developer role, so a worker cannot see the real step format it should follow. Allow a read-only form (e.g. print the steps) for developer roles, or explain in the refusal how to get it. Related: 1012, 1008.
