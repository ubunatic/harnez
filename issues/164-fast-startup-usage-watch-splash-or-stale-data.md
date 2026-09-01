# 164 — Fast Startup for `usage --watch`: Abortable Progress-Bar Splash by Default

**Status**: Open
**Priority**: P2 (Medium)
**Severity**: Minor
**Category**: Agentic Ergonomics
**Related**: internal/usage/watch.go (`RunWatchWithOptions`, `renderFrame`, `draw`), internal/usage/statecache.go, internal/usage/livefetchcache.go, internal/usage/usage.go (`collectAll`)

---

## 1. Problem & Motivation

`harnez usage --watch` takes roughly three seconds between invocation and the first
painted frame. The alt-screen buffer is entered immediately
(`internal/usage/watch.go:1862`), but the terminal then sits blank because
`renderFrame()` is called synchronously before the event loop starts
(`internal/usage/watch.go:1916`), and `renderFrame()` blocks on a live network
fetch (`CollectAll`/`CollectRemote`) before calling `draw()` for the first time.
The user sees an empty alt-screen for the full fetch duration with no feedback
that anything is happening.

## 2. Technical Specification / Findings

Two complementary approaches, not mutually exclusive. **For this first pass, the
progress-bar splash is the default startup behavior** — not just a cold-start
fallback — since it's the simpler of the two to land; stale-cache-first paint
can supersede it as the default later once implemented.

1. **Splash/progress screen (default for this ticket)**: paint a minimal splash
   screen — a simple progress bar or spinner — immediately after entering the
   alt-screen (`watch.go:1862`), before `renderFrame()`'s blocking call to
   `CollectAll`/`CollectRemote` (`watch.go:1916`). This can later be extended
   with more elaborate loading treatments (comparable to other terminal
   loading/splash animations seen elsewhere in the agentic-tooling space), but
   the first cut should stay a single simple progress-bar or spinner frame, not
   an animation system.

   **Escape-to-abort, not quit**: while the splash is showing, Esc must abort
   only the splash/startup wait — dropping straight into the current frame
   (stale/placeholder) and continuing the normal fetch cycle in the
   background — and must NOT quit the app. This is a deliberate deviation from
   main-screen behavior, where `q`/Ctrl-C/Esc (byte 27) all map to
   `watchKeyEffect{quit: true}` (`dispatchWatchKey`, `watch.go:474-524`; help
   text at `watch.go:1353`). The splash's key-reading loop needs its own
   small dispatch (or a startup-phase flag threaded into `dispatchWatchKey`)
   so Esc is scoped to "skip the wait" during splash only; Ctrl-C should still
   quit even during splash, since that's the universal interrupt.
2. **Stale-data-first paint (future default)**: the codebase already maintains
   an on-disk quota cache (`internal/usage/statecache.go: cacheOrLive`,
   `internal/usage/livefetchcache.go: readLiveFetchCache`) and `collectAll` already
   accepts a `useCache` flag (`internal/usage/usage.go:38`). `RunWatchWithOptions`
   could read the cache synchronously (no network) and call `draw()` immediately
   with that stale `UsageSummary` before `renderFrame()` kicks off the first live
   fetch. The existing `applyStaleQuota` staleness-marking machinery
   (`internal/usage/watch.go:1911`) already exists for exactly this kind of
   "data is old, mark it visibly" case and should carry the freshness indicator
   through to this first frame too. Once implemented, this should take priority
   over the splash whenever a usable cache entry exists, with the splash
   remaining as the true-cold-start fallback.

## 3. Implementation & Verification Plan

- Paint a splash frame (progress bar or spinner) immediately after entering the
  alt-screen (`watch.go:1862`), before the blocking `renderFrame()` call
  (`watch.go:1916`) — this is the default startup path for this ticket.
- Read keys during the splash on a path separate from (or flagged within)
  `dispatchWatchKey` so Esc aborts only the wait — drop into the current/stale
  frame and continue the normal fetch/draw cycle — without setting
  `watchKeyEffect{quit: true}`. Ctrl-C still quits during splash.
- Once `renderFrame()`'s first live fetch completes (or Esc aborts the wait),
  proceed with the normal `renderFrame()` → live fetch → `draw()` sequence as
  today.
- Verify manually (pty-based repro per past TUI postmortems, not reasoning-only)
  that: the splash appears immediately instead of a blank screen; Esc during
  splash returns to the dashboard without exiting the process; Ctrl-C during
  splash still exits; Esc on the main dashboard after startup still quits as
  before (no regression to existing behavior).
- Keep scope to the immediate-first-frame problem; do not build a general
  splash/animation framework in this pass. Stale-cache-first paint (§2.2)
  is a follow-up, not required for this ticket's completion.
