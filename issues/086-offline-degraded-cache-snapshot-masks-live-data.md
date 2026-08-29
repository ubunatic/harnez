# 086 — Offline/Degraded Collector Snapshot Is Cached and Served as Fresh, Masking Richer Live Data

**Status**: Resolved — 2026-08-29
**Priority**: P2 (Medium)
**Severity**: Moderate
**Category**: Bug
**Related**: [[082-agent-usage-collector-daemon]], [[085-watch-tui-show-collector-daemon-status]]

## Problem

Found while investigating a user report of "no numbers" in `harnez usage --watch`. Root cause:
during development of [[082-agent-usage-collector-daemon]], `harnez agent-collector --once
--offline` was run as a smoke test, writing a snapshot to
`~/.local/state/harnez/agents/usage/claude.json` that has token totals but **no** quota/session
window data (since `--offline` skips the live Anthropic usage API call).

That snapshot's `fetched_at` timestamp was recent enough to be considered fresh by
`CollectAll`'s cache-first staleness check (< 30 min old), so `harnez usage --watch` served it
as-is — showing token totals but silently missing the Weekly/Session quota bars that a live
collect (or an online daemon run) would have produced. The user perceived this as the tool being
broken ("no numbers"), when the underlying live-collection path actually works fine — confirmed
by manually clearing the cache directory, which restored full quota-bar display via live fallback.

## Desired Behavior

A cache snapshot written from a degraded/partial collect (e.g. `--offline`) should not be
indistinguishable from a full one. At minimum, one of:

- Don't write a cache snapshot at all when the collect was offline/partial (so `CollectAll` falls
  through to live collection or an older, better snapshot).
- Or persist enough metadata in the snapshot (e.g. an `offline: true` / `partial: true` flag) so
  `CollectAll`'s cache-first check can treat it as "usable but not authoritative" and prefer a
  live collect over it, or at least not silently omit fields the user would expect.

## Next Steps

- Decide the exact metadata/skip approach above.
- Add a regression test: an offline-collected snapshot in the cache dir should not suppress
  quota-window data that a live collect would have provided.

## Progress (2026-08-29) — distinct from 7c50f12/issue 101, both fixed here

Investigated in the same session as commit `7c50f12` ("fix(usage): stop collector write path
from clobbering cached quota data", issue 101's write-side follow-up). Read that diff first to
check for overlap.

**Finding: 7c50f12 does not cover this ticket.** 7c50f12's `PersistAgentSnapshot` only skips a
write when the *existing on-disk snapshot* already has quota signal (`hasQuotaSignal()` —
Session/Weekly/ModelGroups/Tokens) that the *new* result lacks. It says nothing about:

1. A snapshot written when there is **no existing richer snapshot to protect** — exactly this
   ticket's repro (`agent-collector --once --offline` as a first-time smoke test). Nothing
   blocked that write, and its `Tokens` field alone was enough to satisfy `hasQuotaSignal()`
   even with no `Session`/`Weekly`/`ModelGroups` — so the write-side guard wouldn't have caught
   it even retroactively.
2. `cacheOrLive`'s fast path: a *fresh* cached snapshot is returned as-is without even attempting
   a live recollect. There's no comparison against what a live collect would show, offline or
   not — trusting freshness alone is the whole point of the cache. So even a snapshot that did
   get blocked from clobbering something richer could still, on its own first successful write,
   mask a live collect for up to `DefaultCacheStaleness` (30 min).

### What changed

- `internal/usage/types.go`: added `hasQuotaWindowSignal()` — deliberately narrower than
  `hasQuotaSignal()`, counting only Session/Weekly/ModelGroups and *not* Tokens. This is the
  predicate this ticket actually needed: the reported repro snapshot had real token totals
  (`hasQuotaSignal() == true`) but no quota windows, which is precisely what was silently masked.
- `internal/usage/statecache.go`: `PersistAgentSnapshot` gained an `offline bool` parameter. When
  `offline` is true and the freshly-collected agent has no quota-window data
  (`!hasQuotaWindowSignal()`), the write is skipped entirely — the snapshot never reaches the
  cache at all (Desired Behavior bullet 1), so `CollectAll`/`cacheOrLive` falls through to a live
  collect or an older, richer snapshot exactly as if the daemon had never ticked. `offline` must
  be true only when the caller explicitly ran `--offline` (skipped the live API on purpose), not
  merely whenever a live recollect happens to come back empty (that's issue 101's separate,
  legitimate "AGY isn't running right now" case, which must still get cached normally).
- `internal/usage/collector.go`: `RunCollector`/`collectTick` gained an `offline bool` parameter,
  threaded through to `PersistAgentSnapshot`.
- `cmd/harnez/main.go`: both `agent-collector --once` and the ticking-daemon `RunE` now pass the
  existing `collectorOffline` flag value through to `usage.RunCollector`/
  `usage.PersistAgentSnapshot`, so the CLI's own `--offline` flag is what actually triggers the
  guard.
- `cacheOrLive` itself is intentionally **unchanged**: making its fresh-cache fast path
  re-compare against a live collect on every read was tried and reverted — it broke
  `TestCollectAllCacheFirst`'s "fresh cache: used instead of live collection" contract (a fresh
  cache with only static fields, e.g. `Installed: true` with no quota data, is legitimately
  supposed to win over a live collect that would report `Installed: false` because the collector's
  home dir doesn't exist in that test). Solving this at the write side (never let a genuinely
  degraded snapshot become "the fresh cache" in the first place) is the correct fix and doesn't
  disturb that existing, correct caching contract.

### Tests

- `internal/usage/statecache_test.go`:
  `TestPersistAgentSnapshotSkipsOfflineWriteWithoutQuotaWindows` — reproduces the ticket's exact
  scenario (offline collect with `Tokens` but no quota windows) and confirms the write is skipped
  entirely; confirms an offline collect that *does* have quota windows still persists normally;
  confirms a non-offline collect without quota windows (the legitimate "AGY isn't running"
  case) still persists as before.
- `internal/usage/collector_test.go`: existing tests updated for the new `offline` parameter
  (passed `false`, preserving their prior behavior/expectations unchanged).

Verified: `go build ./...`, `go vet ./...`, `make check` (full `go test ./...`, all green),
`make install`.

## Progress (2026-08-29, later same day) — separate live-repro gap found and fixed: usage-history fallback

After the fix above landed, the user re-confirmed live: `harnez usage` still showed AGY's box
with no Session/Weekly quota bars. Re-investigated live rather than trusting the fix above was
sufficient:

- `harnez usage` output showed AGY's box with Account/Active Model/Activity/Updated but no quota
  bars, unlike Claude and Codex.
- `~/.local/state/harnez/agents/usage/agy.json` (the persisted collector snapshot) had a fresh
  `fetched_at` but **no** `model_groups` field — this snapshot had been empty for its quota
  fields since before any of this session's fixes existed, so there was nothing richer for
  `7c50f12`'s or this ticket's earlier fix to protect; both fixes only prevent *future*
  degradation, they can't resurrect data that was already lost.
- `~/.claude/harnez/usage-history/*.jsonl` **did** have real AGY `model_groups` quota data, most
  recently from 2026-08-23 (6+ days old at the time of the repro) — a separate store
  (`AppendHistory`/`ReadHistory`, `internal/usage/history.go`) that nothing in the read path ever
  consulted as a fallback.

**Root cause**: `CollectAll`'s cache-or-live read path and the usage-history log are two entirely
separate stores. When the current cache/live reading for an agent has no quota-window data at
all, there was no mechanism to check whether the history log had something better and more
recent than "nothing" to show instead — even though that history log is written by the very same
tool (`harnez usage history record`, and every `--watch`/`--summary` tick, per existing code) and
routinely has richer data than the live/cached snapshot for an intermittently-running agent.

### What changed

- `internal/usage/history.go`: added `latestHistoryQuotaWindow(dir, agentID)` — walks
  `ReadHistory`'s entries backwards (most recent first) for the most recent entry for `agentID`
  that has `hasQuotaWindowSignal()`. Added `fillFromHistoryIfNoQuotaWindows(historyDir, u)` — a
  pure read-side fallback: if `u` already has quota-window data, or no history entry qualifies,
  or the qualifying entry is older than `DefaultDisplayStaleness` (7 days, issue 101's auto-hide
  gate — no point resurrecting data the renderer would hide anyway), `u` is returned unchanged;
  otherwise `Session`/`Weekly`/`ModelGroups` are filled in from that historical entry and
  `LastRefreshed` is stamped to *that entry's own timestamp* (not "now"), so the "last updated"
  display and the 7-day auto-hide gate both stay honest about how old the quota data actually is.
- `internal/usage/usage.go`: `collectAll`'s `useCache` branch now calls
  `fillFromHistoryIfNoQuotaWindows` for each agent after `cacheOrLive`. Deliberately scoped to the
  cache-reading path only (`CollectAll`, what `harnez usage`/`--watch`/`--summary` use) — not
  `CollectAllLive`/`collectTick` (what the daemon persists to disk) — so this is a pure display
  fallback and never implicitly writes historical data back into the collector-daemon cache.

### Tests

- `internal/usage/history_test.go`:
  `TestFillFromHistoryIfNoQuotaWindowsUsesStaleHistoricalQuota` — seeds a 6-day-old history entry
  with `ModelGroups` quota data, confirms a current empty reading gets filled in with that data
  and the historical timestamp; confirms an already-rich current reading is left untouched;
  confirms history older than the 7-day display window is not used.

### Live verification

`go build ./...`, `go vet ./...`, `make check`, `make install`, then real `harnez usage` and
`harnez usage --summary` runs on this machine: AGY's box now shows both quota-window groups
(Gemini Models, Claude and GPT Models) with their bars, tagged `Updated: 6d4h ago` and a
`~/.claude/harnez/usage-history (usage-history, stale)` sources line — confirming the fallback is
live and the staleness display is honest, not fabricated as fresh.

**Status**: Resolved. Both gaps (the original offline/degraded-write repro, and this separate
usage-history read-side gap found via live re-verification) are closed as of this progress
entry.
