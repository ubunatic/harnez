# 085 — Briefly Show Agent-Collector Daemon Status in `harnez usage --watch`

**Status**: Open
**Priority**: P3 (Low)
**Severity**: Minor
**Category**: UX / Agentic Ergonomics
**Related**: [[082-agent-usage-collector-daemon]], [[083-usage-tui-self-hiding-auto-discovery]]

## Problem

Since [[082-agent-usage-collector-daemon]], `harnez usage` can be backed by the
`harnez agent-collector` background daemon's cached snapshots instead of always collecting live.
The TUI currently gives no indication of whether that daemon is running, so the user has no quick
way to tell if they're looking at fresh daemon-backed data, a stale cache, or live-fallback
collection.

## Desired Behavior

- When `harnez usage --watch` starts, briefly show the agent-collector daemon's status
  (e.g. running / not running, and possibly last-collected timestamp) somewhere in the TUI —
  keep it minimal, not a permanent prominent box. A small header/status-line indicator that shows
  once on startup (or is otherwise unobtrusive) is likely sufficient; exact placement/format is an
  implementation detail, not specified here.

## Next Steps

- Decide how to detect "daemon running" (e.g. systemd `--user` unit active state, or simply
  freshness of the per-agent cache snapshots already used by [[082-agent-usage-collector-daemon]]'s
  cache-first `CollectAll`).
- Implement a small, non-intrusive status indicator in `internal/usage/watch.go`.
