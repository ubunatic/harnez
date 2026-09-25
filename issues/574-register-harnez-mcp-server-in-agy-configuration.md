# 574 — Register harnez MCP server in AGY configuration

**Status**: Open
**Priority**: P2
**Severity**: Minor
**Category**: Feature

---

## Overview & Goals
Configure and verify the `harnez mcp` server in AGY (Antigravity CLI / IDE / `~/.gemini/antigravity-cli/mcp_config.json` or `~/.gemini/antigravity-cli/settings.json` or `agy mcp` commands) so that AGY agents can also natively discover and invoke `harnez_spawn_agent`, `harnez_wait_agent`, `harnez_list_agents`, `harnez_agent_status`, `harnez_resume_agent`, and `harnez_stop_agent`.

## Milestones

- **M1 (AGY MCP Configuration & Discovery)**:
  - Identify AGY MCP configuration format and location (e.g. `~/.gemini/antigravity-cli/mcp_config.json`, project `.gemini/mcp_config.json`, or CLI commands).
  - Register `harnez` stdio server pointing to `harnez mcp`.
  - Verify AGY discovers the tool schemas.
- **M2 (Verification & Documentation)**:
  - Verify tool invocation / subagent execution via AGY.
  - Update `docs/HarnezAgentArchitecture.md` with AGY configuration instructions.
  - Clean up test sessions, commit and close ticket.
