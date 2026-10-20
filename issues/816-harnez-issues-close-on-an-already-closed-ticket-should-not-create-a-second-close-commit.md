# harnez issues close on an already-closed ticket should not create a second close commit

Status: Closed — Prevented duplicate close commits; explicit reason amendments and regression coverage delivered in d3b52aa3
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

## Delivery

- Implementation: d3b52aa3 (`fix(issues): prevent duplicate close commits (816)`).
- Already-Closed `close`/`done` calls preserve the ticket, README, and HEAD regardless
  of reason, and print the existing reason plus an `--amend-reason` hint.
- `--amend-reason` explicitly updates a closure reason in one commit; non-Closed
  targets error before writes, including mixed batches. Other status verbs retain
  their existing behavior.
- Tests first reproduced the changed-reason double-close path. Regression coverage
  verifies same/different/empty reasons, the done alias, exact ticket bytes and HEAD,
  one-commit amendments, invalid amendments, and batch validation/no-ops.
- Updated CLI help golden, generated-man-page assertions, full/lite issue-tracking
  docs and root copy, orchestration docs, and managed rule source/copy.
- Verification passed: `make lint`, `go test ./cmd/harnez/`, `make test-short`,
  focused close/help/man tests, and `git diff --check`. `make install` completed.
