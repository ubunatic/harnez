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
- **M2 (Core & MCP Implementation)**: Implement async execution, `harnez agent wait`, and `harnez_wait_agent` MCP tool.
  - **Pre-Work / Implementation Details**:
    - Add `--detach` / `--async` flag to `harnez agent start` and detached worker launcher/lifecycle.
    - Implement `harnez agent wait` command with timeout support.
    - Update `internal/mcp` schemas and handlers: add `async` param to `harnez_spawn_agent` and add `harnez_wait_agent`.
    - Add comprehensive unit/integration tests for async spawn, wait with completion, timeout, and MCP tool handling.
    - Verify with `make test-q1`, run `make install`.
- **M3 (Codex E2E Verification)**: Verify asynchronous spawn followed by `harnez_wait_agent` in Codex CLI, verify cleanup, update architecture documentation, and close ticket.
