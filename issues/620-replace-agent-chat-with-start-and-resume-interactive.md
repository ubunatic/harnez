# 620 — Replace agent chat with start and resume --interactive

**Status**: Closed — delivered in c8d4f3a: chat removed, start/resume -i with flag conflicts, busy-session refusal, opening prompt (claude, codex, agy --prompt-interactive)
**Priority**: P2 (Medium)
**Severity**: Minor
**Category**: Usability / Agent Orchestration
**Related**: #439, #449, #450, #485

---

/goal Remove `harnez agent chat` (incl. `chat attach`) and provide the same behaviour via
`harnez agent start -i` and `harnez agent resume -i`, with tests and updated docs/man page;
stop and report when blocked on a user decision or denied permission.

## 1. Problem & Motivation

`agent chat` (start a user-facing interactive session) and `agent start` (agents dispatching
subagents) are two CLI paths for creating the same kind of session. `chat` inherits
persistent flags it ignores (`--role`, `--timeout`, likely `--allow-exhausted-quota`), and its
help has no examples. The user also wants to take over a session an agent started and
continue it interactively — `chat attach` already does this, but under a confusing name.

Decision (user, 2026-09-28): drop `chat` outright (solo repo, no deprecation alias) and add
`-i/--interactive` to `start` and `resume`.

## 2. Technical Specification / Findings

- Shared layers already exist: session store, `subagent.ResolveModel`, and
  `subagent.InteractiveRunner` (`Chat`/`Attach` in `internal/subagent/interactive.go`).
  Duplication is only CLI wiring in `cmd/harnez/agent.go` (session create + completed/failed
  teardown, copied between `chat` and `attach`).
- `start -i` ≙ today's `chat`; `resume -i` ≙ today's `chat attach`.
- `--role` stays meaningful for `start -i`: `start` resolves role defaults from it.
- Rules for `-i`:
  1. Reject (clear error, not silent ignore) combinations with `--detach`/`--async`,
     `--timeout`, `--json`, stream mode.
  2. Prompt / `-f` files become the opening message where the provider CLI accepts one
     (claude, codex; check agy).
  3. `resume -i` refuses while a background turn is running (point to `agent wait`).
  4. Sessions without a provider session ID stay non-resumable with the existing clear error.
- #450 (input line after resize) lives in the interactive runner and carries over unchanged.

## 3. Implementation & Verification Plan

- Add `-i` to `start`/`resume`, reuse one interactive launch+teardown helper, delete `chat`.
- Tests: flag conflicts, session record lifecycle for `start -i`/`resume -i`, busy-session
  refusal, opening-prompt argument building per provider.
- Update docs, man page, completion; note the replacement in #439 and #450.
- `make install`; manual smoke: `start -i`, exit, `resume -i` of an agent-started session.

## 4. Delivery

M1 (single milestone) delivered in c8d4f3a: shared launch/teardown in `cmd/harnez/agent_interactive.go`,
`chat` removed, `make test` green, `make install` done. The repo has no man-page generator, so no man page was updated.

Manual verification (user, 2026-09-28): `start -i` works for agy, codex and claude — slash-command
menu, a user-invocable skill, and Ctrl+G external editor (nvim on alt screen) all return the cursor
to the right line. `resume -i` works for claude; codex and agy fail to resume → #622.
