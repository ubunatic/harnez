# 1070 — Allow plain harnez release from protected main for the repo admin

**Status**: Closed — Delivered: plain harnez release runs on protected main, other auto-commits stay refused; Repo Setup source fixed and AGit made the team default. make test-q1 passed, reviewer findings fixed. Not yet proven: that Codeberg accepts the admin's push of main; the first real release shows it.
**Priority**: P2 (Medium)
**Severity**: Minor
**Category**: Feature
**Related**: 1039

---

/goal Make a plain `harnez release` work on `main` for the repository admin and for agents acting for them, fix the Repo Setup rule that forbids those agents to push, and verify it in tests, or stop and report when blocked on a user decision or denied permission. On a loop, a hard blocker or no progress, clear the goal and write a failure report here.

## 1. Problem & Motivation
The repository admin runs `harnez release` on `main` and gets this refusal:

```
Error: git.protected_main is set in .harnez/repo.yaml: refusing to release on 'main'; run: git switch -c issue/<n>-<slug>
```

The admin can push to `main` anyway, so the refusal only adds a release branch and a PR to their
release. The rebase-or-fast-forward merge that this route needs to keep the tag on `main`'s
history (`docs/GoRelease.md`, "Protected main") is a further manual step that can go wrong.

## 2. Technical Specification / Findings
Check these against live code before starting:

- Two guards refuse: `gitflow.GuardCommit(opt.Dir, "release")` before the bump
  (`internal/release/runner.go`), and `refuseProtectedBranchPush` at push time
  (`internal/release/push.go`). Both must let this release through; `--continue` only hits the
  second.
- 1039 introduced the guards. `internal/release/gitflow_test.go` asserts the refusals and must
  change with them.
- **The rule forbidding agents to push is a bug (user's call).** AGENTS.md line "Do not commit to
  or push the default branch unless the repository admin explicitly authorizes it in the current
  session" blocks agents acting for the admin. Fix it. `.harnez/rules/Tools.md` still applies:
  agents release only when the user or `/goal` asks.
- **The rule's source is out of sync.** That line sits in the managed `Repo Setup` block of
  AGENTS.md, but `config.yaml` (about line 1136) generates "Never commit to or push the default
  branch." The AGENTS.md wording was hand-edited in 7c5f4c2e and the next `harnez init` overwrites
  it. Fix the `config.yaml` source. `docs/Git.md` (`protected_main`) needs the same change.

Open questions:

- **Who the guard still refuses.** With agents allowed there is no need to tell admin and agent
  apart, so no admin identity or agent detection is needed. The likely design is that harnez stops
  refusing the release and the forge's branch protection decides who can push. Confirm this does
  not remove the protection for other repos using `protected_main` without forge-side protection;
  ask the user if it does.
- **What happens if a remote rejects the push.** The admin reports that pushing works, but this is
  unverified for both dev remotes (Codeberg and GitHub). A rejection must fail clearly and leave
  `harnez release --continue` usable.

Answers from implementation:

- **Who the guard refuses:** nobody on a branch; `harnez release` is refused only on a detached HEAD.
  A new `gitflow.GuardRelease` lets it through on the protected branch and prints a note; every other
  auto-committing command still uses `GuardCommit` and stays refused. On 2026-10-31 this repo was the
  only one under `~/projects` with `protected_main: true`, so no other repo loses a guard.
- **Remote rejection:** `git push --dry-run` runs no server hooks, so the preflight cannot see branch
  protection and the rejection comes on the real push. `--atomic` keeps the tag off that remote, the
  error carries the remote's reason, and `--continue` finishes the release once the push is allowed.
  A test covers this with a `pre-receive` hook.
- **Unverified:** whether Codeberg's protection lets the admin push `main` (the API reports
  `protected: true`; `user_can_push` was only read anonymously). GitHub's `main` is unprotected. The
  first real release shows it. A rejection leaves the bump commit and tag local only.
- **Known consequence:** a release on `main` also pushes any other unpushed commits on it, unreviewed.

## 3. Implementation & Verification Plan
Done when all of these hold:

- A plain `harnez release` from `main` bumps, tags, pushes `main` and the tag, and publishes, with
  no flag and no release branch, whether the admin or an agent acting for them runs it.
- The `release/vX.Y.Z` route still works.
- Tests cover both guards and `--continue` on `main`, plus a push the remote rejects.
- The `Repo Setup` text in `config.yaml` and the regenerated AGENTS.md block no longer forbid agents
  acting for the admin to push.
- The same `Repo Setup` text and `docs/Git.md` make AGit the default for tickets and small PRs on a
  forge that supports it (`git push origin HEAD:refs/for/main/<topic>`), using one reusable local
  branch and no pushed branch. They keep pushed branches for work others push to, long sprints that
  need rebasing, and GitHub, which has no AGit. The local branch stays because harnez refuses
  commits on protected `main` and on a detached HEAD.
- `docs/GoRelease.md` (edit the copyable source and sync the root copy) and `docs/Git.md` describe
  the new behaviour.
- `harnez release --dry-run` on `main` passes the guards without changing anything.
