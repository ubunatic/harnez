# 147 — Debug overlay gauge does not count down to the next usage fetch

**Status**: In Progress — reopened after live watch regression report
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

- [x] Capture one wall-clock timestamp for each watch redraw and thread it
  through compact and per-agent overlay rendering.
- [x] Add deterministic time-controlled compact-overlay coverage for full,
  halfway, empty, future, and overdue timestamps.
- [x] Verify with focused usage tests and `go test ./...`.
- [ ] Reproduce and fix the live compact-watch redraw path so its visible
  gauge advances between collector fetches.
