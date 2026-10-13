# 746 — process-group kill tests fail in containers without a reaping init

**Status**: Draft
**Priority**: P2 (Medium)
**Severity**: Minor
**Category**: Bug / Tests
**Related**: #743

---

## 1. Problem & Motivation
`TestRunExecWrapper_TimeoutKillsProcessGroupDescendant` (`cmd/harnez/exec_test.go:573`)
and `TestFallbackChainTimeoutKillsProcessGroup` (`internal/sound/sound_test.go:382`)
report the killed descendant as still alive in cloud containers.

## 2. Technical Specification / Findings
Both probe with `syscall.Kill(pid, 0)`. PID 1 in the container is `process_api`, not a
reaping init. Hypothesis (unverified): the killed orphan stays a zombie, which
`Kill(pid, 0)` still reports as existing.

## 3. Implementation & Verification Plan
- Confirm by reading `/proc/<pid>/stat` state in the failing test.
- If confirmed, a shared test helper treats state `Z` as dead.
- If not, fix the real leak in the process-group kill.
- Verify in a cloud session and locally.
