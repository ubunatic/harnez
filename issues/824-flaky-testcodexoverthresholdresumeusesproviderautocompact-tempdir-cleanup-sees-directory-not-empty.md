# 824 — Flaky TestCodexOverThresholdResumeUsesProviderAutoCompact: TempDir cleanup sees directory not empty

**Status**: Open
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
