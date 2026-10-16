# 794 — agent resume races cmd.SetContext restore against goroutines reading cmd.Context

**Status**: Draft
**Priority**: P1 (High)
**Severity**: Major
**Category**: Bug
**Related**:

---

## 1. Problem & Motivation
`go test -race ./cmd/harnez` reports a DATA RACE in
TestCodexOverThresholdResumeUsesProviderAutoCompact/still_over_limit. It is a
production race, not a test artefact: concurrent writes and reads of cobra's
`cmd.ctx`.

## 2. Technical Specification / Findings
- `observeSessionProcess` (agent_stop.go) and `withAgentTimeout` (agent_run.go)
  called `cmd.SetContext(...)` and restored the original context with a
  deferred `cmd.SetContext(original)`.
- `runStart`/`runResume` start goroutines that read `cmd.Context()` (e.g. the
  `captureTurnQuota` goroutine, agent_run.go ~832). On error/early-return
  paths these goroutines outlive the function, so the deferred restore writes
  `cmd.ctx` while they read it.
- Other SetContext sites: examples/usage/main.go and one test set the context
  once before Execute; they are not concurrent and stay as is.

## 3. Implementation & Verification Plan
- No more SetContext on the shared command. The context is passed explicitly:
  `withAgentTimeout(cmd, d, func(ctx) error)` -> `runStart(ctx, cmd, ...)` /
  `runResume(ctx, cmd, ...)`; `observeSessionProcess(ctx, ...)` returns a
  derived context instead of a restore closure. All context reads inside
  runStart/runResume use that `ctx`. Detached workers pass
  `agentCommandContext(cmd)`.
- Verify: `go test -race -count=5 -run 'TestCodexOverThreshold|Agent' ./cmd/harnez`.
