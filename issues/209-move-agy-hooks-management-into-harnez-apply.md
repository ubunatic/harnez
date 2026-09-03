# 209 — Move `agy-hooks` Management into `harnez apply`

**Status**: Open
**Priority**: P2 (Medium)
**Severity**: Minor
**Category**: Refactor / Architecture
**Related**: [193-research-agy-hook-surface-for-transparent-exec-distill.md](193-research-agy-hook-surface-for-transparent-exec-distill.md), [196-agy-native-hooks-plan-alongside-claude-hooks.md](196-agy-native-hooks-plan-alongside-claude-hooks.md), [200-codex-native-hooks-preTooluse-wiring.md](200-codex-native-hooks-preTooluse-wiring.md), [119-harnez-hook-agent-hook-management.md](119-harnez-hook-agent-hook-management.md)

---

## 1. Problem & Motivation

`harnez` currently exposes `harnez agy-hooks` as a top-level CLI command to manage Antigravity's `~/.gemini/config/hooks.json` (`apply` and `status`).

However:
1. **Inconsistent CLI Surface**: Claude Code hooks are managed automatically via `harnez apply` (and checked via `harnez status`). Codex hook management was also folded into `harnez apply` (issue 200). Having `agy-hooks` sit as an unmanaged, separate top-level command is an ergonomics anomaly that clutters `harnez --help`.
2. **Setup Friction**: Users configuring an environment with `harnez apply` expect all supported harness integrations (Claude, Codex, AGY) to be applied or managed uniformly, without needing to discover and invoke a separate top-level `harnez agy-hooks apply` command.

## 2. Proposed Solution

1. **Fold AGY Hook Application into `harnez apply`**:
   - Make `harnez apply` configure `~/.gemini/config/hooks.json` by default alongside `~/.claude/settings.json` and `~/.codex/config.toml` (or guarded by an agent target flag, e.g. `--target` / `--agent=all|claude|agy|codex`).
2. **Move Status & Drift Detection into `harnez status`**:
   - `harnez status` should report whether AGY hooks are installed, up-to-date, or drifted, just as it does for Claude and Codex.
3. **Demote or Retain Internal Runtime Hook**:
   - Keep the hook execution endpoint itself (e.g. `harnez agy-hooks hook` or rename to `harnez hook agy` / `harnez exec hook --agent=agy`) as a non-top-level plumbing command called by `hooks.json`.
   - Deprecate or remove `agy-hooks` as a top-level user-facing management command.

## 3. Acceptance Criteria

1. Running `harnez apply` applies the `hooks.json` configuration for Antigravity when `~/.gemini` exists.
2. `harnez status` reports AGY hook status alongside Claude and Codex.
3. `harnez --help` no longer lists `agy-hooks` as a top-level user command (it is either removed or hidden).
4. `go test ./...` passes cleanly across `cmd/harnez` and `internal/agy`.
