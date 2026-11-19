# 1248 — pr wait --contains exits 1 after a detected merge when the local git fetch fails

**Status**: Open
**Priority**: P2 (Medium)
**Severity**: Major
**Category**: Bug
**Related**: 1096, 1249

---

/goal Make `harnez pr wait <n|url> --contains <sha>` report a seen merge correctly even when local git does not work, and never end a seen merge in exit 1; add a test for a merged PR whose fetch fails, or stop and report when the forge API cannot answer the ancestry question or a user decision is needed. On a loop, hard blocker, or no progress, clear the goal and write a failure report.

## 1. Problem & Motivation

Reported from a cati session (inbox draft `harnez-pr-wait-contains-loses-merge-signal.md`, 2026-11-19, cati PR #1):

```
$ HTO=0 harnez pr wait https://codeberg.org/ubunatic/cati/pulls/1 \
    --contains b0721d2b4c61ff9ab507db0bb18827b73137019e
harnez pr wait: git fetch origin failed: warning: unable to access
  '/home/alfred/.gitconfig': Permission denied ...
exit 1   (after 31 s)
```

The PR had merged as `054dc9c`, and `b0721d2` was on `main`. The wait loop saw the merge. The local `git fetch` behind `--contains` then failed: the taia background task could not read `~/.gitconfig`, a taia-side problem. The command exited 1, "usage, configuration, or local git error". The issue skill and `docs/Git.md` say the exit code is the only merge signal. So the agent lost the merge and had to confirm it by hand through the API.

## 2. Technical Specification / Findings

Verified on `main` at bb26257e:

- `internal/prwait/wait.go:108-123` `afterMerge`: if `FetchRemote` or `IsAncestor` fails, it returns `Outcome: Failed` (exit 1) even though `info.Merged` is already true.
- `Result.Summary` (`wait.go:126`) returns `""` for `Failed`. The merge commit SHA the API already returned is never printed.
- `internal/prwait/wait_test.go:237` (`"fetch fails"` -> `Failed, 1`) pins this behaviour. The fix changes the documented contract (`cmd/harnez/pr.go:36-42` exit-code table) and that test.
- The URL form still needs a local checkout with the `origin` remote for `--contains`.

## 3. Implementation & Verification Plan

- [ ] Answer `--contains` through the forge API first: Forgejo `GET /repos/{o}/{r}/compare/{default}...{sha}` (or a commit-status/ancestry endpoint), GitHub `GET /repos/{o}/{r}/compare/{default}...{sha}` (`status: behind|identical` means `sha` is on the default branch). Use a local fetch only when the API cannot answer. Canary the Forgejo endpoint on Codeberg first (`docs/Canary.md`).
- [ ] If the merge was seen but neither path can check `--contains`, exit with a new distinct code (proposed 6, "merged, contains unverified"), not 1. Print the merge commit and the reason.
- [ ] Update the exit-code table in `cmd/harnez/pr.go`, `docs/Git.md`, `docs/commands/issue.md` and every other place that lists the codes (search `exit code is the only merge signal`).
- [ ] Tests: merged PR plus failing fetch gives code 6 and a summary with `merge_commit`. Merged PR with API ancestry `true` gives 0 without touching git. API ancestry `false` gives 5.
