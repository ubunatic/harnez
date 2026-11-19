# 1251 — issue skill treats push --dry-run as a permission check and misreads a branch-protection rejection

**Status**: Open
**Priority**: P3 (Low)
**Severity**: Minor
**Category**: Docs
**Related**: 1039, 1206, 1248

---

/goal Update the issue skill (`docs/commands/issue.md`) so the direct route does not treat `git push --dry-run` as proof of push permission, tells a branch-protection rejection apart from "main moved", and says how to switch to the AGit route; or stop and report when a user decision is needed. On a loop, hard blocker, or no progress, clear the goal and write a failure report.

## 1. Problem & Motivation

Reported from a cati session (inbox draft `harnez-issue-skill-push-dry-run-misses-branch-protection.md`, 2026-11-19, cati issue 127). The direct route's dry run succeeded:

```
To ssh://codeberg.org/ubunatic/cati.git
   bf8a838..b0721d2  b0721d2 -> main
```

The real push was then rejected by Forgejo's pre-receive hook:

```
remote: Forgejo: User 'flashcoder' is not allowed to push to branch 'main' in 'ubunatic/cati'.
 ! [remote rejected] b0721d2 -> main (pre-receive hook declined)
```

A dry run never runs server-side hooks, so it cannot detect branch protection. cati's `.harnez/repo.yaml` had no git workflow mode set, so the agent could not know in advance that the AGit route was needed. The AGit route then worked.

A second, smaller point: on the first AGit push to a new topic, Forgejo printed "Visit the existing pull request" for what was actually a new PR (#1). An agent following "Check the topic's PR is still open" could take this as reuse of an old topic.

## 2. Technical Specification / Findings

`docs/commands/issue.md:151` (direct route): "`push --dry-run` ..., then `push`. ... A rejection means `main` moved: fetch, re-check, and report." This is wrong for a `[remote rejected] ... (pre-receive hook declined)` rejection, which means permission, not a moved `main`. The skill gives no rule for going from a refused direct push to the AGit route. Route 2 only covers "direct push is not allowed" known in advance.

The repo's git workflow mode (1039, `.harnez/repo.yaml`) and `protected_main` (1206) already tell harnez about protection, but only when they are configured.

## 3. Implementation & Verification Plan

- [ ] In the direct route, say that the dry run checks fast-forward and the local `pre-push` hook only, not server permission.
- [ ] Split rejection handling: `! [rejected] (non-fast-forward)` / `fetch first` means `main` moved (current text). `! [remote rejected] ... pre-receive hook declined` or "not allowed to push" means no permission: switch to the AGit route when the remote supports it, and suggest setting `protected_main` / the git mode in `.harnez/repo.yaml`.
- [ ] Optional: before the direct push, check protection through the forge API (Forgejo `GET /repos/{o}/{r}/branch_protections/{branch}`) when a read token is available, and skip to AGit. Canary first. Stop and report if it needs a token the host does not have.
- [ ] Under the AGit route, note that Forgejo's "Visit the existing pull request" wording also appears for a newly created PR. Confirm with `harnez pr wait` / the API, not the push message.
- [ ] Sync the lite variant if one exists, run `make install` and `harnez init -d .`, and commit the source and its copies together.
