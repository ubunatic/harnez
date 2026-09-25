# 583 — mcp: register harnez MCP server for Claude Code

**Status**: In Progress
**Priority**: P2 (Medium)
**Severity**: Moderate
**Category**: Feature
**Related**: [Harnez agent architecture](../docs/HarnezAgentArchitecture.md), [574](574-register-harnez-mcp-server-in-agy-configuration.md)

---

## 1. Problem & Motivation

Claude Code users need a documented way to register the Harnez MCP server and
`harnez apply` must allow the server's tools without prompting on every call.
Codex (ticket 571) and AGY (574) are registered manually; Claude Code should
use its user-scope command so the server is available across projects.

## 2. Technical Specification / Findings

- Add `claude mcp add --scope user harnez -- harnez mcp` and `claude mcp list`
  instructions to the agent architecture guide.
- Add `mcp__harnez__*` through config-driven Claude permissions, preserving
  existing permission entries.
- Claude Code's current documentation says personal/user-scope MCP servers are
  stored in `~/.claude.json`; project servers use `.mcp.json`. `settings.json`
  manages settings and permissions, not MCP server definitions. Therefore the
  prior `mcp_servers` configuration that wrote `mcpServers` into
  `~/.claude/settings.json` was ineffective and must be removed. Harnez will not
  write directly to `~/.claude.json`; the documented Claude CLI owns that file.
- Source: https://code.claude.com/docs/en/mcp

## 3. Implementation & Verification Plan

- Update config, settings generation, status reporting, tests, and stale docs.
- Run `make test-q1` once after a clean-tree check.

Verification note: the single `make test-q1` run exited 2 because an existing
integration test removed the first allow-list entry when simulating drift; the
new configured MCP permission changed that ordering. The test now removes its
named `Bash(journalctl *)` entry instead. Host reran `make test-q1` after 863a446: exit 0.

## Host review

terra:med review: only ticket-text findings. The remaining `mcp_servers` mention above is the historical finding, intended. Verification note corrected.

