# 745 — agent start/resume failures show no reason: read the provider session log for the real error

**Status**: Closed — Claude agent start/resume failures now surface the CLI final error line, preserve the original error as fallback, and have recorded usage-limit coverage.
**Priority**: P2 (Medium)
**Severity**: Minor
**Category**: Bug
**Related**: 595 (Codex case), 498 (claude opaque exit 1), `docs/OrchestratedAgentFlow.md` §4

---

## 1. Problem & Motivation

When a worker turn fails, `harnez agent start|resume` prints only the provider CLI's exit status
or its first stderr line. The host then has to dig through provider logs by hand, and a
transient cause (limit, overload) looks like a broken setup.

## 2. Technical Specification / Findings

Seen 2026-10-06/07:

- Claude (dev-735): `claude resume: exit status 1`. The cause was the last assistant line in
  `~/.claude/projects/<dir>/<session>.jsonl`: "You've hit your session limit · resets 12:20am".
- Codex (595): "Reading additional input from stdin..."; the cause was `server_overloaded` in
  the rollout `~/.codex/sessions/.../rollout-*<thread>.jsonl` (`"error"` on turn completion).

Both logs are known to harnez by session id. On a failed turn, harnez can read the tail of that
log, find the provider's error message, and print it with a class: limit (with reset time),
overloaded/transient (resume once), auth, other.

## 3. Implementation & Verification Plan

/goal A failed `harnez agent start|resume` prints the provider's real error and its class for
Claude and Codex (others: say what is possible), verified with recorded log fixtures for a
session limit, an overload and an auth error; or stop and report when blocked on an owner
decision or a denied permission.

- Fixture tests from the two real cases above (trimmed, no private text).
- When done, shorten the two pitfall rows in `docs/OrchestratedAgentFlow.md` and close or merge 595.

## Live check (2026-10-22, lean sprint): partly current, fixed together with 595/745

Codex: `internal/subagent/codex.go:391` returns the wait error without reading turn errors from the JSONL
stream/rollout (stderr wrapping was fixed earlier). Claude: `internal/subagent/claude.go:32` returns the CLI
exit error without its final error line. `cmd/harnez/agent_run.go:618` stores the raw error and `shortCause`
truncates it. Way forward: one provider error-enrichment step for failed start/resume (Codex rollout/stream
errors, Claude final error line), original error kept as fallback; fixtures for auth, overload and limit.
Docs: `docs/OrchestratedAgentFlow.md`.
