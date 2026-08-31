# 116 — Tool telemetry schema & storage layer (`tool_calls` table)

**Status**: Open
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

## Acceptance Criteria

- [ ] Package builds and is usable by both `harnez rate` (117, `call_type
      = internal`) and `harnez exec` (118, `call_type = shell`) without
      either needing to touch SQL directly.
- [ ] `CGO_ENABLED=0 go build ./...` succeeds with the telemetry package
      included — this was the exact failure mode that disqualified 115's
      DuckDB path, so it's a hard regression check here.
- [ ] First-run schema creation is idempotent (running the CLI twice
      against a missing/existing DB doesn't error).
- [ ] Single-row insert meets the sub-20ms budget stated in the spec —
      add a benchmark test, not just a manual timing.
- [ ] Two concurrent writers (table test, matching 115's canary
      methodology) queue via `busy_timeout` rather than failing the
      connection — the specific behavior that ruled out DuckDB.
- [ ] Unit tests cover insert + basic filtered query using the real
      pure-Go SQLite driver directly in CI (no build-tag-gated skip
      needed — unlike DuckDB, `modernc.org/sqlite` has no cgo/binary
      dependency that would block CI).

## Notes

Storage/schema itself is agent-agnostic — the per-agent scope decision
(v1 ships for whichever of Claude/AGY/Codex the hook model actually
works for, no three-agent-parity gate) lives in [[119]], not here.
