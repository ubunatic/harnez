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

Second occurrence, harnez PR #191 (2026-11-20), from a taia background task in this repo:

```
$ HTO=0 harnez pr wait 191 --contains ba088276
harnez pr wait: git fetch origin failed: hostkeys_find_by_key_hostfile:
  hostkeys_foreach failed for /home/alfred/.ssh/known_hosts: Permission denied
ssh_askpass: exec(/usr/lib/ssh/ssh-askpass): No such file or directory
Host key verification failed.
exit 1   (after 396 s)
```

#191 had merged as `cea59ed6`, and `ba088276` was on `origin/main`. This time the sandbox blocked
ssh's `known_hosts`, not `~/.gitconfig`. So the fix must not depend on one sandbox setting: once
the forge reports the merge, a failed fetch must not turn it into exit 1. The run also took
396 s. It is not yet known whether that time went into polling or into ssh before it failed.

## 2. Technical Specification / Findings

Verified on `main` at bb26257e:

- `internal/prwait/wait.go:108-123` `afterMerge`: if `FetchRemote` or `IsAncestor` fails, it returns `Outcome: Failed` (exit 1) even though `info.Merged` is already true.
- `Result.Summary` (`wait.go:126`) returns `""` for `Failed`. The merge commit SHA the API already returned is never printed.
- `internal/prwait/wait_test.go:237` (`"fetch fails"` -> `Failed, 1`) pins this behaviour. The fix changes the documented contract (`cmd/harnez/pr.go:36-42` exit-code table) and that test.
- The URL form still needs a local checkout with the `origin` remote for `--contains`.

## 3. Implementation & Verification Plan

- [x] Answer `--contains` through the forge API first: Forgejo `GET /repos/{o}/{r}/compare/{default}...{sha}` (or a commit-status/ancestry endpoint), GitHub `GET /repos/{o}/{r}/compare/{default}...{sha}` (`status: behind|identical` means `sha` is on the default branch). Use a local fetch only when the API cannot answer. Canary the Forgejo endpoint on Codeberg first (`docs/Canary.md`).
- [x] If the merge was seen but neither path can check `--contains`, exit with a new distinct code (proposed 6, "merged, contains unverified"), not 1. Print the merge commit and the reason.
- [x] Update the exit-code table in `cmd/harnez/pr.go`, `docs/Git.md`, `docs/commands/issue.md` and every other place that lists the codes (search `exit code is the only merge signal`).
- [x] Tests: merged PR plus failing fetch gives code 6 and a summary with `merge_commit`. Merged PR with API ancestry `true` gives 0 without touching git. API ancestry `false` gives 5.

## 4. Canary and implementation (2026-11-21)

Canary (`docs/Canary.md`), anonymous read-only GETs:

- Codeberg `GET /api/v1/repos/ubunatic/harnez/compare/main...c38acfff` (merged): 200,
  `total_commits: 0`. `compare/0d1cc24a...c20e0edf` (head ahead of base): 200, `total_commits: 2`.
  `compare/main...8ebf032e` (never pushed): 404.
- GitHub (public `cli/cli`, the harnez mirror answers 404 anonymously): `compare/<tip>...<older>`
  gives `status: behind, total_commits: 0`; the reverse gives `status: ahead, total_commits: 3`.
- Both forges share the path and `total_commits` (commits in the sha that the branch lacks), so
  one decoder serves both; `total_commits == 0` means the sha is on the branch.

Implemented: `HTTPFetcher.OnBranch` (`internal/prwait/client.go`) answers `--contains` first;
local `git fetch` plus `merge-base --is-ancestor` runs only when the API fails (unknown sha, auth,
network). A seen merge that neither path can check ends in the new exit 6 (`ContainsUnverified`)
with the merge commit in the summary and both errors on stderr, never exit 1. Exit-code lists
updated in `cmd/harnez/pr.go`, `docs/practices/AGit.md` (and its copy), `docs/commands/issue.md`,
`docs/commands/sprint.md`, `docs/commands/HarnezLand.md`.

Third occurrence: PR #193 (2026-11-20) from a taia background task, `known_hosts` denied again,
while a foreground `git fetch` worked. With this change the forge compare answers before any ssh.
The 396 s and ~10 h runtimes were most likely polling until the merge: the fetch only runs after the
forge reports the merge, and a host-key failure is immediate (not measured per run).
