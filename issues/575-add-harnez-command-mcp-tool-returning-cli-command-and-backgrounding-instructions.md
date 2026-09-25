# 575 — Add harnez_command MCP tool returning CLI command and backgrounding instructions

**Status**: Open
**Priority**: P2
**Severity**: Minor
**Category**: Feature

---

## Overview & Goals
Provide a helper tool in the MCP server (e.g. `harnez_format_command` or `harnez_command`) and updated tool descriptions for `harnez_spawn_agent`:
1. When an agent in an interactive host harness (like Antigravity / Claude Desktop) wants to launch a subagent that appears as a visible, non-blocking background job in the chat UI with automatic reactive wakeups, the MCP tool returns the exact, safely formatted shell command line (e.g. `harnez agent start ...`).
2. Provides explicit instructions/hints advising the agent to execute this command line via its native Bash / `run_command` tool with backgrounding enabled.

## Milestones

- **M1 (Design & Tool Schema Plan)**:
  - Design the `harnez_command` / `harnez_format_command` tool schema taking structured arguments (`prompt`, `model`, `role`, `dir`, `name`, `stream`, etc.).
  - Return `{ command: string, instruction: string }` explaining how to execute in shell for visible background task integration.
  - Update `harnez_spawn_agent` description to clarify direct MCP execution vs shell backgrounding.
  - Produce read-only plan.
- **M2 (Implementation & Tests)**:
  - Implement `harnez_command` tool in `internal/mcp/server.go`.
  - Add unit and protocol tests.
  - Verify with `make test-q1` and `make install`.
- **M3 (Verification & Documentation)**:
  - Verify tool output and schema discovery.
  - Update `docs/HarnezAgentArchitecture.md`.
  - Commit and close ticket.
