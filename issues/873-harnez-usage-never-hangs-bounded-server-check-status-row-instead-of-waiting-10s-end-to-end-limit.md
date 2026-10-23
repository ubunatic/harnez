# 873 — harnez usage never hangs: bounded server check, status row instead of waiting, 10s end-to-end limit

**Status**: Open
**Priority**: P1 (High) — a hang blocks the user's main usage view and loops
**Severity**: Major
**Category**: Bug
**Related**: 874 (slow store read), 714 (M3 heartbeat/catch-up), 829 (usage as collector client), 837 (socket takeover)

---

/goal `harnez usage` (every mode, incl. `--minimal` and `-w`) never waits on the server: it shows stored data
or a clear error within 10 seconds whether the server is down, starting, hung or healthy, proven by tests that
run the real binary in each case; or stop and report when blocked on a user decision.

## Problem (user, 2026-10-23)

After two `make install` restarts, `harnez usage` hung. Cause: a startup deadlock from 714 M3. The client waited
for the server's `health: ok` before its first heartbeat; with no client, the server deferred its first fetch
until a heartbeat; and it reported ready only after a fetch. Each call waited the full `startup_timeout` (30s,
`spec/server.yaml`), until the 5m cycle made the server ready. Fixed in `b47b1263` (ready before the first
fetch, test `TestCollectorReadyBeforeFirstFetch`), but the client can still block on any server that is slow,
starting or hung. User: "It should never hang."

## Behaviour

1. Client checks the server once, bounded (about 1s, value in spec). Down, starting or not answering: start it
   in the background if needed, show stored data at once, and show a status row, e.g. "server starting" or
   "server not answering since 14:52". `-w` keeps retrying in the background and clears the row on recovery.
2. Hard limit on the whole command (10s, in spec): store read, server check and output. On overrun, print what
   it has plus an error, exit non-zero; never block.
3. Server side: health reports why it is not ready; nothing in startup waits on a fetch or a client.

## Verification

Tests that run the installed/built binary `harnez usage --minimal` with a 10s limit against: no server, a
server stuck in "starting", a socket that accepts but never answers, and a healthy server. Plus a pty run of
`-w` against a hung server showing the status row.

## Findings (searcher, 2026-10-23)

- `loadLocalUsage` calls `ensureLocalServer` before reading the store (`cmd/harnez/usage_client.go:41`); it
  polls `/status` up to `startup_timeout` 30s, each call `http_client_timeout` 5s (`spec/server.yaml:5`).
  `--minimal` does not skip it; `--offline` does. `usage_cmd.go:42` has a Go-coded 5s HTTP timeout (move to spec).
- `-w` loads initial data before the watch loop (`usage_cmd.go:109`); frames are built in
  `internal/usage/watch.go:704` (`buildWatchFrame`), the place for a status row via `WatchOptions`.
- `/status` returns `health: starting` (503) or `ok`, no reason (`internal/server/server.go:270`).
- No whole-command deadline (`usage_cmd.go:38`), no built-binary usage test, no hung-socket helper.
  Unit tests: `cmd/harnez/usage_client_test.go:15`.

## Milestones

- **M1 (bounded check + deadline):** spec values `probe_timeout` (~1s) and `usage_deadline` (10s) with schema;
  one bounded probe; when not ok, start the server in the background (no wait), read the store and print with a
  status line; whole-command deadline prints what it has plus an error and exits non-zero. Built-binary tests
  for the four server states (no server, stuck starting, accept-never-answer socket, healthy), each < 10s.
- **M1 delivered (`1834d3af`):** one probe bounded by `probe_timeout` 1s, start only when nobody listens,
  status to stderr, `usage_deadline` 10s; passive counters come from the same probe. Binary tests: no server
  2.0s, starting 2.1s, hung 3.1s, healthy 2.1s, plus slow-response and stale socket. Host live run on the
  real store: 6.0-8.2s per `--minimal`, also right after a server restart (no hang). The 10s budget is
  mostly the store read (874).
- **M2 (watch status row + health reason):**
  Pre-Work: (a) when the deadline hits during the store read, the user must still get the error line on
  stderr and a non-zero exit within 10s, never a silent hang: add a binary test with a store read slowed past
  the deadline (test hook or big fixture). (b) State in the report how much of the 6-8s is store read vs
  process start, measured with the installed binary (input for 874; do not fix 874 here). `-w` shows "server starting" / "server not answering since
  HH:MM" and clears it on recovery; `/status` adds a `reason` while not ready; pty test against a hung socket.
