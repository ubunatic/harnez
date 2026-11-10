# 1062 — Safe git push and git pull ergonomics for AGit repositories

**Status**: Open
**Priority**: P2 (Medium)
**Severity**: Moderate
**Category**: Feature
**Related**: 1054, docs/commands/issue.md, docs/lang/Git.md

---

## 1. Problem & Motivation

In repositories using AGit pull request workflows (such as Forgejo/Codeberg repositories like `loom` and `harnez`), contributors submit changes by pushing to pseudo-refs `refs/for/main/<topic>`.

To avoid typing the full refspec every time, developers often configure `remote.origin.push = HEAD:refs/for/main/<topic>`. However, this introduces several usability footguns:

1. **Rejection when up to date**: If a user runs a bare `git push` when local `HEAD` is already merged into `origin/main`, Forgejo's AGit hook rejects the push with:
   `! [remote rejected] HEAD -> refs/for/main/<topic> (The target branch already contains this commit)`
   This confuses developers and automated agents, appearing as a hard failure when the repo is simply up to date.
2. **Topic reuse after merge**: A static topic configured in `remote.origin.push` is reused across multiple tasks, but Forgejo requires fresh topics once a PR is merged or closed.
3. **Accidental merge commits on pull**: Plain `git pull` can introduce unintended merge commits or diverge local branches unless fast-forward constraints (`pull.ff = only`) are configured.

Harnez currently lacks pre-push hooks, git config helpers, or tooling to safely manage AGit interactions for plain `git push` and `git pull`.

## 2. Goal & Acceptance Criteria

/goal Provide safe git push and git pull ergonomics for AGit repositories in Harnez, including pre-push guards against "target branch already contains this commit" rejections, pull fast-forward hardening, and clean topic management, and verify with tests and documentation. Stop and report when blocked on a user decision or denied permission. On a loop, hard architectural or OS blocker, or no progress, clear the goal and write a failure report explaining why and where to resume.

- Check live configuration and repository git settings before implementation; align with issue 1054 (push rules and roles per repo).
- Provide a pre-push guard (e.g. installed by `harnez init` or managed git hooks) that inspects `HEAD` against `origin/main` before pushing to AGit; if `HEAD` is already an ancestor of `origin/main`, gracefully notify the user ("Everything up to date with origin/main") and cancel the push cleanly without an ugly remote rejection.
- Recommend or configure `pull.ff = only` for AGit repositories so `git pull` safely fast-forwards and prevents merge commit pollution.
- Support dynamic or managed AGit topics (or a `harnez pr` / git alias) so contributors do not hit closed-topic collision errors.
- Document safe AGit setup in `docs/lang/Git.md` and `docs/commands/issue.md`.

## Jules audit (2026-11-10, af9688e0)

Partly done, mostly still valid. Checked: no pre-push ancestor guard and no `pull.ff` setting anywhere in
code (`internal/githooks/` holds only the LFS/GitHub guard); the only managed AGit topic is the release
one (`internal/release/push_pr.go:84`), not contributor topics. Docs: `docs/lang/Git.md:108` shows the
`remote.origin.push HEAD:refs/for/main/<topic>` setup, and `docs/commands/issue.md:74` documents the
manual post-push `merge-base --is-ancestor` check. Overlaps 1206 (pull setup for AGit repos).
Report: `reports/jules/1062.md` (github#150).
