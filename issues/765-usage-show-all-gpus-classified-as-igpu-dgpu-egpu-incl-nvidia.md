# 765 — usage: show all GPUs classified as iGPU/dGPU/eGPU, incl. NVIDIA

**Status**: Open
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
