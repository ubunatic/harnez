# 676 — Add harnez usage --minimal mode

**Status**: Closed — minimal usage mode implemented and verified
**Priority**: P2 (Medium)
**Severity**: Minor
**Category**: Feature
**Related**:

---

## 1. Problem & Motivation
`harnez usage --compact` still includes dashboard chrome and a hidden-items list. Add a minimal mode for users who want the compact usage display without those elements.

## 2. Goal & Acceptance Criteria
`harnez usage --minimal` renders the compact usage view without its title bar, status bar, or `Hidden` list, while retaining the compact usage content.

## 3. Implementation & Verification Plan
Add the `--minimal` option and verify that its output omits those three elements and preserves the compact usage content.

**Goal**: Implement and verify the minimal usage view, or stop and report if blocked on a user decision or denied permission.
