# 131 — `!`-toggled per-agent freshness countdown overlay in `harnez usage --watch`

**Status**: Closed — resolved in `d8a3abe`, `7514ecf`
**Priority**: P3 (Low)
**Severity**: Minor
**Category**: Agentic Ergonomics
**Related**: `internal/usage/watch.go`, `internal/usage/statecache.go` (`DefaultCollectorInterval`), `rograph` (shared bar/sparkline renderer, [[078-rograph-library-shared-bar-sparkline-renderer]]), [[082-agent-usage-collector-daemon]] (background collector tick), [[129-parallelize-usage-collectall]] (per-agent collector parallelization this overlay would help observe), [[132-watch-spec-driven-superscript-hotkeys]] (sibling label-rendering rework — see Scope Boundary note below; implement 132 first)

## Scope Boundary vs. Issue 132

Both this ticket and 132 "steal" characters from existing text to embed a
glyph, but at different render targets — they do not compete for the same
characters and neither ticket's acceptance criteria conflicts with the
other's:

- **This ticket (131)**: modifies **per-agent row labels** inside a box's
  body (e.g. the Claude/AGY/Codex rows in the All Usage table), and only
  while the `!` debug overlay is toggled on — normal-mode rendering is
  unaffected.
- **132**: modifies **box titles** (the header line of each top-level
  box: Claude box, AGY box, Codex box, History, Load, etc.) to show a
  superscript toggle digit, and is always-on (not a debug-only overlay).

Implement 132 first — it's the more foundational rework (introduces the
`spec/actions.yaml` registry and reworks `watch.go`'s key/label dispatch
plumbing this ticket's `!` key would otherwise have to bolt onto
separately). This ticket's `!` toggle should be added to `spec/actions.yaml`
once 132 lands, rather than as a one-off hardcoded key.

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

- Issue 132 has landed (spec-driven hotkey registry, `spec/actions.yaml` +
  `internal/usage/actionsspec.go`, numbered box toggles `1`-`7`, letter
  aliases `C`/`G`/`O`/`H`/`P`/`L` dropped). Add `!` to `spec/actions.yaml`
  as a new action (not a hardcoded Go `switch` case) that toggles a "debug
  overlay" mode on the current watch session (in-process state, not
  persisted), following the pattern 132 established for wiring spec-driven
  keys into `applyWatchSectionKey`/`dispatchWatchKey`.
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

- [x] Pressing `!` in `harnez usage --watch` toggles the debug overlay on/off
      for the current session; pressing it again reverts to normal labels.
- [x] Each agent row's label loses exactly 3 characters of its own text
      (not padding) to the countdown gauge — total row width/box layout is
      unchanged from non-overlay mode.
- [x] Gauge visibly resets to "full" when an agent's `LastRefreshed`
      timestamp updates (e.g. after a live fetch or a background-collector
      tick lands), and visibly drains as `remaining` shrinks.
- [x] Gauge rendering reuses `rograph`'s shared bar/sparkline primitives
      rather than a new one-off implementation.
- [x] Unit test(s) covering the elapsed/remaining-to-gauge-glyph mapping
      (pure function, no TUI rendering needed) at a few sample fractions
      (e.g. ~100%, ~50%, ~0%, and negative/overdue remaining clamped to
      empty rather than wrapping/erroring).
- [x] `go test -race ./internal/usage/...` passes clean.
- [x] `harnez status` shows the tracker still in sync after filing/closing.
