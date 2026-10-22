# 848 — Generated-index merge driver: verify ticket delete or rename on one branch

**Status**: Open
**Priority**: P2 (Medium) — a stale row fails pre-commit lint, which blocks every later commit
**Severity**: Major
**Category**: Bug
**Related**: 847 (merge driver, M4 row union 2039af9f)

---

/goal A merge where one branch deletes, renames or renumbers (`issues mv`) a ticket or study produces a
merge commit equal to a fresh `harnez index`, with a clean tree and passing pre-commit lint, verified by a
real-CLI scratch-repo E2E; or stop and report when blocked.

## Problem

847 M4 makes the driver union generated rows from ancestor/ours/theirs. Untested: a row removed on one
side (ticket deleted, renamed, renumbered, or a study removed). A union may keep the stale row; then
`issues lint --cached` in pre-commit fails and blocks commits until someone runs `harnez index`, the
same failure mode that 847 M3 had.

## Test

Scratch repo, plain `harnez init`; branch A runs `harnez issues mv` on a ticket (or deletes a study),
branch B adds a ticket; merge; assert on `git show HEAD:<file>` and on a clean `git status`.

## Searcher findings (2026-10-22, lean sprint)

- Driver: `cmd/harnez/issues_rebase.go:628`; row merge `internal/index/index.go:170`. Confirmed by reading: row in
  O and A, absent in B → `theirsRow == base` path at line 200 keeps A's stale row.
- Fix: absence on one side with the other side unchanged from O = deletion; drop the row. Keep conflicts for
  competing edits. Rename = old key deleted, new key added.
- Test: extend `cmd/harnez/issues_index_init_merge_e2e_test.go` (real CLI, scratch repo): rename and delete on one
  branch, new ticket on the other; assert `git show HEAD:<file>` has no stale row and `git status` is clean.
