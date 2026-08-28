# 082 — Background Usage-Collector Daemon (systemd user service)

**Status**: Closed — resolved in d5d7566
**Priority**: P2 (Medium)
**Severity**: Moderate
**Category**: Feature
**Related**: [[023-usage-command-token-quota-tracking]], [[030-agy-codex-missing-local-token-counts]], [[033-usage-shared-quota-cache]], docs/studies/2026-08-17-multi-agent-quota-and-usage-monitoring.md

## Problem

`harnez usage` currently collects live (Claude API, Codex API, AGY local RPC) on each invocation,
with only a single Claude-specific on-disk cache (`~/.claude/harnez-quota-cache.json`, see 033).
There is no shared, agent-agnostic, always-on collection mechanism — every TUI/CLI launch pays
the live-fetch cost itself, and Codex/AGY have no cache at all.

Prior art: Omarchy 4.0's Quickshell "Agents" bar widget solves this by running periodic shell
scripts that write JSON usage snapshots to `~/.local/state/omarchy/agents/usage/`, decoupled from
the display layer. We want the same decoupling, but implemented in pure Go (no bash scripts,
matching `docs/lang/Go.md`) and running as a systemd `--user` service so it's independent of any
terminal session being open.

## Desired Behavior

- New `harnez` subcommand (e.g. `harnez agent-collector` or `harnez collect --daemon`) that runs
  the existing Go collectors (`internal/usage/{claude,codex,agy}.go`) on a timer.
- Writes atomic JSON snapshots (write-tmp + rename, no bash, no external lock tooling needed for
  the read side) to a new shared state path, e.g. `~/.local/state/harnez/agents/usage/<agent>.json`.
  No such shared harnez state directory currently exists — confirm/decide the exact XDG-style path
  as part of this ticket.
- `harnez usage` / the TUI reads from this cache first (falling back to live collection only if
  the daemon isn't running / cache is stale/missing), rather than always hitting the network/RPC
  inline.
- Ship a systemd `--user` unit file plus install/enable wiring (likely via `harnez apply` or a
  dedicated install step — follow `docs/CLIDesign.md`'s `apply`/`init` scope separation).
- Supersedes/generalizes the Claude-only cache mechanism from issue 033 into a shared,
  multi-agent cache.

## Notes

- Writer/reader-across-processes on small JSON files is not an efficiency concern: atomic
  rename avoids torn reads, and this is strictly faster for callers than today's inline
  live-fetch-per-invocation model.
- Does not need to solve Codex/AGY's missing local token-count sources (see 030) — this ticket is
  about the collection/caching architecture, not new data sources.

## Resolution

Implemented as `harnez agent-collector` (Cobra subcommand, `internal/usage/collector.go`):

- `--interval` (default `usage.DefaultCollectorInterval` = 900s, matching Omarchy's cadence),
  `--once` (collect and write once, no timer loop — used for cron/manual runs and tests),
  `--offline` (skip live network queries, mirroring `harnez usage --offline`).
- State path: `internal/usage/statecache.go`'s `StateDir(homeDir)` resolves
  `$XDG_STATE_HOME/harnez/agents/usage` and falls back to
  `~/.local/state/harnez/agents/usage` when `XDG_STATE_HOME` is unset. One JSON file per agent:
  `<state-dir>/<agent-id>.json`, containing `{"fetched_at": <RFC3339>, "usage": <AgentUsage>}`.
  Writes are atomic (temp file + `os.Rename`, no flock — the daemon is the sole writer, unlike
  the cross-process `quotaCache` from issue 033).
- `usage.CollectAll` (used by `harnez usage`, `--watch`, `--summary`, and `usage history record`)
  now reads each agent's snapshot first via `cacheOrLive`, using it if fresher than
  `DefaultCacheStaleness` (2x the default collector interval = 30 min), and falling back to a
  live collect per agent otherwise — so behavior is unchanged when the daemon has never run.
  `usage.CollectAllLive` (always live, ignores cache) is what the daemon itself uses on every
  tick, so it never just reads back its own cache.
- Systemd `--user` unit: `systemd/harnez-agent-collector.service`, embedded via `embed.go` and
  installed to `~/.config/systemd/user/harnez-agent-collector.service` by
  `harnez apply --systemd` (opt-in flag — see `internal/claude/apply.go`'s `installSystemdUnit`).
  `{{HARNEZ_BIN}}` in the template is substituted with `os.Executable()`'s absolute path at apply
  time, since a systemd `--user` session does not reliably inherit the invoking shell's `PATH`.
  Kept opt-in (not part of plain `apply`) because it writes to a real, home-relative path outside
  the `-t` target dir that `apply`'s own tests don't sandbox.
- Tests: `internal/usage/statecache_test.go` (atomic write/read roundtrip, missing/corrupt
  snapshot handling, `IsFresh` staleness table, `StateDir` XDG resolution table, `CollectAll`
  cache-first/stale-fallback/`CollectAllLive`-ignores-cache), `internal/usage/collector_test.go`
  (`collectTick` writes one snapshot per agent, `RunCollector` stops promptly on context
  cancellation), `internal/claude/systemd_test.go` (`installSystemdUnit` expands `ExecStart` and
  is idempotent).
