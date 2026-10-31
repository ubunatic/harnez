# 1055 — Automate issue indexes after merges with a Forgejo workflow

**Status**: Open
**Priority**: P2 (Medium)
**Severity**: Minor
**Category**: Feature
**Related**: [1045](1045-avoid-generated-index-drift-after-fast-forward-pulls.md), [1042](1042-use-merge-union-in-gitattributes-for-generated-issue-indexes-to-avoid-forge-merge-conflicts.md)

---

## 1. Problem & Motivation

Generated indexes can drift after merges and fast-forward pulls, leaving a dirty working tree. Add a Forgejo `harnez indexer` workflow that runs after merges to `main` and pushes only regenerated index files to `main` itself.

## 2. Goal & Acceptance Criteria

/goal Implement and verify the Forgejo indexer workflow so merged changes leave tracked indexes current, or stop and report when blocked on a user decision or denied permission. On a loop, hard architectural or OS blocker, or no progress, clear the goal and record a failure report explaining why and where to resume.

- Recheck current workflows, index generation, branch protection and recent history before implementing; define the exact generated index file allowlist.
- Commit and push only changed allowed indexes; no-op when unchanged. Avoid recursive workflow runs, races and overwriting concurrent changes; verify behavior with targeted tests or a safe canary.
- Use least-privilege credentials and document required Forgejo permissions and protected-branch configuration. If direct bot pushes are unavailable, report the blocker rather than weakening branch protection silently.
