# 765 — usage: show all GPUs classified as iGPU/dGPU/eGPU, incl. NVIDIA

**Status**: Closed — All GPUs listed per host: AMD via sysfs, NVIDIA via nvidia-smi (gated, 2s cache, 1s timeout, spec keys); iGPU keeps vram/gtt, dGPU/eGPU show vram; class internal only. Verified live: local Cezanne vram/gtt; x600 Phoenix1 vram/gtt + RTX 3060 vram, matching docs/data/harnez-usage-all-gpus.ansi. eGPU rule unverified on hardware (renders like dGPU).
**Priority**: P2 (Medium)
**Severity**: Moderate
**Category**: Feature
**Related**: 088 (RAM/VRAM/GTT load panel), 090 (remote load panel), 198 (VRAM/GTT split), internal/usage/load.go

---

## Problem & Motivation

The user's x600 (AMD APU) now has an NVIDIA RTX eGPU attached. `harnez usage --minimal` still shows
only one GPU per host, local and remote. GPU collection (`currentGPUs` in `internal/usage/load.go`)
reads only AMD sysfs (`/sys/class/drm/card*/device/gpu_busy_percent`); nvidia-smi was removed
earlier (see comment near `load.go:617`), so the NVIDIA card is not detected at all.

## Goal

/goal `harnez usage --minimal` (and the load box) lists every GPU of a host, local and remote, each
labelled iGPU, dGPU or eGPU; iGPUs keep the VRAM+GTT memory view, dGPUs and eGPUs show VRAM only;
an NVIDIA eGPU on the x600 is shown with load and VRAM; covered by unit tests with fake sysfs/probe
input; or stop and report when blocked on a user decision (e.g. NVIDIA data source: nvidia-smi
subprocess vs. NVML vs. sysfs/hwmon, given the earlier removal of nvidia-smi) or a denied permission.

**Decision (user, 2026-10-10):** read NVIDIA GPUs via `nvidia-smi`. Read the comment near
`load.go:617` first to learn why it was removed, and avoid that problem (e.g. cache, low poll rate).
This unbans `nvidia-smi`: if an NVIDIA card is present, harnez uses it. Update the ban in
`docs/studies/2026-08-28-kernel-standard-metrics-sourcing-policy.md` (superseded for NVIDIA by 765),
`docs/README.md` and the code comment, so no doc or comment still forbids it.

## Target look (user mockups)

- `docs/data/harnez-usage-igpu-only.ansi`: today's view, one iGPU with `gpu (<name>)` load row and
  `gpu vram/gtt` split bar.
- `docs/data/harnez-usage-all-gpus.ansi`: x600 with iGPU (Phoenix1, `vram/gtt` row) plus RTX 3060
  eGPU: an extra `gpu (RTX 3060)` load+temp row and a `gpu vram` row (single bar, no GTT).
- **Decision (user, 2026-10-10):** no iGPU/dGPU/eGPU label anywhere in the UI. The class is
  internal only and decides which memory row is shown. Each card shows only its short name
  (e.g. `Phoenix1`, `RTX 3060`).

## Notes

- Check live code first; the GPU code may have changed since filing.
- Classification idea (verify on hardware): iGPU = AMD APU / `boot_vga` integrated / small VRAM with
  GTT; eGPU = PCI path behind a Thunderbolt/USB4 bridge (`/sys/bus/thunderbolt` or `removable`
  attribute); else dGPU.
- Remote hosts get their GPU data via the same collector; check 090 for how remote metrics flow.

## Progress

- **M1 (AMD multi-GPU, classification, iGPU/dGPU rows):** implemented in `internal/usage/gpu_class.go`
  and `load.go`/`watch.go`. Codename cache is per PCI address; histories are keyed by PCI address.
- **UNVERIFIED ON HARDWARE:** the eGPU rule (PCI `removable` attribute on the GPU or a bridge above it)
  and the iGPU VRAM carve-out threshold (`apuVRAMCarveoutMaxMiB` = 2048 MiB). Both were written without
  a live eGPU or APU to check against. The eGPU is on remote host x600; the host checks live output there.
- The threshold is a Go constant in M1; it moves to `spec/usage.yaml` in M2 per the Spec rule.
- **M2 (nvidia-smi):** `internal/usage/gpu_nvidia.go`; gated on NVIDIA PCI hardware, one call per
  `nvidia_smi_cache_ttl` (2s), `nvidia_smi_timeout` (1s), failures cached. Local machine has no NVIDIA
  card, so the live NVIDIA row is UNVERIFIED here; the host checks it on the x600 (RTX 3060).
- The x600 runs an older binary: upgrade it before its Load box shows the RTX row.
- This ticket stays open until the host closes it after review.
- **M1 delivered (5741bbf7, AMD multi-GPU):** tests green, but host live check of
  `harnez usage --minimal` found two regressions (below).

### M2 Pre-Work / Required Refinements (do first)

1. **iGPU misclassified.** Local Cezanne APU reports `gpu vram 3.3/8.0G`: its BIOS carve-out is
   8 GiB, so the 2048 MiB threshold calls it a dGPU and drops the vram/gtt row. A VRAM-size
   threshold is the wrong signal; drop it. Find a robust sysfs signal for "integrated" (e.g. check
   what the local card exposes: PCIe link attrs, `boot_vga`, bus position, amdgpu attributes) and
   verify it on this machine (Cezanne must be iGPU). Phoenix1 on x600 must also be iGPU.
2. **Old remote fallback.** The x600 runs an older binary, so its Phoenix1 has no class and shows
   `gpu vram 0.5/0.5G 95%` instead of vram/gtt. Make the zero value `GPUClassUnknown`, which keeps
   the pre-765 rendering (vram/gtt row whenever GTT data is present).
3. Add regression tests for both, then rerun the live check yourself and paste the Load box rows.
