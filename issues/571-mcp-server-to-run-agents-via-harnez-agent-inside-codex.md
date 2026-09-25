# 571 — MCP server to run agents via harnez agent inside Codex

**Status**: Open
**Priority**: P2
**Severity**: Minor
**Category**: Feature

---

## Overview & Goals
Provide an MCP (Model Context Protocol) server subcommand or standalone capability in `harnez` (e.g. `harnez mcp` or `harnez agent mcp`) that exposes tools for dispatching, inspecting, and managing subagents via `harnez agent`. Configure and prove it inside Codex CLI/MCP configuration so an agent in Codex can call subagents via native structured MCP tool calling rather than raw shell commands.

## Milestones

- **M1 (Design & Read-Only Plan)**: Assess MCP architecture/libraries in Go, define tool schemas (`harnez_spawn_agent`, `harnez_list_agents`, etc.), verify Codex MCP configuration mechanism (`~/.codex/config.toml` or CLI args/env), and produce plan.
- **M2 (MCP Implementation)**: Implement MCP server (JSON-RPC stdio protocol) exposing subagent dispatch tools backed by `harnez agent`.
- **M3 (Codex Integration & Verification)**: Wire MCP into Codex configuration and verify end-to-end execution from within Codex.
