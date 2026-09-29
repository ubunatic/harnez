# 608 — Promote CLI command over MCP to ensure background job visibility in host sessions

**Status**: Closed — merged into 643
**Priority**: P1 (High)
**Severity**: Major
**Category**: Architecture
**Related**: #547 (turn detach and wait reattach), #611 (zero polling & scheduling)

---

## 1. Problem & Motivation

When AI assistants (e.g., in Antigravity, Claude Code, Cursor) invoke Harnez subagents via MCP tools (`mcp__harnez__spawn_agent` with `async: true`), the spawned process runs as a detached worker in the Harnez daemon. 

However, MCP tool execution does not integrate with the host assistant's native background task UI / process manager. As a result:
1. The user cannot see the active background job in the chat UI task monitor.
2. The user has no visibility into what is running or whether zombie processes are accumulating.
3. Unmonitored subagents spawning further subagents without visual counters degrades process hygiene and violates the Zero Zombie Guarantee.

To provide real backgrounding with host UI visibility, interactive sessions must promote running the CLI command directly via the host's background runner (`run_command` / bash in background) rather than relying on detached MCP execution.

## 2. Technical Specification / Findings

- `harnez agent start --name <name> --role <role> --model <model> -p <prompt>` executed in the host terminal/bash runner registers as a native background task with live UI status, stdout/stderr tracking, and reactive notification on completion.
- MCP tools lack a standard protocol mechanism to register host-level background tasks with interactive IDE/chat task managers.
- Rules in `Tools.md` and `Subagents.md` should prioritize and instruct assistants to use `harnez agent start` (via `harnez_command` or CLI directly in background runner) in interactive host environments.

## 3. Implementation & Verification Plan

1. Update rule templates (`Tools.md`, `Subagents.md`, `AgenticLoop.md`) to explicitly recommend the CLI `harnez agent start` command via host background runner for all interactive sessions.
2. Ensure MCP documentation clarifies that `harnez_command` -> host background runner is preferred when UI task tracking is required.
3. Verify that interactive sessions display active subagents in host background task lists.

