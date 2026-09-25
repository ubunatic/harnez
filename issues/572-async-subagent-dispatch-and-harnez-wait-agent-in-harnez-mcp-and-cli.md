# 572 — Async subagent dispatch and harnez_wait_agent in harnez mcp and CLI

**Status**: Open
**Priority**: P2
**Severity**: Minor
**Category**: Feature

---

## Overview & Goals
Allow agents calling `harnez` via MCP (or CLI) to spawn subagents asynchronously in the background, continue other tasks, and later wait on completion using a dedicated `harnez_wait_agent` tool (or CLI command `harnez agent wait`).

## Milestones

- **M1 (Design & Read-Only Plan)**:
  - Inspect `harnez agent start` lifecycle, session execution, process management, and status storage in `internal/subagent`.
  - Design `--detach`/`--async` behavior for CLI and MCP `harnez_spawn_agent(..., async=true)`.
  - Design `harnez agent wait` / MCP tool `harnez_wait_agent(session_id, timeout_seconds?)`.
  - Produce architectural plan without modifying code.
- **M2 (Core & MCP Implementation)**:
  - Implement async process execution / session state transitions.
  - Implement `harnez agent wait` command and `harnez_wait_agent` MCP tool.
  - Add unit and protocol tests, verify with `make test-q1`, run `make install`.
- **M3 (Codex E2E Verification)**:
  - Verify asynchronous spawn followed by `harnez_wait_agent` from within Codex CLI session.
  - Clean up test sessions, update architecture docs, commit and close.
