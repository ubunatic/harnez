# 1076 — agent start --context-mode is silently ignored for Claude, Codex and AGY

**Status**: Open
**Priority**: P2 (Medium)
**Severity**: Minor
**Category**: Bug
**Related**: 1027

---

## 1. Problem & Motivation
1027 shipped `--context-mode` (`default`, `no-skills`, `plain`; spec `spec/agent.yaml > context_modes`) "starting
with Pi". Only the Pi driver honours it: `internal/subagent/pi.go` passes `o.ContextMode` to `piRunArgs`, while
the Claude, Codex and AGY drivers receive `RunOptions.ContextMode` and drop it. `harnez agent start --model
claude:opus:med --context-mode plain` runs with full skills, MCP and `CLAUDE.md`, with no error or warning, so a
host believes it saved context when it did not.

/goal Make `--context-mode` honest for every provider: non-Pi drivers either apply the mode or refuse it with a
clear error, and `plain`/`no-skills` work at least for Claude. Stop and report when blocked on a user decision or a
denied permission. On a loop, a hard blocker or no progress, clear the goal and write a failure report here (what
was tried, why it failed, where to resume).

## 2. Technical Specification / Findings
- Found 2026-11-01 by reading `cmd/harnez/agent_run.go` (forwards `ContextMode` to `RunOptions` and the worker
  argv) and `internal/subagent/{claude,codex,agy}.go` (no use of `ContextMode`).
- Re-check the live drivers first; this may have moved since.
- Open question: which Claude Code CLI switches map to `skills: false`, `mcp: false`, `context_files: false` and a
  restricted tool list? Probe them per `docs/Canary.md` before relying on them. Same question for Codex and AGY;
  where no reliable switch exists, refuse the mode for that provider.

## 3. Implementation & Verification Plan
- M1: unsupported provider + non-default mode fails fast with an error naming the supported providers (test per
  driver).
- M2: Claude driver applies `no-skills` and `plain` via the probed switches; test asserts the argv, plus a live
  canary showing a reduced context.
- Codex and AGY: support where a probed switch exists, otherwise stay on the M1 error.

## Audit 2026-11-03

Jules audit (PR GH#70, report `reports/jules/1076.md`, checked against b0dcb501, all 5 references held): still valid. `cmd/harnez/agent_run.go:439` forwards `ContextMode`; `claudeRunArgs` (`internal/subagent/claude.go:54`), `CodexDriver.Run` (`codex.go:98`) and `AgyDriver.Run` (`agy.go:121`) never read it; only `pi.go:34` does. Smallest step: reject an unsupported mode in those drivers, or map it to a provider switch.

## Jules runs

| kind | session name | Jules ID | PR | state |
| --- | --- | --- | --- | --- |
| audit | jules-audit-1076 | 8540512660876518399 | github #95 (closed) | done: still valid, 5/5 refs hold; repeats 2026-11-03 audit |
| implement | harnez-jules-impl-1076 | 12946028318458480440 | none | failed 2026-11-08: no PR after 2 replies; last message (lead, unverified): added contextMode to Resume/ResumeStream in internal/subagent/driver.go |
| implement (M1 slice) | jules-impl-1076 | 5881870373175353258 | none | failed 2026-11-10: stalled awaiting plan approval after 2 replies, no PR |
