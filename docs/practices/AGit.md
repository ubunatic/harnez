---
title: AGit
weight: 30
---

<!-- harnez:bundled -->
# AGit

This repository uses AGit: a PR is opened by pushing to a magic ref, not by pushing a branch.
Forgejo and Gitea support it (Codeberg runs Forgejo). GitHub has no AGit.

## Push

- Commit on a local topic branch, never on the default branch.
- Push with `git push origin HEAD:refs/for/<default-branch>/<topic>`, e.g.
  `git push origin HEAD:refs/for/main/flashcoder`. No branch is created on the remote.
- "Push", "push it" and "push to main" mean this AGit push. Never push `refs/heads/<default-branch>`
  directly unless the repository admin explicitly authorizes it in the current session.
- The topic is per pusher, by default the forge username, and is reused across tickets and small PRs.
- To skip typing the refspec: `git config remote.origin.push HEAD:refs/for/main/<topic>`.

## PR lifecycle

- A push to a topic with an open PR updates that PR.
- Once that PR is merged or closed, the next push to the same topic opens a new PR. The push output
  names the PR URL; report that number.
- Forgejo (seen on Codeberg) prints "Visit the existing pull request" even when the push just opened
  a new PR. Trust the PR number in the URL, not that wording.
- A non-fast-forward update is rejected. Never override it with `-o force-push=true`, and never
  force-push.
- After rebasing an open AGit PR onto an updated `<default-branch>`, pushing the rebased commits
  to the same topic is rejected as non-fast-forward; push to a new topic suffix (e.g.
  `<topic>-2`) to open a clean replacement PR, and close the superseded PR.
- After a squash or rebase merge, reset the local branch to `origin/<default-branch>`, or the merged
  commits reappear in the next PR.

## Merge signal

- After each push, start `harnez pr wait <number> --contains <sha>` in the background. Exit 0 means
  merged and `<sha>` is on the default branch; 2 closed without merge; 3 still open at timeout;
  4 API error; 5 merged but `<sha>` is missing.
- Report "merged" only after exit 0, never from chat or a guess. Until then the PR is open and the
  work is not on the default branch.

## When not to use AGit

Push a branch (`issue/<n>-<slug>`, `sprint/<n>`) for work others push to, long sprints that need
rebasing, forks, and GitHub.
