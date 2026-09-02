# 191 — Load Box RAM Chart, Hardware Module Label, and Chart Mode Spec

**Status**: Closed
**Priority**: P2 (Medium)
**Severity**: Minor
**Category**: TUI / Hardware Monitoring
**Related**: [[089-load-panel-combine-gpu-vram-gtt-row]], [[090-load-panel-ram-vram-gtt-memory]], `internal/usage/watch.go`, `internal/usage/load.go`, `internal/usage/indicatorsspec.go`

---

## 1. Problem & Motivation

In `harnez usage --watch`, the Load box currently formats system RAM as plain text without an aligned chart:
```text
cpu (16 cores)   [▁▂▄▅▆▇█] 12% (48°C)
ram              20.1/45.1G 44%
gpu (RX 7900)    [▁▁▂▄█] 8% (42°C)
gpu vram/gtt     [████░░░░] 3.2/16.0G 20%
```

This causes two inconsistencies:
1. **Lack of Chart & Misaligned Columns**: RAM has no visual chart (`[<chart>]`), leaving an empty gap where the CPU/GPU sparkline/bar sits.
2. **Missing Hardware Module Details in Label**: CPU and GPU labels include hardware context (`cpu (16 cores)`, `gpu (RX 7900)`), while RAM is a bare unpadded `"ram"`. It should display module/capacity geometry when discoverable (e.g. `ram (2x16G)` or `ram (45G)`).
3. **Inconsistent Chart Modes (Bar vs. Time-Series Sparkline)**: Currently, CPU and GPU utilization use rolling historical sparklines (`[▁▂▄▅█]`), while VRAM/GTT uses instantaneous progress bars (`[████░░░░]`), with no explicit configuration or specification defining which metrics use bars vs. rolling sparklines.

---

## 2. Technical Specification

### 1. RAM Hardware Label & Capacity Formatting
- In `internal/usage/load.go`:
  - Enhance `SystemMemory` (or hardware discovery) to detect DIMM geometry (e.g., via `/sys/devices/system/edac/mc/` or `/sys/class/dmi/id/` or fallback to rounded total GiB like `(45G)`).
  - Format label as `padLoadLabel("ram (" + geom + ")")`, e.g., `ram (2x16G)` or `ram (48G)`.

### 2. RAM Visual Chart Alignment
- Format the RAM line with full column parity against CPU/GPU:
  ```text
  ram (2x16G)      [████░░░░░░] 20.1/45.1G 44%
  ```
  or with time-series sparkline:
  ```text
  ram (2x16G)      [▁▂▃▄▄▅▅▆▆█] 20.1/45.1G 44%
  ```

### 3. Unified Chart Mode Specification (Bar vs. Time Series)
- Define a clear, configurable policy in `spec/indicators.yaml` (or CLI/config options) governing chart presentation for each hardware resource:
  * **Time-Series Sparkline Mode** (`sparkline` / `timeseries`): Shows recent rolling utilization history (e.g., last ~10 samples of % usage).
  * **Instantaneous Bar Mode** (`bar` / `gauge`): Shows current capacity fill fraction (`[████░░░░░░]`).
- Allow user preference / toggle between rolling sparklines and current capacity bars across CPU, GPU, RAM, and VRAM+GTT.

---

## 3. Verification Plan

1. Unit tests in `internal/usage/watch_test.go` and `internal/usage/load_test.go` validating:
   - `formatSystemMemoryLine` produces aligned `ram (geometry) [<chart>] X/YG Z%` output matching `loadLabelWidth` (16 cols).
   - Both bar mode and sparkline mode render valid visual charts without breaking row width bounds.
2. Verify visual alignment across all terminal sizes with `TestBuildWatchFrameRowsFitWidth`.
3. Test in `harnez usage --watch` on local workstation.
