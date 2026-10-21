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
- Needed: use each stored snapshot's `LastRefreshed` with the collector cadence for the
  existing overlay glyph. No fetch-status socket API or new indicator is needed.
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
Keep the existing single-cell `braille-snake-2x3` sequence (`timeoutSnakeGlyph`) unchanged;
the bug is its time input. Drain from `LastRefreshed` over `collector_cadence` in `spec/usage.yaml`
(currently 5m), then refill when the collector stores a newer snapshot. Do not add a fetch-status
socket route or change `spec/indicators.yaml`.

## Implementation findings (issue 838 M1)
- `buildAllUsageBoxAt` received the watch interval and passed it to the overlay countdown, so the
  glyph reached its empty frame after one watch interval even though stored snapshots update on
  the collector cadence.
- For debug-overlay rendering, it now uses `DefaultCollectorInterval()` from the embedded usage
  spec. The normal watch redraw interval and the existing Braille sequence are unchanged.
- Regression coverage checks the half-cadence glyph, refill after a newer `LastRefreshed`, and
  unchanged frame geometry.
- `make test-q1` passed; `grep -- '--- FAIL'` found no failures. `make install` succeeded and
  `harnez server status` confirmed the systemd collector unit remained active.
- Live PTY verification was attempted, but the exec wrapper killed the interactive watch before
  it rendered (stopped process group, exit 125); the cadence behavior is verified by the frame test.

## 3. Implementation & Verification Plan
- Host review remains; do not close this ticket before that review.
