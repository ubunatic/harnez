# 1286 — Detect unpushed work on main that origin has since rewritten (loose-ends check verb/hook)

**Status**: Open
**Priority**: P2 (Medium)
**Severity**: Major
**Category**: Feature
**Related**: issue 836 (unpushed repos at wrap), `/harnez-land` skill, taia `make tidy` (`scripts/tidy.sh`), `docs/AgenticLoop.md` (Hygiene phase)

---

## 1. Goal

/goal Harnez notices loose ends early: commits on the local default branch that were never pushed, unmerged branches, dirty or stray worktrees, and stashes. It reports them at session start and at sprint teardown, and warns when origin has meanwhile changed the same files. Implemented as a harnez verb that a hook and the sprint skills call. Stop and report when the hook surface or the checks need an owner decision on blocking versus warning.

## 2. Problem (taia, 2026-11-24)

A lean sprint (taia #334, #330, #359) committed 12 sandbox fixes on local `main` and never pushed them. In parallel, another session landed #387 on `origin/main`, a rewrite of the same subsystem (`internal/sandbox`, `Runner.Call`, `spec/profiles.yaml`). By the next session, local `main` was 15 commits ahead of origin and 54 behind:

- None of the 12 commits applied any more. Porting them took a new four-milestone sprint.
- Cherry-picking blindly would have added a second protected-path list beside #387's spec-driven `deny_write`.
- A second copy of the same 15 commits sat in a stray worktree (`.taia/worktrees/lmcoder-taia`), with uncommitted work in `internal/tools/tools.go`. It survived only because `/harnez-land` audited every worktree before removing anything.
- Two more worktrees and branches were stale. Three commits had already reached origin through other PRs with different SHAs.

The repo rule says `main` is the only long-lived branch and `make tidy` runs at the end of each sprint or session. taia's `make tidy` would have flagged the unmerged commits and the stray worktree, but nothing made the sprint run it. The check lives in one repo's Makefile, so other repos don't get it at all.

## 3. Ask

Add a harnez verb, for example `harnez tidy` or `harnez loose-ends`, that works in any repo:

- Local default branch ahead of `origin/<default>`: list the commits, and say whether they are patch-equivalent upstream (`git cherry`).
- Behind as well (diverged): name the files that both sides changed (`git diff --name-only <merge-base> origin/<default>` intersected with the local-only diff), so a parallel rewrite shows up before it turns into a port.
- Unmerged local branches, linked worktrees (dirty, detached, or holding unique commits), and stashes.
- Exit codes that separate "clean", "loose ends", and "diverged with overlap", so hooks and the sprint skills can branch on them.

Wire it in:

- A SessionStart hook that runs the verb and prints a short warning when it finds anything.
- The teardown step of `lean-sprint`, `factory-sprint` and `/wrap`, which runs it and does not report done while loose ends remain (as taia's `make tidy` exits 1).
- Optionally a pre-sprint check: warn when the sprint's write set overlaps files that origin changed since the local base.

`/harnez-land` stays the interactive fix-up. The new verb is the cheap detector that tells the user when to run it.

## 4. Open questions

- Should the hook warn or block? A warning in the session-start banner is probably enough.
- Should the verb replace per-repo `make tidy` scripts or wrap them, by calling `make tidy` when the target exists?
- Fetch cost: should it run `git fetch` itself, or work from the last fetch with a staleness note?

## 5. Verification

- Scratch-repo test: diverged main with overlapping files, a stray worktree with uncommitted edits, a stash, and a squash-merged branch. Each one is reported, and the squash-merged branch shows as merged.
- Session-start hook output stays within a few lines and is silent on a clean repo.
