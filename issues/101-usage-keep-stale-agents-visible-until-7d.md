# 101 — `harnez usage` hides/blanks agents too soon when not recently active (e.g. AGY)

**Status**: Open
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
