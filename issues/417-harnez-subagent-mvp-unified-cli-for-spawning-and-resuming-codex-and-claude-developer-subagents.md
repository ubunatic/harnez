# 417 — harnez agent MVP: unified CLI runner and token telemetry for Codex, Claude, and AGY subagents

**Status**: Open
**Priority**: P1 (High)
**Severity**: Major
**Category**: Agentic Ergonomics / Infrastructure
**Related**: #342, #435, `docs/HarnezAgentArchitecture.md`, `docs/practices/AgenticLoop.md`

---

## 1. Problem & Motivation

Spawning subagents must provide full token telemetry (`tokens_cumulative`, `tokens_turn`, `cached_tokens`) and automated compaction to avoid context bloat across multi-turn handoffs.

---

## 2. /goal & Acceptance Criteria

### /goal
Implement the underlying `Driver` engine for headless Codex, Claude, and AGY session execution and resumption with structured token telemetry parsing, auto-compaction triggers, and SQLite logging.

### Acceptance Criteria
1. `internal/subagent/driver.go` defines the universal `Driver` interface for `Run`, `Resume`, `Compact`, `Stop`, and `Delete`.
2. `internal/subagent/codex.go` implements Codex execution extracting session IDs, tokens used, and return values.
3. Automatically triggers compaction when cumulative context exceeds 100–150k tokens.
4. Outputs structured JSON with `tokens_cumulative`, `tokens_turn`, `cached_tokens`, `duration_ms`, and `reconnect_cmd`.
5. Agent executing this ticket checks live codebase status before starting.

---

## 3. Implementation & Verification Plan

### 3.1 Subagent Driver Backend (`internal/subagent/`)
- Implement `Driver` interface and provider drivers (`codex.go`, `claude.go`).
- Parse stdout/stderr and JSON stream payloads for token telemetry.

### 3.2 Auto-Compaction & KV-Cache Guard
- Check token thresholds before dispatching turns in resumed sessions.
- Invalidate stale sessions if idle beyond provider KV cache limits (~5-10m).

### 3.3 Automated Verification Target
- `go test -v ./internal/subagent/...` with mock CLI streams and integration checks.
