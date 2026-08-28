# 083 — Self-Hiding, Auto-Discovery Agent Display in Usage TUI

**Status**: Closed — resolved in HasUsageData self-hiding filter (uncommitted, pending transplant onto main)
**Priority**: P3 (Low)
**Severity**: Minor
**Category**: UX / Agentic Ergonomics
**Related**: [[023-usage-command-token-quota-tracking]], [[082-agent-usage-collector-daemon]], docs/studies/2026-08-17-multi-agent-quota-and-usage-monitoring.md

## Problem

`harnez usage` currently renders a box/row for every supported agent (Claude, Codex, AGY)
regardless of whether that agent is actually configured/used on the machine, which can produce
empty or error-state rows for agents the user doesn't run.

Prior art: Omarchy 4.0's Quickshell "Agents" widget only shows an icon/panel for a provider once
a scan actually finds real recorded usage for it — it self-hides by default and agents
appear/disappear automatically as usage is discovered.

## Desired Behavior

- Only render a box/row for an agent once real recorded usage data exists for it on this machine
  (i.e. collector successfully found local state/credentials/history for that agent), instead of
  showing empty or error rows for agents that aren't installed/configured.
- Applies to both the one-shot `harnez usage` summary and `--watch` TUI.

## Notes

- Depends conceptually on [[082-agent-usage-collector-daemon]] for a clean "has this agent ever
  produced a usage snapshot" signal, but can likely be implemented against the current
  live-collection path too — do not block on 082 landing first unless implementation makes that
  clearly easier.

## Resolution

Added `AgentUsage.HasUsageData()` (`internal/usage/types.go`), reusing existing struct fields
rather than adding a new one: an agent counts as having real data only when `Installed` is true
*and* at least one of `Authenticated`, `Tokens`, `Session`, `Weekly`, `ModelGroups`,
`ModelTokens`, `Sources`, `Error`, or `QuotaFetchError` is populated. A bare, unconfigured config
directory (`Installed: true`, nothing else set) still self-hides, since it carries no other
signal.

Wired into both render paths:
- `RenderText` (`internal/usage/usage.go`, the default one-shot `harnez usage` report) now skips
  any agent box for which `HasUsageData()` is false, and prints a short explanatory paragraph
  instead of a blank report when every agent is absent.
- `buildWatchFrame` (`internal/usage/watch.go`, backing both `harnez usage --summary` and
  `--watch`) filters agents into a `discovered` slice before the visibility-toggle logic runs, so
  an agent with no data never gets a box *or* a "hidden: [x]" toggle hint (which would otherwise
  wrongly imply the user hid it). When zero agents are discovered, the frame body leads with an
  explanatory note pointing at installing/configuring an agent or running
  `harnez agent-collector --once`, instead of a silently agent-less screen.

`RenderJSON`/`CollectAll` output is intentionally left unfiltered — the raw summary (including
absent agents) stays available for scripting/debugging; only the human-facing renderers self-hide.

Tests added: `internal/usage/types_test.go` (table-driven `HasUsageData` cases: not installed,
installed-but-empty, authenticated, local-token-only, quota-window-only, sources-only,
error-only), plus `TestRenderText_AllAgentsAbsent` (`usage_test.go`) and
`TestBuildWatchFrame_SelfHidesAgentsWithoutUsageData` /
`TestBuildWatchFrame_AllAgentsAbsent` (`watch_test.go`). Updated `TestRenderSummary` to match the
new self-hiding behavior for its not-installed codex fixture. `go build ./...`, `go vet ./...`,
and `go test ./...` all pass (147 tests, 0 failures).
