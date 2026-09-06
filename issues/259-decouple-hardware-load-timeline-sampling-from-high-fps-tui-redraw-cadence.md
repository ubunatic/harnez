# 259 — Decouple Hardware Load Timeline Sampling from High-FPS TUI Redraw Cadence

**Status**: Open
**Priority**: P2 (Medium)
**Severity**: Moderate (high-frequency mic redraws accelerate CPU/GPU/RAM history scrolling 20x, destroying sparkline time-series resolution)
**Category**: Performance / UX / Architecture
**Related**: [[258-dynamic-refresh-rate-for-live-mic-meter-high-frequency-ui-redraw-on-speech-activity]],
[[201-double-timeseries-resolution-and-add-btop-style-braille-sparklines]],
`internal/usage/load.go`, `internal/usage/watch.go`

---

## 1. Problem & Motivation

Ticket 258 introduced speech-activated high-frequency UI redraws (10–20 Hz, ~50ms cadence) in `harnez usage --watch`
to provide a fluid real-time microphone VU meter.

However, during high-FPS redraws, the CPU, GPU, and RAM load timeline graphs scroll violently fast across the screen:
1. In `internal/usage/load.go`, `CurrentCPULoad()`, `CurrentGPUs()`, and `CurrentSystemMemory()` append new samples
   to their rolling time-series ring buffers (`cpuHistory.append(aggPct)`, `gpuHistory.append(...)`, `memHistory.append(...)`)
   **synchronously on every invocation of `draw()`**.
2. When the dashboard redraws 20 times per second for mic activity, the hardware sparklines and Braille charts
   sample `/proc/stat` and advance their histories at 20 Hz instead of the intended 1 Hz.
3. This compresses what should be 10–20 seconds of load history into less than a second, rendering the load
   graphs unreadable and misleading whenever the user speaks.

The hardware load timeline sampling cadence must be strictly decoupled from the UI repaint/redraw rate.

---

## 2. Technical Design & Architecture

### 2.1 Time-Paced Hardware Sampling (Rate-Throttled History Append)
- In `internal/usage/load.go`:
  - Enforce a minimum sampling duration between history appends (e.g. `minLoadSampleInterval = 1 * time.Second`).
  - When `CurrentCPULoad()`, `CurrentGPUs()`, or `CurrentSystemMemory()` is called:
    - If `time.Since(lastSampleTime) < minLoadSampleInterval`, return the current instantaneous load values
      and the existing `PercentHistory` snapshot without appending a new slice to `cpuHistory`.
    - If `time.Since(lastSampleTime) >= minLoadSampleInterval`, sample the `/proc` delta, append the new sample,
      and update `lastSampleTime`.
- This guarantees that rapid UI redraws (whether from 20 Hz mic audio, terminal resizes, or keypresses) only
  repaint the screen and never distort the temporal scale of hardware timelines.

### 2.2 Independent Load Snapshot Cache in `watch.go`
- Alternatively / complementarily, `RunWatchWithOptions` can maintain a cached `localLoadSnapshot` updated
  only on `loadTicker.C` (1 Hz), while high-frequency `draw()` calls during speech pass the cached load
  snapshot into `buildWatchFrameAt` without triggering redundant `/proc/stat` reads.

---

## 3. Scope of Implementation

1. **`internal/usage/load.go`**:
   - Throttle history appends (`cpuHistory`, `gpuHistory`, `memHistory`) to a fixed minimum interval (1s).
   - Ensure delta calculations against `/proc/stat` remain accurate when read between sample intervals.
2. **`internal/usage/load_test.go` & `watch_test.go`**:
   - Add unit tests verifying that invoking `CurrentCPULoad()` / `CurrentGPUs()` 50 times in rapid succession
     does not over-append to the history buffer and maintains 1s time-series pacing.

---

## 4. Acceptance Criteria

- [ ] High-FPS mic redraws (10–20 Hz) do not accelerate the scrolling speed of CPU, GPU, VRAM, or RAM sparklines/Braille charts.
- [ ] Hardware load timelines maintain an authentic 1-sample-per-second temporal resolution regardless of UI repaint rate.
- [ ] Redundant `/proc/stat` and sysfs polling during high-FPS frames is eliminated or throttled.
- [ ] All existing tests and new unit tests pass (`make check`).

---

## 5. Verification

- **Automated**: `go test -v ./internal/usage/...` testing rapid successive calls to `CurrentCPULoad()`.
- **Manual Verification**: Run `harnez usage --watch --mic`; speak continuously for 5–10 seconds; verify that the CPU/GPU sparklines advance at steady 1-second intervals while the live mic level bar bounces at 20 FPS.

