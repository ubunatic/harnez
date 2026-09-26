# 592 — agent: runtime token watchdog stops, compacts or kills running agents

**Status**: Closed — Runtime token watchdog with mid-turn stream monitoring and process group kill escalation
**Priority**: P2 (Medium)
**Severity**: Moderate
**Category**: Feature
**Related**: [591](591-agent-default-auto-compact-threshold-200k-tokens-for-all-agents.md)

---

## Goal

`/goal`: Implement a runtime token watchdog for running agent turns that detects threshold crossings in active sessions, requests task stopping and session compaction, and cleanly terminates the process group if recovery fails or exceeds grace limits.

## 1. Problem & Motivation

Ticket #591 enforces configured token limits before dispatching each prompt. However, long-running agent turns can generate massive context expansions or enter tool loops mid-turn, blowing past token budgets before the next turn starts.

A runtime token watchdog is required to:
- Monitor live running turn token metrics (via streaming heartbeat records and runtime token tracking).
- On threshold crossing: trigger an interrupt/stop, compact the session context, verify the compaction drop, and resume.
- If compaction or graceful stopping fails within a bounded grace period: cleanly terminate the entire process group (ensuring Zero Zombie guarantee) and report the kill reason to the orchestrator.

## 2. Technical Specification & Findings

### Control & Recovery Sequence
1. **Canary & Control Probing**:
   - Verify interrupt & compaction feasibility across supported providers (Claude, Codex, AGY). Note: Codex uses internal auto-compaction (`model_auto_compact_token_limit`), while Claude/AGY accept `/compact` commands.
2. **Watchdog Runtime Monitor**:
   - Track tokens accumulated during streaming turns.
   - When `tokens >= compact_threshold_tokens`, trigger the watchdog intervention.
3. **Graceful Recovery vs. Hard Termination**:
   - Attempt graceful interrupt and compaction.
   - If unresponsive after grace timeout (e.g. 15–30s), kill the child process group (`syscall.Kill(-pgid, syscall.SIGKILL)`).
   - Record watchdog intervention events in telemetry.

## 3. Sprint Milestones

- **M1 — Canary Probing & Watchdog Interface**:
  - Implement canary tests for mid-turn token monitoring and interruption control.
  - Define `TokenWatchdog` struct with injectable process killer, token monitor, and clock in `internal/subagent/`.
- **M2 — Runtime Watchdog Wiring & Graceful Recovery**:
  - Wire `TokenWatchdog` into synchronous and streaming agent drivers.
  - Implement graceful interrupt $\to$ compact $\to$ verify recovery loop with process group termination fallback.
  - Add comprehensive unit and mock tests; verify with `make test-q1`.
- **M3 — Verification & Sprint Teardown**:
  - Verify clean tree and close ticket #592.

## 4. Acceptance Criteria

- Active turns exceeding the configured token limit are intercepted by the runtime watchdog.
- Unresponsive sessions exceeding the grace period are cleanly killed with no orphaned child processes.
- Watchdog events are logged with session name, token count, and outcome.
- `make test-q1` passes across the repository.

