# 143 — Show Git status in all agent status bars

**Status**: Open
**Priority**: P2 (Medium)
**Severity**: Moderate
**Category**: Feature
**Related**: [[095-restore-cwd-after-shell-tool-use-and-statusline-cwd-display]], [[141-codex-status-bar-agent-count]], `internal/statusline`, `internal/claude/apply.go`

---

## 1. Problem & Motivation

Agent status bars do not show the Git state of the active workspace. Users
need a compact, consistent indication of branch and working-tree status
across all supported agent integrations, so uncommitted work and repository
context are visible without a separate terminal command.

## 2. Technical Specification / Findings

- Define the supported status-bar integration surface for Claude, Codex, AGY,
  and other managed agents before assuming a shared protocol.
- Render useful Git state compactly: repository/branch identity and whether
  tracked or untracked changes are present; handle detached HEAD, non-Git
  directories, and Git command failures without breaking the status bar.
- Reuse one shared Git-status implementation so each agent surface cannot
  drift in formatting or semantics.

## 3. Implementation & Verification Plan

- Inventory every supported agent status-bar integration and document any
  platform limitations or fallback behavior.
- Add shared Git-status collection and compact rendering, with bounded
  latency suitable for frequent status-bar refreshes.
- Wire the renderer into every supported agent status bar while preserving
  existing status-bar content and graceful degradation.
- Add focused tests for clean, modified, untracked, detached-HEAD, non-repo,
  and Git-error states; verify each available integration manually.
