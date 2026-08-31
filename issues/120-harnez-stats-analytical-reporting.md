# 120 — `harnez stats`: analytical reporting over tool_calls

**Status**: Closed — resolved in TBD-commit-hash
**Priority**: P2 (Medium)
**Severity**: Minor
**Category**: Feature
**Related**: [[116-tool-telemetry-schema-and-storage-layer]]

## Problem

Once telemetry accumulates via [[117]]/[[118]], the user needs a
terminal report over it — call frequency, average score, failure rate,
and distillation byte savings, filterable by tool/agent/ticket.

## Scope

Command signature:

```
harnez stats [--tool <name>] [--agent <name>] [--ticket <ticket_id>] [--json]
```

- Query [[116]]'s storage layer (not raw SQL duplicated in the CLI
  package) and render:
  - Call frequency per tool and per agent.
  - Average score per tool, formatted to 2 decimal places.
  - Failure rate: `exit_code != 0 OR score <= 2`, per tool/agent.
  - Byte savings from distillation: `1 - (distilled_bytes / raw_bytes)`
    aggregate, only over rows where `distilled_bytes IS NOT NULL`.
- `--json` emits machine-readable output for scripting; default is a
  formatted terminal table.
- Filters combine with AND when multiple are given.

## Acceptance Criteria

- [x] Each metric verified against a small seeded dataset (fixture rows
      inserted directly via [[116]]'s insert function) with hand-
      computed expected output.
- [x] `--json` output is valid JSON and round-trips the same numbers as
      the table view.
- [x] Empty result set (no rows match filters) prints a clear "no data"
      message rather than an empty table or a crash.
- [x] `harnez stats --help` documents flags per this repo's self-
      documenting CLI convention.

## Resolution

Added `internal/telemetry.GroupStats` (call frequency, avg score, and
failure count — `exit_code != 0 OR score <= 2` — per distinct key) plus
`(*DB) AggregateByTool`/`AggregateByAgent` (SQL `GROUP BY`), and a
dedicated `(*DB) DistillationSavings(Filter) (DistillationSavings, error)`
restricted to rows with non-NULL `distilled_bytes`. All SQL stays in
`internal/telemetry/query.go` per this ticket's Notes.

`cmd/harnez/stats.go` wires `--tool`/`--agent`/`--ticket`/`--json` onto
`telemetry.Filter`, builds one `statsReport` struct consumed by both the
table renderer and the JSON encoder (so the two can't drift), and prints
"no data: no tool_calls rows match the given filters" for an empty
result set (table) or `"empty": true` (JSON).

Tests: `internal/telemetry/telemetry_test.go` (`TestAggregateByTool`,
`TestAggregateByAgent`, `TestAggregateByToolExcludesNullDistilledBytes`,
`TestDistillationSavings`, `TestDistillationSavingsNoRows`) and
`cmd/harnez/stats_test.go` (hand-computed fixture, JSON validity/parity
with the table view, filter combination, empty-result table+JSON, and
`--help` flag documentation). `go build`, `CGO_ENABLED=0 go build`,
`go vet`, and `go test ./...` all pass; manual smoke test against a
throwaway `~/.harnez/tool_catalog.sqlite` (backed up and restored)
confirmed sane table and JSON output for both a populated and an
empty/filtered result.

Environment note (out of this ticket's scope, reported not fixed): the
pre-existing local `~/.harnez/tool_catalog.sqlite` still had the old
`distilled_bytes NOT NULL` schema from before issue 118's follow-up
nullability fix — `harnez rate`/`harnez exec` failed against it with a
NOT NULL constraint error until the file was deleted and recreated.
`schemaDDL`'s `CREATE TABLE IF NOT EXISTS` does not retroactively alter
an existing table's column constraints, so any already-initialized DB
predating that fix needs a manual `rm ~/.harnez/tool_catalog.sqlite` (or
a real migration) to pick it up — this repo has no migration framework
(see schema.go's doc comment).

## Notes

Depends on [[116]] providing a filtered-aggregate query surface, not a
raw connection handle — keep SQL out of the CLI command file.
