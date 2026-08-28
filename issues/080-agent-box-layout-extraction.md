# 080 — Extract Agent-Box Content Layout (Label / Graph / Trailing Info Sizing) Into a Small Layout Library

**Status**: Open
**Priority**: P3 (Low)
**Severity**: Minor
**Category**: Refactor
**Related**: [[078-rograph-library-shared-bar-sparkline-renderer]], [[079-consistent-10-char-shrink-to-fit-graphs]], `internal/usage/watch.go`

---

## 1. Problem

Optional follow-up once [[078-rograph-library-shared-bar-sparkline-renderer]] and
[[079-consistent-10-char-shrink-to-fit-graphs]] land. Not required to fix the width
bug, but named by the user as the natural next layer of cleanup.

`buildAgentBox` (`watch.go:656-...`, notably the row-layout block around
`watch.go:695-733`) currently mixes three concerns in one function:

1. Fixed-width label formatting (`label(16)`, truncate-with-ellipsis at 16 runes —
   same pattern as `padLoadLabel`/`loadLabelWidth` in the Load box, duplicated rather
   than shared).
2. Dynamically-sized graph width computation (`contentW - 26 - visLen(resetStr)`,
   clamped) — the actual bar/sparkline call.
3. Dynamically-sized trailing info (`resetStr`, the `%4.1f%%` percent field) that
   competes with the graph for the same remaining space, with an explicit fallback
   (drop `resetStr` first, then shrink the bar) when both don't fit.

This "fixed label + shrinkable graph + shrinkable trailing info, packed into
`contentW`" pattern is generic row-layout, not usage-specific, and currently only
exists inline in `buildAgentBox`. The Load box (`formatCPULine`/`formatGPULine`) has
its own separate fixed-label logic (`padLoadLabel`) that could use the same
primitive.

## 2. Goal

Extract a small layout helper (e.g. `internal/rograph/row.go` or a sibling package)
that, given `(totalWidth, label, graphMaxWidth, trailingParts...)`, computes how much
space the graph actually gets and which trailing parts (if any) get dropped, using a
defined priority order (today: drop `resetStr` before shrinking the bar below its
target). `buildAgentBox` and `formatCPULine`/`formatGPULine` both consume it instead
of hand-computing offsets (`26`, `visLen`, etc.) inline.

## 3. Scope

- Read-only refactor: behavior must not change from what [[079]] establishes.
- Consolidate `padLoadLabel`/`loadLabelWidth` and the agent-box label truncation
  (currently two near-identical 16-char-truncate-with-ellipsis implementations) into
  one shared label-formatting helper.
- Keep `usage.go`'s buildAgentBox and the Load box functions thin: fetch data, call
  the layout helper, call `rograph` to draw.

## 4. Non-Goals

- No change to what information is shown or in what order — purely mechanical
  extraction.
- Not blocking on wider box-layout concerns beyond the agent/load boxes (e.g. the
  History box, `buildHistoryBox`, is out of scope unless it turns out to share the
  same pattern trivially).
