# 1192 — harnez release on protected main goes through an AGit PR, tags after the merge

**Status**: Open
**Priority**: P1 (High)
**Severity**: Major
**Category**: Bug
**Related**: 1070, 1039, 1189, docs/practices/GoRelease.md

---

## Goal

/goal `harnez release` on a protected default branch needs no elevated push permission: it pushes
the bump commit as an AGit PR, pauses, and `harnez release --continue` after the merge pushes the
tag, mirrors the other remotes and publishes. Done when a test covers pause, update-before-merge
and tag-after-merge, the docs say so, and v0.5.6 ships this way. Stop and report when blocked on a
user decision or denied permission; on a loop or no progress, clear the goal and write a failure
report here.

## Problem

1070 let release run on protected main and left the push to the forge. The v0.5.6 release proved
Codeberg refuses it ("Not allowed to push to protected branch main"), so a release needed the owner
to relax branch protection.

## Delivered

- `internal/release/push_pr.go`: on protected main, fetch the forge's default branch from its push
  URL; bump commit not merged → push `refs/for/<default>/release-vX.Y.Z` and pause (exit 0);
  merged → push the tag, mirror the merged branch and tag to the other remotes, fast-forward local.
- GitHub forge: refused with a pointer to the `release/vX.Y.Z` route (no AGit).
- Test `TestReleaseOnProtectedMainGoesThroughPR` with a bare remote that rejects `main`.
- GoRelease docs (full and lite) updated.

- Review (sonnet): refuses a tag on a merge commit (a tag recreated after pulling main). Not
  covered by tests yet: mirroring to a second remote, the ff of local main, squash merges.

## Open questions

- Squash or rebase merge drops the bump commit; `--continue` keeps pausing. Detect and say so?
- Forgejo tag protection, if enabled, would refuse the tag push; unverified on Codeberg.
