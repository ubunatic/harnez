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

#### Milestone 1: Subagent Driver Engine & Provider Parsers (`internal/subagent/driver.go`, `codex.go`, `claude.go`)
- Define `Driver` interface: `Run(ctx, opts) (*TurnResult, error)`, `Resume(ctx, sessionID, prompt) (*TurnResult, error)`.
- Implement `CodexDriver`: invoke `codex exec -m <model> --dangerously-bypass-approvals-and-sandbox`, stream/parse session id, tokens used (input, output, cached), response.
- Implement `ClaudeDriver`: invoke `claude -p "<prompt>" --dangerously-skip-permissions --model <model>`, parse tokens and response.
- Add unit tests in `internal/subagent/driver_test.go` with mock output fixtures.

#### Milestone 2: Session Lifecycle Manager, Ancestry Tracking & Compaction Guard (`internal/subagent/session.go`)
- Implement `SessionStore` persisting active session metadata in `~/.harnez/agents/<session_id>.json`.
- Track `parent_session_id`, `caller_pid`, `harness_type`, `tokens_cumulative`, `last_active_at`.
- Enforce lineage invariant: an agent session can only stop or delete its own child descendants.
- Implement auto-compaction trigger when `tokens_cumulative >= 100k`.
- Add unit tests in `internal/subagent/session_test.go`.

#### Milestone 3: CLI Command Surface (`cmd/harnez/agent.go`)
- Implement `harnez agent start <provider>:<model>[:<tier>] "<prompt>"`, `resume`, `list`, `status`, `compact`, `stop`, `delete`.
- Print Reconnect Banner on start.
- Provide `--json` flag for machine consumption.
- Register `agent` command in `cmd/harnez/root.go`.

#### Milestone 4: End-to-End Test Suite & Verification
- Comprehensive tests for `harnez agent` commands, mock runners, and failure modes.
- Verify `make test` / `go test ./...`.

### 4. Milestone Progress & Execution Log
- [x] **Milestone 1: Core Driver Engine & Provider Parsers** (Delivered in commit `0b1b6d6`)
  - Universal `Driver` interface, `Model` resolver, and `TurnResult` telemetry types in `internal/subagent/driver.go`.
  - `CodexDriver` with JSON stream parsing and session/token extraction in `internal/subagent/codex.go`.
  - `ClaudeDriver` with JSON output extraction in `internal/subagent/claude.go`.
  - Complete unit test suite in `internal/subagent/driver_test.go`.

- [ ] **Milestone 2: Session Manager & Lineage Hygiene**
  - **Pre-Work / Refinement Instructions**:
    1. Implement `internal/subagent/session.go` and `internal/subagent/session_test.go`.
    2. Manage session records in JSON files under `~/.harnez/agents/<session_id>.json` (or customizable store directory).
    3. `Session` struct fields: `ID`, `Name`, `Provider`, `Model`, `Tier`, `WorkingDir`, `ParentSessionID`, `CallerPID`, `HarnessType`, `Status` (running, idle, parked, completed), `TokensCumulative`, `TokensTurn`, `CachedTokens`, `CreatedAt`, `LastActiveAt`.
    4. Provide methods: `Save(s)`, `Get(id)`, `List(opts)`, `Delete(id)`.
    5. Enforce Lineage Invariant: `CanManage(callerParentID, targetSession)` returns true only if `callerParentID == ""` (human/root) or target session is direct/indirect child of `callerParentID`.
    6. Implement Compaction Guard: `ShouldCompact(tokensCumulative)` (returns true when >= 100,000 tokens).
    7. Unit tests with full test coverage for store, lineage checks, and compaction triggers.

