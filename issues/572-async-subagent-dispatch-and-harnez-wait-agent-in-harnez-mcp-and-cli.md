# 572 — Async subagent dispatch and harnez_wait_agent in harnez mcp and CLI

**Status**: Open
**Priority**: P2
**Severity**: Minor
**Category**: Feature

---

## Overview & Goals
Allow agents calling `harnez` via MCP (or CLI) to spawn subagents asynchronously in the background, continue other tasks, and later wait on completion using a dedicated `harnez_wait_agent` tool (or CLI command `harnez agent wait`).

## Milestones

- **M1 (Design & Read-Only Plan)**: Completed.
  - Plan approved:
    - CLI: `harnez agent start --detach` (alias `--async`) starts worker process detached, records state as `running` with PID, writes logs to session log paths in store, updates to `completed`/`failed` upon completion.
    - CLI: `harnez agent wait <session> [--timeout <duration>]` polls session store until terminal state or timeout.
    - MCP: `harnez_spawn_agent` supports optional `async: boolean` (default false). When true, launches detached and immediately returns running session info (`session_id`, `status`).
    - MCP: `harnez_wait_agent(session_id: string, timeout_seconds?: integer)` waits on session completion and returns completed record / result or timeout status.
    - Stale worker handling: if process is dead without terminal record, mark status failed with appropriate error message.
- **M2 (Core & MCP Implementation)**: Completed in commit `bde87ae`.
  - Implemented `harnez agent start --detach` / `--async` with detached worker launcher and process tracking.
  - Added session stdout/stderr log paths, result capture in session record, and stale-worker detection.
  - Implemented `harnez agent wait <session> [--timeout <duration>]` polling the session store until completion or timeout.
  - Updated MCP schemas: added `async` boolean to `harnez_spawn_agent` and added `harnez_wait_agent(session_id, timeout_seconds?)`.
  - Added unit/integration tests covering async execution, wait completion, timeouts, and MCP tool schemas. Verified with `make test-q1` and `make install`.
- **M3 (Codex E2E Verification)**: Verify asynchronous spawn followed by `harnez_wait_agent` in Codex CLI, verify cleanup, update architecture documentation, and close ticket.
  - **Pre-Work / Implementation Details**:
    - Run an end-to-end verification inside Codex CLI using the registered `harnez` MCP server.
    - Call `harnez_spawn_agent` with `async: true` to get a session ID and confirm immediate non-blocking return.
    - Call `harnez_wait_agent` with the session ID and verify that the completed result is returned cleanly.
    - Clean up test sessions.
    - Update `docs/HarnezAgentArchitecture.md` with async spawn and wait documentation.
    - Commit and report completion.
