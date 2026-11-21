---
name: harnez-land
description: Commit the working tree, push it through AGit, and switch to the latest main; report leftover branches and PRs. Only on explicit /harnez-land.
disable-model-invocation: true
---

# Harnez Land

Bring the repository to a clean, up-to-date default branch. Mechanical: run the steps, stop at
the first failure and report it. Triggers: only on explicit `/harnez-land`. Below, `main` stands
for the default branch (`git symbolic-ref --short refs/remotes/origin/HEAD`).

## 1. Commit

- Clean tree: skip to 2.
- On `main` with changes: `git switch -c <topic>` first; never commit on `main`.
- Commit everything with a conventional message from the diff. Ask for a message only when the
  diff has no obvious summary.

## 2. Push

- Skip when `git log origin/main..HEAD` is empty.
- Forge with AGit (Forgejo, Gitea): `git push origin HEAD:refs/for/main/<topic>`. The topic is
  the one in `git config remote.origin.push`, else the forge username, else ask.
- GitHub or a branch others push to: `git push -u origin HEAD`.
- Never push `refs/heads/main`, never force-push, never `-o force-push=true`. A rejected push
  stops the run: report it (`docs/AGit.md` covers the non-fast-forward case).
- PR number in the push output: start `HTO=0 harnez pr wait <number> --contains <sha>` in the
  background, and report "merged" only on its exit 0. On exit 6 the forge saw the merge but
  `<sha>` is unverified: confirm with a foreground `git fetch` and `git merge-base --is-ancestor`.

## 3. Switch

- `git status --porcelain` must be empty; otherwise stop.
- `git fetch origin`, `git switch main`, `git merge --ff-only origin/main`. A refused
  fast-forward stops the run; never reset or rebase `main`.

## 4. Report

For each local branch other than `main`, `merged` means its content is already in `main`:
`git merge-tree --write-tree origin/main <branch>` prints the tree of `origin/main^{tree}`.
This catches squash merges that `git branch --merged` misses.

List each branch and each open PR of yours with one proposed action: delete (merged), keep
(open PR), push (local commits, no PR) or rebase (behind `main` and conflicting). Do not delete,
reset or rebase anything without the user's go.
