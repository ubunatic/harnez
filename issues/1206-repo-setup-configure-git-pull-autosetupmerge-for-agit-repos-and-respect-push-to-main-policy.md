# 1206 — repo setup: configure git pull autoSetupMerge for AGit repos and respect push-to-main policy

**Status**: Closed — M1 and review refinements delivered (96f338d8, 54ee3cf9): harnez init configures local branch.autoSetupMerge=always and checkout.defaultRemote=origin on AGit repos, GuardPushTarget + harnez push-guard + pre-push enforce protected_main on AGit remotes while keeping GitHub mirrors/solo/release working, plus synced Git/AGit docs; make lint, touched packages, and un-skipped internal/githooks (17 subtests) passed.
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

## Decision

- **AGit detection in `harnez init`**: Detect AGit repositories when ANY of: (1) `.harnez/repo.yaml` has `git.mode: team` (`protected_main: true` or `mode: team`), (2) the local `origin` remote URL resolves to a `kind: forgejo` host in `spec/pr.yaml`, or (3) repo-local `remote.origin.push` configures `refs/for/`. Only touch repo-local git config (`git config --local branch.autoSetupMerge always` and `checkout.defaultRemote origin`) inside actual git worktrees, skipping silently on non-git directories and skipping writes under `--dry-run`. Do NOT overwrite if not AGit-enabled (`solo` + non-AGit remote leaves config untouched). (Host `/decide`, 2026-11-19.)
- **Push-to-main policy parity**: Provide a shared `gitflow.GuardPush` / policy helper matching `GuardCommit`: when `git.protected_main` is `false` (or `git.mode: solo`), pushing directly to `refs/heads/<default>` is allowed without hindrance; when `git.protected_main` is `true`, direct pushes to `refs/heads/<default>` are refused with a pointer to `refs/for/<default>/<topic>` (while `refs/for/...`, topic branches, and explicit `/disable repo-rules push-main` / `--push-main` override remain allowed). Wire this check consistently in `scripts/git-hooks/pre-push` (`internal/githooks`) and check `internal/guard` / `internal/release` so direct-push vs AGit-only policies never invert. (Host `/decide`, 2026-11-19.)

## M1 — AGit `git pull` ergonomics & push-to-main policy enforcement

- Write set (expected): `internal/claude/init.go` (+ helper file if clearer), `internal/gitflow/gitflow.go`, `internal/githooks/` & `scripts/git-hooks/pre-push` (if hook updated), `internal/guard/` (if push guard exists/added cleanly), `internal/claude/init_test.go`, `internal/gitflow/gitflow_test.go`, `internal/githooks/pre_push_test.go`, `docs/lang/Git.md`, root `docs/Git.md` (via `make install && ./harnez init -d .`), `docs/practices/AGit.md` & `docs/AGit.md` (if needed for pull config guidance).
- Locked tests (temp git repos + `t.TempDir()`, never touching global `~/.gitconfig`):
  1. `harnez init` on a temp git repo with AGit enabled (`protected_main: true`, or `solo` + `codeberg.org` `origin` URL, or `remote.origin.push = HEAD:refs/for/main/topic`) sets local `branch.autoSetupMerge=always` and `checkout.defaultRemote=origin`, is idempotent on re-run, and enables `git pull` on a newly created branch tracking `main` (`git checkout -b topic origin/main` / `git branch --track`).
  2. `harnez init` on a temp git repo with `mode: solo` and a non-AGit remote (e.g. local bare `/tmp/...` or `github.com` without `refs/for/`) leaves `branch.autoSetupMerge` and `checkout.defaultRemote` unset, and non-git `t.TempDir()` exits 0 cleanly without errors.
  3. Push policy differentiation (`gitflow` + `pre-push` hook):
     - `protected_main: false` / `solo`: pushing `refs/heads/main` is allowed.
     - `protected_main: true`: pushing `refs/heads/main` directly is refused with AGit `refs/for/...` guidance; pushing `refs/for/main/<topic>` and `refs/heads/feature` succeeds cleanly.
  4. Docs synced (`TestRootDocCopiesMatchSources` passes).

## Refinements (from `rev-1206` review of `96f338d8`)

1. **[HIGH] Remote-aware `GuardPush` (do NOT block GitHub mirror or non-origin/non-AGit pushes)**:
   - Pass remote name & URL from `scripts/git-hooks/pre-push` (`harnez push-guard --remote "$1" --url "$2" "$remote_ref"` -> `gitflow.GuardPushTarget` / `GuardPush` reusing `spec/pr.yaml` `ForgeFor`!).
   - When `remoteURL` (or `git remote get-url <remote>`) resolves to a non-AGit forge (`kind: github`), OR when a named non-`origin` remote is neither `origin` nor a `kind: forgejo` URL (e.g. a fork or GitHub mirror), `GuardPush` allows direct push to `<default>` cleanly (so `github` mirrors and `both:*github.com*` keep passing `GuardPush` while `origin` and `both:*codeberg.org*` / Forgejo URLs still enforce `protected_main`). Fix the `both` test in `internal/githooks/pre_push_test.go` so `both` refuses Codeberg `main` AND allows GitHub `main` when matching `origin/main`!
2. **[HIGH] `harnez release` + docs accuracy (`internal/release/push.go`, `push_pr.go`, `docs/lang/Git.md`, `docs/practices/AGit.md`)**:
   - Inspect `internal/release/push.go:70` and `push_pr.go:150` to verify how release behaves with `scripts/git-hooks/pre-push` installed:
     - First push tries direct push (`push.go` -> `push_pr.go` fallback) OR AGit `refs/for/...`; mirroring after merge pushes `refs/heads/main` to secondary/GitHub remotes (allowed by item 1!). Verify with a test covering release push / mirror with `GuardPush` / `pre-push` installed (especially ensuring `release --continue` mirroring to GitHub is NEVER blocked by `push-guard`).
   - Reword `docs/lang/Git.md` and `docs/practices/AGit.md` (+ regenerate root copies via `make install && ./harnez init -d .`) so they state the exact truth without false claims.
3. **[MEDIUM] Stale `harnez` binary without `push-guard` on `PATH`**:
   - In `scripts/git-hooks/pre-push`, check BOTH `command -v harnez >/dev/null 2>&1` AND `harnez push-guard --help >/dev/null 2>&1` before enforcing `push-guard`; if `harnez` lacks `push-guard`, warn via `printf '%s\n' ... >&2` and skip without breaking AGit/topic pushes! Add a test with a fake `harnez` failing `push-guard --help`.
4. **[MEDIUM] Bare-remote `git pull` test**:
   - Add a test using a bare `origin` remote where after `harnez init` (with `autoSetupMerge=always` & `checkout.defaultRemote=origin`), `git checkout -b topic origin/main` tracks `origin/main` and plain `git pull` fetches and merges a new commit pushed to bare `origin`'s `main`.
5. **[LOW] Bash style (`docs/Bash.md`)**:
   - Replace `echo >&2` in our new `scripts/git-hooks/pre-push` block with `printf '%s\n' ... >&2`.
