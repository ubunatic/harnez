# 147 — Debug overlay gauge does not count down to the next usage fetch

**Status**: Open
**Priority**: P2 (Medium)
**Severity**: Moderate
**Category**: Bug
**Related**: [[131-watch-debug-freshness-countdown-overlay]], [[140-debug-overlay-does-not-work]], `internal/usage/watch.go`, `internal/usage/statecache.go`

---

## 1. Problem & Motivation

The `!` debug overlay now renders in `harnez usage --watch --compact`, but
its per-agent freshness gauge does not visibly drain as time passes. The
feature is intended to count down to each agent's next usage-data fetch, so a
static gauge misrepresents collector freshness and prevents the overlay from
serving its diagnostic purpose.

## 2. Technical Specification / Findings

- The gauge must derive its remaining fraction from each agent's
  `LastRefreshed` timestamp and `DefaultCollectorInterval` on every watch
  redraw, not only when fresh usage data arrives.
- Confirm the watch refresh/redraw cadence is sufficient for a visible
  countdown and that time-dependent gauge rendering is not cached with the
  prior frame.
- Keep per-agent gauges independent and clamp overdue/future timestamps to
  valid empty/full states.

## 3. Implementation & Verification Plan

- Reproduce the static display in an interactive compact watch session and
  isolate whether the cause is redraw cadence, time calculation, or frame
  caching.
- Add deterministic time-controlled tests proving a gauge is fuller shortly
  after refresh and lower as the same agent approaches the next interval.
- Ensure the live watch redraws the overlay without triggering unnecessary
  usage fetches or changing collector cadence.
- Verify manually across several redraws and run focused usage tests plus
  `harnez status`.
