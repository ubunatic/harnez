# 051 — Multi-host remote usage monitoring & interactive host navigation

**Status**: Open
**Priority**: P2 (Medium)
**Severity**: Moderate
**Category**: Architecture
**Related**: [issues/050-remote-host-flag-and-watch-hotkey.md](file:///home/uwe/projects/harnez/issues/050-remote-host-flag-and-watch-hotkey.md), [internal/usage/watch.go](file:///home/uwe/projects/harnez/internal/usage/watch.go)

## Summary

Expand remote usage monitoring into a complete multi-host system supporting multiple targets configured via CLI (`--hosts host1,host2`), config file, and interactive host switching/navigation in the `--watch` TUI.

## Requirements

1. **Multi-Host Configuration**:
   - Support comma-separated `--hosts <h1,h2,...>` flag or `hosts` list in `~/.claude/harnez/config.yaml` / harness config.
   - Support aliases and custom display names for remote hosts.

2. **Async Polling & Resilience**:
   - Background polling worker per remote host with independent timeouts (prevent slow SSH connections on one host from blocking others or UI responsiveness).
   - Per-host quota cache and fallback to last-known-good metrics on transient network dropouts.

3. **Interactive Multi-Host TUI Navigation**:
   - Host selection bar in TUI header (e.g. `[local]  [workstation]  [gpu-box]`).
   - Quick navigation hotkeys (`Tab`, `Shift+Tab`, `Left`/`Right`, or numeric keys `1-9`) to switch active host view.
   - Dedicated `[R] Remote Summary` overview box showing high-level status (status, token burn rate, active agent count) for all remote hosts at once.

4. **Multi-Host History Aggregation**:
   - Background fetch integration into `~/.claude/harnez/usage-history/` across all configured hosts.
