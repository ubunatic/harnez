# 1236 — telemetry: OpenReadOnly lacks busy_timeout, causing SQLITE_BUSY on stats --agents

**Status**: Closed — Added _pragma=busy_timeout(5000) to telemetry.OpenReadOnly DSN and added TestOpenReadOnlyWaitsForBusyLock proving raw mode=ro fails under lock while OpenReadOnly waits and succeeds (make test-q1 and make lint exit 0)
**Priority**: P2 (Medium)
**Severity**: Minor
**Category**: Bug
**Related**: 1078, 1181

---

## 1. Problem & Motivation
During live host smoke verification alongside the active `harnez-agent-collector.service`, running `harnez stats agents` failed once with `SQLITE_BUSY` before succeeding on an immediate retry.

## 2. Technical Specification / Findings
- `internal/telemetry/telemetry.go:187` (`Open`) opens the SQLite telemetry database with `path + "?_pragma=busy_timeout(5000)&_pragma=journal_mode(WAL)"`.
- `internal/telemetry/telemetry.go:241` (`OpenReadOnly`) opens the database with `path + "?mode=ro"`, omitting `_pragma=busy_timeout(5000)` (or a bounded read-only retry/busy timeout matching `Open`).
- When the collector holds a write/checkpoint lock briefly, concurrent read-only CLI callers fail immediately with `SQLITE_BUSY` instead of waiting up to `busy_timeout`.

## 3. Implementation & Verification Plan
- Append `_pragma=busy_timeout(5000)` (preserving `?mode=ro` read-only semantics) in `telemetry.OpenReadOnly` (`internal/telemetry/telemetry.go:237-241`).
- Add a concurrency regression test in `internal/telemetry/telemetry_test.go` holding an active SQLite write lock across `OpenReadOnly` + query within the `busy_timeout` window and verifying the reader waits and succeeds instead of returning `SQLITE_BUSY`.
- Verify with `go test ./internal/telemetry` and `make lint`.
