# 164 — Fast Startup for `usage --watch`: Show Stale Data or a Splash Screen Immediately

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

Two complementary approaches, not mutually exclusive:

1. **Stale-data-first paint**: the codebase already maintains an on-disk quota
   cache (`internal/usage/statecache.go: cacheOrLive`,
   `internal/usage/livefetchcache.go: readLiveFetchCache`) and `collectAll` already
   accepts a `useCache` flag (`internal/usage/usage.go:38`). `RunWatchWithOptions`
   could read the cache synchronously (no network) and call `draw()` immediately
   with that stale `UsageSummary` before `renderFrame()` kicks off the first live
   fetch. The existing `applyStaleQuota` staleness-marking machinery
   (`internal/usage/watch.go:1911`) already exists for exactly this kind of
   "data is old, mark it visibly" case and should carry the freshness indicator
   through to this first frame too.
2. **Splash/progress screen fallback**: for the remote-host case (`CollectRemote`)
   or on a cold start with no cache yet, there's no stale data to show. In that
   case, paint a minimal splash screen immediately after entering the alt-screen
   (before `renderFrame()` blocks) — a simple progress bar / spinner is enough to
   start. This can later be extended with more elaborate loading treatments
   (comparable to other terminal loading/splash animations seen elsewhere in the
   agentic-tooling space), but the first cut should stay a single simple
   progress-bar or spinner frame, not an animation system.

## 3. Implementation & Verification Plan

- Add a synchronous, network-free "try cache, else show splash" step between
  entering the alt-screen (`watch.go:1862`) and the first `renderFrame()` call
  (`watch.go:1916`).
- If a usable cached `UsageSummary` exists, `draw()` it immediately (reusing
  `applyStaleQuota` so staleness is visually indicated), then proceed with the
  normal `renderFrame()` → live fetch → `draw()` sequence as today.
- If no cache is available (cold start, or remote host with nothing cached yet),
  paint one static splash frame (progress bar or spinner) instead of a blank
  screen, then proceed as today.
- Verify manually (pty-based repro per past TUI postmortems, not reasoning-only)
  that the first frame appears well under 3s on a warm cache, and that a cold
  start shows the splash immediately instead of a blank screen.
- Keep scope to the immediate-first-frame problem; do not build a general
  splash/animation framework in this pass.
