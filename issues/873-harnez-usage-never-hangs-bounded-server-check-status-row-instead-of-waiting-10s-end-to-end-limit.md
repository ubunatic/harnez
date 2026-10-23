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
