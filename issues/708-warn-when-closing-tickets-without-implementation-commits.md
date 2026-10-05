# 708 — Warn when closing tickets without implementation commits

**Status**: Open — warn when closing without implementation history
**Priority**: P2 (Medium)
**Severity**: Minor
**Category**: Bug
**Related**:

---

## 1. Problem & Motivation
`harnez issues close N` can close a ticket even when the repository history contains no implementation change associated with N, allowing a ticket to be closed with only issue tracker edits.

## 2. Technical Specification / Findings
On close, inspect repository history for a commit whose message references ticket N and whose changed paths include files outside `issues/`. If none exists, print a warning to stderr while still succeeding. The check should use the ticket number robustly and exclude issue index/ticket paths.

## 3. Implementation & Verification Plan
Add a regression test proving close warns when no qualifying commit exists and remains successful. Run the test suite and `make install`.
