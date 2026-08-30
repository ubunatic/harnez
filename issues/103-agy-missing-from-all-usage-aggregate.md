# 103 — Antigravity missing from All Usage aggregate despite recent historical data

**Status**: Open
**Priority**: P1 (High) — bumped from P2; part of the collector-story investigation the user flagged as high priority on 2026-08-30
**Severity**: Moderate
**Category**: Bug
**Related**: [[086-offline-degraded-cache-snapshot-masks-live-data]], [[101-usage-keep-stale-agents-visible-until-7d]], [[093-usage-tui-layout-planner]], [[094-usage-watch-controls-overlay-and-presets]], commit 7c50f12 (write-path clobber fix), `docs/studies/2026-08-29-a-day-of-fresh-sprints.md`

## Problem

The user hasn't logged into Antigravity (AGY) in a while and expected
`harnez usage --summary --compact`'s `[a] All Usage` aggregate section to
still show AGY's last known quota/reset-time state, clearly marked stale —
the same way issue 086's history-fallback keeps it visible elsewhere.
Right now AGY is entirely absent from `[a] All Usage`:

```
$ harnez usage --summary --compact
┌─ [a] All Usage ────────────────────────────────┐
│ Claude Code   [░░░░] 7% 6d4h  [██░░] 56% 4h15m │
│ OpenAI Codex  [░░░░] 2% 6d23h [░░░░] 12% 4h47m │
└────────────────────────────────────────────────┘
```

No AGY row at all.

## Where the data actually lives

- `~/.claude/harnez/usage-history/*.jsonl` (`AppendHistory`/`ReadHistory`,
  `internal/usage/history.go`) — two files on this machine:
  `t14.jsonl` (667 lines, 1.6 MB, last write 2026-08-23 11:11:18+02:00) and
  `um760.jsonl` (last write 2026-08-23 11:01:13+02:00). Both carry real AGY
  quota snapshots, e.g. from `um760.jsonl`:
  `{"agent_id":"agy","name":"Antigravity (AGY)", ..., "model_groups":[{"name":"Gemini Models", "windows":[{"name":"Weekly Limit Remaining","used_percent":86.55,...,"reset_at":"2026-08-24T16:27:56Z"}, {"name":"Five Hour Limit Remaining", ...}]}, {"name":"Claude and GPT models", ...}]}`.
  This is exactly the same history log the 086/7c50f12 saga's pass-three
  fallback (`fillFromHistoryIfNoQuotaWindows`) reads from.
- No separate AGY-specific cache file exists on this machine beyond the
  collector-daemon's per-agent snapshot (`~/.claude/harnez/state/agy.json`
  equivalent, via `StateDir`/`snapshotPath`) and the live-fetch cache the
  running process itself would write to
  `~/.gemini/antigravity-cli/harnez-quota-cache.json` (referenced in
  `internal/usage/agy.go:317,392,413`) — that file doesn't currently exist
  here because AGY hasn't run recently enough to write it.

## What's actually there for AGY right now

The *last real AGY quota answer* recorded anywhere on disk is timestamped
**2026-08-23T11:11:18+02:00** (both history files' final entries). As of
this investigation (2026-08-30T14:5x CEST) that's **~7 days 3–4 hours**
old — i.e. it has just crossed the 7-day (`DefaultDisplayStaleness`,
`internal/usage/statecache.go:49`, `7 * 24 * time.Hour`) threshold that
the 086 history-fallback itself uses as its own freshness gate.

## Root cause (file:line)

1. **`fillFromHistoryIfNoQuotaWindows`** (`internal/usage/history.go:335-349`)
   is the 086/7c50f12-era fallback that fills `Session`/`Weekly`/
   `ModelGroups` from the most recent `usage-history` entry that had
   quota-window data. It gates on:
   ```go
   hist, at, ok := latestHistoryQuotaWindow(historyDir, u.AgentID)
   if !ok || time.Since(at) > DefaultDisplayStaleness {
       return u   // <-- unchanged: no Session/Weekly/ModelGroups filled
   }
   ```
   `at` is the *historical entry's own recorded timestamp*
   (2026-08-23T11:11:18+02:00 here), not a fixed one-time check — this
   re-evaluates on every render. Since that timestamp is now >7 days old,
   this fallback silently no-ops: `u.ModelGroups`/`u.Session`/`u.Weekly`
   stay nil, exactly as if no history existed at all.

2. **`allUsageLines`** (`internal/usage/watch.go:600-648`), which backs the
   `[a] All Usage` aggregate box (`buildAllUsageBox`, `watch.go:592-598`),
   only appends a row for an agent when it has actual quota windows:
   ```go
   if len(agent.ModelGroups) > 0 {           // watch.go:612
       ...
       continue
   }
   var wins []QuotaWindow
   if agent.Weekly != nil { wins = append(wins, *agent.Weekly) }   // watch.go:629-634
   if agent.Session != nil { wins = append(wins, *agent.Session) }
   if len(wins) > 0 {                         // watch.go:635
       rows = append(rows, allUsageRow{...})
   }
   ```
   With `ModelGroups`/`Weekly`/`Session` all empty for AGY (per #1), no row
   is ever appended — regardless of the earlier `agent.HasUsageData()`
   check at `watch.go:609` passing (it does pass, since AGY is
   `Authenticated` and has non-empty `Sources`). The aggregate's exclusion
   is a direct, mechanical consequence of #1: it isn't a separate
   "requires live/active" flag or an extra staleness gate specific to the
   aggregate box — `allUsageLines` doesn't apply `IsStale()`/
   `DefaultDisplayStaleness` at all; it just has nothing to render.

3. **Correction to the working assumption that the per-agent panel "must
   still be showing" AGY quota data**: it currently does not either. The
   AGY box in `harnez usage --summary` (non-compact) renders with account
   and model info but **no quota bars**, labeled "updated just now":
   ```
   ┌─ [G] Antigravity (AGY) ────────┐
   │ u***l@gmail.com · Consumer     │
   │ model: Gemini 3.7 Flash (Low)  │
   │ updated just now               │
   └────────────────────────────────┘
   ```
   That misleading "updated just now" is itself explained by
   `internal/usage/usage.go` (~line 73-75):
   ```go
   if agyUsage.LastRefreshed.IsZero() {
       agyUsage.LastRefreshed = now
   }
   ```
   Because `fillFromHistoryIfNoQuotaWindows` never set `LastRefreshed` (it
   returned early), this stamps the current wall-clock time onto an agent
   that in fact hasn't refreshed in 7+ days. That in turn makes
   `agent.IsStale(DefaultDisplayStaleness)` (`internal/usage/types.go:97-102`)
   return `false`, which is why the per-agent panel isn't auto-hidden by
   the issue-101 7-day gate at `watch.go:1258-1262` — it just has an empty
   box instead. So the per-agent panel isn't "leniently" showing stale
   data where the aggregate is strict; both are equally starved of quota
   data right now, and the per-agent panel's continued visibility is a
   side effect of this separate `LastRefreshed` defaulting behavior, not a
   working historical-fallback path the aggregate simply fails to reuse.

## Why the historical fallback doesn't feed the aggregate

`fillFromHistoryIfNoQuotaWindows` runs once, upstream, in `CollectAll`
(`internal/usage/usage.go`, applied to `claudeUsage`/`agyUsage`/
`codexUsage` before building `UsageSummary`). Both `buildWatchFrame`'s
per-agent panels and `buildAllUsageBox`'s aggregate read from that same
already-filled `AgentUsage` struct — there is no separate aggregate-only
data path to "wire up" a fallback into. When the fallback succeeds (i.e.
`time.Since(at) <= 7d`), `ModelGroups` gets populated and `allUsageLines`
*would* include AGY's row correctly, since it only checks for non-empty
windows. The bug is therefore upstream of the aggregate: the *same* 7-day
staleness gate on `fillFromHistoryIfNoQuotaWindows` governs both surfaces,
and once real data ages past 7 days, both go blank simultaneously — this
ticket's gap is that "past 7 days" is being treated as "discard/never
show" rather than "still show, but label as stale historical data," per
the desired behavior below.

## Acceptance Criteria

1. As a user, if I have used Antigravity within the last 7 days, I want to
   see Antigravity in the `[a] All Usage` section with the best metrics
   currently available — even if that data is stale/historical rather than
   freshly fetched — clearly labeled as such (e.g. a "(stale)"/age
   indicator consistent with how the per-agent panel already marks
   history-sourced data via its `Sources` string).
2. Antigravity usage data from previous days must be preserved so it can
   be used to assess Antigravity's current status: the historical fallback
   (`usage-history/*.jsonl` via `fillFromHistoryIfNoQuotaWindows`) must not
   be silently discarded once its data crosses the current 7-day cutoff —
   it needs to keep feeding both the per-agent panel *and* the `[a]` All
   Usage aggregate, not just whichever one happens to still pass today's
   staleness check. (Whether the fix is extending/decoupling the
   aggregate's freshness tolerance from `DefaultDisplayStaleness`, or some
   other mechanism, is left to whoever picks this up — but data that
   exists on disk and used to render fine one day should not vanish from
   the aggregate view the next day purely because a hardcoded 168h window
   ticked over.)

## Notes for whoever picks this up

- Reproducing this is time-sensitive: at the time of writing, AGY's last
  real history entry (2026-08-23T11:11:18+02:00) is right at the 7-day
  boundary, so this may need `time.Since(at) > DefaultDisplayStaleness` in
  `history.go:337` temporarily lowered, or a synthetic history entry
  added, to get a stable repro without waiting for real data to age past a
  week again.
- Consider whether `allUsageLines` should independently label
  history-sourced/stale rows (it currently has no concept of staleness at
  all, unlike the per-agent panel's "updated Nd Nh ago" line) as part of
  acceptance criterion 1.
- The `LastRefreshed`-defaults-to-`now`-when-zero behavior in
  `internal/usage/usage.go` (causing the misleading "updated just now" on
  an agent that hasn't actually refreshed) is adjacent but distinct from
  this ticket's scope — flagged here for visibility, not folded into this
  ticket's acceptance criteria.
