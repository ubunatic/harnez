# 790 — harnez init --docs creates stray go.work symlink and skips managed doc refresh

**Status**: Open
**Priority**: P2 (Medium)
**Severity**: Minor
**Category**: Bug
**Related**:

---

## 1. Problem & Motivation
Reported by a cloud session (2026-10-16): running `harnez init -d . --docs git` in harnez
(1) created an untracked `go.work` symlink pointing to `../voxi`, and
(2) did not refresh the managed `docs/Git.md` copy after `docs/lang/Git.md` changed; it had to be fixed by hand.

## 2. Technical Specification / Findings
- Not yet reproduced locally.
- Check what in init/apply writes `go.work` and why it targets a sibling repo.
- Check the managed-doc copy path for `--docs <name>`: is the copy skipped when the destination exists?

## 3. Implementation & Verification Plan
- Repro test in a temp repo: `harnez init --docs git` must not create `go.work`, and must update a stale managed copy.
- Fix, then rerun the repro.
