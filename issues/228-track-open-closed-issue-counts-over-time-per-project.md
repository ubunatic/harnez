# 228 — Track open/closed issue counts over time per project

**Status**: Open
**Priority**: P3 (Low)
**Severity**: Minor
**Category**: Feature
**Related**: [[036-harnez-status-issues-tracker-linter]]

## Problem

Issue status (Open/Closed/etc.) lives only as plain text in each
`issues/*.md` ticket header, rolled up by `harnez index` into
`issues/README.md`. That rollup is a point-in-time snapshot — nothing
retains history, so there is no way to see how many tickets were
open/closed per project at a given point in the past, or to chart
throughput/backlog trends over time.

## Scope

- On `harnez index -d <repo>` (or a lightweight companion step),
  append a dated snapshot row — timestamp, project/repo, open count,
  closed count (and optionally per-priority breakdown) — to a small
  append-only log (e.g. `issues/.status-history.jsonl` or a
  `stats_history` table in the existing telemetry SQLite DB under
  `internal/telemetry`, whichever keeps SQL/format ownership in one
  place per [[120]]'s "keep SQL out of the CLI" convention).
- Add a read path (`harnez find issues history` or a `harnez stats
  --issues` addendum) that renders the recorded snapshots as a simple
  table, filterable by project like [[227]]'s `--project`.
- Snapshot writes must be idempotent/cheap enough to run on every
  `harnez index` invocation without noticeably slowing it down, and
  must not fail `index` if the history write fails (log-and-continue,
  not command-failing).

## Acceptance Criteria

- [ ] Running `harnez index` twice in a row with no ticket changes
      produces at most one meaningful new history entry (or clearly
      dedupes identical consecutive snapshots) rather than growing
      unboundedly on repeated no-op runs.
- [ ] History read path shows open/closed counts per project across
      at least two distinct recorded points in time, verified against
      a seeded fixture.
- [ ] Storage choice (flat file vs. telemetry DB) and its rationale
      documented in the ticket's Resolution.
- [ ] `harnez index --help` (or the new read command's `--help`)
      documents the feature.

## Notes

Scope this to counts only (open/closed, optionally by priority) — do
not attempt full issue-level history/diffing (title/status-transition
audit trails) in this ticket; that would be a much larger, separate
feature if ever needed.
