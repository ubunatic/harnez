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

## Notes

- Check live code first; the GPU code may have changed since filing.
- Classification idea (verify on hardware): iGPU = AMD APU / `boot_vga` integrated / small VRAM with
  GTT; eGPU = PCI path behind a Thunderbolt/USB4 bridge (`/sys/bus/thunderbolt` or `removable`
  attribute); else dGPU.
- Remote hosts get their GPU data via the same collector; check 090 for how remote metrics flow.
