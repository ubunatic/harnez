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

## Also consider

The "harnez tip" message goes to stderr from the root `PersistentPreRunE`, so it can also
land in the output of any nested `harnez agent` call, not only in tests. Consider not
printing it for commands whose stderr is part of a protocol (`agent` streaming), or when
running under `go test`, in addition to fixing the test isolation.

## M1 delivered (d7307bb): tip hook skipped under `go test`

Review (terra) rejected the approach. The fix checks `os.Args[0]` for `.test` in
production `main.go`, and `resolve.Session` still falls back to the parent PID.

## M2 Pre-Work / Required Refinements

- Remove the `.test` check from `sessionTipHook`.
- Isolate in `TestMain` or a scoped test hook in `internal/resolve`, so no session
  resolves unless a test sets one (including the PPID fallback).
- Keep the new regression test; it must still fail without the fix.
- A test must still be able to exercise `sessionTipHook` tips by setting a session.
- `TestMain`: restore `HOME` to unset when it was unset (it now points at a deleted temp dir).
