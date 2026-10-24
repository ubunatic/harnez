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

## Sprint findings (2026-10-24, searcher haiku:med)

- `captureStdoutClaudeSkills` (`internal/claude/claudeskills_test.go:189-210`) swaps `os.Stdout` for an `os.Pipe`
  and drains it only after `f()` returns; copies in `status_test.go:19`, `telemetry_hook_test.go:20`,
  `apply_settings_test.go:21`, `integration_test.go:19`, `repo_config_test.go:292`. Output over the pipe
  buffer (64 KiB) deadlocks.
- Not reproduced: 5 solo runs, 3 `-p 4` runs, 4 concurrent processes all passed. Current output is small
  (drift loop `status.go:316-318` only with `issues/README.md` in cwd), so the 64 KiB path is latent.
- Fix: one shared helper that drains in a goroutine before `f()` and restores with `defer`; regression test
  writing ~256 KiB. Close noting the original hang was not reproduced.
