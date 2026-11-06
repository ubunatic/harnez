# 1175 — usage watch: debug frame loses lines after a collector refresh (TestBuildWatchFrameAtDebugOverlayUsesCollectorCadence fails on main)

**Status**: Open
**Priority**: P2 (Medium)
**Severity**: Minor
**Category**: Bug
**Related**: 838, 1121, 1122

---

## Problem

`make test-q1` on 2026-11-06 (main after 74e2ca78) fails one test; it also fails on 74e2ca78~1, so it
predates that change:

```
internal/usage/watch_test.go:2298: collector refresh changed debug frame geometry:
before=89x24/16 lines after=89x24/13 lines
```

Reproduce: `go test ./internal/usage -run TestBuildWatchFrameAtDebugOverlayUsesCollectorCadence -count=1`.
Last `internal/usage` commits: e989eef6 (1122), 40d51542 and 4add26f4 (1121). Not verified which one
introduced it; bisect between them.

## Done when

- The test passes, and the frame keeps its line count across a collector refresh.

## Searcher findings (2026-11-06, search-1175, codex:luna:low)

- The test builds the first compact debug frame from one authenticated Claude agent with a weekly quota
  and `LastRefreshed=refreshed` (`internal/usage/watch_test.go:2263`), the second with
  `LastRefreshed=midCycle` (`:2288`). The 3 lost lines are the All Usage box's top border, quota row and
  bottom border. `buildWatchFrameAt` reserves debug-box lines from the body budget
  (`internal/usage/watch_frame.go:365`); the refreshed gauge changes the panel's natural height.
- None of e989eef6, 40d51542, 4add26f4 explains it convincingly; likely an older layout interaction.
- The test is right: `watch_test.go:2202` asserts the debug overlay preserves frame geometry; 1121 and
  1122 intend no debug-frame change.
- Likely write set: `internal/usage/watch_usage_box.go` (`buildAllUsageBoxAt`, `allUsageLinesAt`,
  around :45) and `watch_frame.go` body budgeting; regression coverage in `watch_test.go`. Related tests:
  `TestDispatchWatchKeyDebugOverlayRendersAndRestoresFrame` (:2155),
  `TestAllUsageLinesAtDebugOverlayCountsDown` (:2226), `TestUsageDebugKeysAndFrames`
  (`watch_debug_test.go:11`).

## Decision

- Developer `tui` trait (smart, codex:sol:med): layout cause not pinned, TUI geometry. Codex has quota
  (5h 5%), Claude does not (73%). Plan-only first turn.
