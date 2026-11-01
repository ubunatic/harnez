# 1090 — Pick up the pending Jules implement run on 1087 and finish 1088

**Status**: Open
**Priority**: P2 (Medium)
**Severity**: Minor
**Category**: Chore
**Related**: 1087, 1088

---

/goal The pending Jules run below is checked, its findings are captured as `/jules` describes, and 1088 is closed, or stop and report when blocked on a user decision or denied permission. Circuit breaker: on a loop, a hard blocker or no progress, clear the goal and write a failure report here.

## Pending run

- Jules session ID: `5832791262383075829`
- URL: https://jules.google.com/session/5832791262383075829
- Harnez session name: `jules-implement-1087b` (local ID `3b47a40c-57de-486b-8194-a2b30ee62a98`), started from the machine that holds `~/.harnez/agents`
- Task: implement ticket 1087, based on `main` at `d6b738e3` or later. Do not send Jules a message.

## Pick up

- On the starting machine: `harnez agent status --name jules-implement-1087b`, then `HTO=0 harnez agent resume --name jules-implement-1087b wait` if it is still running. Exit 0 can mean the wait ran out, so read the state in the output.
- On another machine: open the URL above, and look for its PR with `gh pr list --state open --repo ubunatic/harnez`.

## Finish

1. Check out the PR's head and run the tests. Review it normally; any fix is made by us, not Jules. Merge or close it, and close 1087 only if the fix lands.
2. If the run ended with a question or no PR, record it as failed in 1088.
3. Update the `## Jules runs` table in 1088, and close 1088 and this ticket. Google stops accepting new Jules work on 2026-11-11, so a retry must start before then.
