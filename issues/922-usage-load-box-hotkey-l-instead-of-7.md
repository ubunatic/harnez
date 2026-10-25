# 922 — usage: Load box hotkey L instead of 7

**Status**: Closed — 47b8967b: l/L toggles Load, title [L] Load; d/D toggles diagnostics; 7 unbound. Host live tmux check of installed binary: l shows then hides Load, 7 no change, d opens and closes diagnostics; go test ./internal/usage green
**Priority**: P3 (Low)
**Severity**: Minor
**Category**: Feature
**Related**: Epic 921, 132 (spec-driven hotkeys)

---

/goal The local Load box toggles with `L` (matching its label and `R` for Remote Load) instead of `7`, which
becomes free; set in `spec/actions.yaml`, title glyph and help updated, with a test; or stop and report.
Circuit breaker: on a loop or no progress, clear the goal and write a failure report here.

Owner request (2026-10-25). Check that `L` is not bound already.

## Decision (owner, 2026-10-25)

`l`/`L` bind `toggle_diagnostics` today (`spec/actions.yaml:89`). Move diagnostics to `d`/`D` (unbound, checked)
and give `l`/`L` to `toggle_load` (`spec/actions.yaml:39`, now `7`). Tests: `actionsspec_test.go:110-134`,
`watch_test.go:1288,1783`; update help, superscript glyph and `docs/data/harnez-usage.ansi`.

Host, from the d922 plan: no superscript L exists in the pixel font, so the title uses the same bracket form as
Remote Load, `[L] Load` (matches `[R] Remote Load`, no font work). Inside the diagnostics overlay `d`/`D` close
it; scrolling down stays on `j` (the old `d` scroll is dropped).
