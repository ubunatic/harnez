# 1249 — pr wait reports any git remote failure as no such remote and drops git stderr

**Status**: Open
**Priority**: P3 (Low)
**Severity**: Minor
**Category**: Bug
**Related**: 1096, 1248

---

/goal Make `harnez pr wait` report git's exit code and stderr when a git call fails, and say "no such remote" only when the remote is really missing; add a test with an unreadable or invalid git config, or stop and report when the git layer cannot tell these cases apart. On a loop, hard blocker, or no progress, clear the goal and write a failure report.

## 1. Problem & Motivation

Reported from a cati session (inbox draft `harnez-pr-wait-masks-git-errors.md`, 2026-11-19), run as a taia background task:

```
$ cd /home/alfred/projects/cati && HTO=0 harnez pr wait 1 --contains b0721d2
harnez pr wait: git remote "origin": no such remote
```

`origin` exists. The real failure was that git could not read `~/.gitconfig` (exit 128, `fatal: unknown error occurred while reading the configuration files`). Because the message was wrong, the agent spent time debugging the wrong thing.

## 2. Technical Specification / Findings

Verified on `main` at bb26257e. `internal/prwait/git.go:21-27` `SystemGit.RemoteURL` runs `git remote get-url` with `.Output()`. On any error it returns `git remote %q: no such remote` and throws away both the exit code and stderr. By contrast, `FetchRemote` (same file) uses `CombinedOutput` and keeps git's message, and `IsAncestor` drops both too (`git merge-base --is-ancestor ... failed`).

`git remote get-url` exits 2 with `error: No such remote 'x'` for a missing remote. Other failures, such as a config read error or not being a repository, exit 128.

## 3. Implementation & Verification Plan

- [ ] `RemoteURL`: capture stderr. Report "no such remote" only for the missing-remote case (exit 2). For anything else, report `git remote get-url <remote>: exit <n>: <stderr>`.
- [ ] `IsAncestor`: include the exit code and stderr for codes other than 0 and 1.
- [ ] Test with a real temp repo (`internal/prwait` already has a git test main): (a) missing remote gives "no such remote"; (b) a broken config (`GIT_CONFIG_GLOBAL` pointing at an invalid file) gives git's own message, not "no such remote".
