---
name: harnez-land
description: Commit and land changes (direct to main when push is allowed and confident, else via AGit/PR), switch to latest main, and audit/clean up stale worktrees, local/remote branches, and AGit topics/PRs with a concrete plan (interactive or auto). Only on explicit /harnez-land.
disable-model-invocation: true
---

# Harnez Land

Bring the repository to a clean, up-to-date default branch and triage all leftover work so no
stale branches or lost experiments remain. Mechanical through step 3 (stop on first failure and
report it); evidence-driven in steps 4–5. Triggers: only on explicit `/harnez-land` (optional
`auto` mode: `/harnez-land auto`). Below, `main` stands for the default branch
(`git symbolic-ref --short refs/remotes/origin/HEAD`).

## Push Policy (`main` vs PR)

`main` is the development branch; only releases are immutable/safe. Note that a forge-protected
`main` often blocks force-push/deletion while still allowing normal fast-forward pushes for
authorized accounts — check whether pushing `main` is actually allowed, not just whether branch
protection is enabled on the forge.

- **Push to `main` allowed + good confidence**: if repo policy allows pushing `main` (not
  PR-only via `.harnez/repo.yaml` `protected_main: true` / `AGENTS.md`, and not blocked by a
  hook/remote) and you have good confidence in the changes, commit on or merge into `main` and
  push `main` directly (`git push origin main`).
- **PR-only repo or needs review**: when repo policy requires PRs (`.harnez/repo.yaml`
  `protected_main: true`, `AGENTS.md`, or push to `refs/heads/main` is rejected) or confidence is
  low, never commit on `main` and never push `refs/heads/main` — use a topic branch and PR.
- **Always**: never force-push, never `-o force-push=true`, never reset or rebase `main`.

## 1. Commit

- Clean tree: skip to 2.
- On `main` with changes in a protected-`main` repo (or when opening a PR): `git switch -c <topic>`
  first; never commit on `main`.
- Commit everything with a conventional message from the diff. Ask for a message only when the
  diff has no obvious summary.

## 2. Push

- Skip when `git log origin/main..HEAD` is empty.
- **Direct-to-`main` repo + good confidence**: if on a topic branch, fast-forward or merge into
  `main` first, then `git push origin main`.
- **Protected-`main` / PR repo**:
  - Forge with AGit (Forgejo, Gitea): `git push origin HEAD:refs/for/main/<topic>`. The topic is
    the one in `git config remote.origin.push`, else the forge username, else ask.
  - GitHub or a branch others push to: `git push -u origin HEAD`.
  - PR number in the push output: start `HTO=0 harnez pr wait <number> --contains <sha>` in the
    background, and report "merged" only on its exit 0. On exit 6 the forge saw the merge but
    `<sha>` is unverified: confirm with a foreground `git fetch` and `git merge-base --is-ancestor`.
    On exit 7 the PR conflicts with its base: merge the base in, push with the `git push` line the
    wait printed, and start a new wait.
- A rejected push stops the run: report it (`docs/AGit.md` covers the non-fast-forward case).

## 3. Switch

- `git status --porcelain` must be empty; otherwise stop.
- `git fetch --prune origin`, `git switch main`, `git merge --ff-only origin/main`. A refused
  fast-forward stops the run; never reset or rebase `main`.

## 4. Audit & Cleanup Plan

Inventory **all four surfaces** before proposing any action:

1. **Linked worktrees** (`git worktree list --porcelain`): including detached worktrees and any
   uncommitted or untracked files inside them.
2. **Local branches** (`git for-each-ref refs/heads`): every branch other than `main`.
3. **Local AGit topics & open/stale PRs**: configured AGit push refspecs
   (`git config --get-all remote.origin.push`) and your open/stale PRs on the forge.
4. **Remote branches** (`refs/remotes/origin/*` excluding `HEAD` and `main`): flag yours or
   unowned stale branches; leave teammates' active branches untouched.

### Evidence check (never judge by branch name or PR state alone)

- **Merged / equivalent**: check ancestry (`git merge-base --is-ancestor <ref> origin/main`),
  tree equivalence (`git merge-tree --write-tree origin/main <branch>` equals `origin/main^{tree}`,
  catching squash merges), and patch equivalence (`git cherry origin/main <branch>` all `-`).
- **Unique work**: inspect `git log --oneline origin/main..<ref>`, `git diff --stat origin/main...<ref>`,
  and any dirty/untracked files in linked worktrees before choosing a disposition.

### Dispositions for stale work & old code

Assign every item exactly one disposition and a one-line reason:

1. **Drop** — Verified merged, patch-equivalent, or superseded with no unique value. Delete the
   local branch, remove the clean worktree, or delete the remote branch (if ours).
2. **Rescue uncommitted** — Dirty or detached worktree / stash with local edits or untracked files.
   Inspect first; commit to a recovery branch, fold into an active PR, or drop if scratch/generated.
3. **Integrate as-is** — Clean, scoped, good-quality work still relevant to `main`. Push directly
   to `main` (if push to `main` is allowed and confident), or push via AGit (`refs/for/main/<topic>`)
   / merge its open PR, wait for merge, then drop.
4. **Fixup & PR** — Valuable work that is stale, conflicting, or needs minor cleanup/tests. Rebase
   or cherry-pick onto `origin/main` (new AGit topic suffix if replacing a conflicting AGit PR),
   fix up, and land via `main` or PR (or hand off to `/lean-sprint` if non-trivial).
5. **Distill to issue & drop** — Stale, low-quality, or out-of-scope code that still holds useful
   ideas, edge cases, or small snippets. File/update an issue capturing the core learnings, key
   snippets, and commit SHA, then delete the branch / close the PR.
6. **Archive as reference** — Large prototype, spike, or benchmark worth keeping intact without
   cluttering active branches/PRs. Tag (`archive/<slug>`) or link its commit SHA in a tracking
   issue, then remove the active branch / close the PR.
7. **Keep / defer** — Active work, owned by someone else, waiting on external review/CI, or
   uncertain evidence. Leave untouched and state why + next action.

Present **one consolidated cleanup plan** (grouped by surface or disposition) with the final
verdict for each item.

## 5. Execute & Verify

- **Default (interactive)**: Do not delete, reset, rebase, or close anything without the user's
  go. Bundle all proposed actions into one approval request so the user can approve all, approve
  a subset, or override any disposition.
- **Auto-approve** (`/harnez-land auto`, or when the user explicitly pre-authorized cleanup under
  `/decide`):
  - Execute safe, reversible, or verified-equivalent steps automatically: `Drop` verified-merged
    local branches and clean merged worktrees, `Integrate` clean work, and `Distill` clearly
    superseded experiments into issues before dropping.
  - **Still stop and ask** before: deleting remote branches, discarding unverified unique commits
    or dirty worktree files, touching teammates' branches/PRs, or any force-push (never allowed).
- Recheck each ref immediately before deletion (abort that item if its SHA moved).
- Verify the final state (`git status`, `git worktree list`, `git branch`, remaining open PRs)
  and report what was cleaned, integrated, distilled, archived, or kept/blocked.
