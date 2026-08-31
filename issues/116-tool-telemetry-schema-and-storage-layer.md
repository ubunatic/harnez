# 116 — Tool telemetry schema & storage layer (`tool_calls` table)

**Status**: Open
**Priority**: P2 (Medium)
**Severity**: Moderate
**Category**: Architecture
**Related**: [[115-canary-duckdb-go-embedding-for-tool-telemetry]], [[117-harnez-rate-command]], [[118-harnez-exec-shell-interceptor]], [[120-harnez-stats-analytical-reporting]], `docs/other/Spec.md`

## Problem

`harnez rate` (117), `harnez exec` (118), and `harnez stats` (120) all
need a shared storage layer for tool-call telemetry — a single
`tool_calls` table (DDL below) plus a Go package that owns
connection lifecycle, schema migration, and typed insert/query
functions. Building this once as a shared `internal/telemetry` (or
similar) package avoids three commands each hand-rolling their own
DuckDB access.

Blocked on [[115]]'s go/no-go decision on the storage engine.

## Scope

1. New internal package (e.g. `internal/telemetry`) owning:
   - Connection open/close against `~/.harnez/tool_catalog.duckdb`
     (or the alternative engine chosen in 115).
   - Idempotent schema creation (`CREATE TABLE IF NOT EXISTS` + the 5
     indexes from the spec) run on first use, not a separate migration
     step — matches this project's "just change the code" bias, no
     migration framework unless a real schema-evolution need appears.
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
- [ ] First-run schema creation is idempotent (running the CLI twice
      against a missing/existing DB doesn't error).
- [ ] Single-row insert meets the sub-20ms budget stated in the spec (as
      canary-verified in 115) — add a benchmark test, not just a manual
      timing.
- [ ] Unit tests cover insert + basic filtered query without requiring a
      real DuckDB binary in CI if the driver can't run there (fall back
      to build-tag-gated integration test, matching this repo's existing
      test conventions for external-tool-dependent code).

## Notes

Depends on [[115]]'s canary outcome for which engine this actually
targets — do not start implementation before that ticket closes with a
go/no-go decision.
