# 161 — Collector: Absorb Remote-Load Control Host + Opt-In Prometheus Exposition

**Status**: Open
**Priority**: P2 (Medium)
**Severity**: Minor
**Category**: Architecture
**Related**: [[082-agent-usage-collector-daemon]] (the daemon this ticket extends),
  [[110-remote-load-batch-vs-streaming-collection-modes]] (the SSH `ControlMaster` this ticket
  relocates), [[160-watch-viewer-server-split-feasibility]] (split out from — 160 stays scoped to
  the `--watch` viewer/frame-painting split, kept separate from `agent-collector`; this ticket is
  the collector-architecture side, deliberately kept apart so 160 doesn't mix the two), `internal/usage/watch.go`
  (`runRemoteLoadManager`, `RunWatchWithOptions`'s remote-Load lifecycle wiring), `internal/usage/remote.go`,
  `internal/usage/collector.go`

## Problem / Motivation

Two related refinements to the collector architecture, decided in discussion after 160's
feasibility assessment:

1. **Remote-Load's SSH `ControlMaster` is currently owned by the `--watch` process**
   (`runRemoteLoadManager`, started/torn down inside `RunWatchWithOptions`'s own lifecycle —
   see `watch.go` around the `remoteLoadHost`/`remoteStreamStop` wiring). That's data-gathering,
   not display, so unlike 160's frame-building/display split (kept separate from
   `agent-collector`), this piece belongs *inside* `agent-collector`'s collection boundary. Today,
   remote-Load streaming dies with `--watch` and has to reconnect from scratch every time
   `--watch` restarts (including every restart during `harnez` development itself — the exact
   friction 160 is about, but for the remote side specifically).
2. **`agent-collector` should optionally expose its collected data as Prometheus metrics.** The
   user already runs Grafana on `x600` (homeserver); a `/metrics` endpoint on `agent-collector`
   would let Claude/Codex/AGY usage and remote-Load data be graphed there instead of only in the
   `--watch` TUI. Must be **opt-in** (flag/config, off by default) — `agent-collector` is
   currently a pure filesystem-writer with no network listener, and that no-network default
   should be preserved for anyone who doesn't want it.

## Desired Outcome

1. `agent-collector` owns the remote-Load `ControlMaster` lifecycle: establishes and maintains
   the SSH control connection independent of whether any `--watch` viewer is currently attached,
   persists across `--watch` restarts, and is torn down on the collector's own shutdown (not
   the viewer's).
2. `--watch` (and, if/when 160's viewer/server split lands, the thin viewer) reads remote-Load
   data from the collector the same way it already reads local agent usage snapshots (082's
   `~/.local/state/harnez/agents/usage/<agent>.json` pattern), rather than managing its own SSH
   connection.
3. `agent-collector` gains an opt-in `--metrics-addr` (or config-file equivalent) that, when set,
   serves collected usage/remote-load data at `/metrics` in Prometheus text exposition format.
   Off by default; setting it is the only way to open a network listener.

## Acceptance Criteria

- [ ] Remote-Load SSH `ControlMaster` setup/teardown moves from `internal/usage/watch.go`'s
      `RunWatchWithOptions` into `agent-collector`'s own process lifecycle.
- [ ] `--watch` no longer starts/stops the SSH control connection itself; it consumes
      collector-published remote-load data instead.
- [ ] Remote-Load streaming survives a `--watch` restart (verify: start `--watch` with
      `--remote-load-host`, kill/restart `--watch`, confirm no fresh SSH handshake / reconnect
      delay on the second launch).
- [ ] `agent-collector` accepts an opt-in flag to serve `/metrics`; absent the flag, no listener
      is opened (verify via `ss`/`lsof` — no listening socket without the flag).
- [ ] `/metrics` output is valid Prometheus text exposition format (verify with
      `promtool check metrics` or equivalent) covering at minimum per-agent usage (tokens,
      quota windows) and remote-load GPU/CPU/memory data.
- [ ] Existing filesystem-snapshot consumers (082's JSON files) are unaffected — Prometheus
      exposition is additive, not a replacement for the existing read path.
