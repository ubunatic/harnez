# 1105 — usage --json reports the Codex monthly window in the weekly field

**Status**: Closed — usage --json puts a Monthly window under "monthly" (round-trips into the long slot); live: codex monthly=Monthly, claude weekly unchanged (9231ebc3); go test ./... green
**Priority**: P2 (Medium)
**Severity**: Moderate
**Category**: Bug
**Related**: taia (codeberg.org/harnez/taia), 1083 (grooming for taia focus)

---

/goal `harnez usage --json` labels the Codex monthly window as monthly, with a test, and consumers are told of the change. Stop and report when blocked on a user decision or denied permission; on a loop or no progress, clear the goal and write a failure report here.

## Problem

Reported by the taia session (2026-11-03): `harnez usage --json` puts Codex's monthly quota window in the `weekly` field, so consumers such as `horg pick` read a monthly limit as a weekly one. See `internal/usage/codex.go` and `types.go` (`Weekly *QuotaWindow`). Fix: report the window under its real period (e.g. a `monthly` field, or a window with its own `name`/period), keep `weekly` for true weekly windows, and note the JSON change for consumers.
