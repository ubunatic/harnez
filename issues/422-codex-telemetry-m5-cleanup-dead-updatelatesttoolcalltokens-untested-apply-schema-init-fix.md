# 422 — Codex telemetry M5 cleanup: dead UpdateLatestToolCallTokens, untested apply schema-init fix

**Status**: Open
**Priority**: P3 (Low)
**Severity**: Minor
**Category**: Code Quality / Telemetry
**Related**: #420 (Codex analytics hooks — M5)

---

## 1. Problem & Motivation

Final review of #420's M5 (separate cumulative and per-turn Codex token
metrics) found two small quality gaps in the commits that landed after the
ticket's own M5 completion note, neither of which blocks correctness today
but both of which are worth cleaning up before they cause confusion or a
silent regression later.

## 2. Technical Specification / Findings

- `internal/telemetry/update.go`'s `UpdateLatestToolCallTokens(sessionID,
  total *int64)` is now dead code. Commit `0b18f26` ("feat(telemetry):
  separate Codex token snapshots") replaced its only caller
  (`cmd/harnez/codexhooks.go`'s `updateLatestCodexTokens`) with
  `UpdateLatestProviderUsage`, which sets `actual_tokens` alongside the four
  new per-turn/cumulative columns. `grep -rn UpdateLatestToolCallTokens`
  across the repo shows only the function's own definition and doc comment —
  no caller, no test. Per this repo's own conventions (no
  backwards-compatibility shims for code confirmed unused), it should be
  deleted rather than left as unreferenced surface area.
- `cmd/harnez/main.go`'s `ensureTelemetrySchema()` (commit `7c8d714`, "fix(apply):
  initialize telemetry schema") has no test coverage. It was added post-hoc
  after a live-session regression: `harnez apply` could install the Codex
  `PostToolUse` hook before the shared telemetry DB had migrated to schema
  v4, so `codex-telemetry` exited 1 on the new token columns (see #420's
  "Post-close regression fix" note, commit `14a5d0e`). The schema-version fix
  itself (`ec1e518`) is covered by the existing generic
  `TestOpenMigratesStaleSchemaVersion`, but the `apply`-specific fix — that
  `apply` now proactively opens/migrates the DB via `ensureTelemetrySchema`
  before installing hooks — has no regression test, and #420 was never
  updated to record commit `7c8d714`. A later edit to `apply`'s command flow
  (e.g. reordering `RunE`, or someone removing the call believing it's
  redundant with `telemetry.Open`'s own lazy migration) would not be caught
  by CI.

## 3. Implementation & Verification Plan

### M1 — Remove dead code

- Delete `UpdateLatestToolCallTokens` from `internal/telemetry/update.go`
  and its doc comment.
- Verification: `go build ./...` and `go vet ./...` stay clean; existing
  `internal/telemetry` test suite passes unchanged (no test currently
  references the removed function).

### M2 — Cover the apply schema-init fix

- Add a test asserting that running `harnez apply`'s `RunE` path (or a
  narrower unit test around `ensureTelemetrySchema`) against a DB stamped at
  a stale schema version leaves it migrated to the current `schemaVersion`
  before any hook install step runs.
- Verification: the new test fails against a revert of commit `7c8d714` and
  passes on current `main`.

## 4. Acceptance Criteria

- No unused exported telemetry update function remains.
- A regression in `apply`'s schema-initialization-before-hook-install
  ordering is caught by the test suite, not just by a live Codex session.

## 5. Verification Guidance

`go build ./... && go vet ./... && go test ./internal/telemetry/... ./cmd/harnez/...`
after each milestone.
