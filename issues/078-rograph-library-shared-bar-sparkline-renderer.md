# 078 — `rograph`: Shared Read-Only Graph Library for Usage Bars & Sparklines

**Status**: Closed — resolved in aa06ad7
**Priority**: P2 (Medium)
**Severity**: Moderate
**Category**: Refactor
**Related**: [[079-consistent-10-char-shrink-to-fit-graphs]], [[080-agent-box-layout-extraction]], `internal/usage/util.go`, `internal/usage/watch.go`

---

## 1. Problem & Motivation

`internal/usage` currently has three separate, unrelated single-row graph renderers:

- `RenderProgressBar(usedPercent float64, width int)` (`util.go:52-71`) — filled/empty
  block bar (`█`/`░`) in brackets, explicit `width` param, no shrink logic of its own
  (callers clamp).
- `percentSparkline(pcts []float64)` (`watch.go:476-489`) — absolute 0-100% scaled
  sparkline (`▁▂▃▄▅▆▇█`), grey background, width is implicit (one glyph per input
  value — no width param at all).
- `RenderSparklineWidth`/`RenderSparklineInt64Width`/`RenderSparkline` (`util.go:159-231`)
  — relative (per-series min/max) scaled sparkline family, used for token-history.

The bar and the load sparkline are visually and mechanically almost the same kind of
thing (a single-row, fixed-max-width, shrink-to-fit read-only graph), but are
implemented, parameterized, and clamped independently. `usage.go` hardcodes bar width
`24` in three places; `watch.go` computes a dynamic `barW` clamped to `[1, 12]`; the
load sparkline has no width control at all — its "width" is just how much sample
history `loadHistoryLen` happens to keep.

## 2. Goal

Extract a tiny, dedicated library — `rograph` (read-only graph) — that knows how to
draw exactly two kinds of single-row, fixed-max-width terminal graphs:

1. **Usage bar** — filled/empty block bar (today's `RenderProgressBar` behavior,
   ported as-is).
2. **Sparkline** — variable-height glyph timeline, absolute 0-100% scale (today's
   `percentSparkline` behavior, ported as-is, becoming width-parameterized).

Both share one shrink-to-fit contract: given a max width and an available width, both
render at `min(maxWidth, availableWidth)`, shrinking all the way down to a single
character when space is tight — no separate ad hoc clamps per call site (`24`, `[1,12]`,
implicit-from-history-length, etc.).

This ticket **only ports and unifies existing, already-working rendering logic** — no
new visual behavior. Do not touch the relative-scaled `RenderSparklineWidth` family;
it's a different semantic (per-series min/max) and out of scope here.

## 3. Scope

- New package (e.g. `internal/rograph` or `internal/usage/rograph` — decide based on
  whether other future callers outside `internal/usage` are anticipated; default to
  `internal/rograph` since the point is reusability).
- Move `RenderProgressBar` and `percentSparkline`/`percentSparkChars` into it, ported
  verbatim except for adding the shared shrink-to-fit width contract.
- Both entry points take an explicit max-width parameter; neither call site computes
  its own ad hoc clamp anymore.
- Update all call sites (`usage.go:83,101,130`, `watch.go:509-544,732`) to call
  through `rograph`.
- `go test`/existing usage tests still pass; add unit tests for the new package's
  shrink-to-one-character behavior (currently untested at the edges).

## 4. Explicit Non-Goals (deferred to follow-up tickets)

- Making all call sites actually use a consistent max width of 10 — that's
  [[079-consistent-10-char-shrink-to-fit-graphs]].
- Extracting the surrounding box/label/layout logic — that's
  [[080-agent-box-layout-extraction]].
