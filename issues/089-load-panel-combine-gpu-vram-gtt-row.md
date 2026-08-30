# 089 — Load Panel: Combine GPU VRAM/GTT Row

**Status**: Open
**Priority**: P2 (Medium)
**Severity**: Minor
**Category**: UX
**Related**: [[088-load-panel-ram-vram-gtt-memory]], [[078-rograph-library-shared-bar-sparkline-renderer]], [[110-remote-load-batch-vs-streaming-collection-modes]]

Priority bumped from P3 to P2: user intends to pick this up once the
in-progress remote GPU/CPU load-collection work (ticket 110) lands.

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

- Replace the current separate `gpu mem` and `vram/gtt` rows with one row in `internal/usage/watch.go`.
- Preserve the existing RAM row and GPU utilization row.
- Use existing graph helpers where practical; do not add a new rendering dependency.
- Keep the row within normal `[L] Load` panel width; narrow terminals may clip from the right, but
  the label and bars should remain visible.
- Update formatter/layout tests to assert the new one-line shape.
