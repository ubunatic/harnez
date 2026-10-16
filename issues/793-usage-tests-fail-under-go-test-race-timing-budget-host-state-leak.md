# 793 — usage tests fail under go test -race (timing budget, host-state leak)

**Status**: Closed — race-build timing budget scaled; concurrency test made deterministic and isolated.
**Priority**: P2 (Medium)
**Severity**: Minor
**Category**: Bug
**Related**:

---

## 1. Problem & Motivation
`go test -race ./internal/usage/` failed with no DATA RACE report in
TestCollectAllRunsCollectorsConcurrently and in TestWatchPTYLifecycle
(startup/splash/refresh quit subtests).

## 2. Technical Specification / Findings
- TestWatchPTYLifecycle: "quit took ~1.02s, want <1s". The Go race runtime
  sleeps ~1s at process exit (GORACE atexit_sleep_ms), so the 1s quit budget
  can never hold in a race build. Pure timing budget.
- TestCollectAllRunsCollectorsConcurrently: two causes. (a) A 120ms wall-clock
  budget was too tight under race instrumentation (collectAll also publishes
  to a sqlite store). (b) The test did not isolate HOME/XDG, so a warm AGY
  snapshot left in the telemetry store by an earlier run skipped the live
  AGY fetch; the test then depended on run history.

## 3. Implementation & Verification Plan
- race_test.go / norace_test.go: `raceEnabled` const (build tag race / !race).
- Quit budget in watch_lifecycle_test.go is 1s, x4 when raceEnabled.
- Concurrency test now uses a barrier (all three collectors must be in flight
  before any is released) instead of a wall-clock budget, and isolates
  HOME, XDG_DATA_HOME, XDG_STATE_HOME.
- Verified: go test -race -count=3 and plain -count=3 on both tests; race run of
  internal/usage, internal/tui/..., internal/subagent.
