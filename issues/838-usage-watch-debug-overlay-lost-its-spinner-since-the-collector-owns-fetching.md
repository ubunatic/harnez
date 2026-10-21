# 838 — usage watch: ! debug overlay lost its spinner since the collector owns fetching

**Status**: Open
**Priority**: P2 (Medium)
**Severity**: Minor
**Category**: Bug
**Related**: 829-837 (client-server sprint, collector as single writer), 831

---

/goal Make the `!` debug overlay in `harnez usage -w --minimal` show a live spinner that
reflects the data collector's fetch activity, verified by a test and a live check, or stop
and report when blocked on a user decision or denied permission.

## 1. Problem & Motivation
Pressing `!` in `harnez usage -w --minimal` toggles the debug overlay, which used to show an
animated per-agent freshness spinner/gauge. It no longer animates. Likely cause: since the
client-server sprint (829-837) the background collector owns fetching, so the watch client
no longer sees its own fetch cycle and has nothing to animate. The overlay should again show
when data is being collected.

## 2. Technical Specification / Findings
- Key handling: `dispatchWatchKey(state, '!', …)` toggles `debugOverlay`
  (test: `internal/usage/watch_test.go` around line 1990, glyph `timeoutSnakeGlyph`).
- Needed: a way for the watch client to observe the collector (e.g. last-fetch / in-flight
  state per agent exposed over the server socket) and map that to the spinner.
- Unverified: whether the existing test still passes because it renders a static frame;
  check whether the gap is in the data path or the render path before designing.

## 3. Implementation & Verification Plan
- Re-verify live behaviour and recent commits first (ticket may be stale).
- Expose collector activity to the client; animate the overlay spinner from it.
- Test: overlay frame changes while the collector reports a fetch in flight; live check
  with the installed collector service after `make install`.
