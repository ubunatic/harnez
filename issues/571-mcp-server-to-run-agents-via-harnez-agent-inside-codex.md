# 571 — MCP server to run agents via harnez agent inside Codex

**Status**: Open
**Priority**: P2
**Severity**: Minor
**Category**: Feature

---

## Overview & Goals
Provide an MCP (Model Context Protocol) server subcommand or standalone capability in `harnez` (e.g. `harnez mcp` or `harnez agent mcp`) that exposes tools for dispatching, inspecting, and managing subagents via `harnez agent`. Configure and prove it inside Codex CLI/MCP configuration so an agent in Codex can call subagents via native structured MCP tool calling rather than raw shell commands.

## Milestones

- **M1 (Design & Read-Only Plan)**: Completed.
  - Plan approved: Root Cobra command `harnez mcp` running a clean JSON-RPC 2.0 stdio server, tool implementations in `internal/mcp` reusing `internal/subagent` & `cmd/harnez` session logic, initial tools (`harnez_spawn_agent`, `harnez_list_agents`, `harnez_agent_status`, `harnez_resume_agent`, `harnez_stop_agent`), stdout reserved strictly for MCP JSON-RPC messages with stderr diagnostics.
- **M2 (MCP Implementation)**: Completed in commit `d629ffe`.
  - Implemented `harnez mcp` subcommand and stdio JSON-RPC 2.0 protocol engine (`internal/mcp/server.go`).
  - Added tools: `harnez_spawn_agent`, `harnez_list_agents`, `harnez_agent_status`, `harnez_resume_agent`, `harnez_stop_agent`.
  - Added unit tests for protocol lifecycle (`initialize`, `notifications/initialized`, `tools/list`, `ping`), schema validation, tool execution, and error handling. Verified via `make test-q1` and `make install`.
- **M3 (Codex Integration & Verification)**: Wire MCP into Codex configuration and verify end-to-end execution from within Codex.
  - **Pre-Work / Implementation Details**:
    - Configure Codex to use `harnez mcp` (e.g. via `~/.codex/config.toml` or `codex mcp add harnez -- harnez mcp`).
    - Verify `codex mcp list` recognizes `harnez` server and tools.
    - Perform an end-to-end verification proving a Codex agent session can invoke subagent operations using the MCP server.
    - Ensure teardown/cleanup of any test sessions, and document usage in relevant docs or test notes.
    - Run tests, install binary if changed, and commit M3 completion.
