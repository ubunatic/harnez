# 892 — TestClaudeSkillsTargetRoundTrip hangs 10m blocked writing captured output in RunStatus

**Status**: Open
**Priority**: P2 (Medium)
**Severity**: Minor
**Category**: Bug
**Related**: 881 (test catalog), 859/891 sprint (2026-10-24)

---

/goal `go test ./internal/claude/...` never hangs in `TestClaudeSkillsTargetRoundTrip`; reproduce first (parallel
package runs), fix the blocking write, verify with repeated runs; or stop and report if it cannot be reproduced
within a few attempts.

## Problem

During the 859/891 sprint, a standalone `go test ./internal/claude/...` (output redirected to a file, started while
other package suites ran in parallel) hit the 10-minute test timeout in untouched `TestClaudeSkillsTargetRoundTrip`.
The developer saw the goroutine blocked writing captured output inside `RunStatus`, with no CPU use; no assertion
failed. The following `make test-q1` passed, so it is intermittent.

## Suspect

A captured-output writer (pipe or buffer swap of stdout/stderr) that nobody drains while `RunStatus` writes more
than the pipe buffer. Check for `os.Pipe` stdout capture in the test helpers.
