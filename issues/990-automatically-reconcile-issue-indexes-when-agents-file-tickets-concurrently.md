# 990 — Automatically reconcile issue indexes when agents file tickets concurrently

**Status**: Open
**Priority**: P2 (Medium)
**Severity**: Moderate
**Category**: Bug
**Related**: #232, #280, #859, #976, #988

---

## 1. Problem & Motivation
Two agents filing or updating tickets in one checkout can leave the shared Git index and generated issue indexes out of sync. While filing #988, another agent had staged tracker changes, an index lock blocked staging, and a ticket-only commit failed tracker validation because its README entry was missing. A separate temporary Git index worked, but left a stale staged entry for the other agent. This should resolve automatically without losing either agent's work or requiring manual Git commits.

## 2. Goal

/goal Make issue filing and tracker validation automatically reconcile concurrent agents' ticket and index changes, and verify the result, or stop and report when blocked on a user decision or denied permission. On a loop, a hard architectural or OS blocker, or no progress, clear the goal and write a failure report explaining why.

`harnez index` or the Git check should account for concurrent filing and find a resolution automatically; when in doubt, recreate the generated index files and commit. Preserve unrelated staged and unstaged changes and ensure ticket metadata and committed indexes agree.

Consider adding `harnez issues update` to bring all unstaged issues into good shape, and `harnez issues update --commit` for one issue plus its indexes. Determine the exact single-issue selection syntax during implementation. These flows should handle the tracker commit themselves, without requiring plain Git commits.

## 3. Verification

Reproduce two agents filing or updating issues in the same checkout, including a stale staged index and lock contention. Verify automatic reconciliation, valid committed tracker indexes, and preservation of both agents' work and unrelated staged changes. Re-check current code and recent history before implementation.
