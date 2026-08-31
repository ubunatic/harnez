# 131 — `!`-toggled per-agent freshness countdown overlay in `harnez usage --watch`

**Status**: Open
**Priority**: P3 (Low)
**Severity**: Minor
**Category**: Agentic Ergonomics
**Related**: `internal/usage/watch.go`, `internal/usage/statecache.go` (`DefaultCollectorInterval`), `rograph` (shared bar/sparkline renderer, [[078-rograph-library-shared-bar-sparkline-renderer]]), [[082-agent-usage-collector-daemon]] (background collector tick), [[129-parallelize-usage-collectall]] (per-agent collector parallelization this overlay would help observe)

## Problem

`harnez usage --watch` shows each agent's quota/usage row with a label
(`internal/usage/watch.go:602-638`, padded via `rograph.PadLabel`) but gives
no visibility into *when* that agent's numbers were last refreshed or how
soon the next background-collector tick (`DefaultCollectorInterval`,
`internal/usage/statecache.go:26`, 900s / 15min) will bring fresh data. The
user has to trust the numbers are current, or diff timestamps by hand.

This becomes more relevant after issue 129 parallelized the three per-agent
live collectors in `collectAll` — the boxes now update closer together in
wall time, and a lightweight per-agent freshness indicator would make it
easy to eyeball whether each pipeline is actually ticking independently on
schedule, rather than watching total request latency alone.

## Scope

- Add a new hotkey, `!`, to the key-handling switch in `watch.go` (alongside
  the existing `q`/`r`/`c`/`g`/`o`/`h`/`t`/`p`/`l`/`a` cases around
  `watch.go:476-529`) that toggles a "debug overlay" mode on the current
  watch session (in-process state, not persisted).
- While debug overlay mode is on, each agent row's label has its **last 3
  characters replaced** (not appended — no width/box resize) with a 3-char
  countdown gauge, e.g. `[▁]`/`[▄]`/`[█]`-style glyphs from the existing
  `rograph` sparkline/bar rendering (issue 078), reusing that shared
  renderer rather than hand-rolling new bar-drawing logic.
- Countdown semantics, **per agent** (not a single global gauge):
  - `elapsed := now - agent.LastRefreshed`
  - `remaining := DefaultCollectorInterval - elapsed`
  - Gauge is full the instant `LastRefreshed` updates (fresh data just
    landed for that agent) and drains toward empty as `remaining` shrinks
    toward zero (next background-collector tick due soon for that agent).
  - Each agent's gauge is independent — agents tick on their own schedule
    via the background collector, so e.g. AGY's gauge and Claude's gauge
    will generally be out of phase with each other.
- Scope is display-only: no change to collection timing, caching, or the
  `DefaultCollectorInterval` value itself — this only visualizes existing
  `LastRefreshed` data that `AgentUsage` already carries.
- Update `internal/usage/watch.go`'s in-TUI help/hint line (if one exists)
  to mention `!` for debug overlay, consistent with how other hotkeys are
  documented there.

## Acceptance Criteria

- [ ] Pressing `!` in `harnez usage --watch` toggles the debug overlay on/off
      for the current session; pressing it again reverts to normal labels.
- [ ] Each agent row's label loses exactly 3 characters of its own text
      (not padding) to the countdown gauge — total row width/box layout is
      unchanged from non-overlay mode.
- [ ] Gauge visibly resets to "full" when an agent's `LastRefreshed`
      timestamp updates (e.g. after a live fetch or a background-collector
      tick lands), and visibly drains as `remaining` shrinks.
- [ ] Gauge rendering reuses `rograph`'s shared bar/sparkline primitives
      rather than a new one-off implementation.
- [ ] Unit test(s) covering the elapsed/remaining-to-gauge-glyph mapping
      (pure function, no TUI rendering needed) at a few sample fractions
      (e.g. ~100%, ~50%, ~0%, and negative/overdue remaining clamped to
      empty rather than wrapping/erroring).
- [ ] `go test -race ./internal/usage/...` passes clean.
- [ ] `harnez status` shows the tracker still in sync after filing/closing.
