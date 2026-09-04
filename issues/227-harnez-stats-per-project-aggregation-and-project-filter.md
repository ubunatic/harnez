# 227 — harnez stats: per-project aggregation and --project filter

**Status**: Closed — resolved in <pending-commit-sha>
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

- [x] `AggregateByProject` verified against a small seeded dataset
      (fixture rows inserted via [[116]]'s insert function) with
      hand-computed expected output, mirroring `TestAggregateByTool`.
- [x] `--project` filters combine with `--tool`/`--agent`/`--ticket`
      via AND.
- [x] `--json` output includes `by_project` and round-trips the same
      numbers as the table view.
- [x] Empty result set still prints the existing "no data" message,
      not a crash or an empty table.
- [x] `harnez stats --help` documents the new `--project` flag.

## Resolution

Added `(*DB) AggregateByProject(Filter) ([]GroupStats, error)` in
`internal/telemetry/query.go`, a thin wrapper over the existing
`aggregateGroupedBy("project_name", f)` — the same pattern
`AggregateByTool`/`AggregateByAgent` already use, so no new SQL shape
was needed. Added a `Project` field to `telemetry.Filter`, wired into
`whereClause()` via the same `add("project_name", f.Project)` call as
the pre-existing `ToolName`/`AgentID`/`TicketID`/`CallType` fields, so
it combines with them via AND for free.

`cmd/harnez/stats.go` gained a `--project` flag (wired into
`telemetry.Filter.Project`), a `ByProject []telemetry.GroupStats` field
on `statsReport` (`json:"by_project,omitempty"`), a call to
`AggregateByProject` in `buildStatsReport`, a `PROJECT\tCALLS\tAVG
SCORE\tFAILURE RATE` block in `renderStatsTable` alongside the
TOOL/AGENT blocks, and `Empty` now also considers `len(byProject) == 0`.
All SQL stays in `internal/telemetry/query.go`; the CLI file only
builds the filter, calls the aggregate methods, and renders. Per the
ticket's Notes, grouping defaults to `project_name` (not
`working_dir`) as the more stable identity across relocations of a
checkout.

Tests: `internal/telemetry/telemetry_test.go` (`TestAggregateByProject`,
`TestFilterProject`) mirror `TestAggregateByTool`/`TestAggregateByAgent`
with hand-computed expected `GroupStats`. `cmd/harnez/stats_test.go`'s
shared fixture (`seedStatsFixture`) now assigns `ProjectName: "harnez"`/
`"voxi"` to its rows, and existing tests were extended to assert on
`by_project`/`PROJECT` alongside `by_tool`/`by_agent`
(`TestRunStatsTable_MatchesHandComputedFixture`,
`TestRunStatsJSON_ValidAndMatchesTable`,
`TestRunStatsEmptyResult_TableAndJSON`); a new
`TestRunStatsFilters_Project` covers `--project` alone and its
AND-combination with `--tool` (including the resulting empty-result
case), and `TestStatsCmdHelp_DocumentsFlags` now also checks for
`--project`.

`go build ./...`, `go vet ./...`, and `go test ./...` all pass; `make
install` updated the installed `~/go/bin/harnez` binary.

## Notes

Decide whether to group by `project_name` or `working_dir` (or both) —
`project_name` is the more stable identity across relocations of a
checkout, but `working_dir` disambiguates multiple checkouts of the
same project. Default to `project_name` unless empty, matching how
`ticket_id`/other optional fields already default.
