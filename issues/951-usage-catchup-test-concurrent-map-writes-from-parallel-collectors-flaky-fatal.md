# 951 — usage catchup test: concurrent map writes from parallel collectors (flaky fatal)

**Status**: Open
**Priority**: P2 (Medium)
**Severity**: Minor
**Category**: Bug
**Related**: 945 (seen in its M2 developer run)

---

/goal The `internal/usage` catch-up test no longer crashes intermittently: a reproduction under `-race` (or
`-count` loop) fails first, then the fix makes it pass; production code is checked for the same race. Or stop and
report when the race is in production code with a design impact. Circuit breaker: on a loop or no progress, clear
the goal and write a failure report here.

## Evidence

Developer `make test-q1` run for 945 M2 (2026-10-25, log `/tmp/harnez-945-m2-test-q1.log`):
`fatal error: concurrent map writes` at `internal/usage/catchup_test.go:32`, called from
`internal/usage/registry.go:180`, goroutine created by `(*Registry).collectWithFilter` (`registry.go:172`).
The test's collector funcs (`catchup_test.go:25-40`) do `fetched["pN"]++` on one unguarded map while the registry
runs collectors in parallel. The next host full run passed, so it is timing-dependent.
