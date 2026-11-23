---
kind: sprint
ticket: 1045
date: 2026-11-22
branch: sprint/1045
---

# Two-Agent Lean Sprint: 1045 — Generated Index Drift After Fast-Forward Pulls

## 1. Searcher (`s1045`, `claude:haiku:low`, 47s)
- Codex was at its monthly quota, so `cheap` fell back from `luna` to `haiku`.
- Root cause: `StudiesTable` used `os.ReadDir`, so untracked notes and dotfiles reached `docs/README.md`. The trigger was a scout facts dotfile after `git pull --ff-only`.
- The `--cached` lint renders from a `checkout-index` temp snapshot, and 1042's `merge=union` does not apply to fast-forwards.

## 2. Developer (`d1045`, `claude:haiku:high`) milestones
| Milestone | Commit | Content |
|---|---|---|
| Plan | - | Read-only turn. Found the temp-dir render, the `maintenance`→`index` import cycle, duplicate entries for unmerged paths and the staged-edit hint loop. |
| M1 (tracked-only studies) | 2da5631d | `ls-files -s -z` plus one `cat-file --batch`, dotfiles skipped, non-git fallback, ff-pull e2e |
| M2 (pull workflow) | 93559eb8, f28f0f4d | Hook env scrub, snapshot render without git, untracked-study hint, linked-worktree e2e, docs; the help golden regeneration also fixed 1269 |
| M3 (review hardening) | ef587a23 | `--stage` skips hand edits, `-z` porcelain, test isolation |
| M4 (rewritten candidates) | 58b037a8 | Every `--stage` candidate is held to the hand-edit rule; unmerged files are reported |

## 3. Reviews
- **M1** (`agy:neon:med`): found a high-severity bug. Hook-exported `GIT_DIR` made the lint and post-merge list the wrong tree. No test ran in a linked worktree.
- **M2** (`agy:neon:med`): `--stage` would stage hand-written prose, and porcelain-quoted paths were dropped.
- **M3** (`claude:haiku:low`, 39s): files that `--stage` rewrote were still staged unchecked, and drift tests were missing.
- Each review found a real gap that the host's diff review had passed.

## 4. Verification
- Every milestone ended with a dev Quota-1 run with no `--- FAIL`. M1's only failure was the pre-existing `TestHelpGolden` (1269).
- Host checks:
  - `make install && harnez init -d .` left the tree clean.
  - A live scratch repo with the installed hooks was tested: an ff pull with an untracked dotfile study leaves no drift; an issue-only commit passes the pre-commit lint; a prose hand edit stays unstaged with a warning; generated drift is staged.
- The host's own rerun was blocked by Quota-1 because no source had changed since the developer's run. It was not bypassed; the developer quoted the run's log instead.

## 5. Learnings
- Git hooks may export `GIT_DIR`, `GIT_INDEX_FILE` and related variables. Any git call in code that hooks reach must scrub them or pass explicit paths.
- `harnez init` installs the issue hooks only when `issues/README.md` exists. The host's first live check silently ran without hooks.
- The "plan only" first turn paid off. It surfaced four design facts before any code was written.
