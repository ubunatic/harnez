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
