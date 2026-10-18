# 824 — Flaky TestCodexOverThresholdResumeUsesProviderAutoCompact: TempDir cleanup sees directory not empty

**Status**: Closed — Fixed production quota-capture lifetime with cancellation and join on every start/resume exit; repro passes 50 normal and 50 race repetitions
**Priority**: P3 (Low)
**Severity**: Minor
**Category**: Test
**Related**: 821 (seen during its verification), docs/practices/TestTiers.md

---

## 1. Problem

On 2026-10-18 a `make test-short` run failed in
`TestCodexOverThresholdResumeUsesProviderAutoCompact` (`cmd/harnez/agent_resume_test.go:561`).
The t.TempDir cleanup reported `directory not empty` while the directory was still being
written. The rerun passed.

The likely cause is a background writer, such as a session store, a log or a spawned fake
provider, that is still running when the test returns.

## 2. Goal

/goal Make the test join every goroutine and child process that writes into its TempDir
before it returns. Verify with `go test -short -race -count=20 -run
TestCodexOverThresholdResumeUsesProviderAutoCompact ./cmd/harnez/`. Stop and report if
the writer cannot be found from the test.

## 3. Delivery

- Root cause: `runResume` launched the before-turn quota capture concurrently,
  but the Codex auto-compaction refusal returned without receiving its result.
  `CaptureTurnQuotaSince` calls `ProviderSnapshotFromStore`, opening/migrating the
  SQLite usage store under `$HOME/.local/share/harnez` after command/test return.
  Start/resume watchdog exits skipped the same join.
- Fix: `startTurnQuotaCapture` gives both production runners a deferred cancel
  and completion join on every exit, retaining concurrent capture and existing
  quota-boundary persistence on normal/error paths. No sleeps or test-only drain.
- Regression coverage: channel barriers hold quota capture until cancellation,
  then assert completion before returning from compaction rejection and both
  watchdog exits. The sibling `TestAgentResumeStatusIsRunningDuringTurn` now
  releases and joins its worker even when an assertion aborts the test.
- Before: requested `go test -short -count=50 -run
  TestCodexOverThresholdResumeUsesProviderAutoCompact ./cmd/harnez/` failed
  **11/50 (22%)**; the same loop with `-race` failed **0/50**.
  `go test -short -count=1 ./cmd/harnez/` reproduced this cleanup failure
  in **1/1 package runs**.
- After: both `go test -short -count=50` and `go test -short -race -count=50`
  with `-run 'TestCodexOverThresholdResumeUsesProviderAutoCompact|TestAgentEarlyReturnJoinsQuotaCapture|TestAgentResumeStatusIsRunningDuringTurn'`
  passed: original repro **0/50 failures each**, plus 50 repetitions of each
  regression and sibling test. `go test -short -race -count=1 ./cmd/harnez/`,
  `make lint`, `make test-short`, `make install`, and `git diff --check` passed.
- Implementation: `6677d565` (`fix(agent): join quota capture on early turn exits (824)`).
- Review: checked buffered-result delivery, cancellation plus join on every
  return, existing quota persistence, and sibling failure-path cleanup.
  No open problems; no child agents or detached processes started.
