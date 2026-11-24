# 1265 — Make harnez-land propose a complete cleanup plan after returning to main

**Status**: Open — skill updates (`HarnezLand.md`, `Wrap.md`, `config.yaml`) and unit tests merged in PR #220 (`244d829d`) and applied; remaining: optional live scratch-repo acceptance verification before closing
**Priority**: P2 (Medium)
**Severity**: Minor
**Category**: Agentic Ergonomics
**Related**: [#1239](1239-add-a-user-invocable-harnez-skill-that-commits-pushes-and-safely-switches-to-latest-main.md), [#1060](1060-add-a-branch-cleanup-skill-with-optional-autonomous-decisions.md), [Harnez Land](../docs/commands/HarnezLand.md)

---

## 1. Problem & Motivation

In TAIA, /harnez-land verified [PR #364](https://codeberg.org/harnez/taia/pulls/364) and synced main. The user still had to ask how to clear the leftovers. The initial report suggested rebasing nine branches whose patches were already on main. One branch held a unique test for an older protocol, and one linked worktree held uncommitted tests. Deleting merged branch names also left detached worktree directories behind.

## 2. Goal & Acceptance Criteria

/goal Make /harnez-land propose and, where already authorized, complete a plan that leaves a clean, synced default checkout with no leftover worktrees, topic branches or cleanup-related open PRs except items explicitly retained or blocked. Stop and report when a user decision or denied permission blocks progress. On a loop, hard architectural or OS blocker, or no progress, clear the goal and record the failure and resume point.

- After syncing the default branch, inventory linked worktrees (including detached ones), local topic branches and relevant PRs. Check ancestry, tree and patch equivalence, and inspect unique commits, local edits, and PR state before recommending a disposition.
- Present one concrete cleanup plan. Recommend removal for integrated work, recovery or archival for useful local work, and merge or closure for outstanding PRs as appropriate. Identify worktrees with uncommitted or untracked files and show how to preserve them. Reuse the branch assessment from #1060 where available.
- Carry out authorized steps, bundle any remaining decisions into one approval request, and verify the final state. Report every retained or blocked item with its reason and next action. Preserve unique work and seek approval before irreversible cleanup beyond the user's authorization.
- Recheck current behavior before implementation. Verify the workflow with patch-equivalent and unique commits, a dirty worktree, a detached worktree, and open and closed PRs.
