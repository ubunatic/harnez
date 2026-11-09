# 1200 — agent delete: unrated-session refusal names the agent rate command to run

**Status**: Open
**Priority**: P3 (Low)
**Severity**: Minor
**Category**: UX
**Related**: 526 (unrated delete refusal)

---

/goal When `harnez agent delete` refuses an unrated session, the error names the exact command that rates it
(e.g. `harnez agent rate ... <1-5> "<reason>"` for that session) and `--force` as the alternative, so an agent
rates instead of guessing. Stop and report when blocked on a user decision; on a loop or no progress, clear the
goal and write a failure report here.

## Problem

Seen 2026-11-09 after sprint 1198/1199: `harnez agent delete --name dev-1198` printed only
`delete: refusing to delete unrated session "dev-1198" without --force`
(`cmd/harnez/agent_manage_cmds.go:487`, bulk form at :375). The host then ran `harnez rate` (the tool-call
rating, a different thing) and the sessions stayed unrated. `harnez agent rate --help` shows
`agent rate <1-5> <reason>` with no visible way to pick a session by name.

## Want

- Refusal text: `... unrated; rate it with: harnez agent rate <selector> <1-5> "<reason>", or pass --force`.
- If `agent rate` cannot target a named session today, add that (`--name`, like delete/resume), or state in the
  hint how it chooses the session.
- Test pins the message for single and bulk delete.

## Notes

- Unverified: how `agent rate` selects the session (latest in `-d`?). Check before writing the hint.
