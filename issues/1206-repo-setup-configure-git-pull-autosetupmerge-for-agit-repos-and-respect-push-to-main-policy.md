# 1206 — repo setup: configure git pull autoSetupMerge for AGit repos and respect push-to-main policy

**Status**: Open
**Priority**: P2 (Medium)
**Severity**: Moderate
**Category**: Feature
**Related**: 1062, 1205, docs/lang/Git.md

---

## 1. Problem & Motivation

In AGit-enabled repositories (such as `taia` on Codeberg), contributors submit PRs via `git push origin HEAD:refs/for/main -o topic=...` instead of publishing feature branches on the remote.

When developers branch off `main` to work on sprints or topics, Git by default creates branches without upstream tracking. Running plain `git pull` on these branches fails with:
`There is no tracking information for the current branch. Please specify which branch you want to merge with.`

Configuring `branch.autoSetupMerge always` and `checkout.defaultRemote origin` ensures that newly created branches automatically track `main` (or the parent branch), allowing plain `git pull` to work seamlessly across the repository.

### Respecting Repository-Specific Push-to-Main Policy
Repository permissions are not uniform across projects:
- **Direct push allowed**: In some repositories, direct pushes to `main` are authorized and expected. Harnez must continue to permit direct push workflows in those repositories without introducing unnecessary blocking detours.
- **Protected main (AGit only)**: In other repositories (such as `taia` on Codeberg), pushing directly to `main` is forbidden and rejected by the server; all changes must go through AGit PRs.
- **Primacy**: Any feature that Harnez builds for git workflow, issue commits, repo setup, and release pipelines must strictly respect whether the host repository permits or forbids direct pushes to `main`.

## 2. Technical Findings

1. `git config branch.autoSetupMerge always` sets up automatic tracking against the start-point branch (e.g. `main`), so `git pull` knows where to pull from.
2. `git config checkout.defaultRemote origin` eliminates ambiguity when checking out branches that exist on the remote.
3. Repositories indicate their mode via `.harnez/repo.yaml` (`git.mode`, `git.protected_main`), remote capabilities (AGit refspec support), and remote branch protection. Harnez setup should detect AGit-enabled remotes and configure git pull ergonomics while honoring the repo's specific push policy.

## 3. Goal & Acceptance Criteria

/goal Configure branch.autoSetupMerge and checkout.defaultRemote in harnez repo-setup/init for AGit-enabled repositories so plain git pull works smoothly on newly created branches, and ensure Harnez git features respect per-repository push-to-main policies (allowing direct push where permitted and enforcing AGit/PR flow where main is protected), verified with tests and documentation; or stop and report when blocked on user input or a denied permission. On a loop, an architectural blocker, or no progress, clear the goal and write a failure report.

- `harnez init` / repo setup configures `branch.autoSetupMerge always` and `checkout.defaultRemote origin` when AGit flow is detected or enabled.
- Projects with direct push to `main` allowed retain full direct push capability.
- Projects with protected `main` enforce AGit PR creation and refuse direct pushes.
- Unit tests cover git config application and push policy differentiation.
- Documented in `docs/lang/Git.md`.
