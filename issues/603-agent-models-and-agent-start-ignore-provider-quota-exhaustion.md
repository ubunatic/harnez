# 603 — agent models and agent start ignore provider quota exhaustion

**Status**: Open
**Priority**: P1 (High)
**Severity**: Major
**Category**: Bug
**Related**: 023 (harnez usage quota tracking), 104 (AGY quota collector)

---

## 1. Problem & Motivation
In a neus `/lean-sprint 19` run (2026-09-27), the host picked `agy:flash38:med` from
`harnez agent models` and `harnez agent start` accepted it and reported "Started agent".
The agy provider was out of quota. Nothing told the host: `agent models` has no quota
column, and `agent start` does not check quota before starting. The session was
stopped with 0 tokens used, so it never did any work. The user had to catch the mistake.

## 2. Technical Specification / Findings
- `harnez agent models` shows COST/EFF/SKILLS/ROLES but no availability or quota state.
- `harnez agent start` launched a detached session on an exhausted provider without a warning.
- Quota data already exists (`harnez usage`, the shared quota cache from 033, the AGY collector from 104).

## 3. Implementation & Verification Plan
- `agent models`: mark or hide models whose provider quota is exhausted, using the cached quota
  state (showing "unknown" when there is no data).
- `agent start`: fail closed, or at least warn loudly, when the chosen provider is known to be
  exhausted, and name cheaper available alternatives.
- Tests: a fake quota cache with agy exhausted gives the marker in `agent models` and makes
  `agent start --model agy:...` refuse (or warn).
