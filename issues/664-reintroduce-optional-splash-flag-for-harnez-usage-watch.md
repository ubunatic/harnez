# 664 — Reintroduce optional --splash flag for harnez usage --watch

**Status**: Closed — Optional --splash flag added for harnez usage --watch with clean terminal restoration and test coverage
**Priority**: P2 (Medium)
**Severity**: Minor
**Category**: Feature
**Related**: 164, 168, 169, 252, 254, 660; [watch loop](../internal/usage/watch.go), [CLI entrypoint](../cmd/harnez/main.go)

---

## 1. Problem & Motivation

Issue 660 resolved watch startup latency and terminal restoration regressions by painting the dashboard immediately on launch with asynchronous data collection, bypassing the startup splash screen.
However, visual feedback during initial provider fetching (including spinner, determinate progress bar, fetch-stage status lines, and completed provider badges) remains desirable as an opt-in mode.
A new `--splash` flag on `harnez usage --watch` allows users to re-enable the startup splash screen when desired without degrading the default fast startup experience.

## 2. Technical Specification / Findings

- Splash rendering infrastructure (`buildSplashFrame`, `splashStatusLine`, `splashBadgesLine`, `splashStatusAdvance`, etc.) remains implemented in `internal/usage/watch.go` and covered by `internal/usage/watch_test.go`.
- Add `--splash` flag to `harnez usage` in `cmd/harnez/main.go` and propagate it via `usage.WatchOptions{Splash: ...}`.
- When `--splash` is set with `--watch`:
  - Display the animated splash screen during the initial live collection phase.
  - Support `Esc` to skip the splash and freeze/transition immediately to the dashboard.
  - Support `q` / `Q` / `Ctrl-C` to exit promptly with guaranteed terminal restoration (retaining the raw termios and cleanup guarantees established in issue 660).
- When `--splash` is false (default):
  - Retain the immediate interactive dashboard rendering introduced in issue 660.
- Update tests to cover the `--splash` option and flag validation.

## 3. Implementation & Verification Plan

- **/goal**: `/goal Implement the optional --splash flag for harnez usage --watch, wire it through WatchOptions, verify that --splash displays the startup splash during initial fetch while preserving terminal restoration and prompt quit/skip handling, and verify default --watch remains fast-start without splash; stop and report when blocked on a user decision or denied permission.`
- Add unit tests in `internal/usage/watch_test.go` and CLI flag tests in `cmd/harnez/usage_test.go`.
- Verify with `make test-q1` and verify live behavior with `harnez usage --watch --splash`.
