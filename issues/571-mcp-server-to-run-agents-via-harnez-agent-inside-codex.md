# 571 — MCP server to run agents via harnez agent inside Codex

**Status**: Closed — harnez mcp stdio server implemented and verified end-to-end with Codex
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
  - Registered the installed Harnez binary using `codex mcp add harnez -- /home/uwe/go/bin/harnez mcp`; `codex mcp list` reports it enabled and `codex mcp get harnez --json` confirms stdio transport and command arguments.
  - Codex 0.156.1 discovered the five tools. With `codex exec --approve-for-me`, a real `harnez_spawn_agent` call returned `MCP_E2E_OK` from the requested `codex:luna:low` session (`mcp571-e2e`, ID `01a0d879-3f2a-7db2-aec7-46dff68b9744`).
  - The end-to-end call exposed missing `agent` argument forwarding in the MCP subprocess. Fixed it and strengthened the subprocess test to assert `agent start`; `make test-q1` passed and `make install` completed.
  - Cleanup remains blocked: this worker's enforced `developer` leaf role rejects `harnez agent delete` with “must not start, resume or manage agents.” The completed test record remains at `~/.harnez/agents/01a0d879-3f2a-7db2-aec7-46dff68b9744.json`; no child process remains running. Do not bypass the role guard to remove it; an authorized orchestrator can delete it.
