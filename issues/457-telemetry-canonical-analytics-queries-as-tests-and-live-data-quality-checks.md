# 457 — Telemetry canonical analytics queries as tests and live data-quality checks

**Status**: Open
**Priority**: P2 (Medium)
**Severity**: Moderate
**Category**: Telemetry / Data quality
**Related**: 424, 425, 428, 127, 341, 446, 445, 421, `docs/Spec.md`

---

## Problem

Telemetry bugs have surfaced only when someone happened to look (424: compaction events
inserted without the model column; 341: concurrent writers losing rows; 425: schema drift).
There is no fixed set of analytics queries that is guaranteed to keep working, and nothing
checks the live store for odd-looking data. Odd data is the signal that the model is too
thin or unclean, but today nobody is told.

## /goal

A small, named set of basic analytics queries (for example calls per tool/agent/model,
sessions per day, NULL/empty rate per key column, orphaned or duplicate events,
tokens-per-session distribution) that must always work. They are defined once, run against
fixture data as part of the test suite, and run against the live store as a new data-quality
check that reports where the data looks odd and therefore where to focus (missing columns,
unclassified rows, unexpected NULLs, implausible values). Findings feed decisions on a
cleaner or more complete data model.

## Notes

- Define the queries as data in `spec/` (per `docs/Spec.md`); Go must not duplicate the SQL.
- The same query set powers both paths: tests assert results on a seeded fixture store, the
  live check runs them read-only against the real database and prints a short pass/warn
  report with row counts and offending percentages. Prefer a subcommand or `stats`/`usage`
  flag over a new top-level command; decide during planning.
- Read-only against the live store; never mutate telemetry from the check.
- Sequence after 424/425 so the schema is trusted; coordinate with 127 (SQL moving to
  `spec/`), which this should build on rather than duplicate.
- Record anything the check surfaces that needs a model change as a note in this ticket
  rather than fixing it inline.
- Re-verify against live code and recent history before starting; the goal is the durable
  north star.
