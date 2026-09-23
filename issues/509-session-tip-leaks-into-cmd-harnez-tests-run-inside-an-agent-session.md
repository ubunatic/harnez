# 509 — Session tip leaks into cmd/harnez tests run inside an agent session

**Status**: Open
**Priority**: P2 (Medium)
**Severity**: Minor
**Category**: Bug
**Related**: `cmd/harnez/main_env_test.go` (TestMain isolation), `cmd/harnez/main.go` (`sessionTipHook`), `internal/resolve/resolve.go`, `internal/sessionstate/sessionstate.go`

## Problem

`make test-q1` run from a Claude Code session (2026-09-23) failed on:

```
--- FAIL: TestStreamingResumeKeepsStderrQuiet
    agent_test.go:1234: stderr = "harnez tip: 40+ calls since any `harnez rate` call this session — ...", want empty while streaming
```

The failure is unrelated to the change under test. `TestMain` clears `HARNEZ_AGENT_ROLE`
and `HARNEZ_SESSION_ID` and sets a temp `HOME`, but `sessionTipHook` still resolved a
session. Hypothesis (unverified): `resolve.Session` finds the host agent session some
other way (process ancestry or another env var). The suite's own commands then add up past
the 40-call threshold in the temp state dir, and the tip is printed mid-suite, so the result
depends on test order and on running inside an agent session.

Under Quota-1 this costs the only test run of the step, and agents may misread the failure
as caused by their own change.

## /goal

The `cmd/harnez` suite gives the same result inside and outside an agent session:
`TestMain` (or `resolve`'s test hook) guarantees no session is resolved unless a test sets
one explicitly.

## Done when

- The root cause is confirmed and fixed in test isolation, not by loosening the stderr
  assertion.
- A test proves the tip hook is silent under `TestMain` defaults.
