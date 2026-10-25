# 921 — Epic: harnez server as monitor for local and remote AI hosts

**Status**: Open
**Priority**: P2 (Medium)
**Severity**: Normal
**Category**: Epic
**Related**: 912, 917, 919, 920, 922, 879

---

/goal The local harnez server is the monitoring system for this host and its configured remotes (CPU, RAM, swap,
all GPUs): it keeps remote binaries current, collects remote load itself, and the UI only renders. Stop and report
when a ticket needs an owner decision. Circuit breaker: on a loop or no progress, clear the goal and write a
failure report here.

## Tasks

| # | Ticket | Pri | What | Status |
|---|---|---|---|---|
| 1 | 919 | P2 | remote host config, version check, upgrade older remotes on server start | Open |
| 2 | 920 | P2 | server collects remote load over ssh; slow polling without viewers | Open |
| 3 | 922 | P3 | Load box hotkey `L` instead of `7` | Open |
| 4 | 917 | P3 | server-driven boxes and items from spec | Open, after 920 |
| 5 | — | — | live check of 912's swap row on x600 (after 919) | Pending |

## Code fanout (searcher e921-search, haiku:low, 2026-10-25)

Web fanout skipped: internal work.

- 919 (M): host is `load.watch_host: x600` in `~/.config/harnez/local.yaml` (`internal/usage/localconfig.go:59`).
  Version is hand-set `version.go:3` `0.2.1`, printed by `harnez --version`; no ldflags, no commit. Server
  start hook: `cmd/harnez/server_cmd.go:35`. No delivery code exists.
- 920 (M): the server already polls remote load (`collector_remote_load.go:54`, one rate
  `spec/usage.yaml:48` `remote_load_poll_interval: 1m`); the UI reads the cache. `ClientCounter`
  (`server.go:22`) exists but does not drive the rate.
- 922 (S): `toggle_load` is `7` at `spec/actions.yaml:39`; `l`/`L` already bind `toggle_diagnostics` (`:89`).
  Tests: `actionsspec_test.go:110-134`, `watch_test.go:1288,1783`.

Phases: 922 independent; 919 before 920; 917 after 920; 912 x600 check after 919.

## Out of scope

- Server-to-server protocol instead of ssh (later idea).
- Downgrading a remote that runs a newer harnez.
