# 747 — agent foreground resume test fails in cloud sandbox: session not found

**Status**: Draft
**Priority**: P2 (Medium)
**Severity**: Minor
**Category**: Bug / Tests
**Related**: #743

---

## 1. Problem & Motivation
`TestForegroundStartAndResumeDetachIntoWaitableWorkers/resume` fails in a cloud
session with `session "foreground-resume" not found` (`cmd/harnez/agent_test.go:370`).
The log also shows `run 'harnez agent selftest --step hello' first`.

## 2. Technical Specification / Findings
Root cause not yet known. Suspects: test state leaking from the real `$HOME`
(`~/.harnez`) or from installed agent CLIs, or the 100ms exec timeout set by the test
being too short on a slow container.

## 3. Implementation & Verification Plan
- Reproduce in a fresh cloud session with `go test ./cmd/harnez -run TestForegroundStartAndResume -v`.
- Isolate `HOME`/XDG dirs in the test if state leaks.
- Verify: passes in a cloud session and locally.
