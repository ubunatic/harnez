# 574 — Register harnez MCP server in AGY configuration

**Status**: Open
**Priority**: P2
**Severity**: Minor
**Category**: Feature

---

## Overview & Goals
Configure and verify the `harnez mcp` server in AGY (Antigravity CLI / IDE / `~/.gemini/config/mcp_config.json` via `agy mcp add`) so that AGY agents can also natively discover and invoke `harnez_spawn_agent`, `harnez_wait_agent`, `harnez_list_agents`, `harnez_agent_status`, `harnez_resume_agent`, and `harnez_stop_agent`.

## Milestones

- **M1 (AGY MCP Configuration & Discovery)**: Completed.
  - Identified AGY MCP configuration: `~/.gemini/config/mcp_config.json` via `agy mcp add harnez "$(command -v harnez)" mcp`.
  - Global `mcpServers` format: `"harnez": {"command": "/home/uwe/go/bin/harnez", "args": ["mcp"]}`.
- **M2 (Verification & Documentation)**:
  - **Pre-Work / Implementation Details**:
    - Run `agy mcp add harnez /home/uwe/go/bin/harnez mcp` (or register in `~/.gemini/config/mcp_config.json`).
    - Verify with `agy mcp list`.
    - Update `docs/HarnezAgentArchitecture.md` documenting both Codex (`codex mcp add`) and AGY (`agy mcp add`) setup and tool availability.
    - Run `make test-q1` if repo files are modified, and commit M2 changes.
