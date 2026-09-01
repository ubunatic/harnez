# 089 — Load Panel: Combine GPU VRAM/GTT Row

**Status**: Closed — resolved in 6be83d2
**Priority**: P2 (Medium)
**Severity**: Minor
**Category**: UX
**Related**: [[088-load-panel-ram-vram-gtt-memory]], [[078-rograph-library-shared-bar-sparkline-renderer]], [[110-remote-load-batch-vs-streaming-collection-modes]]

Priority bumped from P3 to P2: user intends to pick this up once the
in-progress remote GPU/CPU load-collection work (ticket 110) lands.

**Unblocked (2026-09-01)**: both prerequisites are Closed —
[[078-rograph-library-shared-bar-sparkline-renderer]] (the `internal/rograph`
lib) and [[110-remote-load-batch-vs-streaming-collection-modes]]. Ready to
implement.

## Problem

Issue [[088-load-panel-ram-vram-gtt-memory]] added GPU memory telemetry to the `[L] Load` panel,
but the current two-row rendering is more verbose than necessary. The aggregate GPU memory row and
the `vram/gtt` split can be compressed into one scannable line.

## Desired Behavior

Render GPU memory as a single row shaped like:

```text
gpu vram/gtt  [██░░][█░░░] 9.7/19.6G 63%
```

The row should communicate:

- label: `gpu vram/gtt`
- compact VRAM bar and compact GTT bar, adjacent
- combined used/total memory as VRAM + GTT
- combined percentage as VRAM+GTT used divided by VRAM+GTT total

## Acceptance Criteria

- Replace the current separate `gpu mem` and `vram/gtt` rows (`formatGPUMemoryLines`,
  `internal/usage/watch.go:933-948` — currently plain text, no bars at all) with one row.
- Preserve the existing RAM row and GPU utilization row.
- Use existing `internal/rograph` helpers (`rograph.RenderBar`, `rograph.PadLabel`) — the
  adjacent dual-bar layout already used for the agent quota row
  (`formatAllUsageTableLine`, `watch.go:706-763`, two `RenderBar` calls side by side with a
  midblock percent/label between them) is the direct reference pattern to adapt here. Do not
  add a new rendering dependency or hand-roll a second bar renderer.
- Keep the row within normal `[L] Load` panel width; narrow terminals may clip from the right, but
  the label and bars should remain visible.
- Update formatter/layout tests to assert the new one-line shape.
