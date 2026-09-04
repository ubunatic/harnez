# 227 — harnez stats: per-project aggregation and --project filter

**Status**: Open
**Priority**: P2 (Medium)
**Severity**: Minor
**Category**: Feature
**Related**: [[120-harnez-stats-analytical-reporting]], [[116-tool-telemetry-schema-and-storage-layer]]

## Problem

`tool_calls` (`internal/telemetry/schema.go`) stores `project_name` and
`working_dir` per row, and `export.go` includes `ProjectName` in
exports — but `harnez stats` (`cmd/harnez/stats.go`) never surfaces
either as a breakdown dimension. It only aggregates `AggregateByTool`
and `AggregateByAgent`; `telemetry.Filter` has no project/dir field.
A user working across multiple projects/repos has no way to see call
frequency, average score, or failure rate broken down per project, or
to filter the report down to one project.

## Scope

- Add `(*DB) AggregateByProject(Filter) ([]GroupStats, error)` in
  `internal/telemetry/query.go`, following the existing
  `aggregateGroupedBy` pattern used by `AggregateByTool`/`AggregateByAgent`.
- Add a `Project` field to `telemetry.Filter` and wire a `--project`
  flag on `harnez stats` (`cmd/harnez/stats.go`), combining with
  existing filters via AND per the existing convention.
- Render a `PROJECT\tCALLS\tAVG SCORE\tFAILURE RATE` block in the table
  output alongside the existing TOOL/AGENT blocks, and a `by_project`
  field in the JSON report (`statsReport`).
- Keep all SQL in `internal/telemetry` — no raw SQL in the CLI file,
  per [[120]]'s explicit convention.

## Acceptance Criteria

- [ ] `AggregateByProject` verified against a small seeded dataset
      (fixture rows inserted via [[116]]'s insert function) with
      hand-computed expected output, mirroring `TestAggregateByTool`.
- [ ] `--project` filters combine with `--tool`/`--agent`/`--ticket`
      via AND.
- [ ] `--json` output includes `by_project` and round-trips the same
      numbers as the table view.
- [ ] Empty result set still prints the existing "no data" message,
      not a crash or an empty table.
- [ ] `harnez stats --help` documents the new `--project` flag.

## Notes

Decide whether to group by `project_name` or `working_dir` (or both) —
`project_name` is the more stable identity across relocations of a
checkout, but `working_dir` disambiguates multiple checkouts of the
same project. Default to `project_name` unless empty, matching how
`ticket_id`/other optional fields already default.
