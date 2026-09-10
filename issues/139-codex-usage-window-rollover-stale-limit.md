# 139 - Codex Usage Window Rollover Keeps Stale Limit State

**Status**: Closed — resolved: Codex quota rollover recomputation, cache rejection, and expiry staleness handling are complete
**Priority**: P2  
**Severity**: Medium  
**Category**: Usage / Quota UI

## Problem

### Audit — 2026-09-10

- **Conclusion: unsolved (source inspection).** `buildCodexQuotaWindow` still
  freezes `DurationLeft` at collection; `compactDurationText`, compact group
  rendering, and `RenderText` still read that stored duration. `QuotaWindow`
  has no render-time expiry/remaining-time accessor.
- Existing per-agent stale marking helps on fetch failure, but does not detect
  a reset timestamp passing while the snapshot otherwise looks fresh.
- `TestCollectCodexExpiredCacheTriggersLiveFetch` tests cache **age** using
  `FetchedAt`, not quota-window rollover. `TestBuildCodexQuotaWindow` tests
  construction, not a subsequent frame crossing reset. The deterministic
  rollover regression required below remains missing.
- **Measured:** `go test ./...` passes; this does not establish the missing
  rollover behavior. No live quota-reset observation was performed.

`harnez usage` can show stale Codex quota-window state when an existing
watch/UI session spans a limit-window reset. The observed UI started
before the window reset and later showed Codex with the short window at
100% even though the reset timer had reached the rollover point:

```text
OpenAI Codex  [▋   ] 16% 6d19h  [████] 100% 1m
```

The expected behavior is that once the limit window rolls over, usage
and reset-time state refresh coherently rather than leaving the old
window pinned at 100% until the user restarts or triggers some unrelated
collector refresh.

## Impact

- The user cannot trust the Codex quota panel around rollover time.
- A stale 100% bar can make Codex look unavailable when the new window
  should have started.
- Long-running `harnez usage --watch` sessions become less useful for
  planning agent work near quota resets.

## Expected Behavior

- When a Codex usage window reaches or passes its reset timestamp, the
  collector/UI should treat the quota snapshot as expired.
- The next display update should either fetch fresh Codex quota data or
  clearly mark the data stale/degraded.
- The UI should not keep showing a fully exhausted old window with a
  near-zero or expired countdown as if it were current.

## Investigation Notes

- Check whether Codex quota cache records include enough reset-time
  metadata to expire a single window independently.
- Check whether `usage --watch` recomputes rollover state from cached
  data every frame or only when a collector writes a new snapshot.
- Check whether stale fallback logic from previous quota-cache fixes is
  masking rollover expiry.
- Reproduce with a fixture/cache snapshot whose reset timestamp is in
  the past, then render the Codex panel without live network access.

## Acceptance Criteria

- Add a test fixture or unit test for a Codex quota snapshot whose reset
  time has passed while the UI/watch process remains alive.
- The UI no longer renders the expired window as current 100% usage.
- The behavior is deterministic without requiring a real Codex quota
  window reset.
- If fresh live fetch fails after rollover, the panel marks the data as
  stale/degraded instead of presenting the old exhausted window as fresh.

---

## Implementation Plan

### Root-cause analysis (from source, no repro needed)

Three independent contributors, all confirmed by reading the code:

1. **`DurationLeft` is frozen at collect time.** `buildCodexQuotaWindow`
   (`internal/usage/codex.go:85-115`) computes `qw.DurationLeft = t.Sub(now)`
   once and stores it in the `QuotaWindow` struct
   (`internal/usage/types.go:9-15`). That value is serialized into both the
   live-fetch cache (`~/.codex/harnez-quota-cache.json`) and the collector
   snapshot. Every renderer reads the stored field rather than recomputing from
   `ResetAt`: `compactDurationText` (`watch.go:904`),
   `formatCompactGroupLineWithLabelWidth` (`watch.go:1442`), and
   `usage.go:261-320`. A snapshot taken 1 minute before rollover therefore
   renders `1m` forever, which is exactly the reported symptom.
2. **Nothing marks a window expired.** There is no `ResetAt <= now` check
   anywhere. A window whose reset has passed still renders its old
   `UsedPercent` (100%) as current.
3. **The stale-snapshot fallback can pin it indefinitely.** `cacheOrLive`
   (`internal/usage/statecache.go`) serves `snap.Usage` tagged
   `(cached, stale)` whenever a live recollect loses quota signal, and
   `DefaultDisplayStaleness` is 7 days. Post-rollover fetch failures therefore
   keep serving the pre-rollover exhausted window.

Note the bug is **not Codex-specific** — `claude.go:230-252` and
`agy.go:237-239` build windows the same way. Fix at the `QuotaWindow` level so
all three agents benefit; keep the ticket's Codex test as the regression case.

### Steps

1. `internal/usage/types.go` — add two pure methods on `QuotaWindow`:
   - `func (w QuotaWindow) RemainingAt(now time.Time) time.Duration` — if
     `ResetAt != nil`, return `max(0, ResetAt.Sub(now))`; otherwise fall back to
     the stored `DurationLeft` (windows built from `ResetAfterSeconds` with no
     absolute reset still have `ResetAt` set, so the fallback is only for
     legacy/handwritten values).
   - `func (w QuotaWindow) ExpiredAt(now time.Time) bool` — `ResetAt != nil &&
     !ResetAt.After(now)`.
   Do **not** change the JSON shape or remove the `DurationLeft` field —
   existing snapshots on disk must keep decoding.
2. `internal/usage/watch.go` — thread the already-available frame `now` (the
   watch redraw already captures one; see `buildAllUsageBoxAt`/`buildAgentBoxAt`)
   into the duration formatters:
   - `compactDurationText(w, now)` and
     `formatCompactGroupLineWithLabelWidth(..., now)` use `w.RemainingAt(now)`.
   - When `w.ExpiredAt(now)`, render the countdown slot as a stale marker
     (reuse the existing dim styling, e.g. `~` or `--`) and dim/annotate the
     percentage rather than showing a confident `100%`. Keep the exact same
     visible width so `uix.Layout` column planning is unaffected.
3. `internal/usage/usage.go:255-325` — same substitution for the non-watch
   one-shot renderer, using `time.Now()` at the top of the render call.
4. `internal/usage/codex.go` — in the live-fetch cache hit branch
   (`codex.go:228`), also treat the cached payload as unusable when both its
   windows are expired, so a rollover forces a live refetch rather than waiting
   out `MinWatchInterval`. Mirror in `claude.go`/`agy.go` only if trivially
   symmetric; otherwise leave for a follow-up.
5. `internal/usage/statecache.go` — in `cacheOrLive`'s "don't blank a
   stale-but-real snapshot" branch, keep serving the snapshot (that guard is
   correct and load-bearing per issue 101) but ensure the expired-window
   rendering from step 2 is what the user sees. No change needed here if step 2
   is done at render time — verify with a test rather than editing.

### Tests

All fixture-driven, no network, deterministic:

- `internal/usage/types_test.go` — table test for `RemainingAt`/`ExpiredAt`:
  reset in the future, exactly now, in the past, `ResetAt == nil`.
- `internal/usage/codex_test.go` — `buildCodexQuotaWindow` with a `ResetAt` in
  the past yields a window that `ExpiredAt(now)` reports true for.
- `internal/usage/watch_test.go` — golden-ish assertion: a `QuotaWindow{
  UsedPercent: 100, ResetAt: now-5m, DurationLeft: 1*time.Minute}` renders
  without a `1m` countdown and with the stale marker; the same window with
  `ResetAt: now+1m` still renders `1m`. This is the ticket's required
  regression test.
- `internal/usage/statecache_test.go` — a snapshot whose windows are all expired
  still round-trips (no blanking), proving step 5 is a render-layer fix.

### Design decisions / tradeoffs

- **Recompute at render, don't rewrite state.** Mutating `DurationLeft` in the
  cache would need a writer on every read path and would fight the flock
  protocol in `livefetchcache.go`. A pure `RemainingAt(now)` accessor is
  testable and touches no I/O.
- **Keep `DurationLeft` in the JSON.** Removing it breaks decoding of existing
  snapshots and the exported history (`internal/usage/export.go`,
  `history.go`).
- **Degrade, don't hide.** Per the Acceptance Criteria, an expired window is
  marked stale rather than dropped — dropping it would make Codex look
  uninstalled.

### Risks / open questions

- Compact-layout width: the stale marker must not change `visLen`, or the
  `uix.Layout` measure/render two-pass in `watch.go:1786+` will re-plan columns.
  Pick a marker of equal width to the longest countdown it replaces, or reuse
  the existing "no duration" path (empty string) plus a dimmed percentage.
- Open question: should an expired *weekly* window be presented differently from
  an expired *session* window? Weekly rollovers are rarer and a stale weekly
  reading is less misleading. Suggest identical treatment for v1; revisit if
  noisy.
- Codex's `wham/usage` may legitimately report a `reset_at` slightly in the past
  during the server-side rollover window. Consider a small grace (e.g. treat as
  expired only after `ResetAt + 60s`) to avoid a flapping stale marker; decide
  with one live observation.

### Scope

**Medium** — small, well-bounded code change across 4 files, but it touches the
shared quota-window type used by all three collectors and both renderers, plus
five focused test additions.
