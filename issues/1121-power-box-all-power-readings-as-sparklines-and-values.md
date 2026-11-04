# 1121 — Power box: all power readings as sparklines and values

**Status**: Open
**Priority**: P3 (Low)
**Severity**: Minor
**Category**: Feature
**Related**: PR #103 (CPU/GPU power in the Load box), 088, 201

---

## Problem
PR #103 adds watts to the Load box as a `(NN°C/NNW)` suffix on the CPU and GPU lines. That shows the current value only, with no trend, and no total. The owner wants a separate "Power" box in `harnez usage --watch` that shows every power reading as a sparkline with its value.

## Goal
/goal Add a toggleable Power box to `harnez usage --watch`. It has one row per power source the host reports, each with a sparkline of recent watts and the current value, plus a total row when there are two or more sources. Hide the box when no source is readable. Stop and report if the key choice below is still open, or if a source cannot be told apart from another without double counting.

## Sources (as of PR #103)
- CPU: RAPL `package-*` zone `energy_uj` deltas, when readable (root-only on many hosts). `readRAPLPower` in `internal/usage/load.go`.
- APU package (amdgpu `power1_label` = `PPT`): CPU and iGPU together. Shown on the CPU line when RAPL is unreadable.
- dGPU/eGPU: amdgpu hwmon `power1_average`/`power1_input`; NVIDIA `power.draw` from `nvidia-smi`.
- Not yet read: battery `power_now` (laptops), power caps (`power1_cap`). Decide whether caps show as a limit marker.

## Design (owner, 2026-11-04)
Layout A: one row per source in the Load box style, plus a total row.
```
┌─ [W] Power ─────────────── peak 20s · boot ─┐
│ cpu+igpu (PPT)   [⣀⣀⣀⣠⣤⣀⣀⣀⣀⣀] 13W  22W   38W │
│ gpu (Navi 31)    [⣀⣀⣀⣀⣀⣀⣀⣠⣀⣀]  7W   9W   21W │
│ total            [⣀⣀⣀⣠⣤⣀⣀⣠⣀⣀] 20W  31W   52W │
└─────────────────────────────────────────────┘
```
- Columns: current watts, then the peak in the shown window, then the boot peak.
- **Chart scale = boot peak.** Each sparkline's top is the highest value harnez has measured for that source since boot, whatever the window length. The sensors keep no history, so the boot peak covers only the times harnez sampled. Keep it in a small state file keyed by `/proc/sys/kernel/random/boot_id`, so restarts keep it and a reboot resets it. Several watchers may write the file at once; write it atomically and take the max on merge.
- The header names both spans.
- The key is `[W]` unless the owner picks another.

## Notes and open points
- **Key clash:** `[P]` is already the Processes box (`internal/usage/watch_frame.go:445`). The owner must pick: move Processes, or give Power another key such as `[W]`.
- **No double counting in the total:** the APU PPT figure already includes the iGPU, and RAPL package power overlaps it.
- Reuse the history and sparkline helpers from the Load box (`sampleHistory`, `watchPercentSparkline`, `padHistory`). Watts need an absolute scale, not 0-100%. One option is to scale against the cap or the max seen so far (unverified).
- Remote snapshots (`[R]`) carry `PowerW` only from binaries that include PR #103.

## Done when
- A golden test shows the box with fake CPU, PPT and dGPU sources, with sparklines and values.
- The box is hidden when no power source is readable, and its key toggles it.
- The Load box suffixes stay as they are, unless the owner decides to drop them once the box exists.
