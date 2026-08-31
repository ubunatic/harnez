# 115 — Canary: evaluate DuckDB Go embedding for tool-call telemetry storage

**Status**: Closed — resolved in 857ff99
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

- [x] Canary script exists, runs standalone, and its output is captured
      in this ticket (latency numbers, concurrency result, dependency
      footprint).
- [x] Explicit go/no-go decision recorded for DuckDB before 116 starts
      implementation.
- [x] If "no-go," an alternative storage approach is named here so 116
      isn't blocked without direction.

## Findings

Canary: `scripts/canary-duckdb/main.go` (standalone, `go.mod`-isolated
from the root module, not imported by `internal/`). Two run modes:
`solo <path>` (200 sequential inserts, latency histogram) and
`writer <path> <n> <tag>` (used two-at-a-time to probe cross-process
locking). Deleted after this ticket closed, per Canary.md's Scope
section (it validated availability/latency/lock-behavior at a point in
time; the go/no-go decision itself, not the probe, is the durable
artifact worth keeping — see "Canary disposition" below).

Driver: `github.com/marcboeker/go-duckdb/v2` v2.4.3 (the module path
`github.com/duckdb/duckdb-go` referenced in some upstream docs is
deprecated and redirects to this one — confirmed by trying to `go get`
it directly and getting a "module declares its path as
github.com/marcboeker/go-duckdb" mismatch error).

**1. DDL / schema** — PASS. `tool_calls` table created with the 15
columns from the ticket. Assumption: the ticket says "the 5 indexes"
without listing them; no other harnez doc specifies the set, so this
canary indexed the 5 columns a `harnez stats` aggregation query would
plausibly filter/group by: `session_id`, `ticket_id`, `tool_name`,
`agent_id`, `created_at`. Flagging this assumption for 116 to confirm
or override once the actual `harnez stats` query shapes are known.

**2. Single-row insert latency** — PASS, well inside budget.
200 sequential inserts on this machine's real disk (not `:memory:`):
avg **0.81ms**, max **2.05ms**, target was sub-20ms.

**3. Concurrent writers (two processes, same file)** — **FAIL**.
Launched two `writer` processes against the same `.duckdb` file at
the same time. First process opened and completed its 50 inserts
fine (58.75ms elapsed). Second process's `open()` **failed outright**:

```
IO Error: Could not set lock on file ".../canary-concurrent.duckdb":
Conflicting lock is held in .../canary-duckdb (PID 1139332) by user uwe.
```

This is not degraded throughput or a retry-then-succeed queue — the
second writer's connection attempt errors immediately and the process
would need to be written to poll/retry the *open*, not just the insert.
This matches and confirms the concern already raised in
`docs/studies/2026-08-28-usage-collector-daemon-architecture.md` §4
("a weaker multi-process write-concurrency story than SQLite"). Since
`harnez rate`/`harnez exec` from concurrent agent sessions on the same
machine is exactly the write pattern issue 115/116 target, a hard
single-writer-file lock is disqualifying without an additional
serialization layer (e.g. a lock file + retry wrapper, which reintroduces
the flock-coordination machinery §4 was trying to avoid building twice).

**4. Dependency footprint** — cgo is **required**: a
`CGO_ENABLED=0 go build` fails to compile (`undefined: bindings.Type`
and similar, in `go-duckdb/mapping`). `go build` (cgo enabled, warm
module cache) takes ~1.6s incrementally, ~8.1s cold. Resulting binary
grew from harnez's current 12MB (`~/go/bin/harnez`) to 65MB for the
canary binary alone (+53MB) — this is the canary's own binary, not
harnez with duckdb linked in, but it is a lower bound on what `make
install` would grow to. The module cache added ~1.4GB across
`github.com/marcboeker/...` and `github.com/duckdb/...` (bundled
per-platform native libraries, `apache/arrow-go`, etc.) — a lot of
transitive weight for a "avoid deps" repo (`docs/lang/Go.md`).

## Decision: **NO-GO** on DuckDB for `tool_calls`

Two independent disqualifiers, either one sufficient alone:

- **Hard multi-process write lock.** DuckDB's single-writer-per-file
  lock fails the connection outright rather than queuing, so concurrent
  `harnez rate`/`harnez exec` calls from parallel agent sessions on the
  same machine would need a hand-rolled retry-the-open wrapper — the
  exact flock-coordination cost §4 already rejected once for a smaller
  problem, now needed *in addition to* a cgo dependency.
- **cgo dependency + large footprint.** Fails `docs/lang/Go.md`'s
  "Minimise external deps" / no-cgo-without-approval bias outright:
  +53MB binary size, ~1.4GB module cache, no `CGO_ENABLED=0` build path.

**Alternative for issue 116**: `modernc.org/sqlite` (pure-Go SQLite,
no cgo, single binary stays lean). It directly answers the OLAP-shaped
aggregation need issue 120 wants (`avg score per tool`, `failure rate`,
`byte-savings`, all in-SQL over an accumulating table) — this is the
actual reason DuckDB was considered instead of the flock+JSON pattern
in the first place, and SQLite fits it equally well while keeping pure
Go. SQLite in WAL mode with a `busy_timeout` PRAGMA serializes
concurrent writers by queuing at the transaction level rather than
failing the connection at open — matching the workload's actual shape
("many short synchronous inserts from separate processes") much better
than DuckDB's exclusive-file-lock model. 116 should canary
`modernc.org/sqlite`'s own insert latency and concurrent-writer
behavior before building on it, per the same canary-first rule, since
this ticket only measured DuckDB.

## Canary disposition

Per `docs/other/Canary.md`, canaries are normally kept as the durable
regression reference. This one is the exception the doc's own decision
gate creates: the canary answered "no" — nothing in `internal/` will
ever be built against this driver, so there is no future regression to
protect against, and every subsequent `go build ./...` in this repo
would otherwise pay the ~1.4GB module-cache/cgo tax for a mechanism the
repo explicitly is not using. `scripts/canary-duckdb/` (including its
own `go.mod`/`go.sum`, kept isolated from the root module so this
never affected the main build) is deleted after this ticket's findings
and decision are committed here; the decision and the numbers that drove
it are the artifact worth keeping, and they now live in this ticket.

## Notes

This ticket exists specifically because the spec assumes DuckDB without
justification beyond "storage_engine: DuckDB" — canary-first process
requires that assumption be verified against this repo's actual
dependency-avoidance bias before any of 116–122 write production code
against it.
