# 920 — usage: server collects remote load over ssh, slow polling without viewers

**Status**: Closed — f993083b (M3 shared ssh options, transition logging) + 929 f0031376: live x600 load refreshes every ~10s with a UI attached ('updated 0m ago'); 5m detached cadence covered by unit test only; make test-q1 green
**Priority**: P2 (Medium)
**Severity**: Minor
**Category**: Feature
**Related**: Epic 921, 919, 879, 912

---

/goal The local harnez server, not the UI, holds the ssh connection to each configured remote and collects its
load (CPU, RAM, swap, all GPUs); it polls slowly while no UI is attached and switches to the normal streaming
rate when a client connects. Verified with a test for both rates and a live run against x600; or stop and report
on an owner decision. Circuit breaker: on a loop or no progress, clear the goal and write a failure report here.

## Notes (owner, 2026-10-25)

- Transport stays ssh streaming; a server-to-server protocol is a later idea, out of scope.
- Check what the UI does today (watch-side stream, `remote_load_poll_interval`, `remote_load_staleness`) and
  move it behind the collector registry (docs/UsageCollection.md §registry).
- Slow and fast rates are configurable values: they go in `spec/` with schema (docs/Spec.md).

## Decision (owner, 2026-10-25)

- The server already polls (`collector_remote_load.go:54`, one rate `spec/usage.yaml:48` 1m); `ClientCounter`
  (`server.go:22`) exists. Split into two spec keys: 10s with a UI attached, 5m with none; switch on client count.
- Owner: "more spec is always good"; allow a user override from `~/.config/harnez` (local.yaml) for these
  values, if the spec loader supports overrides; otherwise park the override in its own ticket.

## M3 Pre-Work: reopened, server never polls x600 (owner, 2026-10-25)

Owner: after a minute the watch still shows `updated 24m ago · stale`. Host findings: the remote-load cache file was
last written 13:04 (by the old watch-side poller); since the 13:17 restart the server log has no remote-load line.
`internal/usage/remote.go:74` runs `ssh -q -o BatchMode=yes ...` without `-F ~/.ssh/config`, so it fails in the unit
sandbox exactly like 919 did, and `-q` plus no log hides the failure.

1. One shared ssh/scp argument builder for remote upgrade and remote load (`-F <home>/.ssh/config` when present,
   BatchMode, ConnectTimeout); no second copy.
2. The server logs a remote-load failure with trimmed ssh output once per state change (ok→failing, failing→ok),
   not every poll.
3. Test: the poller's command includes `-F` when the config exists; a failure is logged once.
4. Host verifies: restart the unit, the cache file updates within 10s while a watch is open, the box shows
   `updated <10s ago`, and the log shows no failure; with no UI the cadence drops to 5m.
