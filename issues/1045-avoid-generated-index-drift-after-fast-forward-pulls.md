# 1045 — Avoid generated index drift after fast-forward pulls

**Status**: Open
**Priority**: P2 (Medium)
**Severity**: Moderate
**Category**: Bug
**Related**: 1042, 1044; `.gitattributes`; `docs/README.md`

---

## Problem & Goal

After a fast-forward pull of main in this checkout, a post-merge hook regenerated `docs/README.md` to include a newly pulled study, leaving an unstaged index change. A subsequent issue-only commit failed its generated-index lint because the staged snapshot lacked that update. `merge=union` handles conflicting merge changes, not a fast-forward or subsequent hook-generated drift. Agents should not have to discover and untangle this after ordinary pulls.

/goal Make generated index handling after pulls reliable for agents, including fast-forwards, without losing user changes or silently staging unrelated work. Verify the resulting commit workflow, or stop and report when blocked on a user decision or denied permission. On a loop, hard architectural or OS blocker, or no progress, clear the goal and record a failure report and resume point.

Before implementation, verify current hook, index, lint and merge-driver behavior and recent history; distinguish the fast-forward case from issue 1042's concurrent merge conflicts. Determine an appropriate remedy rather than assuming `merge=union` solves both. Exercise real Git pulls and issue-only commits with clean and dirty worktrees, generated docs and issue indexes, and genuine merge conflicts; ensure user edits are preserved and report any unavoidable manual steps.

## Findings (Searcher, 2026-11-22)

- post-merge hook (`internal/claude/init.go:1388-1393`) runs `harnez index -d <top>` without `--stage`; pre-commit runs `harnez issues lint --cached`, which renders from the staged snapshot (`internal/issues/maintenance/rebase.go:779-811`, `:848-866`).
- `StudiesTable` (`internal/index/index.go:597-621`) enumerates `docs/studies` via `os.ReadDir`: untracked files and dotfiles are indexed. Observed 2026-11-21: an untracked `docs/studies/.story-1121-facts.md` landed in `docs/README.md` after `git pull --ff-only`, leaving unstaged drift; `harnez index --stage` would re-render the worktree and re-add it, so the lint hint can loop.
- A fast-forward runs no merge, so `merge=union` (1042) does not apply; 1042 stays separate.
- The lint's untracked-file hint (`rebase.go:781-783`) covers `issues/` only.

## Decision

- Studies index enumerates files in git's index (`git ls-files docs/studies`), skipping dotfiles; outside a git repo fall back to `os.ReadDir` minus dotfiles. Reason: matches the `--cached` lint's staged snapshot, so worktree-only notes never reach the index and a staged new study is still indexed.
- post-merge keeps regenerating without staging. Reason: staging there would silently stage unrelated work; with tracked-only enumeration a correct pulled commit produces no drift.

- Study titles are read from git's index blobs (one `git cat-file --batch`), worktree only in the non-git fallback. Reason: a staged-then-edited study would otherwise make the worktree render differ from the `--cached` lint, so `harnez index --stage` would loop.
- `studyFiles` is a local helper in `internal/index` (stdout-only `exec`, `-z`, `git -C docsDir`). Reason: `maintenance` imports `index` (cycle), and `gitOutput` trims leading spaces.
- Unmerged studies are deduped. Reason: `ls-files -s` lists them once per stage, and the merge driver renders docs during conflicts.
- The untracked-study hint goes into the docs-drift message (`rebase.go:809`), collected via `porcelainStatus` with `--untracked-files=all`. Reason: the 781-783 hint is issues-only.
- The e2e (a) ff-pull reproduction lands red-first in M1. Reason: reproducing it in M2 would require reverting M1.
- `harnez index --stage` also stages a drifted issues README; this is accepted, and e2e (b) assumes no such drift.

## Milestones

### M1 — Tracked-only studies enumeration
- `StudiesTable` uses `git ls-files` (tracked + staged) and skips dotfiles; non-git fallback skips dotfiles.
- Tests first: untracked study ignored; dotfile (tracked or not) ignored; staged-but-uncommitted study included; non-git fallback.

**M1 delivered (2da5631d): tracked-only studies enumeration.** `studySources` reads `git ls-files -s -z` + one `cat-file --batch`, skips dotfiles, symlinks and submodules, dedupes conflicted paths, and falls back to `os.ReadDir` outside git. It adds 8 unit tests and an ff-pull e2e (`cmd/harnez/issues_pull_index_e2e_test.go`). Quota-1 run 21:35Z: only `--- FAIL: TestHelpGolden`, which is pre-existing and unrelated; filed as 1269. Review: `r1045m1` (agy:neon:med).

### M2 — Pull + commit workflow verification and lint hint

**Pre-Work / Required Refinements (from the M1 review, do these first):**
1. [high] Hook env leak. Git hooks may export `GIT_DIR`, `GIT_WORK_TREE` and `GIT_INDEX_FILE` (absolute `GIT_DIR` in linked worktrees). `gitInWorkTree`/`ls-files` then resolve against the wrong tree:
   - the `--cached` lint's temp-dir render (`rebase.go:805`) lists the real repo's studies, or none;
   - post-merge `harnez index` can drop every study row.
   Fix both:
   - (a) The lint's temp-tree render selects the directory source explicitly instead of relying on git detection.
   - (b) The git calls in `internal/index` run with `GIT_DIR`, `GIT_WORK_TREE`, `GIT_INDEX_FILE` and `GIT_PREFIX` removed from the env, so `git -C docsDir` discovery is authoritative.
   Tests:
   - a unit test with `GIT_DIR` set to another repo;
   - an e2e in a linked worktree (`git worktree add`): an issue-only commit passes the pre-commit lint, and post-merge keeps the study rows.
2. [med] Test isolation. Set `HOME`, `GIT_CONFIG_GLOBAL=/dev/null`, `GIT_CONFIG_NOSYSTEM=1` and `commit.gpgsign=false` in `internal/index/index_test.go` git helpers and every git call in `issues_pull_index_e2e_test.go`. Some currently pass a nil env.
3. [med] Surface `ExitError.Stderr` in the `ls-files` and `rev-parse` errors. In `catFileBlobs`, reject `size < 0` and require the terminator byte to be `'\n'`.
4. [low] Assert exact rows rather than substrings: `TestStudiesTableUsesStagedContent` should check the full `| [studies/x.md](studies/x.md) | Staged topic |` row, and the ff-pull e2e should check the full row.
5. [low] Symlink parity: `dirStudySources` also skips symlinks, matching the git source.
6. [med] Document that a new study must be staged (`git add`) before `harnez index` lists it, while `issues/*.md` still reads the worktree. Cover it in the `harnez index` help (`cmd/harnez/index.go`) and the IssueTracking docs (below). After changing help, regenerate `cmd/harnez/testdata/help.golden` in its own commit `test(help): regenerate help golden (issue 1269)`. That also clears 1269's stale `pr wait --fixup` text; check that the golden diff contains only those two help changes.

- E2E in a scratch repo (extend `cmd/harnez/issues_index_init_merge_e2e_test.go` or sibling): (a) ff pull with an untracked study and a user-edited unrelated file → worktree index clean, user edit preserved, issue-only commit passes the pre-commit lint; (b) pulled commit whose `docs/README.md` is stale → post-merge regenerates, lint fails with an actionable hint, the hinted command fixes it staging only `docs/README.md`, user edits preserved.
- Extend the untracked hint at `rebase.go:781-783` to `docs/studies/`.
- Update `docs/practices/IssueTracking.md` (+ lite source if present) and its root copy via `harnez init -d .`, if the hook/lint behaviour description changes.
