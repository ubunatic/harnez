# 643 — Instruct AGY host sessions to use native background tasks instead of blocking harnez_wait_agent MCP

**Status**: Open
**Priority**: P1 (High)
**Severity**: Major
**Category**: Architecture
**Related**: 608 (merged into 643), 587 (MCP tool timeouts), 630 (skills host-native background jobs), 611 (zero-polling invariant)

---

## 1. Problem & Motivation

In user-driven / interactive host sessions running under AGY (Antigravity), agents discovering Harnez MCP tools attempt to wait for background or detached subagent sessions by invoking the MCP tool `harnez_wait_agent` (or `harnez_spawn_agent` synchronously).

Because MCP tool calls in AGY and IDE clients execute synchronously on the host's main turn thread, invoking `harnez_wait_agent` blocks the entire chat session until the subagent finishes or the client MCP request times out. This causes severe UX and operational issues:
1. The chat UI freezes and blocks the user from sending follow-up prompts or seeing live progress.
2. The subagent is invisible in the AGY / host native task monitor UI (`manage_task`).
3. If the subagent runs for several minutes, the synchronous MCP call hits hard client-side RPC timeouts (e.g. 180s) resulting in `context deadline exceeded` errors.

In contrast, AGY provides first-class native background tasks (`run_command` with small `WaitMsBeforeAsync` / background execution, tracked via `manage_task`), where completion output is delivered reactively as a high-priority system notification without blocking the chat.

Interactive AGY host sessions must be instructed to use standard AGY background tasks to manage waiting for agents in the background (`run_command` executing `harnez agent wait <session>` or running `harnez agent start` as a background task) instead of shooting for blocking `harnez_wait_agent` MCP calls.

/goal Update Harnez MCP tool definitions, rule templates, and subagent policy documentation to direct AGY and interactive host sessions to manage subagent execution and waiting via standard host background tasks (`run_command` / `harnez agent wait`) rather than blocking MCP tool calls like `harnez_wait_agent`; or stop and report when blocked on a user decision or denied permission.

---

## 2. Technical Specification / Findings

1. **MCP Tool Description Guidance (`internal/mcp/server.go`)**:
   - `harnez_wait_agent`: Explicitly state in the tool description that this tool blocks the caller synchronously and should not be used in interactive/user-driven chat sessions. Direct host sessions to format/run `harnez agent wait <session_id>` via their host background runner (`run_command` / bash background job).
   - `harnez_command`: Reinforce that formatting `wait` (`harnez agent wait <session_id>`) or `start` commands for execution in the host's background runner is the canonical mechanism for non-blocking wait with UI visibility and reactive wakeups.
   - `harnez_spawn_agent`: Clarify that interactive host orchestrators should prefer host background commands over synchronous or detached unmonitored MCP spawns.

2. **Rule Templates & Subagent Policies (`.harnez/rules/Tools.md`, `Subagents.md`, `AgenticLoop.md`)**:
   - Explicitly document the rule for AGY and interactive host orchestrators: use standard host background tasks (`run_command` in background) for agent execution and waiting.
   - Forbid interactive host orchestrators from shooting for synchronous `harnez_wait_agent` MCP tool calls that lock up the chat.

3. **Consolidation of Duplicate Issue #608**:
   - Issue #608 ("Promote CLI command over MCP to ensure background job visibility in host sessions") identified the start/spawn visibility gap.
   - This issue #643 unifies both start and wait lifecycle stages under standard host-native background tasks and supersedes #608.

---

## 3. Implementation & Verification Plan

1. **Update MCP Tool Definitions**:
   - In `internal/mcp/server.go`, update descriptions for `harnez_wait_agent`, `harnez_command`, and `harnez_spawn_agent` with clear guidance against synchronous blocking in interactive chats.
2. **Update Rule Templates and Guidance**:
   - Update `internal/claude/init.go`, `.harnez/rules/Tools.md`, `.harnez/rules/Subagents.md`, and `docs/practices/AgenticLoop.md` to instruct AGY / interactive hosts to use native background tasks for waiting on agents.
3. **Verify Documentation & Unit Tests**:
   - Run `go test ./...` and `make install`.
   - Verify `harnez mcp` schema output and rule generation.
4. **Close / Merge Issue #608**:
   - Close issue #608 as merged into #643.
