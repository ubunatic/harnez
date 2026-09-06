# 259 — Decouple Hardware Load Timeline Sampling from High-FPS TUI Redraw Cadence

**Status**: Closed — decoupled load timeline sampling from tui redraw rate
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

---

## 2. Core Invariant & Architectural Principles

> **General Principle: The TUI refresh is the most expensive operation in the system. No measurement or recording process should ever trigger a UI repaint beyond the configured target FPS, and no background sampling rate should be coupled to the display frame rate.**

### 2.1 The Two-Domain Decoupling
1. **Measurement / Sampling Domain**:
   - Background producers (audio PCM streams, `/proc/stat` samplers, network telemetry, daemon updates) run at their own intrinsic domain frequencies (e.g. 20–50 Hz for audio chunks, 1 Hz for CPU/GPU load).
   - They write data purely into in-memory buffers or shared-state structs (e.g. `micLiveMeter`, `cpuHistory`).
   - Time-series histories advance according to **wall-clock elapsed time**, never according to screen repaints.

2. **Display / Rendering Domain**:
   - TUI redraw (`draw()`) is the CPU and I/O bottleneck (ANSI string generation, box layout, cursor repositioning, TTY writes).
   - Redraw requests from background measurement hooks must be **coalesced and rate-limited** by a strict token bucket / debounce gate that enforces a hard ceiling:
     $$\text{MinFrameInterval} = \max(\text{high-fps-delay-ms},\; 50\text{ms}) \implies \text{MaxFPS} \le 20$$
   - A rapid burst of 100 audio samples will update the in-memory rolling meter 100 times, but the UI will redraw at most once per 50ms, rendering the latest state.

---

## 3. Implementation Plan

1. **Decouple Load Sampling in `internal/usage/load.go`**:
   - Rate-throttle `cpuHistory`, `gpuHistory`, `memHistory` appends to a minimum 1-second interval (`minLoadSampleInterval = 1 * time.Second`).
   - Multiple `CurrentCPULoad()` calls within that 1-second window return the current values and snapshot without advancing the historical time-series.
2. **Debounce & Coalesce Redraws in `internal/usage/watch.go`**:
   - Enforce a minimum interval between consecutive `draw()` executions (`high-fps-delay-ms`, default 50ms) regardless of how frequently background goroutines signal `requestRedraw()`.
   - Prevent any single producer from flooding the render loop.
3. **Unit Testing**:
   - Verify that 100 rapid `CurrentCPULoad()` calls append exactly 1 sample to `cpuHistory`.
   - Verify that 100 rapid `requestRedraw()` calls trigger at most 1 draw per minimum frame interval.

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

