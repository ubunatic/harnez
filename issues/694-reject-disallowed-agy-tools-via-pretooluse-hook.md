# 694 — Reject disallowed AGY tools via PreToolUse hook

**Status**: Closed — resolved in 226a71d7 (spec-driven PreToolUse rejection)
**Priority**: P2
**Severity**: Medium
**Category**: Feature / AGY
**Related**: [[537-agy-route-shell-commands-through-harnez-exec-via-hooks-json]], [[196-agy-native-hooks-plan-alongside-claude-hooks]]

---

## 1. Problem & Motivation

Antigravity (AGY) exposes built-in tools such as `Schedule` (which sets background timers and cron intervals). Under Harnez workflow rules (`.harnez/rules/Tools.md`), agents are forbidden from using timers or cron polling loops, and must instead run background commands and await reactive completion notifications.

AGY's native lifecycle hook system allows `PreToolUse` hooks (`harnez hook agy`) to intercept tool calls before execution and return `{"decision": "deny", "reason": "..."}`. We need to reject disallowed AGY tools deterministically at the hook boundary, with tool rejection rules declared in `spec/` backed by a JSON Schema rather than hardcoded in Go code.

## 2. /goal

Reject disallowed AGY tools (starting with `Schedule`) via the AGY PreToolUse hook (`harnez hook agy`) with rejection rules and explanation reasons defined in `spec/`, or stop and report when blocked on a user decision or denied permission.

## 3. Technical Specification

1. **Spec Declaration**:
   - Define disallowed AGY tools and their denial explanation in `spec/` (e.g. `spec/agent.yaml` or dedicated schema/spec) referencing the Harnez rule prohibiting timers/cron jobs.
   - Update the corresponding JSON schema in `spec/schemas/` to validate tool rejection entries.
2. **Hook Interception (`cmd/harnez/hook.go`)**:
   - In `runAgyToolHook`, check if `payload.ToolCall.Name` matches a configured disallowed tool (case-insensitively).
   - If matched, emit `{"decision": "deny", "reason": "<configured reason>"}` and record telemetry with `callType = "hook:deny"`.
3. **Verification**:
   - Unit tests in `cmd/harnez/` and `internal/agy/` asserting that `Schedule` tool invocations return a denial with the spec-provided explanation, while allowed tools (e.g. `run_command`, `view_file`) continue uninterrupted.
