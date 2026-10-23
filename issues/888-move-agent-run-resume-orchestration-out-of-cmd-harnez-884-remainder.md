# 888 — Move agent run/resume orchestration out of cmd/harnez (884 remainder)

**Status**: Backlog — parked 2026-10-24; promote when run/resume work needs a narrower scope
**Priority**: P3 (Low)
**Severity**: Minor
**Category**: Refactor
**Related**: 884 (split cmd/harnez, closed with this remainder), 881, docs/studies/TestSuiteSprintSession.md

---

/goal `cmd/harnez` keeps only flags, help, wiring and exit codes for the agent commands; run/resume
orchestration lives in an internal package with behaviour unchanged and process regressions still green;
or stop and report when a boundary needs a user decision.

## Why parked

884 M9 (`04b01baa`) planned ~5k lines and moved 157 (39,502 → 39,345). The agent policy core went to
`internal/agent`; run/resume/stop/stream orchestration (`agent_run.go`, `agent_async.go`,
`agent_interactive.go`, `agent_send.go`, `agent_stop.go`, `agent_stream.go`) stays in `cmd/harnez`.
The advisor (884 Plan v3) estimated only ~3–4 s of test-time relocation. The slow tests are process tests
that stay in `cmd/harnez` anyway, so the remaining gain is less code for agents to read. The move is high
risk: the process handling it touches only just became reliable (883, 884 M5/M7).

## When picked up

Re-check live code first. An advisor proposes boundaries, then the move goes one file group per
milestone. The 884 boundary rules apply: no `*cobra.Command` in internal packages, context and writers
injected, no import cycles.
