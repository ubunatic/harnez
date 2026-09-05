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

---

## Implementation Plan

### Current state (verified 2026-09-04)

- Remote support is **single-host, two independent knobs**:
  - `harnez usage --host <h>` (`cmd/harnez/main.go:263`) → `usage.CollectRemote`
    (`internal/usage/remote.go`), one stateless `ssh -o BatchMode=yes <h>
    "harnez usage --json"` round trip, whole-summary replacement.
  - `load.watch_host` in `~/.config/harnez/local.yaml`
    (`internal/usage/localconfig.go:57`) → the separate `[R] Remote Load` panel,
    with a real streaming manager (`runRemoteLoadManager`,
    `internal/usage/watch.go:2613`) plus a 10s batch-poll fallback.
- `LocalConfig` has exactly two scalar host fields (`usage.default_host`,
  `load.watch_host`) — no list, no aliases (`internal/usage/localconfig.go:50-74`).
- `--watch` key handling is a byte dispatcher (`dispatchWatchKey`,
  `watch.go:481`) over single-key panel toggles; panels are assembled as a
  `[]panel` with letter keys (`watch.go:1813`). There is **no notion of an
  active host** anywhere in the watch state — the remote host is a fixed string
  captured once at startup (`watch.go:2231`).
- 050 (`--host` flag + watch hotkey) is **Closed**; this ticket is the
  strictly larger follow-on.

### Sequencing decision: split into three landable stages

The user's dominant workflow is one workstation with `--watch` permanently
open; remote is the secondary case. Building the full multi-host TUI up front
would be premature. Land in this order, each independently useful:

**Stage A — multi-host config & data plumbing (no UI change)**
1. `internal/usage/localconfig.go`: add `LocalUsageConfig.Hosts []HostEntry`
   with `HostEntry{Name, SSHHost string}` (`yaml:"hosts"`, entries `- name: gpu-box`
   / `ssh: gpu-box.lan`). Keep `default_host` working unchanged — it stays the
   single-host shorthand; when both are present `default_host` is treated as
   the initially-selected entry, and a bare string list entry (`- gpu-box`)
   must parse as `{Name: "gpu-box", SSHHost: "gpu-box"}` via a custom
   `UnmarshalYAML` so the common case stays terse.
2. `cmd/harnez/main.go`: add `--hosts h1,h2` on `usageCmd`, mutually exclusive
   with `--host` (extend `validateUsageFlags`, which currently only guards
   render-target conflicts — keep that function the single place flag conflicts
   are rejected). Resolution order mirrors `resolveUsageHost`: `--hosts` >
   `--host` > `usage.hosts` > `usage.default_host`.
3. New `internal/usage/multihost.go`: `type HostSummary{Host HostEntry;
   Summary UsageSummary; Procs *AgentProcessCount; FetchedAt time.Time; Err error}`
   and `CollectHosts(ctx, []HostEntry, includeProcs) []HostSummary` — one
   goroutine per host, `context.WithTimeout` **per host** (not one shared
   deadline; requirement 2's core point is that a slow host must not stall the
   others), results collected in input order. Reuse `CollectRemoteProgress` so
   the existing `--watch` startup splash gets per-host `FetchStarted/Done/Failed`
   events for free.
4. Per-host last-known-good cache: reuse the existing state-cache pattern
   (`internal/usage/statecache.go`, `livefetchcache.go`) keyed by host name
   rather than inventing a new store; on a failed fetch, serve the cached
   summary and mark it stale via the existing `applyStaleQuota` " (stale)"
   convention rather than a new field.
5. Tests: extend `remote_test.go`-style table tests with a fake collector —
   assert (a) one slow host does not delay the others' results, (b) a failing
   host yields cached-and-marked-stale rather than dropping out, (c) config
   parse of both terse and full `hosts` forms.

**Stage B — `[R] Remote Summary` overview panel (read-only, no navigation)**
6. `internal/usage/watch.go`: add `buildRemoteSummaryBox(width, []HostSummary)`
   next to `buildRemoteLoadBox`, one line per host: name, status/staleness,
   tokens-per-min burn rate, agent count. Register it in the `panels` slice
   (`watch.go:1813`) under its own letter and add its toggle to
   `applyWatchSectionKey`/`watchSections` so it follows the existing panel
   plumbing exactly. Add a golden-ish render test alongside the existing
   `watch_test.go` box tests, including a host with `Err != nil`.
7. Wire a `runMultiHostManager` alongside `runRemoteLoadManager` in
   `RunWatchWithOptions` — same shape (mutex + `last*` pointer + `requestRedraw`),
   deliberately *not* a generalized framework.

**Stage C — interactive host switching (only if B proves worth it)**
8. Add `activeHost int` to the watch key state and handle `Tab`/`Shift+Tab`
   and `1`-`9` in `dispatchWatchKey`. Note `dispatchWatchKey` takes a single
   `byte`: `Shift+Tab` is a multi-byte CSI sequence (`ESC [ Z`), so either
   restrict to `Tab` + digits + `[`/`]`, or first widen the reader to decode
   escape sequences — a real, separate refactor that should be its own ticket
   if chosen.
9. Header host-selection bar rendered from the same `[]HostSummary`,
   highlighting `activeHost`; the per-agent panels then render that host's
   summary instead of the local one.

**Requirement 4 (history aggregation)** — defer to its own ticket. `history.go`
writes `~/.claude/harnez/usage-history/` with no host dimension; adding one
touches the on-disk format and the `harnez stats` aggregation work already in
flight (227/228). Do not fold it into this ticket.

### Design decisions / tradeoffs

- **`usage.hosts` is additive to `default_host`, not a replacement.** Breaking
  the existing single-host config would break the primary workflow for a
  secondary feature.
- **Reuse `harnez usage --json` over SSH as the transport.** No new remote
  protocol, no ControlMaster/persistent connections (issue 110 Decision §2
  already settled that everything except the streaming load panel uses plain
  stateless SSH).
- **`load.watch_host` stays separate.** Merging remote *load* and remote
  *usage* host lists is tempting but they are deliberately independent
  (`localconfig.go:57-63` says so). Leave it.
- **Aliases are a display concern only** — `HostEntry.Name` is never passed to
  `ssh`; `validSSHHostRe` still validates `SSHHost`.

### Risks / open questions

- **Escape-sequence decoding** (Stage C step 8) is the one genuine
  architectural change; scoping the TUI to `Tab` + digits avoids it entirely.
- **Screen budget**: a multi-host bar plus a remote-summary box in an already
  panel-dense `--watch` may not fit at typical widths. Stage B should be
  validated against `scripts/canary-watch-pty.sh` at 80 and 120 columns.
- **Is Stage C actually wanted?** If the aggregate overview from Stage B
  answers "how are my other boxes doing", per-host drill-in navigation may
  never be needed. Re-evaluate before starting C.
- Fan-out SSH on every watch tick against N hosts multiplies connection cost;
  keep the multi-host poll on its own slower cadence (10s+, like
  `remoteLoadRetryInterval`) rather than the watch interval.

### Scope

**Large overall.** Stage A small-to-medium, Stage B medium, Stage C medium
(large if escape-sequence decoding is included). Requirement 4 split out.
