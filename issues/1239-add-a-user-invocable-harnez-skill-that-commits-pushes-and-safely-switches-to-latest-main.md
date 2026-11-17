# 1239 — Add a user-invocable harnez skill that commits, pushes and safely switches to latest main

**Status**: Open
**Priority**: P2 (Medium)
**Severity**: Minor
**Category**: Feature
**Related**: 1062 (safe AGit push and pull ergonomics), 1205 (ticket commits on protected main), 1039 (git.mode team)

---

## 1. Problem & Motivation

After a topic branch merges, getting back to a clean `main` takes several manual steps: commit leftovers, push through AGit, wait for the merge, switch, fast-forward. Doing it by hand is slow and easy to get wrong. The user wants one mechanical command that brings the repo to a clean state at the latest `main`. `/wrap` covers a wider session wrap-up; this command is narrower and should not need judgement.

## 2. Technical Specification / Findings

- Invocation: user-invocable only, as a harnez skill or command (name to be chosen in the implementation).
- Flow:
  1. Commit the working tree on the current topic branch. Ask for a message only if the tree has no obvious summary.
  2. Push through AGit: `git push origin HEAD:refs/for/main/<topic>` (see `docs/AGit.md`). Never push `refs/heads/main`.
  3. If the push opened a PR, optionally start `harnez pr wait <number> --contains <sha>` in the background.
  4. Switch to `main` and fast-forward it to `origin/main` with `--ff-only`. Do not start the switch while the tree is dirty.
  5. Report unmerged local branches and open PRs, and propose one action for each: delete, keep, push, or rebase.
- Team mode with protected main (`.harnez/repo.yaml`, `git.mode: team`, `protected_main: true`): direct pushes to `main` are refused, so AGit is the route.
- Existing precedent: PR #179 for the fish env change took this path by hand on 2026-11-17.

## 3. Implementation & Verification Plan

- Keep the skill prose short. The skill should be mechanical, not a judgement step.
- Tests: a scratch repo with a topic branch, an AGit-style remote and a dirty tree. The command must commit, push, switch and fast-forward. It must refuse a dirty switch and never force-push.
- Verify the unmerged-branch report on a repo with one merged and one unmerged branch.

/goal Implement the skill, verify it on a scratch repo, and stop and report when blocked on a user decision or a denied permission.

## 4. Implementation

- Skill `/harnez-land` (`docs/commands/HarnezLand.md`), registered in `config.yaml` as
  user-invocable-only; install test `internal/claude/harnez_land_skill_test.go`.
- Merged-branch check uses `git merge-tree --write-tree origin/main <branch>` against
  `origin/main^{tree}`, so squash merges count as merged; `git branch --merged` misses them.
- Scratch-repo check (bare remote, AGit-style push, squash merge from a second clone): the
  squash-merged topic was reported merged, an unmerged branch was reported unmerged, a dirty file
  showed in `git status --porcelain`, and `git merge --ff-only origin/main` advanced `main`. A
  local bare remote has no AGit, so the PR-number and `harnez pr wait` steps were not exercised.
