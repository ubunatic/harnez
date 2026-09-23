# 498 — harnez agent claude sessions cannot be resumed when harnez runs inside Claude Code

**Status**: Closed — Implemented Claude session ID persistence, working-directory resume, matching resume flags, and stderr reporting; live Claude check was blocked by the enforced developer leaf role.
**Priority**: P1 (High)
**Severity**: Major
**Category**: Bug
**Related**: 497, `internal/subagent/claude.go`, `docs/ModelAdvisoryEval.md`, `docs/practices/ModelRoles.md`

## /goal

`harnez agent resume` works for `claude:*` sessions started from inside a Claude Code session,
and a failing `claude` call reports its stderr.

## Problem

Found on 2026-09-22 during the 491 lean sprint. A Claude Code host started a developer with
`harnez agent start --role developer --model claude:sonnet` (session d1315dff…). The start
turn worked; the resume failed:

```
Error: agent resume "dev491s" (claude:sonnet:low) failed: claude resume: exit status 1
```

- No transcript for d1315dff (or for the earlier `claude:sonnet` advisor d38dc869) exists
  anywhere under `~/.claude/projects/`, so `claude -p --resume <id>` has nothing to load.
- The child `claude` inherits the host's environment: `CLAUDECODE=1`,
  `CLAUDE_CODE_CHILD_SESSION=1`, `CLAUDE_CODE_SESSION_ID=<host>`,
  `CLAUDE_CODE_MESSAGING_SOCKET`/`_TOKEN`, `CLAUDE_EFFORT`. Most likely one of these makes the
  child a non-persisted child session. Unverified: confirm with a canary.
- `ClaudeDriver.command` uses `exec.Cmd.Output()`, so stderr is dropped and the error says
  only "exit status 1".
- `ClaudeDriver.Resume` also omits `--dangerously-skip-permissions` and `--model`, which
  `Run` passes. So even a working resume would run with different permissions.

Impact: in a Claude-hosted sprint, every `claude:*` worker is single-turn. Plan-first
("read-only plan, then resume to write") and multi-milestone resumes don't work. The
workaround is a fresh session per milestone, with the context carried by the ticket.

## Plan

1. Canary: from inside Claude Code, run `claude -p … --output-format json "hi"` with and
   without scrubbing the `CLAUDECODE`/`CLAUDE_CODE_*` variables, and check whether a transcript
   appears and whether `--resume` then works. Record which variable matters.
2. Scrub the host-session variables from the child environment in `ClaudeDriver` (the provider
   CLI is a separate session, not a child of the host). Add a driver test on the env.
3. Capture stderr (`exec.ExitError.Stderr`) into the returned error.
4. Make `Resume` pass the same permission flag and `--model` as `Run`. Test the args.
5. Live check: start plus resume one `claude:haiku` turn from inside Claude Code.

## Finding 2026-09-24 (loom 107 sprint, dev107 73deb143…)

Canary refutes the inherited-env hypothesis: `claude -p --output-format json`
from inside Claude Code persists its transcript with and without
`CLAUDECODE`/`CLAUDE_CODE_*` unset. Root cause is in `ClaudeDriver`:
`parseClaude` ignores the JSON `session_id`, so `TurnResult.SessionID` stays
empty and harnez resumes with its own session UUID, which Claude never saw
(`No conversation found with session ID: 73deb143…`). Fix: parse
`session_id` into `TurnResult.SessionID` (as agy/codex do). The transcript is
stored under the child's cwd project dir, so resume must also run in the
same `-d`.

## Handoff 2026-09-24 (519 sprint)

flash37 (agy) planned the fix, then hit "Individual quota reached … Resets in 90h" on
resume; nothing was written. Moved to `codex:luna`. Fix, per the finding above (do not
strip environment variables, the canary refuted that):

- `parseClaude` reads `session_id` into `TurnResult.SessionID`.
- `ClaudeDriver` gets a `Dir`, set in `agentDriver`, so resume runs in the start `-d`.
- A non-zero exit includes the child's stderr (`exec.ExitError.Stderr`).
- `Resume` passes `--model` and `--dangerously-skip-permissions`, like `Run`.
- Tests for each, then an end-to-end `claude:haiku` start + resume from a Claude Code shell.
