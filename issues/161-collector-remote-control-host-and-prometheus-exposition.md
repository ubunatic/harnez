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

---

## Implementation Plan

### Research notes (2026-09-04)

- `runRemoteLoadManager` is at `internal/usage/watch.go:2613` (helpers `drainRemoteLoadStream`,
  `pollOnce`, `waitOrDone` follow it to EOF at 2688). Its lifecycle wiring lives in
  `RunWatchWithOptions` at **watch.go:2176**, specifically 2225-2256 (`remoteLoadHost`,
  `remoteStreamStop`, `setRemoteStreamStop`, the `go runRemoteLoadManager(...)` launch and the
  deferred synchronous teardown) and 2365-2377 (feeding `lastRemoteLoad` into the frame).
- The manager is already written as a self-contained goroutine parameterised by callbacks
  (`mu`, `last`, `redraw`, `setStop`, `setStreaming`). Only `redraw` and `setStreaming` are
  display concerns; `setLast` is pure data. **This is the good news: the split is mostly mechanical**
  — replace `redraw` with a no-op and `setLast` with a snapshot writer, and the manager runs
  headless unchanged.
- Collector side: `RunCollector` (`internal/usage/collector.go:25`) is a plain
  collect-once-then-ticker loop writing via `PersistAgentSnapshot` into `StateDir(homeDir)`
  (`internal/usage/statecache.go:80`, `~/.local/state/harnez/agents/usage/<agent>.json`). Read path
  is `ReadAgentSnapshot` (statecache.go:165) / `cacheOrLive` (:205), gated by
  `AgentSnapshot.IsFresh` (:61).
- Collector command: `cmd/harnez/main.go:389-421`. **Note [[152]] plans to move it to
  `harnez usage collector`** — coordinate; whichever lands second rebases onto the other. This
  ticket adds a flag, 152 moves the command, so they conflict only in `main.go`.
- `agent-collector` today opens **no network listener** and its systemd unit
  (`systemd/harnez-agent-collector.service`) runs under `ProtectSystem=strict` /
  `ProtectHome=read-only` with `ReadWritePaths=%S/harnez`. Two consequences: (a) the no-listener
  default is a real, currently-true invariant worth a test, and (b) SSH `ControlMaster` needs
  `~/.ssh` **and a writable control-socket path** — `ProtectHome=read-only` will break it. This is
  the sharpest hidden risk in the ticket and is not mentioned in its Acceptance Criteria.

### Steps — Part A: relocate the remote-Load ControlMaster

1. **Add a snapshot path for load data** (`internal/usage/statecache.go`): `WriteLoadSnapshot` /
   `ReadLoadSnapshot` writing `<StateDir>/load/<host>.json`, using the same atomic
   write-temp-then-rename and the same freshness stamping as `WriteAgentSnapshot`. Reuse
   `LoadSnapshot` as the payload; do not invent a second envelope shape.
2. **Make `runRemoteLoadManager` headless-capable** (`watch.go` → move to `internal/usage/remote.go`
   or a new `remoteload.go`): keep the signature's callback shape but export a wrapper
   `RunRemoteLoadCollector(ctx, host, sink func(*LoadSnapshot))`. The existing `--watch` call site
   passes its mutex/redraw sink; the collector passes a sink that calls `WriteLoadSnapshot`.
   No behaviour change to the retry/fallback logic — it is already correct (issue 110's
   stream-then-batch-fallback), so do not rewrite it.
3. **Wire into `RunCollector`** (`collector.go:25`): add a `remoteLoadHost string` parameter (or a
   small `CollectorOptions` struct — preferable, since step B adds a second option and the function
   already takes 6 args). When set, launch `RunRemoteLoadCollector` as a goroutine tied to the
   collector's `ctx` and tear it down on return. Add `--remote-load-host` to `collectorCmd`
   (`main.go:389-421`) and to the systemd unit's `ExecStart` only if the user configures one —
   default stays empty, so the collector remains network-free unless asked.
4. **Switch `--watch` to read, not connect** (`watch.go:2225-2256`): if a fresh
   `ReadLoadSnapshot(host)` exists, consume it and do **not** start the manager; fall back to
   starting its own manager when no collector-published data is present (or it is stale). This
   fallback is what makes the change safe for anyone not running the collector daemon, and it
   preserves the AC-3 verification (no fresh handshake on `--watch` restart) only when the daemon
   is actually running — state that explicitly in the ticket's AC rather than implying it is
   unconditional.
5. **Fix the systemd sandbox** (`systemd/harnez-agent-collector.service`): `ProtectHome=read-only`
   blocks the ControlMaster socket. Either relax to `ProtectHome=tmpfs` + explicit
   `BindReadOnlyPaths=%h/.ssh` plus a writable `%t/harnez` for the control socket, or set
   `ControlPath` under `%t` (`$XDG_RUNTIME_DIR`) and add it to `ReadWritePaths`. Prefer the latter
   — narrower. `internal/claude/systemd_test.go` asserts unit contents; extend it.

### Steps — Part B: opt-in Prometheus exposition

6. **New file `internal/usage/metrics.go`**: `WriteMetrics(w io.Writer, summary UsageSummary, load
   *LoadSnapshot)` emitting Prometheus text exposition directly (`# HELP` / `# TYPE` / samples).
   **Do not add a `prometheus/client_golang` dependency** — the exposition format is ~30 lines of
   `fmt.Fprintf` and this repo is otherwise dependency-light; a client library would also drag in
   a registry/collector model that does not fit a snapshot-reading daemon.
   Metric set (minimum, per AC): `harnez_agent_quota_used_ratio{agent,window}`,
   `harnez_agent_tokens_total{agent}`, `harnez_agent_snapshot_age_seconds{agent}`,
   `harnez_remote_load{host,resource}` for GPU/CPU/memory. Escape label values.
7. **HTTP listener** (`collector.go`): when `--metrics-addr` is set, `http.Server` with a single
   `/metrics` handler serving from the *last collected* summary (guard with a mutex; do not
   collect on scrape — a scrape must never trigger a live quota API call). Shut down via
   `Shutdown(ctx)` on the collector's ctx. Default empty = no listener, no goroutine.
8. **Tests**:
   - `metrics_test.go`: golden-string assertion on `WriteMetrics` output for a fixture summary,
     asserting specific metric names/labels/values — not merely "output is non-empty".
   - `collector_test.go`: assert that with no `--metrics-addr`, `RunCollector` starts no listener
     (structural: the server field stays nil / no port bound), and with one set, `/metrics`
     returns 200 with a `harnez_` line.
   - Manual gate per AC: `promtool check metrics < out.txt`, and `ss -ltnp` showing no socket
     without the flag.
9. **Docs**: add a `usage collector` row (or update it, per [[152]]) in `docs/CLIDesign.md`'s
   command table noting the opt-in listener; document the metric names somewhere durable —
   recommend a short section in `docs/CLIDesign.md` rather than a new doc, since Grafana dashboards
   will encode these names and renaming later is a breaking change.

### Design decisions / tradeoffs

- **Snapshot file as the collector→viewer channel**, not a socket or an in-process API. It matches
  082's existing pattern exactly, needs no new IPC, and degrades gracefully (stale file = fall back
  to direct connect). Cost: sub-second load updates become file-write-rate updates — acceptable,
  since the stream already batches.
- **`--watch` keeps its own fallback path.** Removing it entirely would make `--watch
  --remote-load-host` useless without the daemon, a regression for anyone not running it.
- **Hand-rolled exposition over `client_golang`** — see step 6.
- **Do Part A before Part B**; they are independent and A is the one with a real user-visible
  payoff (surviving `--watch` restarts).

### Risks / open questions

- **systemd sandbox vs. SSH (step 5) is the likeliest thing to silently not work.** Unit tests will
  pass while the deployed daemon fails to open a ControlMaster. Verify on the real unit
  (`systemctl --user restart` then check for the control socket), not just locally.
- Multi-host: the current design assumes one `--remote-load-host`. Keying the snapshot by host
  (step 1) leaves room for more later without a format change; do not build multi-host now.
- Opening a listener in a daemon that previously had none is a security posture change. Default-off
  covers it, but the flag help should say "binds a plaintext, unauthenticated HTTP listener —
  bind to localhost or a trusted interface". No auth is in scope.
- `main.go` conflict with [[152]] — coordinate ordering.

### Scope estimate

**Large** overall; splittable:
- Part A (ControlMaster relocation + snapshot channel + systemd fix): **medium**.
- Part B (Prometheus exposition): **medium**.
Consider splitting into two tickets if they are not done in one sitting — the ACs already separate
cleanly along that line.
