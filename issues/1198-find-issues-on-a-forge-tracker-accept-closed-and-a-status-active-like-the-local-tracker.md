# 1198 — find issues on a forge tracker: accept closed and -a status:active like the local tracker

**Status**: Open
**Priority**: P2 (Medium)
**Severity**: Minor
**Category**: Bug
**Related**: 1139 (forge status filters), 1195 (forge create), skill /trial

---

/goal In a repo whose tickets live on the forge, `harnez find issues closed` and `harnez find issues -a status:active`
list forge tickets (closed, resp. open) instead of failing with "no local issue tracker". Stop and report when
blocked on a user decision; on a loop or no progress, clear the goal and write a failure report here.

## Problem

Reported by the taia session (2026-11-09, codeberg.org/harnez/taia, harnez c792e7e8):
`harnez find -d . issues closed` errors "find: no local issue tracker (issues file does not exist) ...".
1139 already maps `status:open`/`status:closed` to the forge state query, so the bare `closed` word and the
`-a status:active` form are what remain. The /trial and /issue skills use both forms.

## Want

- `closed` on the forge path behaves as `status:closed`; `-a status:active` as `status:open` (forges have no
  In Progress/Blocked/Draft; say so in help).
- Other local-only queries keep a clear error that names the forge equivalent.
- Tests next to 1139's (`TestFindStatusFilterSendsStateToForge`).
