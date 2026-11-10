# 1185 — agent list drops a Jules run from running when its local wait times out

**Status**: Open
**Priority**: P2 (Medium)
**Severity**: Major
**Category**: Bug
**Related**: 1089, 1095

---

/goal `harnez agent list` shows a Jules run as running while its Jules session is still open, whether or not a local wait is attached, with a test. Stop and report when blocked on a user decision or denied permission; on a loop or no progress, clear the goal and write a failure report here.

## Problem

Reported by the taia session on 2026-11-08 during a multi-repo Jules wave. When the local wait of
`harnez agent start --model jules` exits 3 (wait ran out, run goes on), `harnez agent list
--all-sessions` stops showing that run as `running`, although the session is still open on Jules
and still counts against the account's open-session cap (14 that night).

Hosts throttled starts with `harnez agent list --all-sessions | grep -c 'jules.*running'`. Taia runs
170 and 176 were missing from that count, so hosts saw 13 while 14 or more were open, and starts got
HTTP 400 "Precondition check failed".

Workaround in use: re-attach `harnez agent resume --name <session> wait` after every exit 3.

## Want

- `list` reports the Jules state (or a "remote open, not attached" state) for Jules runs, not only
  the local wait state.
- A test covers a Jules session whose local wait ended with exit 3.

## Notes

Not verified in code yet where `list` derives the state; check `harnez agent list` and the Jules
session record before changing anything. Whether `list` may call the Jules API per row (cost,
latency) is open; a cached remote state written by `status`/`resume` may be enough.

Starting point (checked 2026-11-10 at af9688e0, after a discarded Jules report with drifted references):
`cmd/harnez/agent_manage_cmds.go:91` filters rows on the local `x.Status == "running" || x.Status ==
"active"`; the RESUME column comes from `resumeState(x)` (`agent_manage_cmds.go:121`) and does not affect
the filter. `TurnResult.StillRunning` (`internal/subagent/driver.go:304`) marks a wait that ended while
Jules runs on.
