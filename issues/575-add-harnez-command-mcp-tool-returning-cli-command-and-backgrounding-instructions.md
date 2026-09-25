# 575 — Add harnez_command MCP tool returning CLI command and backgrounding instructions

**Status**: Open
**Priority**: P2
**Severity**: Minor
**Category**: Feature

---

## Overview & Goals
Provide a helper tool in the MCP server (`harnez_command`) and updated tool descriptions for `harnez_spawn_agent`:
1. When an agent in an interactive host harness (like Antigravity / Claude Desktop) wants to launch a subagent that appears as a visible, non-blocking background job in the chat UI with automatic reactive wakeups, the MCP tool returns the exact, safely formatted shell command line (e.g. `harnez agent start ...`).
2. Provides explicit instructions/hints advising the agent to execute this command line via its native Bash / `run_command` tool with backgrounding enabled.

## Milestones

- **M1 (Design & Tool Schema Plan)**: Completed.
  - Plan approved:
    - Tool schema: `harnez_command(action: "start"|"resume"|"wait"|"status", prompt?: string, model?: string, role?: string, name?: string, dir?: string, session_id?: string, stream?: string)`
    - Formats safe, shell-quoted `harnez agent <action> ...` invocation.
    - Returns `{ command: string, instruction: string }` explaining that running this command via the host's native `Bash` or `run_command` tool creates a visible background task in the chat UI with automatic completion wakeup.
    - Updated description on `harnez_spawn_agent` highlighting that `harnez_spawn_agent` runs directly inside MCP, whereas `harnez_command` prepares a command for shell-level background execution.
- **M2 (Implementation & Tests)**: Completed in commit `741b44c`.
  - Implemented `harnez_command` in `internal/mcp/server.go`.
  - Added robust POSIX shell argument quoting (`shellQuote`).
  - Added unit/protocol tests covering all action mappings (`start`, `resume`, `wait`, `status`), escaping, validation, and JSON-RPC dispatch. Verified with `make test-q1` and `make install`.
- **M3 (Verification & Documentation)**:
  - **Pre-Work / Implementation Details**:
    - Update `docs/HarnezAgentArchitecture.md` documenting `harnez_command` and when to use it vs direct `harnez_spawn_agent`.
    - Verify schema discovery and tool execution via MCP.
    - Clean up developer session `dev-575-m1`, commit documentation, and close issue.
