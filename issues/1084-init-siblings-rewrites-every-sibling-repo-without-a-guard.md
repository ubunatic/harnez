# 1084 — init-siblings rewrites every sibling repo without a guard

**Status**: Open
**Priority**: P2 (Medium)
**Severity**: Major
**Category**: Bug
**Related**: uman issue 015

---

/goal `make init-siblings` cannot rewrite sibling repos by accident or from an unfinished build, or stop and report when blocked on a user decision or denied permission. Circuit breaker: on a loop, a hard blocker or no progress, clear the goal and write a failure report here.

## Problem

On 2026-11-01 at about 16:55, `uman harness-sync --help` ran `make -C ~/projects/harnez init-siblings`, because uman custom commands ran on `--help` (fixed in uman issue 015). The target ran `harnez init -d <repo> -y` in every sibling and left 48 repos with uncommitted changes.

The harnez side has three gaps:

- The target has no confirmation, so one stray call changes every repo.
- It depends on `build`, so it used the checkout as it stood: a topic branch with uncommitted work, while other sessions were active.
- `harnez init` created `version.yaml` (`0.1.0`) in six repos without release tags: conreel, goha, harnez.org, homeserver, mdview, vimconf. The owner did not want them, and they were deleted by hand.

## Want

- `init-siblings` needs an explicit confirmation and refuses a dirty tree or a branch other than `main`.
- Decide whether a non-interactive `harnez init -y` should create `version.yaml` in a repo that has never released.

## Notes

Six repos refused the sync commit, because their commit hook's issue lint already fails on old tickets or an index: books, cati, psync, smarthome, ubunatic.com, voxi. Their sync files are still uncommitted. The check is `issues lint`, with errors such as `unknown status ""`.
