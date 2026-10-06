# 725 — Record user-launched Pi sessions in harnez telemetry via a Pi extension hook

**Status**: Open
**Priority**: P3 (Low)
**Severity**: Minor
**Category**: Feature
**Related**: [#716 Pi skills and hooks](716-install-harnez-skills-and-hooks-into-pi-instances.md), [#723 agent container](723-consolidated-agent-container-with-in-container-lmcoder-smoke-test.md)

---

## 1. Problem & Motivation
`harnez apply` gives Codex telemetry hooks (`harnez codex-telemetry` on
session start/end, tool use and compaction), so Codex sessions the user starts
show up in harnez telemetry. Pi gets only the Distill extension (#716), so Pi
sessions started outside `harnez agent` are invisible.

/goal Pi sessions record the same telemetry as Codex sessions through a
harnez-managed Pi extension installed by `apply`, shown by `status`, removed
by `revert`. Stop and report if Pi's extension events lack the session id or
usage data needed.

## 2. Technical Specification / Findings
- Pi 1.0.4's extension docs list `session_start`, `session_shutdown`,
  `tool_result`, `turn_end` and `agent_end` events.
- Follow the Distill extension (`internal/claude/distill_adapters.go`) and
  `runCodexTelemetry` in `cmd/harnez/codexhooks.go`.

## 3. Implementation & Verification Plan
- Tests as for the Distill adapter; add a check to the #723 smoke test that a
  Pi session appears in telemetry.
