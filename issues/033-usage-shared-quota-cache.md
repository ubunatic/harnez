# 033 — Shared disk-backed quota cache to stop concurrent `harnez` instances from double-polling

**Status**: Closed — implemented 2026-08-18
**Category**: Feature / robustness
**Discovered**: 2026-08-18, after a live `HTTP 429` from `api.anthropic.com/api/oauth/usage` while
two `harnez usage --watch` processes were running concurrently (one at `--interval 30s`, one at the
default 60s), each independently polling the same account's quota endpoint.

---

## Problem

Every `harnez usage`/`--watch`/`--summary` invocation calls `CollectClaude`
(`internal/usage/claude.go`), which fetches live quota from
`api.anthropic.com/api/oauth/usage` with no cross-process awareness. Running
two `harnez` processes at once (e.g. one leftover `go run` from testing plus
a real `--watch` session — this has already happened once, see chat history
2026-08-18) means both poll independently forever, multiplying the request
rate against the same account and eventually drawing a `429`.

Separately, per [issue 031](031-usage-quota-fetch-errors-silent.md) and
[issue 032](032-usage-watch-no-stale-fallback-on-fetch-failure.md), a `429`
or other fetch failure currently has only an **in-memory** stale fallback,
scoped to a single `--watch` process's own `lastSummary`. A one-shot
`harnez usage` or `--summary` call has no prior frame at all, so on failure
it just shows "quota: unavailable" with nothing to fall back to — even
though a sibling `harnez` process, or this same process a minute ago, may
have a perfectly good recent reading sitting nowhere durable.

## Proposed design

Add a small shared cache file, written next to Claude's own local cache
files rather than a new directory: `~/.claude/harnez-quota-cache.json`,
alongside `stats-cache.json` and `.credentials.json`. Contents: `fetched_at`,
the `Session` and `Weekly` `QuotaWindow` values (whatever `CollectClaude`
currently populates on a live hit).

**Read path (every process, every collect):**

1. Before making the live HTTP call, stat/read the cache file. If its
   `fetched_at` is within a TTL (reuse `MinWatchInterval`, 30s), use the
   cached values directly and skip the network call entirely. This is what
   actually stops the continuous double-polling that caused the 429 — after
   the first process refreshes, every other process's next tick sees a warm
   cache and skips the request.
2. If the cache is stale or missing, proceed to the write path below.
3. On a live fetch failure (per issue 031's `QuotaFetchError`), fall back to
   the cache file's values regardless of its age, labeled stale (see issue
   032's `" (stale)"` convention) — this replaces/subsumes issue 032's
   in-memory-only fallback, since the disk version also covers one-shot
   `--summary`/`usage` calls and survives process restarts. Confirm with the
   user whether to keep the in-memory fallback as a fast path or drop it in
   favor of always reading disk.

**Write path (only entered when the cache was stale/missing, i.e. rarely):**

Use `flock()` (advisory, exclusive) on the cache file (or a sidecar lock)
held only around this critical section — do the live fetch and the write
under the lock, release immediately after. This was discussed at length and
is the recommended mechanism over a hand-rolled timestamp-based
optimistic-check protocol:

- `flock` releases automatically when the holding process's file descriptor
  closes, which the kernel guarantees on any process exit — normal exit,
  crash, `panic`, SIGKILL, OOM-kill. No stale-lock detection or PID-file
  bookkeeping needed.
- It does not survive a hang (a deadlocked-but-alive holder keeps it), so
  a waiter must not block indefinitely — use a bounded wait (e.g.
  `flock` with `LOCK_NB` and a short retry budget, or just skip this tick's
  live fetch and try again next interval if the lock isn't free quickly).
- It does not persist across a reboot, which is fine — no process survives
  a reboot to need reconciling either.
- It's advisory only (cooperating `harnez` processes respect it; nothing
  stops an unrelated process from ignoring it), which is an acceptable
  scope — this is inter-`harnez` coordination, not a security boundary.

Write itself should still go through the standard atomic pattern (write to
a `.tmp` file, `rename()` into place) so a reader can never observe a
torn/partial write — that part doesn't need the lock's help, `rename` is
already atomic on the same filesystem, but doing both together is simplest
and avoids two consumers writing partial state in the (rare, bounded-by-lock)
window.

## Explicitly rejected approach

A three-step optimistic-check protocol (check freshness before fetching,
re-check-before-write to avoid clobbering a fresher write, read-back after
writing to detect being clobbered) was discussed and designed in detail
before landing on `flock`. It was rejected as the implementation approach
because it approximates optimistic concurrency control without a true
atomic compare-and-swap primitive (POSIX files don't offer one), leaves a
TOCTOU gap between check and write that only a read-back partially detects,
and ends up being *more* code than `flock` for a weaker guarantee. Keep
this context so the implementer doesn't reintroduce it — `flock` was chosen
specifically because it fully closes the race with less code and has no
stale-lock cleanup burden (see above).

## Scope

Claude only for now (`internal/usage/claude.go`) — this is the agent with
the live, rate-limited endpoint that actually produced a 429. AGY/Codex use
different local RPC/HTTP shapes and can get the same treatment later if
they turn out to need it; don't generalize preemptively.

## Implementation notes (2026-08-18)

Implemented in `internal/usage/claude.go`: `quotaCache` (json: `fetched_at`,
`session`, `weekly`), `quotaCachePath`, `readQuotaCache`, `writeQuotaCache`
(tmp-file + rename), and `lockQuotaCache`/`unlockQuotaCache` (advisory
`syscall.Flock` on a `<cache>.lock` sidecar, `LOCK_EX|LOCK_NB` with 5 retries
at 50ms apart — ~250ms bounded wait, never blocks indefinitely). No new
dependency was needed: `golang.org/x/sys` isn't in `go.mod`, and
`internal/usage/watch.go` already calls `syscall` directly (`syscall.Flock`,
`syscall.SIGTERM`, etc.), so `syscall.Flock` was used to match existing style
rather than adding `x/sys/unix`.

Read path: before the live HTTP call, `CollectClaude` reads the cache; if
`fetched_at` is within `MinWatchInterval` (30s) it uses the cached
Session/Weekly directly and returns without touching the network. Otherwise
it takes the lock (best-effort, bounded retry), does the live fetch, and —
only if the lock was actually acquired — writes the fresh reading to disk
under the lock before releasing it. If the lock isn't free in time, the live
fetch still happens and its result is still used for this call; only the
disk write is skipped (a sibling process is presumably writing its own fresh
reading concurrently).

On a live fetch failure (`QuotaFetchError` set), `CollectClaude` now falls
back to the disk cache regardless of its age, reusing `staleQuotaWindow` from
`watch.go` to apply the existing `" (stale)"` label convention (issue 032).

**Resolving the issue-032 overlap**: kept `applyStaleQuota` /
`staleLabel` in `watch.go` as-is rather than removing it. Decision: the disk
cache in `CollectClaude` now runs first and, on fetch failure, already
populates `Session`/`Weekly` with stale-labeled values for Claude — so
`applyStaleQuota`'s per-agent check (`agent.QuotaFetchError != "" &&
agent.Session == nil`) is a no-op for Claude in the common case (the field is
no longer nil by the time `RunWatch` calls it). It remains meaningful as a
backstop for Claude when no disk cache exists yet (fresh install, first
run before any cache has been written), and it's still the only fallback
mechanism for AGY/Codex, which this issue deliberately doesn't touch. No
watch.go changes were made; this is a runtime precedence outcome of the new
claude.go code path, not a code change to the two mechanisms' relationship.

Tests added in `internal/usage/quota_cache_test.go`: read/write round-trip,
missing-cache read, bounded-retry lock contention (`TestLockQuotaCacheBoundedRetry`,
verifies wait stays bounded, ~0.25s observed), lock-then-reacquire happy
path, `CollectClaude` skipping the live call on a warm cache
(`TestCollectClaudeUsesWarmDiskCache`), and `CollectClaude` falling back to a
stale disk cache with `" (stale)"`-labeled windows on an HTTP 429
(`TestCollectClaudeFallsBackToStaleCacheOnFetchFailure`, using a
`RoundTripper` that redirects the hardcoded `api.anthropic.com` host to the
test's `httptest.Server`). Concurrent-process flock contention itself was
exercised by holding a second `flock` on the same lock file from within the
test process rather than spawning real second `harnez` processes.

`go build ./...`, `go vet ./...`, and `go test ./...` all pass. `make
install` run.

## Related

- [Issue 031: live quota fetch failures are silent](031-usage-quota-fetch-errors-silent.md)
- [Issue 032: `--watch` has no stale fallback on fetch failure](032-usage-watch-no-stale-fallback-on-fetch-failure.md) — this issue's disk cache likely subsumes/replaces the in-memory fallback added there; resolve that overlap during implementation
- [Issue 023: `harnez usage`](023-usage-command-token-quota-tracking.md) — original command
