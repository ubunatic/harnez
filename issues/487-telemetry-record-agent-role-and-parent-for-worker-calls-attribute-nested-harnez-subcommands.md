# 487 — Telemetry: record agent role and parent for worker calls, attribute nested harnez subcommands

**Status**: Open

**Priority**: P2 (Medium)
**Severity**: Moderate
**Category**: Agentic Ergonomics
**Related**: #486 (roles), #479 (epic), #116 (`harnez stats`), `docs/feedback/2026-09-22-orchestrated-sprint-flow-report.md`

---

## 1. Problem & Motivation

During the first orchestrated sprint the host wanted to know which commands each
agent used. The telemetry database could not answer it; the Codex rollout logs
(`~/.codex/sessions/.../rollout-*.jsonl`) had to be read instead.

Findings from that analysis (`~/.harnez/tool_catalog.sqlite`, window 2026-09-21T23:50Z on):

- `tool_calls` has rows per Codex thread (`agent_id=codex`, `session_id` equal to the
  thread id), but no column for the agent's role or its parent. Orchestrator and
  developer rows cannot be told apart except by knowing the thread ids.
- `cli_invocations` mostly logs the `exec` wrapper. The orchestrator ran twelve
  `harnez agent ...` calls and two `harnez issues done` calls; the table has one row for
  any Codex worker session in that window and none of those subcommands. Nested
  `harnez` commands run by workers are therefore invisible.
- `~/.local/share/harnez/telemetry.db` is an empty 0-byte file next to the real DB
  (`~/.harnez/tool_catalog.sqlite`); it misled the first analysis.

## 2. Specification

- Add `agent_role` and `parent_session_id` columns to `tool_calls` and
  `cli_invocations`, filled from `HARNEZ_AGENT_ROLE` and `HARNEZ_SESSION_ID` (both set by
  `harnez agent`, see #486). Empty for hosts without a role.
- Record the real subcommand of a wrapped harnez call (`harnez exec -- harnez agent start`
  is stored as `agent start`, with the wrapper noted), so per-subcommand counts and
  failures per session become queryable.
- `harnez stats` gains `--role <name>` and a per-role breakdown (calls, failures, top
  tools/subcommands) next to the existing per-agent one.
- Migration adds the columns without touching existing rows; old rows stay NULL.
- Remove or explain the stale empty `telemetry.db` path (one canonical DB path).

## 3. Verification

- Unit tests for the schema migration, column population from the environment, wrapped
  subcommand extraction, and the `--role` filter and breakdown.
- Live check: repeat an orchestrated sprint and answer "which harnez subcommands did the
  developer thread run" from `harnez stats` alone.
