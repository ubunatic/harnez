---
kind: sprint
date: 2026-10-30
tickets: [1039]
---

# Lean Sprint 1039: Per-Repo Git Workflow Mode

Goal: a `git:` key in `.harnez/repo.yaml` with three modes (`solo`, the default; `team`; `fork`) and an optional
`protected_main` setting. The goal also covered harnez commit and release paths, which are to warn on or refuse
the default branch.

## Agents

| Role | Session | Model | Result |
|---|---|---|---|
| Searcher | search-1039 | codex:luna:low | Gave the line ranges of the repo.yaml loader and schema and of every auto-commit path. Confirmed that the existing `init --repo-mode` setting was never saved. |
| Architect | arch-1039 | claude:opus | Designed the `internal/gitflow` leaf package and split the work into milestones M1 to M5. It also locked the test names and set the scope cuts. |
| Developer | dev-1039 | claude:sonnet:high (`std`) | Wrote a plan-only first turn with 8 corrections, then did M1–M2 and M3–M5 in two write turns. |

## Plan corrections adopted

The Developer's plan corrected the architecture in eight places. The ones that mattered:

- **Guard position:** the guard now runs before the first file write in `issues mv`, rebase and reconcile. Placed
  where the architecture put it, a refusal would have left a renamed or repaired tree uncommitted.
- **No-op close:** closing an already-closed ticket on a protected main stays a no-op instead of being refused.
- **Solo releases:** release refuses to push `main` only when `protected_main` is set, so solo releases keep
  working.

## Milestones

| Milestone | Commit | Scope |
|---|---|---|
| M1 (schema and loader) | 96251950 | `spec/git.yaml` and its schema, the repo.yaml `git` key, `gitflow.Load`, `DefaultBranch` and `ProtectedBranches` |
| M2 (commit guard) | 24753219 | `GuardCommit` in ApplyStatus, `issues mv`, rebase and reconcile, plus a backstop in `gitAddAndCommit` |
| M3 (release guard) | dc33810f | Guard before the version bump, and the protected-branch push refusal |
| M4 (init persistence) | 46d99107 | `--git-mode` and `--protected-main` are saved to repo.yaml and rendered per mode; the solo output is pinned by goldens |
| M5 (docs and skills) | d40fe2e0 | `docs/lang/Git.md`, GoRelease and the sprint and issue skills; this repo is set to team mode with protected main |

## Verification

- The host reran the tests of every touched package after M2; all passed.
- `make test-q1` fails only in the known stale `TestGoldenFontAssets`. `make lint` reports 213 issues, the same as
  before the sprint.
- Scratch repo with the installed binary:
  - `init --git-mode team --protected-main` saved the mode.
  - `issues close` on `main` was refused with rc 1.
  - The same close on `issue/1-probe` committed.
  - `--git-mode solo --protected-main` was rejected with rc 1.
- `harnez init -d .` reported no changes, and the working tree stayed clean.

## Observations

- The plan-only first turn paid off: 3 of its 8 corrections would otherwise have been bugs found in review.
- Having two review gates (after M2, then after M5) kept the host down to two diff reviews.
- The Developer used about 3.1M tokens, cached tokens included, in 6m20s for M1–M2.
- Cut by design: forge API calls and PR creation, tagging after the merge, and automatic branch switching.
