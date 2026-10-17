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
- go.work: harnez tracks a managed `go.work.example` that uses `../voxi`. On a fresh
  clone, `reconcileGoWorkspace` (Case 4) symlinked `go.work -> go.work.example` even
  though `../voxi` was absent, which breaks every go command. Fix: skip the symlink and
  print `go.work skipped: go.work.example uses missing ../voxi` until all `use` dirs exist.
- Doc refresh: reproduced with a current build, `init --docs git` does refresh an edited
  managed section (edits below `harnez:stop` are preserved by design). The skip happens when
  the installed binary predates the `docs/lang/Git.md` edit: docs are embedded at build
  time, so init "refreshes" to the old text. Fix: init warns when the on-disk copyable source
  differs from the bundled doc ("run make install, then rerun init").

## 3. Implementation & Verification Plan
- Repro test in a temp repo: `harnez init --docs git` must not create `go.work`, and must update a stale managed copy.
- Fix, then rerun the repro.
