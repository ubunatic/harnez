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
- `-w` retries stream then batch poll; successful snapshots clear "unavailable" via `setLast`. The cache is written only by `-w`.
- No background remote-load fetch in server/collector; 161 (Backlog) proposes registering it as a collector source.
- Batch polling runs bare `ssh` (`remote.go:74`); tests can isolate it by putting a fake `ssh` first on PATH.

### M1 delivered: `-w` recovery with backoff
`internal/usage/watch_remote_load.go` retries after 10 s, doubles the delay after failed polls, caps it at 2 min,
and resets to 10 s after a successful stream or batch snapshot. These are internal retry-policy constants, not
user-configurable settings, so no `spec/` keys are needed. `TestRunRemoteLoadManager_FakeSSHRecoversWithBackoff`
uses an isolated PATH/HOME/XDG setup and a fake `ssh` that fails before returning a snapshot; it verifies the
backoff schedule, cap, recovery, reset, and one SSH poll per retry. The existing stream fallback test also avoids
real host SSH. Verified with `go test ./internal/claude/ ./internal/usage/` and `make test-q1`.

### M2: one-shot recovery via collector background poll

Decision (owner, 2026-10-24): option A, the collector polls remote load; 161's ControlMaster move and
`/metrics` stay out of scope. Architecture (sol:med architect, Sprint 3 of epic 894):

- Host: existing `$XDG_CONFIG_HOME/harnez/local.yaml` `load.watch_host` (`internal/usage/localconfig.go`), read
  at collector start; empty host or offline disables polling; config change needs a restart.
- One goroutine started in `internal/usage/collector.go`, independent of quota ticks, readiness and client
  count; collector-owned child context, cancelled and joined on every return. SSH must never delay quota work.
- New `internal/usage/collector_remote_load.go` reuses `pollRemoteLoad` / `CollectRemoteLoadSnapshot` and M1
  backoff; do not reuse `runRemoteLoadManagerWithPolicy` (it opens streams and ControlMaster sessions).
- Cache: collector and `-w` both write via temp file + rename; last successful write wins; a failure keeps
  the old data and timestamp. Cache-write errors are nonfatal and logged.
- Recovery needs the running collector service (`harnez-agent-collector.service`); a one-shot `usage` run does not poll.

Corrections from live code (2026-10-24), replacing the earlier plan text:
- The atomic cache write (temp file + rename) already exists in `remote_load_cache.go`; M2 reuses it.
- `freshness.go` has no remote-load code; M2b adds the remote staleness check there.
- The collector had no cancel/join for background work; M2a adds it (`runCollectorWithDeps` joins the poll on every return).
- Offline ambiguity resolved: `--offline` does not disable the poll; an unreachable host backs off.
- Backoff duplication resolved: the collector and `-w` share `nextRemoteLoadRetryDelay`; Go constants 10 s / 2 min stay in code (no spec keys).

**M2a (config and poller):** `collector.go`, `watch_remote_load.go`, `spec.go`, new poller; positive
`remote_load_poll_interval` and `remote_load_staleness` in `spec/usage.yaml` + schema. Tests in new
`collector_remote_load_test.go`, plus `collector_test.go`, `localconfig_test.go`, `spec_test.go`: startup
poll, absent/invalid config, offline, no clients, own cadence, cancel/join, invalid spec values.

**M2a delivered (2026-10-24):** the collector polls `load.watch_host` at startup and every 1 m (`remote_load_poll_interval`), with retry backoff 10 s to 2 min; staleness threshold 5 m (`remote_load_staleness`). `make test-q1` log `/tmp/harnez-test-q1-879-M2a.log`: 0 `--- FAIL` lines, exit 0. `TestOneShotUsageDoesNotInvokeSSH` now checks that the one-shot process never ran ssh (the stub records its parent PID); the detached collector it auto-starts is stopped and waited for in cleanup, with the fake host from the test's own `local.yaml` and isolated HOME/XDG dirs.

**M2b (freshness):** expose cache `updated_at` beside the snapshot (keep the reader API), thread it through
`cmd/harnez/usage_cmd.go` and `internal/usage/{watch_frame.go,watch_load_box.go,watch.go}`; `freshness.go`
takes the remote staleness threshold without changing quota freshness. Tests: `remote_load_cache_test.go`,
`freshness_test.go`, `watch_test.go`, `cmd/harnez/usage_binary_test.go` (concurrent writers, malformed or
legacy cache, unknown/future age, stale retention, fake ssh failing then recovering with only the collector
running; later one-shot reads start no ssh).
