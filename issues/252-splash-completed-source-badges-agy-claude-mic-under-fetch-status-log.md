# 252 — Splash: Completed Source Badges (✓ agy ✓ claude … ✓ mic) Under Fetch Status Log

**Status**: Closed — completed source badges rendered under splash fetch status log
**Priority**: P2 (Medium)
**Severity**: Minor
**Category**: UX / Agentic Ergonomics
**Related**: [[164-fast-startup-usage-watch-splash-or-stale-data]] (added the splash screen),
[[168-splash-determinate-progress-bar-fetch-duration-estimate]] (determinate progress bar),
[[169-splash-status-log-line-per-fetch-stage]] (single-line rolling status of in-flight fetch stages),
[[244-show-system-mic-level-and-recording-on-off-as-new-usage-watch-tui-box]],
[[245-show-live-mic-input-signal-level-peak-rms-alongside-configured-volume-in-usage-watch]],
`internal/usage/watch.go`, `internal/usage/usage.go`

---

## 1. Problem & Motivation

During `harnez usage --watch` startup, the splash screen (issues 164, 168, 169) displays a progress
bar and a single rolling status line reporting the currently active fetch stage (e.g., `fetching agy...`,
`claude done`).

While this informs the user of the active step, it provides no cumulative overview of which collectors
and subsystems have already finished resolving. The user requested adding a row of completed checkmark
badges (such as `"✓ agy"  "✓ claude"  ..  "✓ mic"`) under the splash status log to provide immediate,
at-a-glance confirmation of completed collectors before the live dashboard paints.

## 2. Technical Design

### 2.1 Completed Source State Accumulation
- In `internal/usage/watch.go`, `splashStatusState` (or an accompanying tracker) currently queues and
  advances events to show one event on the status line.
- Extend this state to accumulate successfully completed (or failed) sources as `FetchDone` /
  `FetchFailed` events arrive from `CollectAllProgress` / `CollectRemoteProgress`.
- If local subsystems like Mic or Load/GPU probes complete during the startup sequence, ensure their
  completion is reported into the progress stream so they participate in badge display.

### 2.2 Badge Line Rendering in `buildSplashFrame`
- Render a badge row under the single status log line in `buildSplashFrame`:
  ```
  ⠋  harnez usage

  [============░░░░░░░░░░░░░░░░░░░░]

  fetching codex...
  ✓ agy  ✓ claude  ✓ mic

  Esc to skip
  ```
- Styling: Use spec-driven colors via `ansiWrap` (e.g. green/dim checkmarks, muted labels) without
  hardcoded ANSI escape codes.
- Layout Budgeting: Keep the splash centered vertically and horizontally. If terminal height is
  constrained, ensure the extra badge line does not push content off-screen. If no sources have
  completed yet, keep the line empty or omit it cleanly.

## 3. Scope of Implementation

1. **`internal/usage/watch.go`**:
   - Accumulate completed source badges in `splashStatusState`.
   - Update `buildSplashFrame` to render the completed badges line under the status line.
   - Format badges with spec-driven styling (e.g. `✓ <source>`).
2. **`internal/usage/usage.go`** & probe paths:
   - Ensure all startup sub-probes (agents, mic, load/procs) report completion events to `reportFetchStage`.
3. **`internal/usage/watch_test.go`**:
   - Unit tests for badge line rendering and state accumulation in `buildSplashFrame`.

## 4. Acceptance Criteria

- [x] As each collector/probe finishes during `--watch` startup splash, its checkmark badge (e.g. `✓ agy`, `✓ claude`, `✓ mic`) appears in a dedicated line under the fetch status log.
- [x] Completed badges persist and accumulate across the splash animation until the live dashboard paints.
- [x] Failed sources (if any) are either distinguished (e.g. `✗ name`) or omitted cleanly.
- [x] Layout remains centered, visually balanced, and respects terminal size constraints.
- [x] Unit tests pass and `make check` succeeds cleanly.

## 5. Verification

- **Automated**: `go test -v ./internal/usage/...` covering `buildSplashFrame`, `splashBadgesLine`, `splashStatusRecord`, and badge rendering.
- **Interactive**: Run `harnez usage --watch` on cold and warm cache; verify that completed badges appear sequentially under the status log line before transitioning to the dashboard grid.

## 6. Resolution

- Implemented `splashBadge` tracking and `splashStatusRecord` in `internal/usage/watch.go` to accumulate finished probe stages (`FetchDone`, `FetchFailed`) in real time.
- Implemented `splashBadgesLine` using spec-driven colors (`ansiWrap` with `chart-green` for `✓` and `chart-warm` for `✗`, `dim-grey` for labels).
- Updated `buildSplashFrame` to render the completed badge row directly under the rolling status line while preserving centering and viewport boundary constraints.
- Integrated concurrent startup probe for `mic` in `fetchAndUpdate` so local subsystems report progress stages alongside agent collectors.
- Added comprehensive unit test coverage in `internal/usage/watch_test.go` (`TestSplashBadgesLine`, `TestSplashStatusRecordAccumulatesBadges`, `TestBuildSplashFrameBadgesRow`).


