# 120 — `harnez stats`: analytical reporting over tool_calls

**Status**: Open
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

- [ ] Each metric verified against a small seeded dataset (fixture rows
      inserted directly via [[116]]'s insert function) with hand-
      computed expected output.
- [ ] `--json` output is valid JSON and round-trips the same numbers as
      the table view.
- [ ] Empty result set (no rows match filters) prints a clear "no data"
      message rather than an empty table or a crash.
- [ ] `harnez stats --help` documents flags per this repo's self-
      documenting CLI convention.

## Notes

Depends on [[116]] providing a filtered-aggregate query surface, not a
raw connection handle — keep SQL out of the CLI command file.
