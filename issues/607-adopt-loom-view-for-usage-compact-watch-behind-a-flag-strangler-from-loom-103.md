# 607 — Adopt Loom view for usage --compact --watch behind a flag (strangler, from loom 103)

**Status**: Open
**Priority**: P2 (Medium)
**Severity**: Minor
**Category**: Feature
**Related**: loom issue 103 (`examples/usage` in ../loom), loom 385, 923, 917, Epic 921

---

/goal `harnez usage --watch` draws its boxes, titles, keys and overlays through loom widgets instead of its own
renderer, in stages, each stage at parity with the old view (per-cell `.ansi` comparison and a live tmux check)
before the old code for it is removed; or stop and report on a missing loom feature (file it in ../loom first).
Circuit breaker: on a loop or no progress, clear the goal and write a failure report here.

## State (host, 2026-10-25, owner: "file it")

- loom is used only by the `usage --loom` compact view: 136-line adapter `internal/usage/loom.go`.
- `--watch` is custom: ~3,650 non-test lines in `internal/usage/watch*.go`: own box/border/title drawing
  (`renderWBox`, `watch_layout.go:84`), colours (`ansiWrap`), width clipping (`truncateVisible`), key dispatch
  (`watch_keys.go`), Controls and diagnostics overlays (`watch_frame.go`).
- loom has the counterparts: `Box` (title style, borders), frame/grid layout, keymap/keycap/hint bar, popup.

## Stages (order by gain)

1. Box and title drawing through loom `Box` (needed by 923; loom 385 adds the hotkey-char style).
2. Keys and the Controls overlay through loom keymap/help.
3. Box grid layout through loom frame/grid.

Older notes: the loom 103 example lacked temperature, GPU/VRAM and Braille trend; check what loom has now.
Go before 917 (spec-driven boxes) and 923 in Epic 921.
