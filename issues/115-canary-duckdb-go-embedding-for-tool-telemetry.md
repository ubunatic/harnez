# 115 — Canary: evaluate DuckDB Go embedding for tool-call telemetry storage

**Status**: Open
**Priority**: P2 (Medium)
**Severity**: Moderate
**Category**: Architecture
**Related**: [[116-tool-telemetry-schema-and-storage-layer]], `docs/other/Canary.md`, `docs/lang/Go.md`, `docs/studies/2026-08-28-usage-collector-daemon-architecture.md`

## Problem

A proposed spec (`harnez-tool-observability`, see notes) wants a new
`~/.harnez/tool_catalog.duckdb` store for per-tool-call telemetry
(scores, exec metrics, session/ticket linkage), written synchronously
from `harnez rate` and `harnez exec` on every internal-tool or
shell-tool invocation, with a sub-20ms write budget.

This repo already looked at embedding SQLite/DuckDB once, for a
different problem (`docs/studies/2026-08-28-usage-collector-daemon-architecture.md`
§4, filed during issue 087): it was rejected there because that problem
was a cross-process mutex + 3-small-records cache, not a query problem,
and adding a cgo (or heavy pure-Go) dependency was ruled out against
`docs/lang/Go.md`'s "avoid deps" bias. That same study explicitly flagged
DuckDB as worth reconsidering *if* a genuine historical-query need shows
up later — this ticket is that later case: `harnez stats` (issue 120)
needs aggregation (avg score per tool, failure rate, byte-savings) over
an accumulating table, which is a real OLAP-shaped query, not a mutex.

Per `docs/other/Canary.md` ("canary-first development"), no feature code
should be built on the DuckDB Go binding before it's canary-verified
in this environment — driver availability (cgo vs pure-Go), binary size
impact, write latency under lock, and concurrent-writer behavior across
multiple agent sessions writing to the same file are all unverified.

## Scope

1. Stand up a throwaway canary (outside `internal/`, per Canary.md
   convention) that:
   - Opens/creates `~/.harnez/tool_catalog.duckdb` with the DDL from the
     spec (`tool_calls` table + 5 indexes).
   - Measures single-row insert latency (target: sub-20ms, matching the
     spec's `harnez rate` performance constraint) on this machine's
     actual disk.
   - Exercises concurrent writers (two processes appending inside a
     tight loop) to observe DuckDB's actual multi-process write-lock
     behavior — the study's stated concern for DuckDB specifically.
   - Confirms which Go driver is used (`marcboeker/go-duckdb` or
     equivalent), whether it requires cgo, and what it adds to build
     size / `make install` time for this project's target platforms.
2. Record findings (pass/fail per check) directly in this ticket or a
   linked `docs/studies/` note.
3. **Decision gate**: if concurrent-write latency or binary-size cost is
   unacceptable, this ticket should close with a documented rejection
   (mirroring the 087 study's format) and 116 should be re-scoped to an
   alternative (e.g. one file per session + periodic compaction, or
   `modernc.org/sqlite`) rather than proceeding with DuckDB.

## Acceptance Criteria

- [ ] Canary script exists, runs standalone, and its output is captured
      in this ticket (latency numbers, concurrency result, dependency
      footprint).
- [ ] Explicit go/no-go decision recorded for DuckDB before 116 starts
      implementation.
- [ ] If "no-go," an alternative storage approach is named here so 116
      isn't blocked without direction.

## Notes

This ticket exists specifically because the spec assumes DuckDB without
justification beyond "storage_engine: DuckDB" — canary-first process
requires that assumption be verified against this repo's actual
dependency-avoidance bias before any of 116–122 write production code
against it.
