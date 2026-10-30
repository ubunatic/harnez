# 1045 — Avoid generated index drift after fast-forward pulls

**Status**: Open
**Priority**: P2 (Medium)
**Severity**: Moderate
**Category**: Bug
**Related**: 1042, 1044; `.gitattributes`; `docs/README.md`

---

## Problem & Goal

After a fast-forward pull of main in this checkout, a post-merge hook regenerated `docs/README.md` to include a newly pulled study, leaving an unstaged index change. A subsequent issue-only commit failed its generated-index lint because the staged snapshot lacked that update. `merge=union` handles conflicting merge changes, not a fast-forward or subsequent hook-generated drift. Agents should not have to discover and untangle this after ordinary pulls.

/goal Make generated index handling after pulls reliable for agents, including fast-forwards, without losing user changes or silently staging unrelated work. Verify the resulting commit workflow, or stop and report when blocked on a user decision or denied permission. On a loop, hard architectural or OS blocker, or no progress, clear the goal and record a failure report and resume point.

Before implementation, verify current hook, index, lint and merge-driver behavior and recent history; distinguish the fast-forward case from issue 1042's concurrent merge conflicts. Determine an appropriate remedy rather than assuming `merge=union` solves both. Exercise real Git pulls and issue-only commits with clean and dirty worktrees, generated docs and issue indexes, and genuine merge conflicts; ensure user edits are preserved and report any unavoidable manual steps.
