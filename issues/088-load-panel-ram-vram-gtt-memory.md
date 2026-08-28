# 088 — Load Panel: RAM, VRAM, and GTT Memory

**Status**: Open
**Priority**: P2 (Medium)
**Severity**: Moderate
**Category**: Feature
**Related**: [[023-usage-command-token-quota-tracking]], [[078-rograph-library-shared-bar-sparkline-renderer]], [docs/studies/2026-08-28-kernel-standard-metrics-sourcing-policy.md](../docs/studies/2026-08-28-kernel-standard-metrics-sourcing-policy.md), [docs/studies/2026-08-28-load-box-cpu-gpu-kernel-metrics.md](../docs/studies/2026-08-28-load-box-cpu-gpu-kernel-metrics.md)

## Problem

The `[L] Load` panel shows CPU utilization and AMD GPU utilization/temperature, but it does not
show system memory pressure or total GPU-addressable memory. For local LLM and graphics workloads,
the useful question is often not just "how busy is the GPU?" but "how much memory is available or
already occupied across normal VRAM and GTT?"

This must stay cheap. The Load panel redraws once per second, so memory telemetry must follow the
kernel-standard-only policy: read procfs/sysfs files directly, never call vendor tools such as
`rocm-smi` or `nvidia-smi` from the telemetry path.

## Desired Behavior

- Add system memory usage to the `[L] Load` panel from `/proc/meminfo`.
- Extend AMD GPU memory reporting to include GTT alongside VRAM:
  - `mem_info_vram_used`
  - `mem_info_vram_total`
  - `mem_info_gtt_used`
  - `mem_info_gtt_total`
- Render total GPU memory as VRAM + GTT, with enough detail that VRAM and GTT are distinguishable.
  A compact form such as `gpu mem 6.0/20.0G (vram 5.5/8.0, gtt 0.5/11.6)` is acceptable if it fits
  the existing Load panel width rules.
- Keep NVIDIA and other non-kernel-exposed GPU telemetry as `gpu n/a`; do not add subprocess,
  SDK, or vendor CLI fallbacks.
- Preserve the existing CPU and GPU utilization sparkline behavior.

## Acceptance Criteria

- `internal/usage/load.go` reads RAM, VRAM, and GTT memory via procfs/sysfs only.
- `internal/usage/watch.go` renders the memory data in the `[L] Load` panel without breaking
  narrow terminal layout constraints.
- Missing GTT or VRAM files degrade gracefully instead of reporting misleading zeros.
- Tests cover parsing `/proc/meminfo`, AMD VRAM+GTT aggregation, formatter output, and box-width
  clipping behavior.
- `make test` and `make install` pass after Go changes.

## Notes

On the reference AMD host, amdgpu exposes the required GTT files under:

```text
/sys/class/drm/card1/device/mem_info_gtt_total
/sys/class/drm/card1/device/mem_info_gtt_used
```

Measured command-line probes that use `cat` are dominated by subprocess overhead and should not be
used as implementation guidance. The implementation should use direct Go file reads (`os.ReadFile`)
like the existing Load collector.
