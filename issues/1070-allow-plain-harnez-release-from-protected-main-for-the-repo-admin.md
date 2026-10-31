# 1070 — Allow plain harnez release from protected main for the repo admin

**Status**: Open
**Priority**: P2 (Medium)
**Severity**: Minor
**Category**: Feature
**Related**: 1039

---

/goal Make a plain `harnez release` work on `main` for the repository admin while agents stay refused, and verify both paths in tests, or stop and report when blocked on a user decision or denied permission. On a loop, a hard blocker or no progress, clear the goal and write a failure report here.

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
  (`internal/release/push.go`). Both must allow the admin; `--continue` only hits the second.
- The first guard's hint names `issue/<n>-<slug>`, while `docs/GoRelease.md` and the push guard
  name `release/vX.Y.Z`. The release refusal that remains for non-admins should name the release
  branch.
- 1039 introduced the guards. `internal/release/gitflow_test.go` asserts the refusals, and the
  non-admin cases must keep passing.
- The AGENTS.md rule stays: agents do not commit to or push the default branch unless the admin
  authorizes it in the current session. This ticket must not give agents a way around it.

Open questions:

- **How harnez knows the caller is the admin.** The user asked for the plain command with no
  extra flag. No admin identity exists in `.harnez/repo.yaml` today. Agent sessions may be
  detectable from the environment (`HARNEZ_SESSION_ID`, `HARNEZ_AGENT_MODEL` and similar are read
  in `cmd/harnez/issues.go`), but that is not proven reliable for every host, including a `!`
  command typed in an agent session. Settle this with a small probe and ask the user if it stays
  ambiguous.
- **What happens if a remote rejects the push.** Harnez's guard is not the forge's branch
  protection. The admin reports that pushing works, but this is unverified for both dev remotes
  (Codeberg and GitHub). A rejection must fail clearly and leave `harnez release --continue` usable.

## 3. Implementation & Verification Plan
Done when all of these hold:

- A plain `harnez release` from `main` as admin bumps, tags, pushes `main` and the tag, and
  publishes, with no flag and no release branch.
- An agent session running the same command is still refused, with a hint that names
  `release/vX.Y.Z`.
- The `release/vX.Y.Z` route still works.
- Tests cover admin and agent on both guards, and `--continue`.
- `docs/GoRelease.md` (edit the copyable source and sync the root copy), `docs/Git.md` and
  AGENTS.md describe the new behaviour.
- `harnez release --dry-run` on `main` as admin passes the guards without changing anything.
