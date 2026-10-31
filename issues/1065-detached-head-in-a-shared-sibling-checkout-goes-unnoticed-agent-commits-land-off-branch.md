# 1065 — Detached HEAD in a shared sibling checkout goes unnoticed; agent commits land off-branch

**Status**: Closed — prevented accidental off-branch commits by warning on detached HEAD in session start and commit guards with actionable recovery guidance
**Priority**: P2 (Medium)
**Severity**: Moderate
**Category**: Bug
**Related**: https://codeberg.org/ubunatic/harnez/issues/40, docs/Git.md, AGENTS.md

---

## 1. Problem & Incident (Imported from Codeberg #40)

*Source: Reported by `flashcoder` on Codeberg (2026-10-31)*

One agent session left a shared sibling checkout on a detached HEAD. A second agent session, whose home repo was that checkout, then committed onto it without noticing.

1. **2026-10-30 22:21**: The `neus` session cloned the sibling repo to `~/projects/lmcoder` to read its spec, then ran `git checkout -q v0.1.13` to pin the release tag. It never switched back to `main`.
2. **2026-10-31 11:25**: The `lmcoder` session, which works in that checkout, committed an issue update (`docs(issues): 268 ...`). The commit went onto the detached HEAD, not onto `main`.
3. The problem showed up only at session close (`/wrap`), when `git status` reported `HEAD detached from v0.1.13`. Meanwhile `origin/main` had moved ahead by one commit, so recovering the commit required a checkout of `main`, a fast-forward, and a rebase.

Reflog evidence (`~/projects/lmcoder`):
```text
4fc342a HEAD@{2026-10-30 22:21:58}: clone: from ssh://codeberg.org/ubunatic/lmcoder.git
4fc342a HEAD@{2026-10-30 22:22:10}: checkout: moving from main to v0.1.13
ad98c94 HEAD@{2026-10-31 11:25:44}: commit: docs(issues): 268 record neus 112 wait-until-ready request
```

## 2. Why Harnez is Involved

- The convention in `docs/Git.md` specifies "work on the default branch", but nothing in Harnez detects or prevents a detached HEAD during turn execution.
- The session-start git context displays `Current branch: HEAD`, which is easily overlooked by agents. Neither the SessionStart hook nor intermediate commands flag it until `/wrap`.
- Cross-repo reads by one session (checking out a pinned tag) silently mutate the working state of another active session's repository.

## 3. Proposed Safeguards & Implementation Plan

/goal Prevent accidental off-branch and detached HEAD commits in shared checkouts by adding detection in SessionStart, pre-commit guards, and git read conventions, verified with unit and integration tests.

1. **SessionStart Hook**: Warn loudly when the repository is in a detached HEAD state (or not on the expected default/topic branch), reporting the tag/commit and the remediation command (`git switch main`).
2. **Pre-Commit Check / Guard**: Refuse or warn when creating a commit on a detached HEAD in a Harnez-managed repository.
3. **Convention Updates (`docs/Git.md` / `AGENTS.md`)**: Document that cross-repo tag reads must use `git -C <repo> show <tag>:<path>`, `git worktree add`, or a scratch clone rather than checking out tags in a shared active checkout.
4. **Tool Verification in `/wrap` and `/focus`**: Check branch state first before evaluating `git status --short`.
