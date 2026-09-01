# 169 — Splash: Single-Line Status/Mini-Log of In-Flight Fetch Stages

**Status**: Open
**Priority**: P2 (Medium)
**Severity**: Minor
**Category**: Agentic Ergonomics
**Related**: [[164-fast-startup-usage-watch-splash-or-stale-data]] (resolved in `a1178d4` — added the
splash screen this ticket extends), [[168-splash-determinate-progress-bar-fetch-duration-estimate]]
(in progress at filing time — replaces the splash's sweep bar with a determinate one; this ticket
is a further extension of the same splash and should land after 168 to avoid two concurrent
writers on `internal/usage/watch.go`), `internal/usage/watch.go` (`buildSplashFrame`, `CollectAll`),
`internal/usage/usage.go` (`collectAll`)

---

## 1. Problem & Motivation

The user tried the splash bar from issue 168 and confirmed it looks good. Follow-up request: add a
single status line below the bar (or wherever fits the existing centered layout) that reports what
the startup fetch is currently doing — which sub-fetch has been triggered, which has completed, and
what data has been collected so far. A small rolling status/mini-log, scoped to the splash's
lifetime, not a persistent log file.

## 2. Technical Specification / Findings

- `CollectAll` (`internal/usage/usage.go`) fans out to multiple per-agent quota sources (Claude,
  Codex, etc. — check current source list in `collectAll`) and, for a remote host, `CollectRemote`
  does the equivalent over SSH. Today these run as one opaque blocking call from the splash's
  perspective (`RunWatchWithOptions`'s background fetch goroutine, added in issue 164) — the splash
  has no visibility into which individual sub-fetch is in flight or done.
- To surface real stage-by-stage status, the fetch path needs a way to report progress as it goes
  — e.g. a callback/channel passed into `CollectAll`/`CollectRemote` (or into whatever per-source
  helper functions it calls) that the splash loop can read from and render as the status line,
  updated as sources report "started" / "done" / "failed" state.
- Keep this to a single line (per the user's "single line" framing) that shows the most recent
  event — e.g. "fetching codex quota…" then "codex quota: done, fetching claude…" — not a scrolling
  multi-line log. If a natural implementation produces a short rolling history (last N events)
  instead of just "current status", that's fine as long as the rendered footprint in the splash
  stays one line (e.g. only the latest event shown, older ones dropped) — check with the user before
  expanding the splash's fixed layout to more than one status line.
- **Styling stays spec-driven**, per this project's established convention (issue 164/168 both
  reused `spec/colors.yaml`'s named colors via `ansiWrap` rather than hardcoding ANSI codes) — reuse
  `ansiWrap`/existing named colors for the status line's styling; don't hardcode new escape codes.
- User confirmed "and that also to the agent" was shorthand for "hand this ticket off to a dev
  agent" (ASR/dictation phrasing), not a request for a non-interactive/agent-facing status surface.
  Scope stays limited to the interactive `--watch` splash — no `--summary`/programmatic-caller
  changes needed.

## 3. Implementation & Verification Plan

- Land only after issue 168 is closed, to avoid concurrent edits to `internal/usage/watch.go`.
- Thread a progress-reporting mechanism through `CollectAll`/`CollectRemote` (or their per-source
  helpers) without changing their return contracts for existing non-watch callers (`--summary`,
  `RenderSummary`, etc.) — this must be additive/optional, not a breaking signature change for
  every caller.
- Render the latest stage event as one status line in `buildSplashFrame`'s layout, styled via
  existing spec-driven color/ansiWrap helpers.
- Unit-test the stage-reporting mechanism and the status-line rendering as pure functions, following
  the existing `dispatchSplashKey`/`buildSplashFrame` test pattern in `watch_test.go`.
- Verify with a pty-based repro (per this project's established TUI-bug-fixing practice) that the
  status line updates visibly as different sources complete during a real `--watch` startup.
