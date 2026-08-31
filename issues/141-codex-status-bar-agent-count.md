# 141 — Show the running-agent count in the Codex status bar

**Status**: Open
**Priority**: P2 (Medium)
**Severity**: Moderate
**Category**: Feature
**Related**: `internal/statusline`, `internal/usage/process.go`, [[049-running-agent-processes-watch-panel]], [[095-restore-cwd-after-shell-tool-use-and-statusline-cwd-display]]

---

## 1. Problem & Motivation

Codex's status bar does not show how many coding agents are currently
running. A compact live count would make concurrent-agent activity visible
without opening the usage dashboard.

## 2. Technical Specification / Findings

- `internal/usage/process.go` already exposes `CountRunningAgentProcesses`
  and `AgentProcessCount.Total()` for Claude, AGY, and Codex processes.
- The current `harnez statusline` renderer is Claude Code-specific and only
  renders the working directory. Establish the supported Codex status-bar
  integration point before changing it; do not assume Claude's statusLine
  JSON protocol is accepted by Codex.
- The display should be compact and tolerate a failed process probe by
  retaining the rest of the status bar rather than failing the renderer.

## 3. Implementation & Verification Plan

- Determine and document the supported Codex status-bar customization path.
- Add the total running-agent count to the Codex status bar, using the shared
  process-count implementation rather than a second process scanner.
- Cover count formatting and failure handling with focused tests.
- Verify the count against live agent processes and confirm it changes as
  agents start or exit; run the affected Go tests and `harnez status`.
