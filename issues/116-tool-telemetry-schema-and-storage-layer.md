# 116 — Tool telemetry schema & storage layer (`tool_calls` table)

**Status**: Closed — resolved in PENDING_COMMIT
**Priority**: P2 (Medium)
**Severity**: Moderate
**Category**: Architecture
**Related**: [[115-canary-duckdb-go-embedding-for-tool-telemetry]], [[117-harnez-rate-command]], [[118-harnez-exec-shell-interceptor]], [[120-harnez-stats-analytical-reporting]], `docs/other/Spec.md`, `docs/other/Canary.md`

## Problem

`harnez rate` (117), `harnez exec` (118), and `harnez stats` (120) all
need a shared storage layer for tool-call telemetry — a single
`tool_calls` table (DDL below) plus a Go package that owns
connection lifecycle, schema migration, and typed insert/query
functions. Building this once as a shared `internal/telemetry` (or
similar) package avoids three commands each hand-rolling their own
DuckDB access.

[[115]] closed **NO-GO on DuckDB**: hard single-writer-per-file lock
(second concurrent writer's `open()` fails outright, not a queue) plus
a required cgo dependency (+53MB binary, ~1.4GB module cache) —
disqualifying on both counts against `docs/lang/Go.md`'s dependency
bias. 115 named **`modernc.org/sqlite`** (pure-Go, no cgo) as the
alternative, using WAL mode + `busy_timeout` so concurrent writers
queue at the transaction level instead of failing at connect time.
This ticket now targets that engine — no longer blocked.

## Scope

0. Per `docs/other/Canary.md`, `modernc.org/sqlite` itself has not been
   canary-verified in this repo yet — 115 only measured DuckDB. Before
   writing the `internal/telemetry` package, run a small canary (can be
   folded into this ticket's own dev loop rather than a separate ticket,
   given SQLite/WAL is a much lower-risk, well-precedented mechanism
   than DuckDB was) confirming: pure-Go build with `CGO_ENABLED=0`
   succeeds, single-row insert stays sub-20ms on real disk, and two
   concurrent writers against the same file with `busy_timeout` set
   actually queue instead of erroring (the exact failure mode that
   disqualified DuckDB). Record the result inline in this ticket before
   proceeding to step 1.
1. New internal package (e.g. `internal/telemetry`) owning:
   - Connection open/close against `~/.harnez/tool_catalog.sqlite`
     (WAL mode, `busy_timeout` pragma set on open).
   - Idempotent schema creation (`CREATE TABLE IF NOT EXISTS` + the 5
     indexes: `session_id`, `ticket_id`, `tool_name`, `agent_id`,
     `created_at` — per 115's Findings; revisit once real `harnez
     stats` query shapes from [[120]] are known) run on first use, not
     a separate migration step — matches this project's "just change
     the code" bias, no migration framework unless a real
     schema-evolution need appears.
   - A typed `Insert(ToolCall) error` for single-row synchronous writes.
   - A typed query surface for 120 (`harnez stats`) to filter by tool,
     agent, ticket, and aggregate score/exit-code/byte stats.
2. Per `docs/other/Spec.md`, the DDL is the single source of truth —
   the Go struct/insert code must not duplicate field lists or
   constraints (e.g. score 1–5) that the schema already encodes;
   validate against the schema's shape, don't hardcode a parallel copy.
3. `ToolCall` struct fields per spec DDL:
   `id, created_at, session_id, ticket_id, project_name, working_dir,
   agent_id, tool_name, call_type, score, note, exit_code, duration_ms,
   raw_bytes, distilled_bytes`.

## Canary

Canary: `scripts/canary-sqlite/main.go` (standalone, own `go.mod`, not
imported by `internal/`). Same two-mode shape as 115's DuckDB canary:
`solo <path>` (200 sequential inserts, latency stats) and `writer <path>
<n> <tag>` (run two-at-a-time against the same file to probe locking).
Deleted after this ticket closed, per Canary.md's disposition guidance —
its logic is subsumed by `internal/telemetry`'s own concurrent-writer
test (`TestConcurrentWriters` in step 3 below), so keeping both would be
two copies of the same probe.

Driver: `modernc.org/sqlite` v1.57.0 (pure Go, confirmed via `go list -m`).

**1. `CGO_ENABLED=0 go build`** — **PASS**. Both `go build` and
`CGO_ENABLED=0 go build` succeed against the canary — this is the exact
failure mode that disqualified DuckDB in 115 (`undefined: bindings.Type`
under `CGO_ENABLED=0`), and SQLite has no cgo dependency to trip on.

**2. Single-row insert latency** — **PASS**, well inside budget.
200 sequential inserts on real disk (not `:memory:`): avg **0.91ms**,
max **6.1ms**, target was sub-20ms.

**3. Concurrent writers (two processes, same file)** — **PASS**, with
one finding along the way. WAL mode + `busy_timeout` PRAGMA (DSN:
`?_pragma=busy_timeout(5000)&_pragma=journal_mode(WAL)`) makes two
processes queue at the transaction level rather than failing the
connection outright — the specific DuckDB-disqualifying behavior from
115 does not reproduce here. Confirmed over 8 repeated two-process runs
against a fresh file each time: both processes' 50-insert batches
completed every run (no `SQLITE_BUSY` failures), with elapsed times
showing real serialization (one process consistently ~2x the solo
elapsed while the other ran at solo speed, i.e. queuing, not silent data
loss or corruption).

**Finding — cold-start race on first WAL conversion.** Before adding an
open-retry loop, two processes racing to `Open()` the *same brand-new*
file at the literal same instant could still hit `SQLITE_BUSY` on that
one-time journal_mode→WAL conversion itself, despite `busy_timeout`
being set in the same DSN — `modernc.org/sqlite`'s own pragma-apply order
already pushes `busy_timeout` first regardless of DSN key order (see
`applyQueryParams` in the driver source), so this isn't a DSN-ordering
bug, just a window where the timeout's retry doesn't cover the WAL
conversion's own lock class. Reproduced in 2 of 5 fresh-file two-process
runs. Fix: wrap `Open()`+schema-DDL in a small retry loop (10 attempts,
20ms backoff) rather than a single attempt — this is standalone
mitigation at the `Open()` call site, not the flock-coordination
machinery `docs/studies/2026-08-28-usage-collector-daemon-architecture.md`
§4 rejected, since it needs no separate lock file and only guards the
one-time schema-creation window. With the retry loop, 8/8 fresh-file
runs passed. `internal/telemetry`'s `Open()` carries this retry loop
forward; its own concurrent-writer test exercises the same fresh-file
race.

## Acceptance Criteria

- [x] Package builds and is usable by both `harnez rate` (117, `call_type
      = internal`) and `harnez exec` (118, `call_type = shell`) without
      either needing to touch SQL directly.
- [x] `CGO_ENABLED=0 go build ./...` succeeds with the telemetry package
      included — this was the exact failure mode that disqualified 115's
      DuckDB path, so it's a hard regression check here.
- [x] First-run schema creation is idempotent (running the CLI twice
      against a missing/existing DB doesn't error).
- [x] Single-row insert meets the sub-20ms budget stated in the spec —
      add a benchmark test, not just a manual timing.
- [x] Two concurrent writers (table test, matching 115's canary
      methodology) queue via `busy_timeout` rather than failing the
      connection — the specific behavior that ruled out DuckDB.
- [x] Unit tests cover insert + basic filtered query using the real
      pure-Go SQLite driver directly in CI (no build-tag-gated skip
      needed — unlike DuckDB, `modernc.org/sqlite` has no cgo/binary
      dependency that would block CI).

## Notes

Storage/schema itself is agent-agnostic — the per-agent scope decision
(v1 ships for whichever of Claude/AGY/Codex the hook model actually
works for, no three-agent-parity gate) lives in [[119]], not here.
