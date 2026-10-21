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
- User note: the non-minimal view (`harnez usage -w` without `--minimal`) already shows
  collector freshness in its "Updated" column, so the data reaches the client. Reuse that
  source for the minimal overlay's spinner instead of building a new collector channel.
- Unverified: whether the existing test still passes because it renders a static frame;
  check whether the gap is in the data path or the render path before designing.

## Searcher findings (2026-10-21, s838 luna:low)
- The `!` glyph is an age countdown, not a rotating spinner: `timeoutSnakeGlyph` compares
  `LastRefreshed` with the redraw time against the *watch interval*
  (`internal/usage/freshness.go:20`, `watch_usage_box.go:177`, `indicatorsspec.go:510`).
- Minimal mode redraws every second (`watch.go:881,910`), so render is fine. Since 829 the watch
  reads stored collector snapshots (`watch.go:765`, `readstore.go:23`); the timestamp only moves
  when the collector writes (now every 5 min after 839), so the gauge runs out after one watch
  interval and then sits still.
- Updated column = `FormatAgo(agent.LastRefreshed)` (`usagetable.go:102`), same source. No
  fetch-in-progress signal exists in the read path.

## User decision (2026-10-21)
Show "the same braille progress spinner as before": the overlay animates the braille spinner
(likely `braille-classic-10` from `spec/indicators.yaml`, check pre-829 history for the exact
behaviour) while the collector is fetching. This needs a collector fetch-in-progress /
last-fetch signal in the client read path.

## 3. Implementation & Verification Plan
- Re-verify live behaviour and recent commits first (ticket may be stale).
- Expose collector activity to the client; animate the overlay spinner from it.
- Test: overlay frame changes while the collector reports a fetch in flight; live check
  with the installed collector service after `make install`.
