# harnez issues close on an already-closed ticket should not create a second close commit

Status: Open
Priority: P3
Category: Bug

## Problem
Workers closed tickets twice, leaving two close commits per ticket (loom 289, 290, 324 on
2026-10-17; harnez 803 with a placeholder hash first). The second call rewrote the status.

## Expected
Closing a Closed ticket is a no-op with a message, unless `--amend-reason` is given, which
updates the reason in place without a new ticket-status flip.

## Acceptance
- Second `harnez issues close` on the same ticket creates no commit and exits 0 with a notice.
