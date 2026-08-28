# ADR: Kernel-Standard-Only Device Metrics Sourcing

**Date**: 2026-08-28
**Scope**: `internal/usage/load.go` (Load box CPU/GPU collectors); applies to any future device-metrics collector
**Status**: Accepted

---

## Decision

Device telemetry (CPU, GPU, and any future device class — disk, network, battery, other
accelerators) is read only through interfaces the Linux kernel itself publishes: procfs
(`/proc/stat`, `/proc/loadavg`) and sysfs (`/sys/class/hwmon`, `/sys/class/drm`,
`/sys/class/power_supply`, `/sys/class/net`, `/sys/block`, ...). No vendor CLI or SDK is called
for telemetry — not `nvidia-smi`, not `rocm-smi`, not any future equivalent — regardless of
whether it's proprietary or open-source. If a device's utilization/memory/temperature isn't
exposed through procfs/sysfs, that device's telemetry isn't collected. We don't call a tool to
get a lesser version of the data; we skip it. The fewer external tools this project shells out
to, the fewer things can silently change format, go missing, or add latency underneath it.

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

- **No subprocess-based GPU reader remains at all.** Both `nvidia-smi` calling
  (`readNvidiaSMI`, `nvidiaCodename`, `nvidiaSMICache`) and the `rocm-smi` fallback
  (`readROCmSMI`, `rocmSMICache`) were removed, along with the throttle/cache machinery
  (`gpuSubprocessCache`, `gpuSubprocessThrottle`) that existed solely to make those subprocess
  calls tolerable at redraw speed — once there's nothing left calling a subprocess, that
  machinery is dead weight too. NVIDIA GPUs get no telemetry at all (`gpu n/a`); an AMD GPU
  whose driver doesn't populate the expected sysfs attributes also gets none, rather than
  falling back to `rocm-smi` for a degraded read. This was a two-step removal in the same
  session: NVIDIA first (explicitly named), then `rocm-smi` once it was clear the policy should
  apply uniformly rather than carve out an exception for AMD's own tool.
- **AMD GPU is the only GPU path, and it's sysfs-only**: reads directly from
  `/sys/class/drm/cardN/device/*` (the `amdgpu` driver's sysfs attributes) — utilization, VRAM,
  temperature. If that path fails, the GPU is simply not reported.
- **CPU is fully standard** (`/proc/stat`, `/proc/loadavg`, `/sys/class/hwmon` for temperature)
  with no vendor-tool fallback at all — this was already the shape every collector should aim
  for, and is now the shape GPU collection has too.
- `lspci` (used only for the AMD GPU's codename label, e.g. "Cezanne") was kept: it's not a
  vendor-specific gatekept API — it's a generic, open `pciutils` tool reading the kernel's own
  PCI ID data, works for any PCI vendor, and isn't in the telemetry path at all (cached once per
  process, used for a cosmetic label). Flagged here in case that distinction doesn't hold up on
  reflection and it should go too.
- **Future device classes** (disk, network, battery, other accelerators) should follow the same
  pattern: kernel-exposed counters only, no subprocess fallback of any kind, and a device class
  with no kernel-exposed telemetry at all is out of scope until one exists.

## Related

- `internal/usage/load.go`: `CurrentCPULoad`, `CurrentGPUs`, `readAMDSysfs`,
  `readCPUTempFromSysfs`
- Commit `ae3a9f4` — `perf(usage): read AMD GPU stats from sysfs instead of rocm-smi`
- Commit `8b25059` — `docs+refactor(usage): kernel-standard-only metrics ADR, drop nvidia-smi`
