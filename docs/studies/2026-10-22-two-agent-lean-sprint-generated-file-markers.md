# Two-Agent Lean Sprint: generated-file markers and merge driver (issue 847)

Date: 2026-10-22. Host: Claude Opus 5.5. Searcher `s847` (luna:low), developer `d847` (luna:high).

## Searcher

Found that much of 847 already existed (an init-installed merge driver for `issues/README.md`, lint
freshness for the issue indexes). That narrowed the scope to notices, block markers, driver coverage for
BACKLOG/CLOSED/docs README, and a studies-table freshness check.

## Milestones

| Milestone | Commit | Host review |
|---|---|---|
| M1 top notices + block markers | 66b8387b | OK |
| M2 driver for all generated files, freshness | 27503f72 | FAILED: plain `init` installed no driver |
| M3 init installs driver by default | fa6a7493 | FAILED: merge commit dropped incoming rows |
| M4 driver merges rows from both sides | 2039af9f | OK |

## What the host review caught

Twice the developer's tests were green while the real flow failed:
- M2 tested the driver through an internal function, not through a real `harnez init`.
- M3 asserted on the working tree after the post-merge hook had already repaired it. The merge commit
  itself lacked the other branch's rows, the hook left a dirty tree, and pre-commit lint then blocked
  every later commit.

Lesson: for git integration, the host's end-to-end check uses a scratch repo, the real CLI, and asserts
on `git show HEAD:<file>` plus a clean `git status`.

## Final verification (scratch repo, installed binary)

Two branches each add a ticket and a study: merge exits 0, the merge commit holds all 3 studies, all 3
tickets and the hand-written tail, and the tree is clean. Conflicting hand-written edits in
`docs/README.md` stop the merge with normal conflict markers.

Not tested: a ticket deleted or renamed on one branch (the row union may keep a stale row; lint would flag it).
