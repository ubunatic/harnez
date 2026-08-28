# ADR: Kernel-Standard-Only Device Metrics Sourcing

**Date**: 2026-08-28
**Scope**: `internal/usage/load.go` (Load box CPU/GPU collectors); applies to any future device-metrics collector
**Status**: Accepted

---

## Decision

Device telemetry (CPU, GPU, and any future device class — disk, network, battery, other
accelerators) is read only through interfaces the Linux kernel itself publishes: procfs
(`/proc/stat`, `/proc/loadavg`) and sysfs (`/sys/class/hwmon`, `/sys/class/drm`,
`/sys/class/power_supply`, `/sys/class/net`, `/sys/block`, ...). A vendor's proprietary/closed
userspace tool or library (e.g. NVIDIA's NVML, wrapped by `nvidia-smi`) is not used at all —
not as a fallback, not as a degraded path. If a vendor hides a device's telemetry behind a
closed CLI/SDK with nothing exposed to the kernel, that device isn't supported. We don't call
their tool to get a lesser version of the data; we skip it.

We don't design around what a vendor is willing to show us. We design around what the kernel
guarantees.

## Why

- **Stability**: procfs/sysfs are versioned kernel ABIs, effectively stable for the process
  lifetime and across kernel versions in practice. A vendor CLI's output format, flags, and even
  presence are not a contract — they can change or vanish between driver releases.
- **Availability**: vendor tooling requires that vendor's driver stack, install, and often
  licensing. It's frequently absent by default (minimal installs, containers, CI images) and
  doesn't exist at all for vendors without such tooling. Kernel-exposed counters need nothing
  beyond the driver already required to use the hardware.
- **Cost**: a kernel counter read is a single `read()` syscall (~10-50µs for `/proc/stat`,
  ~1-2ms for the AMD sysfs GPU attributes). A subprocess-based vendor tool costs ~100ms+ per
  call (measured: `nvidia-smi` ~123ms, `rocm-smi` ~123ms) — two to four orders of magnitude
  more, which matters directly for a TUI redrawing at sub-second cadence.
- **Generality**: the same procfs/sysfs-first pattern extends to any device class the kernel
  already instruments — disk I/O (`/proc/diskstats`, `/sys/block/<dev>/stat`), network
  (`/sys/class/net/<iface>/statistics/*`), memory (`/proc/meminfo`), battery
  (`/sys/class/power_supply/*`). A vendor-hidden API doesn't extend to anything; it's a dead end
  specific to that one vendor and that one tool.

## Consequences

- **NVIDIA GPUs are not supported at all.** `nvidia-smi` calling was removed entirely
  (`readNvidiaSMI`, `nvidiaCodename`, `nvidiaSMICache`) rather than kept as a fallback — NVIDIA's
  proprietary driver publishes nothing to sysfs, so there is no kernel-standard path for it. The
  Load box shows `gpu n/a` on an NVIDIA-only system. This is a deliberate gap, not an oversight:
  if NVIDIA later exposes real kernel telemetry (or the open `nouveau`/`nova` driver's sysfs
  surface becomes adequate), that's the path back in — not calling their CLI.
- **AMD GPU is the (only) GPU implementation**: reads directly from
  `/sys/class/drm/cardN/device/*` (the `amdgpu` driver's sysfs attributes) — utilization, VRAM,
  temperature. `rocm-smi` (subprocess) remains as a fallback for AMD systems where those sysfs
  attributes aren't present — it wasn't in scope for this round's removal since it's used only
  when the kernel-standard path is unavailable, but it's the same category of vendor CLI and a
  candidate for the same treatment if that asymmetry turns out to matter.
- **CPU is fully standard** (`/proc/stat`, `/proc/loadavg`, `/sys/class/hwmon` for temperature)
  with no vendor-tool fallback at all — this is the shape every collector should aim for.
- **Future device classes** (disk, network, battery, other accelerators) should follow the same
  pattern: kernel-exposed counters only, and a device class with no kernel-exposed telemetry at
  all is out of scope until one exists, rather than reaching for a vendor SDK to fill the gap.

## Related

- `internal/usage/load.go`: `CurrentCPULoad`, `CurrentGPUs`, `readAMDSysfs`,
  `readCPUTempFromSysfs`, `gpuSubprocessCache`
- Commit `ae3a9f4` — `perf(usage): read AMD GPU stats from sysfs instead of rocm-smi`
