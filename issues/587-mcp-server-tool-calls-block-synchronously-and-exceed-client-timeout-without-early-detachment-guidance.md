# 587 — MCP server tool calls block synchronously and exceed client timeout without early detachment guidance

**Status**: Open
**Priority**: P2 (Medium)
**Severity**: Moderate
**Category**: Architecture
**Related**: [571](571-mcp-server-to-run-agents-via-harnez-agent-inside-codex.md), [572](572-async-subagent-dispatch-and-harnez-wait-agent-in-harnez-mcp-and-cli.md), [575](575-add-harnez-command-mcp-tool-returning-cli-command-and-backgrounding-instructions.md)

---

## 1. Problem & Motivation

When calling `harnez_spawn_agent` synchronously without `async: true`, long-running agent tasks block on the MCP request. Most MCP client harnesses (such as Antigravity/AGY) enforce a hard client-side timeout (e.g. 180s / 3 minutes), which aborts the RPC before the subagent completes. Furthermore, if `harnez_command` hangs or gets cancelled during MCP dispatch, agents cannot easily orchestrate workflows.

## 2. Technical Specification / Findings

- Subagent tasks that run multi-step tool loops often exceed 3 minutes.
- When `async: false` is used, the client MCP connection drops on timeout with `context deadline exceeded`.
- The MCP server should default to or strongly steer toward asynchronous subagent dispatch (`async: true` or returning a session handle immediately) or provide streaming progress heartbeat mechanisms that keep the MCP channel active if supported by the transport.

## 3. Implementation & Verification Plan

1. Clarify and document in `harnez_spawn_agent` description that multi-turn tasks must use `async: true` or CLI backgrounding.
2. Consider defaulting `async: true` for long-running agent actions or adding timeout guards with early detachment.
3. Test end-to-end agent launching via MCP with async polling.

