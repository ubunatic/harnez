# 342 — MVP: harnez agent command and cross-agent dispatch from agy/claude host to codex/claude subagents

**Status**: Open
**Priority**: P1 (High)
**Severity**: Major
**Category**: Agentic Ergonomics / Infrastructure
**Related**: #417, #435, `docs/HarnezAgentArchitecture.md`, `docs/practices/AgenticLoop.md`, #479

---

## 1. Problem & Motivation

Cross-agent task delegation requires remembering disparate CLI flags and sandbox parameters across coding harnesses. Orchestrators in `agy`, `claude`, or `codex` need a clean, uniform command to start, resume, and manage subagent sessions with model shorthand resolution and clear reconnect instructions.

---

## 2. /goal & Acceptance Criteria

### /goal
Implement `harnez agent start`, `harnez agent resume`, `harnez agent stop`, and `harnez agent delete` with vendor model alias normalization, a one-time start reconnect banner, and automatic context compaction.

### Acceptance Criteria
1. `harnez agent start <tool>:<model>[:<tier>] "<prompt>"` parses shorthands (`codex:luna:low`, `codex:sol:low`, `claude:sonnet:low`, `agy:flash:low`) and dispatches the headless runner.
2. Emits a concise **Reconnect Banner** exactly once on start detailing how to resume, check status, stop, and delete the agent session.
3. `harnez agent resume <session_id> "<prompt>"` reconnects to the active session, checking the 100–150k token threshold and triggering auto-compaction when needed.
4. Full telemetry (turn tokens, cumulative tokens, cached tokens, duration) is recorded to `~/.harnez/tool_catalog.sqlite`.
5. Agent executing this ticket checks live codebase status before starting.

---

## 3. Implementation & Verification Plan

### 3.1 Model Shorthand Resolver (`internal/agent/resolver.go`)
- Resolve shorthands to vendor flags:
  - `codex:luna:low` -> `codex -a never -s danger-full-access exec -m gpt-5.6-luna --effort low`
  - `codex:sol:low` -> `codex -a never -s danger-full-access exec -m gpt-5.6-sol --effort low`
  - `claude:sonnet:low` -> `claude -p --model claude-3-7-sonnet-20250219 --effort low`

### 3.2 Command Tree & Reconnect Banner (`cmd/harnez/agent.go`)
- Implement `harnez agent start`, `resume`, `list`, `status`, `compact`, `stop`, `delete`.
- Render one-time reconnect banner on start.

### 3.3 Automated Verification Target
- `go test -v ./internal/agent/... ./cmd/harnez/...`
- Live smoke test verifying start -> reconnect banner -> resume -> teardown loop.

## Epic note (#479)

The CLI surface introduced here is being unified under epic #479 (`--name`, `--model`, `-d`, `-p`, `-c`, prompt files). Lifecycle, compaction and telemetry scope stays in this ticket.
