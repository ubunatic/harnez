# 047 — Add sparkline visualization to show model usage over time

**Status**: Closed — resolved
**Priority**: P2 (Medium)
**Severity**: Minor
**Category**: Feature
**Related**: [023](023-usage-command-token-quota-tracking.md), [internal/usage/history.go](../internal/usage/history.go), [internal/usage/watch.go](../internal/usage/watch.go)

---

## 1. Problem & Motivation

Currently, `harnez usage` tracks usage and quota windows across agents (Claude Code, Antigravity/AGY, OpenAI Codex). In `--watch` mode, an in-memory sparkline shows the overall token consumption rate per agent over a rolling 12-sample window (`rateTracker` in `internal/usage/watch.go`).

However:
1. **No Model-Specific Breakdown Over Time**: Multi-model agents (e.g. AGY switching between Gemini Pro, Claude 3.5 Sonnet, GPT-4o, or Claude Code using Sonnet vs Opus/Haiku) aggregate all token consumption into a single agent-level total. Users cannot visually discern consumption trends across specific models.
2. **Timeline / History Lacks Sparklines**: When inspecting historical snapshots (`harnez usage --timeline` or reading from `~/.claude/harnez/usage-history/*.jsonl`), history is printed as tabular rows without visual sparkline trends showing consumption velocity or quota shifts over time across recorded sessions.

## 2. Proposed Solution

1. **Model-Granular Usage History**:
   - Tag token metrics and history snapshots with model-level breakdowns where available (or model group windows).
   - Record model usage points in the history buffer / snapshot entries.

2. **Terminal Sparkline Visualizer**:
   - Extend sparkline rendering (` ` through `█`) to display per-model or per-agent usage trends over historical time intervals.
   - Support compact sparkline summaries in `--timeline` and `--summary` output (e.g. showing 24h or per-session trajectory).
   - In `--watch` mode, allow toggling model-level breakdown with corresponding rate sparklines.

3. **Multi-Host Aggregation & Rate Metrics**:
   - Ensure the sparkline generation works seamlessly across merged multi-host timeline data loaded by `ReadHistory()`.
   - Compute start tokens, end tokens, and net used tokens over the snapshot interval.
   - Compute consumption velocity / rate per hour (`tok/hr`) and per day (`tok/day`) for both overall agents and per-model breakdowns.
   - Handle sub-minute, zero duration, and zero consumption edge cases gracefully.

## 3. Acceptance Criteria

- [x] History records / data models capture model-level token breakdowns where reported.
- [x] Sparkline renderer supports time-series arrays representing model token consumption or rate changes.
- [x] `harnez usage --timeline` renders ASCII/Unicode sparklines along with start/end tokens, total tokens used, and consumption rates per hour (`/hr`) and per day (`/day`).
- [x] Unit tests in `internal/usage/` cover sparkline generation edge cases (empty data, flat usage, monotonic increases, spikes, single snapshots, sub-minute durations, multi-day spans, zero usage).

