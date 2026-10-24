# 879 — Remote load recovers when the host comes back, also for one-shot usage

**Status**: Open
**Priority**: P2 (Medium)
**Severity**: Minor
**Category**: Bug
**Related**: 874 (one-shot usage stops waiting on SSH), 161 (collector absorbs remote load), 110 (batch/stream modes)

---

/goal When a remote host (e.g. x600) is reachable again, `harnez usage` shows its current load without the user
doing anything, in `-w` and in one-shot runs, and never waits on SSH for it; verified by a test with a fake ssh
that fails, then answers; or stop and report when it needs a user decision (e.g. moving collection into the server).

## 1. Problem & Motivation

874 makes one-shot `harnez usage` skip SSH and show the last cached remote load or "remote load unavailable"
(owner request 2026-10-23: no waiting). Then nothing refreshes the cache unless a `-w` session runs, so after
the host comes back, one-shot runs keep showing stale or no remote data. Owner: "When the host comes back, we
should actually recover and show the host data."

## 2. Technical Specification / Findings

- Check after 874 lands: does `-w` retry after a failed SSH and clear "unavailable" on success, and how often?
- Options for one-shot: the harnez server collects remote load in the background and stores it (fits 161), or
  a one-shot run starts a detached refresh for the next run. The server route matches 873's model (client
  reads stored data, server fetches).
- Retry with backoff while the host is down; no SSH call per status-line render.

## 3. Implementation & Verification Plan

Test with a fake `ssh` on PATH that fails first, then returns a load snapshot: `-w` shows the panel recover; a
later one-shot run shows the fresh value within its normal time. Live check against x600 after it is back.

## Sprint findings (2026-10-24, searcher haiku:med)

- 874 landed: one-shot reads only the cache (`cmd/harnez/usage_cmd.go:97`, `internal/usage/remote_load_cache.go:26`).
- `-w` (`internal/usage/watch_remote_load.go:39`) retries stream then batch poll every 10 s (`:13`), no backoff;
  success clears "unavailable" via `setLast`. The cache is written only by `-w`.
- No background remote-load fetch in server/collector; 161 (Backlog) proposes registering it as a collector source.
- No fake-ssh helper; batch poll runs bare `ssh` (`remote.go:74`), stream path uses seams (`loadstream_test.go:112-191`).

### M1 (this sprint): `-w` recovery with backoff
Fake `ssh` on PATH that fails then answers; panel recovers; backoff while down.

### M2 (needs owner decision): one-shot recovery
Option A: collector daemon fetches remote load in the background (promote 161). Option B: one-shot starts a
detached refresh for the next run.
