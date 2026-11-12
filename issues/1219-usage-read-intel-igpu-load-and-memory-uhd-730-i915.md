# 1219 — usage: read Intel iGPU load and memory (UHD 730, i915)

**Status**: Open
**Priority**: P2 (Medium)
**Severity**: Moderate
**Category**: Feature
**Related**: 765 (all-GPU listing, AMD and NVIDIA only), 088 (RAM/VRAM/GTT load panel), 090 (remote load panel), docs/studies/2026-08-28-load-box-cpu-gpu-kernel-metrics.md (line 85: Intel GPU support does not exist)

---

## 1. Problem & Motivation

`harnez usage --minimal` shows `gpu n/a` on this machine. The host has an Intel iGPU that harnez cannot read:

- `lspci`: `00:02.0 VGA compatible controller: Intel Alder Lake-S GT1 [UHD Graphics 730] [8086:4692]`, `boot_vga` set.
- Bound driver: `i915` (`/sys/bus/pci/devices/0000:00:02.0/driver`).
- DRM nodes: `/sys/class/drm/card1`, `renderD128`.

The GPU readers in `internal/usage` cover AMD (`amdgpu` `gpu_metrics`, `gpu_busy_percent`) and NVIDIA (`nvidia-smi`, `gpu_nvidia.go`). Nothing matches vendor `0x8086`, so the load box renders the `gpu n/a` placeholder (`watch_load_box.go`, around line 109).

## 2. Technical Specification / Findings

- Goal scope is the local iGPU class first. Same-class Intel iGPUs (UHD/Iris Xe on `i915` or `xe`) should work by design, but only this machine is verified.
- The memory row differs from AMD. On an iGPU the memory is system RAM, so the VRAM/GTT split of 765 may not apply. Decide what the row shows before implementing.
- Busy source (probed on this host, kernel 7.3.3): the i915 perf PMU exposes `rcs0-busy`, but `perf_event_open` for it returns EACCES for an unprivileged user (`perf_event_paranoid` 2, no CAP_PERFMON). Rejected for now. `gt/gt0/rc6_residency_ms` is world-readable, so the reader uses busy = 100 × (1 − ΔRC6 / Δwall) per GPU. This is a proxy for "GT awake", not an engine counter. DRM `fdinfo` (`drm-engine-render`) only shows this user's own clients, so it is partial and not used.
- Class: `classifyGPU` takes the PCI address instead of `boot_vga`. The Intel iGPU is the display function at `0000:00:02.0`, and other Intel display functions are dGPUs. Verified on this host only.
- Name: lspci's bracketed name, shortened: `UHD 730`.
- Still open: the memory row. The reader reports no VRAM/GTT, so the box draws no memory row for the iGPU. Decide what, if anything, it shows (system RAM used by GPU clients, or nothing).
- Seed cost: the first reading for each GPU sleeps 0.5 s (`intelSeedWindow`), so the first dashboard frame and one-shot `harnez usage` take about 0.5 s longer on this host. A shorter window would be noisier, and a 1 s window broke the watch startup test.

## 3. Implementation & Verification Plan

/goal Show this machine's Intel iGPU in `harnez usage --minimal` and the load box (name, load, memory row), with a vendor-aware reader for `0x8086` in `internal/usage`, covered by unit tests on fake sysfs input. Verify live on this host. Stop and report when a user decision is needed (memory row semantics) or a permission is denied, or when no reliable busy source exists on this kernel.

- Probe the live host first (driver, PMU and fdinfo sources, memory numbers) and record the findings in section 2.
- Add an Intel reader next to `gpu_nvidia.go`. Keep values in `spec/` if they need configuration, per the Spec rule.
- Tests: fake sysfs fixtures for an `i915` iGPU, including a case where the busy source is missing (renders `n/a`, no crash).
- Live check: `harnez usage --minimal` on this host shows the iGPU instead of `gpu n/a`.
