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
- **M2 (MCP Implementation)**: Implement MCP server (`harnez mcp`) and subagent tools (`harnez_spawn_agent`, `harnez_list_agents`, etc.).
  - **Pre-Work / Implementation Details**:
    - Implement a clean JSON-RPC 2.0 stdio protocol handler or use an established lightweight Go MCP SDK (e.g. `github.com/mark3labs/mcp-go` or minimalist stdio JSON-RPC server without heavy deps if preferred under repo conventions).
    - Provide `harnez mcp` root command. Ensure zero extraneous stdout prints (all logs/diagnostics go to stderr or `slog` with stderr handler).
    - Implement core subagent tools:
      - `harnez_spawn_agent(prompt: string, model?: string, role?: string, dir?: string, name?: string)`
      - `harnez_list_agents(dir?: string)`
      - `harnez_agent_status(session_id: string)`
      - `harnez_resume_agent(session_id: string, prompt: string)`
      - `harnez_stop_agent(session_id: string)`
    - Add comprehensive unit/integration tests for protocol handling, tool schemas, and execution.
    - Run tests via `make test-q1` or `go test ./...` and run `make install`.
- **M3 (Codex Integration & Verification)**: Wire MCP into Codex configuration (`~/.codex/config.toml` or `codex mcp add`) and verify end-to-end execution from within Codex.
