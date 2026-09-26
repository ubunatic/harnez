# 596 — agent start: preflight Codex auth before starting an agent

**Status**: Closed — Preflight Codex auth before start/resume with 1m TTL cache and fail-fast goal guidance
**Priority**: P2 (Medium)
**Severity**: Minor
**Category**: Bug
**Related**: 595, 592

---

## Goal

`/goal`: Fast-preflight Codex authentication before `harnez agent start` and `resume`, caching the check briefly so invalid credentials or OpenAI auth outages fail immediately with actionable guidance (halt loop / switch provider) instead of burning tokens and hanging on broken sessions.

## 1. Problem & Motivation

On 2026-09-26 two reviewer agent starts failed with Codex 401 Unauthorized while `codex login status` reported "Logged in". Each start burned ~40s, showed a misleading error (tracked in #595), and caused orchestrators to get stuck looping on dead goals.

- `codex login status` only inspects local credential files on disk, not server-side token validity.
- When OpenAI is experiencing an outage or tokens expire, agents repeatedly attempting to start Codex workers burn timeout budgets and loop endlessly.
- Orchestrators need immediate, actionable failure signals: stop the goal loop, re-authenticate via `codex login`, or consciously escalate/switch to Claude/AGY (with explicit cost awareness).

## 2. Technical Specification & Findings

### Fast Canary Preflight
- Perform a lightweight authenticated probe or check before spawning a Codex session:
  - Cache the preflight result with a short TTL (e.g. 60s) so rapid successive starts do not spam the auth endpoint.
  - If the probe returns 401 / Unauthorized or connectivity failure, abort immediately.
- Clear error message:
  ```
  Error: Codex authentication rejected (401 Unauthorized). OpenAI auth service may be down or local credentials expired.
  Run `codex login` to re-authenticate. If OpenAI is offline, halt the current goal loop or switch to an alternate provider (claude/agy, noting cost differences).
  ```

## 3. Sprint Milestones

- **M1 — Preflight Canary & Fast Auth Check**:
  - Implement the Codex auth preflight checker with short TTL caching in `internal/subagent/` or `cmd/harnez/`.
  - Wire preflight into `agent start` and `agent resume` prior to process spawn.
  - Fail fast with clear actionable error output.
- **M2 — Unit & Mock Tests**:
  - Add unit tests covering: valid cached auth, 401 rejection failure, network timeout handling, and bypassed checks for non-Codex providers.
  - Verify with `make test-q1`.
- **M3 — Verification & Sprint Close**:
  - Verify clean tree and close ticket #596.

