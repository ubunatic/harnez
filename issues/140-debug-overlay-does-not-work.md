# 140 — `!` debug overlay does not work in `harnez usage --watch`

**Status**: Closed — resolved in `bc3f148`
**Priority**: P2 (Medium)
**Severity**: Moderate
**Category**: Bug
**Related**: [[131-watch-debug-freshness-countdown-overlay]], `internal/usage/watch.go`, `spec/actions.yaml`

---

## 1. Problem & Motivation

The `!` debug-overlay hotkey in `harnez usage --watch` does not produce the
expected per-agent freshness countdown overlay. The feature was implemented
and closed under issue 131, but is not working in the current user-facing
watch TUI.

This removes the only direct way to inspect the age of each agent's cached
usage data while watching the dashboard.

## 2. Technical Specification / Findings

- Reproduce in an interactive `harnez usage --watch` session by pressing `!`.
- Determine whether the fault is in terminal key dispatch, the action-spec
  registration, overlay state mutation, or the per-agent label rendering path.
- Preserve the existing normal-mode layout and all other watch hotkeys.

## 3. Implementation & Verification Plan

- Add a regression test that drives the `!` action through the same dispatch
  path used by the TUI and asserts that overlay mode changes.
- Restore visible per-agent countdown gauges while overlay mode is enabled,
  without changing row or box widths.
- Add or update focused rendering tests for both overlay states.
- Verified manually in `harnez usage --watch --compact` that `!` toggles the
  overlay on and off. `go test -race ./internal/usage/...` passes.
