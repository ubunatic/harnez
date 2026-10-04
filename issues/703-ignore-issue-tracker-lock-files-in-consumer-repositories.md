# 703 — Ignore issue tracker lock files in consumer repositories

**Status**: Open
**Priority**: P3 (Low)
**Severity**: Minor
**Category**: Bug
**Related**: [Issue 672](672-harnez-issues-new-leaves-issues-reserve-lock-untracked-in-consumer-repos.md), [Issue 279](279-avoid-persistent-issues-readme-lock-sidecar-in-working-trees.md)

---

## 1. Problem & Motivation

Tilix ran `harnez init` before it had an `issues/` directory. Later, the first `harnez issues new` created the tracker, but `issues/.reserve.lock` and `issues/README.md.lock` remained visible as untracked files. Issue 672 covers `harnez init` when `issues/` already exists; it does not cover this order of operations. This leaves projects that add a tracker after initialization with dirty working trees after routine issue commands.

**Goal**: `/goal` Ensure first-time tracker creation after `harnez init` also configures consumer repositories to ignore tracker lock files; verify ticket allocation and index updates leave only intended changes, or document the required follow-up if automatic setup is not appropriate.

## 2. Technical Specification / Findings

The zero-byte lock files remained after the Tilix `harnez issues new` and `harnez issues open` commands completed and no Harnez process was using them. Adding `/issues/*.lock` to Tilix's local `.gitignore` hides them. Harnez's own repository has a broad `*.lock` ignore rule, and issue 279 tracks the README lock sidecar lifecycle separately.

## 3. Implementation & Verification Plan

Extend the first-use path so projects initialized before their tracker existed receive the same lock exclusions as projects that already had `issues/` during init. In a clean consumer repo, run `harnez init`, create the first ticket, update the index, and confirm `git status` shows only the intended ticket and index changes.
