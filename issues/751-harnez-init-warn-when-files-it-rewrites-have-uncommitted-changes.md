# 751 — harnez init: warn when files it rewrites have uncommitted changes

**Status**: Closed — Implemented dirty-write warnings with repo-root path matching and symlink de-duplication. Tests: TestRunInit_WarnsForDirtyWriteSetFiles, TestRunInit_WarnsOnceForSymlinkTarget, TestRunInit_WarnsForUntrackedWriteSetFiles, TestRunInit_DoesNotWarnForCleanRepo, TestRunInit_DoesNotWarnOutsideGit, TestRunInit_DoesNotWarnForDirtyFileOutsideWriteSet, TestRunInit_WarnsForDirtyWriteSetInRepoSubdirectory. Verified go vet ./... and make install.
**Priority**: P3 (Low)
**Severity**: Minor
**Category**: Feature
**Related**: [381 — init discards local edits in bundled docs](381-init-silently-discards-local-edits-in-bundled-docs-without-stop-marker.md), [750 — AgenticLoop.lite markup](750-agenticloop-lite-md-heading-markers-inside-bullets-and-pointers-to-an-uninstalled-full-doc.md)

---

## 1. Problem & Motivation
On 2026-10-07 in cati, `harnez init --quota-1 --variant lite` ran while `AGENTS.md` had
uncommitted feature work (a new spec rule and a docs pointer). Init added its own lines to the
same file. To keep feature and init changes in separate commits, the agent had to split
`AGENTS.md` by hand: save the full file, strip its own lines, commit init, restore, commit the
feature. That worked, but it is error-prone and invisible: nothing told the user or agent
that init was mixing into uncommitted work.

**Goal**: Before writing, `harnez init` tells the user which files it will rewrite that already
have uncommitted changes, so they can commit first.

## 2. Technical Specification / Findings
- Files init writes in a typical run: `AGENTS.md` (`CLAUDE.md` is often a symlink to it),
  `.harnez/rules/*.md`, bundled `docs/*.md`, `Makefile` (target reconciliation).
- `git status --porcelain -- <paths>` on that set is enough to detect the overlap.
- Init must not refuse or stash: the user may intend to mix. A warning listing the
  overlapping paths, plus a suggestion to commit first, is enough. `-y` and non-interactive
  runs print the same warning and continue.
- Outside a git repo, or with a clean overlap set, print nothing.

## 3. Implementation & Verification Plan
- Compute init's write set before writing; intersect with dirty paths from git.
- Print one line per overlapping path to stderr: `warning: AGENTS.md has uncommitted changes; init will modify it (commit first to keep changes separate)`.
- Test: temp git repo with a modified `AGENTS.md` → warning names it; clean repo → no
  warning; non-git dir → no warning, no error.
