# 1074 — Sync main to GitHub from this machine and before Jules, without CI, force or pruning

**Status**: Backlog — grooming 1083 (2026-11-03): active list focused on taia support, P1s and open bugs; promote when needed
**Priority**: P3 (Low)
**Severity**: Minor
**Category**: Feature
**Related**: 1071, 1017

---

/goal `harnez release` works beside a local multi-URL push remote, and starting a Jules session brings GitHub `main` level with Codeberg `main` by a plain fast-forward that touches no other GitHub ref, or stop and report when blocked on a user decision or denied permission. Circuit breaker: on a loop, a hard blocker or no progress, clear the goal and write a failure report here.

## Problem

GitHub `main` falls behind Codeberg unless someone runs `git push github origin/main:main`, and a Jules session starts from GitHub. A Codeberg push mirror is not the answer: on 2026-11-01 one filtered to `main` coincided with all 21 other GitHub branches being deleted and 11 PRs closing, so it was removed.

## Want

The owner rejected CI jobs and push mirrors. GitHub may lag between releases, but remote agents such as Jules should start from the latest `main`. Only the owner's machine releases.

- **Local remote (not committed):** that clone gets a remote `both` with Codeberg and GitHub as push URLs, `remote.pushDefault=both`, and `remote.both.push=refs/heads/main:refs/heads/main`. A `pre-push` hook refuses GitHub unless Codeberg already has the commit. `origin` keeps one URL because a hook can only fail a whole push, so topic and AGit pushes through a two-URL `origin` exit 1.
- **Release:** `pushRelease` pushes every remote, and its dry-run preflight runs the hook, so `both` would abort each release before Codeberg has the commit. It must skip a remote whose push URLs are all covered by the other remotes, while still pushing `origin` and `github`. Check that the other remote scans (`internal/release/forge.go`, `internal/subagent/jules.go`) do not pick `both` instead of the real remote.
- **Jules:** before starting a session, compare GitHub and Codeberg `main`. If GitHub is strictly behind, push with `git push <github> <codeberg-sha>:refs/heads/main`: no `--force`, `--prune`, `--mirror` or other refspec. If the push fails or the branches diverged, warn and name the commit Jules will read; do not abort the session. Configurable values go in `spec/`.
- **Docs:** `docs/lang/Git.md` (the source) gets the two configurations: this `both` setup, and `git config remote.origin.push HEAD:refs/for/main/<topic>` for a contributor clone.

## Notes

A Forgejo push mirror is never an option, and nothing here may run `git fetch --prune` on the GitHub remote.

Status: code is in PR #64. The `both` remote and hook are installed on the owner's clone, and a real push moved GitHub `main` to `19e4fd00` with its branch count unchanged. Before closing: merge the PR and run one real Jules start from `main`.
