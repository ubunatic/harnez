# 927 — lint: gokay check fails on usagestore store_test.go errcheck and spec schema conformance

**Status**: Open
**Priority**: P3 (Low)
**Severity**: Minor
**Category**: Bug
**Related**: 924, 919, study `docs/studies/2026-10-25-epic-921-server-monitor-sprint.md`

---

/goal `gokay check` passes its lint and spec stages on main, or each remaining finding is a documented exception;
or stop and report. Circuit breaker: on a loop or no progress, clear the goal and write a failure report here.

## Notes

- Reported by developer d919b (919 M4, 2026-10-25): `gokay check` tests passed, but the lint and spec stages failed
  on errcheck findings in `internal/usagestore/store_test.go` (likely from 924, 7f4d55cb) and on schema conformance
  diagnostics. `make lint` and `make test-q1` were green, so the two linters disagree.
- First step: rerun `gokay check`, list the exact findings, and decide whether `make lint` should cover them.
