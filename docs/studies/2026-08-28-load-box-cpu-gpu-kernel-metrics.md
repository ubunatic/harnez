# Case Study: The Load Box — Kernel-Sourced CPU/GPU Metrics in the Watch TUI

**Date**: 2026-08-28
**Scope**: `internal/usage/load.go`, `internal/usage/watch.go`
**Author**: Claude (Pair Programming with User)
**Related**: [2026-08-28-kernel-standard-metrics-sourcing-policy.md](2026-08-28-kernel-standard-metrics-sourcing-policy.md) (the ADR this implementation follows)

---

## 1. What Got Built

`harnez usage --watch` gained a `[L] Load` panel (commits `90cea91`..`64c3e07`) showing live CPU
and GPU utilization, temperature, and a rolling timeline sparkline — styled like the existing
agent quota lines (`cpu (12 cores)   [spark] 5% (66°C)`). The whole arc, in order:

1. Static load-average box → real-time delta-based CPU% → per-core sparkline → GPU row (AMD
   sysfs + NVIDIA subprocess) → redraw-cadence tuning (100ms → too noisy → 1s) → restyle to match
   the rest of the TUI (fixed-width labels, timeline instead of per-core snapshot) → visual
   polish (background-shaded sparkline) → burst-seeded history at startup → **removal of all
   vendor-CLI GPU fallbacks**, leaving AMD sysfs as the only GPU source.

The end state is a small, dependency-free metrics layer that reads exclusively from Linux
procfs/sysfs, with no subprocess in the steady-state redraw path at all.

## 2. The Kernel Data Sources, and What Each One Actually Requires

| Metric | Source | Notes |
|---|---|---|
| Load averages (1/5/15m) | `/proc/loadavg` | Universal; `uptime` subprocess only as non-Linux fallback |
| Aggregate + per-core CPU % | `/proc/stat`, `cpu`/`cpuN` lines | Delta of two jiffy-counter samples; idle = idle+iowait fields |
| CPU temperature | `/sys/class/hwmon/hwmon*/name` + `temp*_input` | Must match against a *driver name* allowlist (`k10temp`, `coretemp`, `zenpower`, `cpu_thermal`) — hwmon is a generic bus, not CPU-specific (this machine also has `thinkpad`, `BAT0`, `AC`, `mt7921_phy0`, `nvme`, `acpitz` hwmons). Prefer a sensor labeled `Tctl`/`Tdie`/`"Package id 0"`/`CPU` over an arbitrary `temp1_input` when the driver exposes several. |
| AMD GPU utilization | `/sys/class/drm/card*/device/gpu_busy_percent` | `amdgpu`-driver-specific sysfs extension, not a generic DRM contract |
| AMD GPU VRAM | `mem_info_vram_used` / `mem_info_vram_total` (same dir) | Bytes; percent computed locally |
| AMD GPU temperature | `.../device/hwmon/hwmon*/temp1_input` | Card-scoped hwmon, separate glob from the CPU one above |
| AMD GPU codename | `lspci -d 1002: -mm` (cached once) | Not telemetry — a cosmetic label. `lspci` is a generic pciutils tool reading the kernel's public PCI ID database, not a vendor-gatekept API, so it wasn't in scope for the "no vendor tools" removal |
| NVIDIA GPU (any metric) | **none available** | NVIDIA's proprietary driver exposes nothing through procfs/sysfs; the only access path is `nvidia-smi`/NVML, a closed vendor tool. Per the ADR, this project doesn't call it — NVIDIA GPUs simply aren't reported |

Card and hwmon *numbering* (`card0` vs `card1`, `hwmon4` vs `hwmon6`) is driver-assigned at
runtime and not stable across reboots or hardware changes, which is why every sysfs path in this
code is resolved via `filepath.Glob`, never a hardcoded index.

## 3. Reusable Patterns Worth Knowing About

- **Single-read, multi-value parsing**: `/proc/stat` contains both the aggregate `cpu` line and
  every `cpuN` line in one file. Reading it once per frame (`cpuStatFrame`) and slicing out both
  the aggregate and per-core deltas from that one read avoids doubling I/O and — more
  importantly — keeps aggregate and per-core percentages computed from *exactly* the same two
  points in time, so they can't drift apart from being sampled a few microseconds apart.
- **Cold-start burst-seeding**: a kernel counter has no history — `/proc/stat` and the amdgpu
  sysfs attributes are point-in-time snapshots, so there's no way to "read the past" for a
  freshly-started process. The fix used here: on first read, take `loadHistoryLen` (10) samples
  `historyBurstInterval` (100ms) apart — about 1s of real wall-clock, once — instead of growing
  the chart one glyph per redraw over the first ~10 real seconds. Real data, compressed in time,
  not a faked flat line.
- **Decoupled redraw cadence**: the Load panel's numbers are free (no network), but the rest of
  the watch frame is paced by `interval` (min 30s) specifically to avoid hammering live quota
  APIs. Rather than making the whole frame redraw faster, a second `time.Ticker` calls the
  existing cheap `draw()` (which reads cached summary data, no network) on its own cadence. This
  is the general shape for "one part of a UI needs to update faster than the rest": a second
  ticker into the existing redraw path, not a faster global tick.
- **Subprocess throttling, and its retirement**: while `nvidia-smi`/`rocm-smi` were still in the
  picture, `gpuSubprocessCache` memoized their last result for 1s so a faster local redraw tick
  wouldn't re-exec a ~100-200ms subprocess every frame. Once both subprocess readers were
  removed (see the ADR), that whole cache/throttle apparatus became dead weight and was deleted
  in the same pass — worth remembering to sweep for orphaned scaffolding after removing the
  thing it existed to support.
- **Absolute vs. relative sparkline scale**: the existing `RenderSparklineWidth` in `util.go`
  normalizes to the series' own min/max, which is right for token-rate sparklines (where "any
  activity" is the interesting signal) but wrong for a 0-100% utilization value — a flat 5%
  history would render as a maxed-out graph. `percentSparkline` in `watch.go` is a separate,
  deliberately absolute-scale renderer for exactly this reason; don't reach for
  `RenderSparklineWidth` for anything that's already a percentage.
- **Fixed-width label columns**: `padLoadLabel` truncates-then-pads every "cpu (...)"/"gpu (...)"
  label to a constant width (16, matching the existing `%-16s` convention in `buildAgentBox`'s
  quota lines) so every panel's `[spark]` bracket starts in the same column regardless of core
  count or GPU name length. This is the same trick used throughout `watch.go` for column
  alignment — check `visLen`/`truncateVisible` (which already strip ANSI escapes for width
  measurement) before introducing a new styled segment, since they make it safe to add color
  codes without breaking alignment math.

## 4. Honest Gaps

- Per-core CPU history isn't tracked, only spatial per-core + aggregate temporal — good enough
  for this panel's purpose but not a general-purpose per-core profiler.
- Intel GPU support doesn't exist. The `i915`/`xe` sysfs surface differs from `amdgpu`'s; adding
  it would need its own `readIntelSysfs`-shaped function, not a generalization of the AMD one.
- No test coverage was added for `load.go`'s sysfs readers (they weren't table-driven testable
  without a fake sysfs tree, and the session prioritized live verification via `harnez
  usage --summary`/`--watch` runs instead — see the companion feedback doc for why that trade-off
  was made deliberately here rather than skipped).
