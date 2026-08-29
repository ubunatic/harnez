# 101 — `harnez usage` hides/blanks agents too soon when not recently active (e.g. AGY)

**Status**: Resolved
**Priority**: P2 (Medium)
**Severity**: Moderate
**Category**: UX / Agentic Ergonomics
**Related**: `internal/usage/statecache.go`, `internal/usage/usage.go`, `internal/usage/agy.go`, `internal/usage/watch.go`, `internal/usage/types.go` (`HasUsageData`), [[023-usage-command-token-quota-tracking]], [[082-agent-usage-collector-daemon]], [[083-usage-tui-self-hiding-auto-discovery]]

## Problem

An agent that hasn't been started/queried in a few hours (observed with
AGY) stops showing useful usage data, well before it should be considered
gone. Reported directly: "I like to see when it refreshes, even when we
have not queried the state files/API in a while."

Likely contributing factors (need confirming, not yet root-caused):

- `DefaultCacheStaleness` (`statecache.go:33`) is `2 * DefaultCollectorInterval`
  = 30 minutes. Once a cached snapshot is older than that, `cacheOrLive`
  (`usage.go:53-55`) falls back to a live recollect rather than showing the
  last known snapshot.
- For AGY specifically, live quota data (`Tokens`/`Session`/`Weekly`) only
  populates when a running `agy` process is found and answers on a local
  port (`agy.go:179`, step 5, `findAGYPorts`/`QueryAGYLocalQuota`). If AGY
  isn't currently running, that live recollect can't refresh those fields —
  static fields (`Authenticated`, `Account`, `Sources`, conversation count)
  still populate from on-disk state/logs regardless of whether the process
  is running, so the agent box itself likely doesn't fully vanish (issue
  083's `HasUsageData()` would still see those), but the quota/usage
  numbers go stale or blank with no indication of *when* they were last
  real.
- There's currently no displayed "last refreshed" timestamp per agent, and
  no staleness-based hide threshold at all beyond the 083 all-fields-empty
  check — so today's behavior is either "recent-enough to show live
  numbers" or "fields quietly stop updating," not the desired three-state
  behavior below.

## Desired Behavior

1. Once an agent has ever produced real usage data, keep showing its most
   recently known state indefinitely (not gated to a few-hours/30-minute
   window) — a live process outage shouldn't blank out numbers that were
   real minutes or hours ago.
2. Show when that data was last refreshed (e.g. "last updated 3h ago") so
   staleness is visible rather than silently implied.
3. Only auto-hide an agent (return to issue 083's self-hiding behavior)
   once its last known data is 7+ days stale — a genuinely abandoned/
   uninstalled agent, not one that's merely not running right now.

## Notes

- This changes `DefaultCacheStaleness`'s role: it should keep governing
  when a *live* recollect is attempted (fine at 30 minutes, or whatever
  interval), but must stop being conflated with "when to stop trusting/
  showing the last snapshot" — those need to become two separate
  thresholds, with the 7-day one being the actual display-hide gate.
- Needs a "last refreshed" timestamp to be tracked and persisted per agent
  snapshot (likely already implicit in the collector-daemon's cache file
  mtime — check `statecache.go` before adding a new field) and threaded
  through to both `RenderText` (`usage.go`) and `buildWatchFrame`
  (`watch.go`), matching where issue 083's `HasUsageData()` filtering
  already lives.
- Confirm the actual current behavior with a live repro (stop AGY, wait
  past 30 minutes, observe exactly what `harnez usage` and `harnez usage
  --watch` show) before implementing — the mechanism above is inferred
  from code reading, not yet verified against a live run.

## Progress / Resolution (2026-08-29)

Root cause confirmed by code tracing only — **not** by a real 30+ minute
live-AGY-outage repro (judged impractical per the ticket's own note).
Before this fix, `cacheOrLive` (`internal/usage/statecache.go`) had exactly
one behavior once a cached snapshot passed `DefaultCacheStaleness` (30 min):
discard it entirely and return a fresh live collect. For AGY specifically,
that live collect repopulates static fields (`Authenticated`, `Sources`,
conversation counts) from on-disk state regardless of whether the `agy`
process is running, but `Session`/`Weekly` only populate when a running
process answers on a local port (`agy.go`) — so a live recollect after 30
idle minutes silently drops the quota bars the box was showing a moment
before, with no "hidden reason" or timestamp visible anywhere. This was
verified by reading `cacheOrLive`'s control flow and `CollectAGY`'s field
population precisely, then simulating the exact failure mode in
`TestCacheOrLivePreservesQuotaOnLossyRecollect` (aging a cache file
directly instead of sleeping, and feeding `cacheOrLive` a `collect` stub
that mimics "process not running": static fields set, quota fields nil).

### What changed

- `internal/usage/statecache.go`:
  - `DefaultCacheStaleness` (30 min) now documented and used *only* to
    decide when a live recollect is attempted — no longer conflated with
    display/hide behavior.
  - Added `DefaultDisplayStaleness = 7 * 24 * time.Hour`, the new auto-hide
    gate.
  - `cacheOrLive` now stamps `AgentUsage.LastRefreshed` (cached snapshot's
    `FetchedAt`, or `time.Now()` for a live result) and, when a live
    recollect comes back with strictly less quota/token signal than the
    cache already had (via the new `AgentUsage.hasQuotaSignal()` check),
    keeps serving the last known cached snapshot instead of the emptier
    live result — fixing the AGY-not-running blanking case directly.
- `internal/usage/types.go`: added `AgentUsage.LastRefreshed time.Time`
  (JSON `last_refreshed`), `hasQuotaSignal()`, and `IsStale(maxAge)` (a
  zero `LastRefreshed` is never stale — a safe default for callers/tests
  that build `AgentUsage` directly without going through
  `CollectAll`/`cacheOrLive`).
- `internal/usage/usage.go`: `collectAll` backstops `LastRefreshed` to
  `time.Now()` for the live-only code paths that don't go through
  `cacheOrLive`. `RenderText` now hides an agent when
  `agent.IsStale(DefaultDisplayStaleness)` is true (additive to issue 083's
  `HasUsageData()` check, not a replacement), and appends an
  `Updated:      <FormatAgo>` line to each shown box.
- `internal/usage/watch.go`: `buildWatchFrame`'s `discovered` filter adds
  the same `IsStale` gate; `buildAgentBox` appends a dimmed
  `updated <FormatAgo> ago` line to each panel.
- `internal/usage/util.go`: added `FormatAgo(t time.Time) string`
  ("3h ago", "12m ago", "just now" for <1min/zero).

### Tests

- `internal/usage/statecache_test.go`:
  `TestCacheOrLivePreservesQuotaOnLossyRecollect` (the core issue-101
  repro simulation — lossy live recollect keeps the cached quota data and
  `LastRefreshed`; a genuinely richer live recollect still wins),
  `TestAgentUsageIsStale` (table-driven 7-day threshold, including the
  zero-`LastRefreshed` backstop).
- `internal/usage/usage_test.go`:
  `TestRenderText_StaleWithinSevenDaysStillShown` (3h-stale agent still
  renders with its last-known Session data and an `Updated:` line),
  `TestRenderText_SevenDayStaleAgentHidden` (8-day-stale agent hidden,
  falls through to the "No supported agent" message).
- `internal/usage/watch_test.go`:
  `TestBuildWatchFrame_StaleWithinSevenDaysStillShown`,
  `TestBuildWatchFrame_SevenDayStaleAgentHidden` (same two cases for the
  `--watch` TUI path).

Verified with `go build ./...`, `go vet ./...`, `make check`
(`go vet ./... && go test ./...`), and `make install` — full suite green,
no regressions in the existing issue-083/086-adjacent tests.

Not done: a real live-AGY-process-outage repro (stop the daemon, wait 30+
minutes, observe `harnez usage --watch` against a real machine) was not
performed — matching this ticket's own stated epistemic caution about that
being impractical to run as part of this change. The fix is exercised only
via the code-level simulation described above.
