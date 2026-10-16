# 789 — TestDeliverInboxWaitsForIdleAndWritesFIFOBracketedPaste races on a shared bytes.Buffer under -race

**Status**: Closed — Test-only race: terminal buffer in the test is now mutex-guarded (lockedBuffer); production code is not racy.
**Priority**: P3 (Low)
**Severity**: Minor
**Category**: Bug / Tests

---

## 1. Problem & Motivation
`go test -race ./internal/subagent/` fails on Codeberg main (f00a536d): DATA RACE
between the inbox writer goroutine (`interactive.go:439`, `io.Copy` into a
`bytes.Buffer`) and the test reading that buffer (`interactive_test.go:295`).

## 2. Implementation & Verification Plan
- Guard the test's buffer with a mutex-backed writer, or read it only after the writer exits.
- Verify: `go test -race -count=10 -run TestDeliverInbox ./internal/subagent/` passes.
