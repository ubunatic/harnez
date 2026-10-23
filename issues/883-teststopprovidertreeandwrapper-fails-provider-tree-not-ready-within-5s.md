# 883 — TestStopProviderTreeAndWrapper fails: provider tree not ready within 5s

**Status**: Open
**Priority**: P2 (Medium)
**Severity**: Minor
**Category**: Testing
**Related**: 881 (test suite catalog), 811 (fast -short tier), 804 (race-proof budgets)

---

/goal `TestStopProviderTreeAndWrapper` (`cmd/harnez/agent_stop_test.go`) passes reliably on the user's host,
alone and in `make test-q1`, without a longer fixed budget hiding a real bug; or stop and report if the
stop logic itself is broken.

## Findings (host, 2026-10-23)

- Failed in four cases during 882's `make test-q1`, and again when run alone with load ~5:
  `--- FAIL: TestStopProviderTreeAndWrapper/claude/stream=0/ignore-term=0 (5.03s)`,
  `agent_stop_test.go:113: provider tree not ready: []`.
- 882 did not touch this code. Last test changes: 3d1626f4 (811), e5d0cbc0 (804).

## Plan

Find why the fake provider tree is empty after 5s (process not started, or tree lookup misses it); wait on a
readiness event instead of a wall-clock budget.
