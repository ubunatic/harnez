# 1060 — Add a branch cleanup skill with optional autonomous decisions

**Status**: Backlog — grooming 1083 (2026-11-03): active list focused on taia support, P1s and open bugs; promote when needed
**Priority**: P2 (Medium)
**Severity**: Minor
**Category**: Feature
**Related**: 1058, 1059

---

## Problem & Motivation

Stale branches can contain merged, superseded or genuinely unmerged work; ancestry alone misses squash merges and updated PR heads. In this session two unpublished issues needed recovery under new numbers, while an old feature branch contained superseded code and a unique obsolete design document. Provide a reusable branch cleanup workflow that avoids losing work.

## Goal & Acceptance Criteria

/goal Add and verify a branch cleanup skill that inventories, assesses, preserves and safely cleans up branches, or stop and report when blocked on a user decision or denied permission. On a loop, hard architectural or OS blocker, or no progress, clear the goal and write a failure report with the resume point. Recheck current skills, repository state and recent history before implementation.

- Inventory current worktree and fresh local/remote refs; compare unique commits and content with the target branch and relevant PRs, including squash merges and changes after a merged PR head.
- Classify work as merged/equivalent, superseded, useful or uncertain; recover useful work into an appropriate PR or issue, reconcile ticket number collisions, and preserve unrelated or unresolved branches.
- Default to presenting decisions and seeking owner approval before discarding unique work or deleting remote branches. After required PRs merge and disposition is approved, recheck refs, clean up only the approved branches, and report preserved, merged, deleted and outstanding work with final status.
- Support an explicit fully automatic invocation: delegate questions normally put to the user to an independent agent on another model, which assesses evidence and recommends a disposition. Document decision authority, audit trail and escalation when evidence or permissions are insufficient; automatic mode must not silently bypass higher-priority safety rules, permissions or repository protections.
- Never infer merge equivalence from a branch name or PR state alone; avoid force-pushes and account for refs moving between inspection and deletion. Test normal and automatic paths, uncertain or unique work, stale refs and denied operations.
