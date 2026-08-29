# 101 — `harnez usage` hides/blanks agents too soon when not recently active (e.g. AGY)

**Status**: Resolved (read-side fix + write-side follow-up, both 2026-08-29)
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

## Live-Repro Follow-Up (2026-08-29) — real write-path bug found and fixed

User report, live: "I cannot see stale AGY usage yet," despite the above fix
being merged. Investigated for real this time (no AGY process running,
`harnez usage`, `harnez usage --summary`, and `harnez agent-collector --once`
all run live against this machine's actual state) instead of re-reading the
code that already claimed success.

**What was actually true on this machine**: AGY's box *does* render every
time (`HasUsageData()`/`IsStale` gating is correct, `Updated: ... ago` line
renders correctly) — it was never being hidden. But it renders with no
quota bars (no `Session`/`Weekly`), only the static account/model/activity
fields. `~/.claude/harnez/usage-history/*.jsonl` shows AGY *did* have real
`ModelGroups` quota data as recently as 2026-08-23/24 (AGY was running and
answering the local RPC then). The on-disk collector-daemon snapshot
(`~/.local/state/harnez/agents/usage/agy.json`), however, was last written
2026-08-28 11:35 with **no** quota fields at all — that snapshot is what
every subsequent `harnez usage` reads as "the last known state," and it was
already lossy by the time this ticket's read-side fix shipped.

**Root cause**: the original fix only patched the *read* path
(`cacheOrLive` in `statecache.go`) to stop discarding a cached snapshot's
quota signal in favor of an emptier live recollect *at read time*. It did
not touch the *write* path. Both places that persist collector output —
`collectTick` (the `harnez agent-collector` ticking loop) and the
`agent-collector --once` `RunE` in `cmd/harnez/main.go` — called
`WriteAgentSnapshot` directly and unconditionally on every pass. So the very
first collector tick where AGY wasn't running (which is most ticks, since
AGY isn't left running continuously) would overwrite the on-disk snapshot
with a result that had lost `Session`/`Weekly`/`ModelGroups`/`Tokens` —
permanently discarding the last known real quota numbers from disk. From
that point on, `cacheOrLive`'s read-side "don't blank on a lossy live
recollect" guard had nothing richer left to fall back to: the cache itself
was already the lossy version. This is exactly the failure mode issue 101
set out to fix, just one hop earlier in the pipeline than the original fix
looked — a genuine gap, not a "no data yet" case (AGY *did* produce real
quota data on this machine, it was just subsequently clobbered).

### What changed

- `internal/usage/statecache.go`: added `PersistAgentSnapshot(stateDir,
  agent)` — reads the existing on-disk snapshot first, and skips the write
  entirely (leaving the richer snapshot and its `FetchedAt` untouched) when
  the existing snapshot has quota signal (`hasQuotaSignal()`) that the new
  live result lacks. A genuinely richer or equal live result still
  overwrites normally.
- `internal/usage/collector.go`: `collectTick` now calls
  `PersistAgentSnapshot` instead of `WriteAgentSnapshot` directly.
- `cmd/harnez/main.go`: the `agent-collector --once` `RunE` now calls
  `usage.PersistAgentSnapshot` instead of `usage.WriteAgentSnapshot`
  directly.

### Tests

- `internal/usage/statecache_test.go`:
  `TestPersistAgentSnapshotKeepsRicherCacheOnLossyOverwrite` — seeds a rich
  cached snapshot (with `Session`), persists a lossy live result (no quota
  fields) and confirms the on-disk snapshot and its `FetchedAt` are
  unchanged; then persists a genuinely richer live result and confirms it
  does overwrite.

Verified live on this machine: `go build ./...`, `go vet ./...`,
`go test ./...` (full suite green), `make install`, then re-ran `harnez
usage`, `harnez usage --summary`, and `harnez agent-collector --once` for
real. AGY's box continues to render correctly (it was never hidden).

**Known limitation, disclosed to the user**: this fix prevents *future*
quota data from being clobbered — it cannot resurrect the specific
2026-08-23/24 AGY quota numbers already lost from `agy.json` before this
fix landed (that data survives only in `usage-history/*.jsonl`, which
`harnez usage`'s live box does not currently read from). AGY's quota bars
will reappear in `harnez usage` once AGY is running again during a
collector tick or a direct `harnez usage` invocation; until then, showing
account/model/activity with no quota bars is correct, not a bug.

**Status**: Reopened → Resolved again with this write-path fix. The
original read-side fix (2026-08-29 first Progress/Resolution section above)
was real and correct but incomplete; this follow-up closes the gap.
