---
name: wrap
description: Close the current work session (or a user-named scope), preserve or land its changes, and prepare a concise handoff
disable-model-invocation: true
---

# Wrap

Close the current work session and leave the repository and its next steps clear.
Use this at the end of a session or before a context reset or handoff. This is
an explicit, user-invoked workflow; do not commit merely because work happened.

## Scope

Default scope is the **current session's branch, worktree, and PR**. If the user
names a specific branch, worktree, PR, or ticket (for example, when another agent
wraps a finished session), restrict all actions to that named scope. Never touch
unrelated branches, other worktrees, or teammates' PRs — leave repo-wide cleanup
to `/harnez-land`.

## Session Close Checklist

1. Review the current conversation (or named scope) and repository state.
   Identify completed work, unfinished threads, observed bugs, and durable
   decisions or learnings.
2. Stop background helpers and subagents started for this session. Confirm
   their state with the available task/session tools; do not stop unrelated
   sessions.
3. Sync issue tracking. Close completed tickets with a reason, update the
   issue index, and file distinct tickets for substantive bugs, broken
   assumptions, or follow-ups that should survive this session. Do not file
   nitpicks.
4. Update relevant evergreen `docs/*.md` when the session established durable
   architecture, design, or operational knowledge. `/evergreen` curates those
   learnings and related issue/index state; `/wrap` closes the session, checks
   lifecycle hygiene, and prepares the repository handoff. Invoke `/evergreen`
   when that broader curation is needed.
   Retrospective reports must not promise behaviour in future sessions; name a
   durable safeguard added or proposed (check, rule/prompt line, or ticket) and
   where it lives.
5. Preserve repository state in a commit (within scope). Review `git status`
   and the diff inside the scoped worktree; inspect untracked files so real
   work is not lost and scratch is not silently committed.
   - In a protected-`main` repo, never commit on `main`: `git switch -c <topic>`
     first.
   - Commit work in scope with a conventional message. If work is partial or
     broken, first file or update a handoff issue describing the state and
     resume point, then commit with that ticket number in the message. Do not
     imply incomplete work is finished. If the tree is already clean, verify
     with `git status`.
6. Land or hand off the scoped branch (never force-push, never reset or rebase
   `main`):
   - **Ready & authorized to land** (complete work, user asked to push/land or
     `/decide` is active):
     - If pushing to `main` is allowed (`protected_main` is not set in
       `.harnez/repo.yaml`, and the hook and remote do not refuse it; forge
       branch protection may still allow non-force pushes for authorized
       accounts) **and** you have good confidence in the changes, push to
       `main` directly (`main` is the development branch; only releases are
       safe).
     - Otherwise (protected main, push to `main` refused, or needs review), push
       via AGit where the forge supports it (`git push origin HEAD:refs/for/main/<topic>`),
       else a topic branch (`git push -u origin HEAD`), and start
       `HTO=0 harnez pr wait <number> --contains <sha>` in the background
       (report "merged" only on exit 0; handle exit 6/7 per `docs/AGit.md`).
     - Once landed/merged, `git fetch origin`, `git switch main`,
       `git merge --ff-only origin/main`, and remove **only** this scope's
       verified-merged branch or clean worktree.
   - **Partial / handoff** (or push not requested): keep the topic branch and
     worktree intact (push only if asked for backup) and record the exact
     resume point in the handoff issue.
7. Confirm the final status with `git status` and report:
   - current `HEAD` and scoped disposition (`pushed to main`, `merged & cleaned`,
     `PR #N open`, `unpushed local handoff on <branch>`, or `kept dirty worktree`),
   - completed work, remaining work or handoff ticket, active open issues,
   - recommended next command or action (mention `/harnez-land` if unrelated
     stale branches or worktrees exist). Keep the report concise and factual.
