# 975 — harnez find issues should list the Priority

**Status**: Open
**Priority**: P2 (Medium)
**Severity**: Minor
**Category**: Feature
**Related**: 194 (harnez find issues next), 318 (issues list verb)

---

/goal `harnez find -d <repo> issues -a status:active` prints each active ticket's Priority (P0-P3) next to its status,
verified by a test and a live run; or stop and report when blocked on a user decision or denied permission.

## 1. Problem & Motivation

`harnez find -d <repo> issues -a status:active` lists number, status, title and file path, but not the ticket's
Priority. Users and agents must open each file to see how urgent a ticket is before choosing what to work on next.

## 2. Technical Specification / Findings

- The text listing is produced by the issue find path (`cmd/harnez/issues.go`, `runIssuesList` and the find output
  it feeds). Confirm the exact column source in code before changing it; the Priority is already in each ticket's
  `**Priority**:` header line.
- Keep the existing columns and their order; add Priority as a new column. Show the code only (`P0`-`P3`), not the
  parenthesised label, to keep the rows short.
- Tickets without a Priority header show a blank cell rather than failing the listing.

## 3. Implementation & Verification Plan

1. Add the Priority column to the active-issue text listing; keep `-I/--image` output unchanged unless it shares the same row builder.
2. Add or extend a test that asserts the Priority appears in the listed row for a known ticket.
3. Verify with `harnez find -d /home/uwe/projects/harnez issues -a status:active`, then `make test-q1` and `make install`.
