# 079 — Apply Consistent Max-10-Char Shrink-to-Fit Width to All Usage Graphs

**Status**: Open
**Priority**: P2 (Medium)
**Severity**: Moderate
**Category**: Bug
**Related**: [[078-rograph-library-shared-bar-sparkline-renderer]], `internal/usage/usage.go`, `internal/usage/watch.go`

---

## 1. Problem

Depends on [[078-rograph-library-shared-bar-sparkline-renderer]] landing first.

Today, graph widths across the `usage`/`watch` boxes are inconsistent and mostly
uncapped-by-design:

- Quota bars in `usage.go` (non-watch/plain report path): hardcoded `width=24`.
- Quota bars in `watch.go`'s agent box: dynamic `barW`, clamped `[1, 12]`.
- Load box CPU/GPU sparklines: effectively fixed at 10 glyphs, but only because
  `loadHistoryLen = 10` (`load.go:341`) happens to be 10 — not because anything
  renders with a width cap. Never shrinks below that.

The user's ask: every single-row graph in the TUI (quota bars *and* load sparklines)
should behave exactly like the current agent-usage-box bars already do — a max width
of 10 characters, shrinking down to a minimum of 1 character when the box doesn't have
room, and never growing past 10 regardless of how much space is available.

## 2. Fix

Using the `rograph` primitives from [[078-rograph-library-shared-bar-sparkline-renderer]]:

1. Introduce one shared constant, e.g. `rograph.MaxWidth = 10`, replacing the
   `24` hardcode in `usage.go` and the `12` clamp in `watch.go`.
2. Pass `min(rograph.MaxWidth, availableWidth)` at every call site — bars and load
   sparklines alike — instead of each call site inventing its own clamp math.
3. Load sparklines (`formatCPULine`/`formatGPULine`, `watch.go:509-544`): keep
   `loadHistoryLen` as the sample-buffer size (a data concern), but decouple render
   width from it — rendering should ask for `min(10, availableWidth)` glyphs, drawn
   from the most recent N history points, not implicitly render exactly however many
   history points exist. This matters once box width can go below 10.
4. Verify visually (`harnez usage --watch`) at a few terminal widths, including narrow
   enough to force sub-10 shrinking, that bars and sparklines shrink identically and
   stay visually aligned with their labels.

## 3. Acceptance

- No call site hardcodes a bar/sparkline width other than through the shared
  `rograph.MaxWidth` constant and the shrink-to-fit helper.
- Resizing the terminal narrower shrinks every graph (bars and sparklines) the same
  way, down to 1 character, with no panic/negative-width edge cases.
