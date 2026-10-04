# 703 — Ignore issue tracker lock files in consumer repositories

**Status**: Open
**Priority**: P2 (Medium)
**Severity**: Minor
**Category**: Bug
**Related**:

---

## 1. Problem & Motivation

In a Tilix checkout using Harnez issues, ticket allocation and index updates left `issues/.reserve.lock` and `issues/README.md.lock` visible as untracked files. Harnez's own `.gitignore` ignores `*.lock`, but consumer repositories may not have that rule, so routine issue commands dirty their working tree.

**Goal**: `/goal` Ensure Harnez issue commands do not leave tracker lock files as untracked consumer-repository changes; verify allocation and index updates in a clean repository, or document a safe setup requirement if the files must persist.

## 2. Technical Specification / Findings

The zero-byte lock files remained after the Tilix `harnez issues new` and `harnez issues open` commands completed and no Harnez process was using them. Adding `/issues/*.lock` to Tilix's local `.gitignore` hides them; the Harnez repository already has a broader `*.lock` ignore rule.

## 3. Implementation & Verification Plan

Review the issue tracker locking lifecycle and consumer setup. Either remove lock files safely after unlocking or make the ignore requirement part of Harnez's project initialization/docs. In a clean consumer repo, allocate a ticket and update the index, then confirm `git status` shows only the intended ticket and index changes.
