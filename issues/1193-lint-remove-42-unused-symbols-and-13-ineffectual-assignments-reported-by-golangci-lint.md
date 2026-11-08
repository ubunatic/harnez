# 1193 — lint: remove 42 unused symbols and 13 ineffectual assignments reported by golangci-lint

**Status**: Closed — 18a5359f: golangci-lint unused 0, ineffassign 0 (1403 -> 1347); make test and make lint green
**Priority**: P3 (Low)
**Severity**: Minor
**Category**: Chore
**Related**: 927 (lint epic)

---

/goal `golangci-lint run --max-same-issues=0 --max-issues-per-linter=0 ./...` reports 0 `unused` and
0 `ineffassign` findings, with `make test` green. Stop and report when a symbol looks used by
reflection, build tags or a planned feature. On a loop or no progress, clear the goal and write a
failure report here.

## Findings (gokay check --all, 2026-11-09, main 579ccf5a)

golangci-lint v2.15.0, uncapped: 1403 issues (errcheck 1193, staticcheck 155, unused 42, ineffassign 13).
This ticket takes the 55 `unused` and `ineffassign` findings; errcheck and staticcheck stay in 927.

- `unused` clusters: `cmd/harnez/exec.go` (old wrappers around `internal/exec`), `internal/usage/*`
  (watch/indicator helpers, AMD sysfs readers), `internal/server/server.go` (status handlers),
  `internal/claude/apply.go`, `cmd/harnez/agent_stream.go` (preamble constants),
  `internal/release/runner.go` (`artifactVersionPattern`).
- `norace_test.go` `raceEnabled` constants (index, sound, telemetry) are paired with `race_test.go`
  build-tag files: keep them if the race variant uses them; fix the pairing rather than deleting one half.
- Suspected behaviour issues checked and found harmless: `internal/usage/claude.go:365`
  (`status = retryStatus` is never read after), `internal/telemetry/export.go:119` (both branches
  assign `cat`). Remove the dead assignments.

## Acceptance

- 0 `unused` / `ineffassign` findings; `make lint` and `make test` green.
