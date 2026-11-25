---
title: Git Conventions
weight: 63
---

<!-- harnez:bundled -->
# Git conventions

Applies unless overridden by project-local instructions for coding agents,
commonly stored in an `AGENTS.md` file at the repository root.

## Workflow
- Workflow follows `git.mode` in `.harnez/repo.yaml` (key absent = solo; set it with
  `harnez init --repo-mode <mode> [--protected-main]`; the project's Repo Setup restates it):
  - `solo`: work local, on the repo's default branch (usually `main`, sometimes `master`);
    no local feature branches.
  - `team` / `fork`: push in this order; a refused push falls through to the next step:
    1. Direct push to the default branch (`git push origin main`, fast-forward only, never force)
       when `protected_main` is not set and the forge or hook accepts it.
    2. AGit where the forge supports it (Forgejo, Gitea): `git push origin HEAD:refs/for/<default-branch>/<topic>`,
       no branch. Topics, the PR lifecycle and the `harnez pr wait` merge signal are in `docs/AGit.md`
       (`harnez init --docs agit`).
    3. A topic branch, one per ticket or sprint (`issue/<n>-<slug>`, `sprint/<n>`), and a PR. Forks and
       GitHub, which has no AGit, use this step.
  - `protected_main` (team/fork only): when true, the default branch refuses direct pushes, so use
    AGit or a branch. When false, direct pushes to the default branch are allowed (this repo sets
    `protected_main: false`). Never force-push.
    The pre-push hook enforces the `true` case for raw `git push` (see "AGit pulls and push policy").
    `harnez issues` commits may use `--push-main` or matching-session `/disable repo-rules push-main` when authorized; otherwise they refuse and print the `git switch -c` command. Other auto-committing commands remain guarded. `harnez release` is the exception: the admin, or an agent asked to release,
    may run it there; it tries a direct push first and falls back to an AGit release PR when that push is refused (see "AGit pulls and push policy").
- Commit proactively:
  - Default preference: commit autonomously after intermediate steps once tests are clean, after finished features (with review pass), between iteration attempts on a stuck bug, and immediately after filing issue-tracker entries.
  - Ask-first harness interaction: If running in a harness that mandates explicit user confirmation for `git commit`, ask the user **upfront before starting work / during session kickoff** for authorization to commit local checkpoints proactively. This enables the user to step away without returning to uncommitted progress.
  - Orchestrator delegation: When an orchestrating agent operates under an ask-first rule, it must clarify commit authority with the user at kickoff and specify whether spawned subagents commit directly or return diffs for the orchestrator to commit on their behalf.
- Review before commit:
  - small fixes: commit directly if all tests pass and existing test assertions remain intact
  - non-trivial changes / features: perform a dedicated review pass (host orchestration or a fresh reviewer subagent) before committing to verify test rigor, doc/ticket accuracy, and codebase clarity
- Do not push until the user confirms pushing. One confirmation covers the session:
  push later finished work without asking again, unless the user narrows or revokes consent.
- Solo: "push" means `main` and tags to both dev remotes. Ask again only for force pushes,
  new remote branches, or a remote that diverged. Team/fork: "push" means the push order above:
  the default branch when allowed, else AGit, else a topic branch.
- Solo: do not create remote branches or PRs unless asked. Team/fork: a direct push to the default
  branch needs no PR; AGit opens its PR with the push; a topic branch is delivered by a PR the host
  or owner opens (`tea`/`gh` per forge).

## Commit messages
- Use [Conventional Commits](https://www.conventionalcommits.org/en/v1.0.0/#summary):
  `type(scope)?: summary`, e.g. `feat(cli): add init command`.
- Common types: `feat`, `fix`, `docs`, `refactor`, `test`, `chore`.
- Mark a breaking change with `!` after the type or scope and a `BREAKING CHANGE:` footer, e.g.:
  ```
  feat(cli)!: drop the --old flag

  BREAKING CHANGE: the --old flag is removed; use --new instead.
  ```

## Hygiene
- Do not commit secrets.
- Do not commit large files unless tracked via git-lfs.
- Do not commit PII.
- Harnez hooks warn when starting a session or committing on a detached HEAD.
- Cross-repo tag and spec reads must never check out tags in a shared checkout; use
  `git -C <repo> show <tag>:<path>`, `git worktree add`, or scratch clones. Checking out
  a tag leaves the repository in a detached HEAD state and risks causing subsequent agent
  commits in that shared checkout to land off-branch.

## AGit pulls and push policy
- AGit repositories: `harnez init` sets two keys in the repo's own `.git/config` (never the global
  config): `branch.autoSetupMerge always` and `checkout.defaultRemote origin`. It does this when
  `git.mode` is `team`, when `origin` is a Forgejo host in `spec/pr.yaml`, or when `remote.origin.push`
  has a `refs/for/` refspec. Re-running init changes nothing. Solo repositories on GitHub or a local
  bare remote keep both keys unset.
- Why: a branch cut from a local `main` has no upstream by default, so plain `git pull` stops with
  "There is no tracking information". With `always`, the branch tracks its start point. A branch cut
  from a local `main` tracks the local `main` (`branch.<n>.remote` = `.`), so `git pull` merges that
  local copy, which may lag `origin/main`. To pick up Codeberg's `main`, run `git pull origin main`,
  or cut the branch from `origin/main`.
- Push policy: with `git.protected_main: true` in `.harnez/repo.yaml`, the pre-push hook
  (`scripts/git-hooks/pre-push`, installed by `make install-git-hooks`) refuses a direct push to the
  default branch on origin, and on any remote whose URL is a Forgejo host in `spec/pr.yaml`. The hook
  runs `harnez push-guard --remote <name> --url <url> <remote-ref>` for each ref and each push URL of
  the remote, and the refusal names `git push origin HEAD:refs/for/<default>/<topic>`.
  Not refused: AGit `refs/for/` pushes, tags, topic branches, and any push to a GitHub URL or to a
  named remote that is neither origin nor a Forgejo URL (a fork or a mirror). GitHub has no AGit, so
  this policy does not refuse a direct push of `main` to GitHub. Solo repositories and
  `protected_main: false` allow direct pushes. `/disable repo-rules push-main` in the owning session
  unlocks a direct push, as it does a protected commit.
- The hook can only refuse what git tells it: `git push both main` runs it once per push URL, so
  Codeberg's leg is refused first and GitHub is not reached. A `pre-push` hook also runs during
  `git push --dry-run`.
- A stale or missing `harnez` (one without `push-guard --remote --url`, or none on `PATH`) makes the
  hook warn and skip the policy instead of refusing every push. A skipped policy means a direct push
  to `main` is not refused; only the warning is shown. Run `make install` to restore the check.
- `harnez release` on a protected default branch tries a direct push first: a preflight dry-run, then
  the push, to each release remote. With the hook installed, the origin refusal comes from the hook
  before the forge is asked, and the release falls back to an AGit release PR, `refs/for/<default>/release-<tag>`.
  Without the hook, the forge's rejection decides, and the fallback runs only when its message names
  "protected"; any other rejection stops the release. The release then pauses until the PR merges
  (`harnez pr wait <n> --contains <sha>`), and `harnez release --continue` finishes it. The tag goes to
  origin first. Then the merged `main` and the tag are mirrored, atomically, to the other release
  remotes; a GitHub mirror passes the hook, but a mirror on a Forgejo URL is refused like origin. A
  failed mirror stops the release; retry with `--continue`. See `docs/GoRelease.md`.
- Without the hook (no `make install-git-hooks`), the policy is advisory: the forge's branch protection
  is the enforcement.

## Remotes
- Two dev remotes: Codeberg (`codeberg.org/<username>/<reponame>`) and GitHub (same path).
  Development happens on both; neither is a mirror.
- Codeberg is canonical for releases: `go get` resolves there, so tags go to Codeberg first.
- Keep both `main` branches equal: every `main` update fast-forwards and goes to both remotes.
  Never force either `main`; never set a push mirror (it overwrites the other side).
  A branch filter does not make a mirror safe: a Codeberg mirror filtered to `main` still
  coincided with every other GitHub branch being deleted and the open PRs on them closing.
- Check every destination for divergence before pushing anything. `harnez release` uses
  all Git remotes configured in the repo, checks branch and release tag updates first,
  then pushes both atomically per remote. A later connection failure or concurrent remote
  update can still interrupt synchronization; resolve the error and retry with `--continue`.
- `uman push` uses the upstream by default. It pushes multiple remotes only when
  `push.default_remotes` lists them in `.uman.toml`; it does not sync tags or check all
  destinations before the first push. Do not assume an unconfigured `uman push` syncs both.

### Keeping GitHub `main` level, without CI or a mirror
- Before a Jules session on `main`, harnez fast-forwards GitHub `main` to Codeberg's `main` with one
  plain push: no force, prune or mirror. A failed or diverged sync warns and names the commit Jules
  will read. Settings live in `spec/agent.yaml` (`jules.sync_*`).
- Owner's clone, optional: one remote `both` whose push URLs are Codeberg and GitHub, so one push
  sends `main` to both. `origin` keeps one URL, because a hook can fail a whole push but only for
  that remote:
  ```
  git remote add both <codeberg-url>
  git remote set-url --push both <codeberg-url>
  git remote set-url --add --push both <github-url>
  git config remote.pushDefault both
  git config remote.both.push refs/heads/main:refs/heads/main
  ```
  Use the exact strings that `git remote get-url --push origin` and `git remote get-url github`
  print for `<codeberg-url>` and `<github-url>`. The release skip compares URL strings exactly, so
  `.git` suffixes, `ssh://` against `https://`, or a trailing slash make it miss the match.
  A `pre-push` hook on this clone can refuse a push to GitHub unless Codeberg already holds the
  commit. `harnez release` skips a remote whose push URLs are all pushed by other remotes, so
  `both` does not run during a release.
  Pitfalls for such a hook: it runs during `git push --dry-run`, it can fail only the whole push to
  that URL (not one ref), and it runs with empty stdin when that URL is already up to date. A plain
  `git push` here sends `main` even from a topic branch. The hook lives in `.git/hooks`, so it is
  per clone and unversioned.
- Contributor clone on a forge with AGit: send each push to the topic's PR without typing the refspec:
  ```
  git config remote.origin.push HEAD:refs/for/main/<topic>
  ```
  Replace `<topic>` with the topic name in that clone.

### Branches from either remote
Cloud agents (Claude Code web, Jules, Codex) usually work on GitHub branches.
- At start and before the final push, merge Codeberg `main`:
  `git fetch https://codeberg.org/<username>/<reponame> main && git merge FETCH_HEAD`.
- Reserve ticket numbers after that fetch; on a clash, renumber yours.
- Integrate by fast-forward, then push `main` (and tags) to both remotes
  (`<cb>`/`<gh>` = your remote names, e.g. `origin`/`github`):
  `git merge --ff-only <branch>`, then check both destinations before any actual push:
  `git push --dry-run --atomic <cb> main --tags && git push --dry-run --atomic <gh> main --tags`.
  After both checks succeed:
  `git push --atomic <cb> main --tags && git push --atomic <gh> main --tags`.

<!-- harnez:stop -->
