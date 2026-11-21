# 1261 — pr wait keeps polling a PR with merge conflicts; exit so the waiter can react

**Status**: Open
**Priority**: P2 (Medium)
**Severity**: Minor
**Category**: Agentic Ergonomics
**Related**: 1096, 1248

---

## 1. Problem & Motivation

`harnez pr wait <n>` only ends on merged (0), closed (2), timeout (3), API error (4) or the
`--contains` outcomes (5, 6). A PR that cannot merge because of conflicts stays open, so the
waiter polls until the timeout while nobody can merge it. The agent that opened the PR is waiting
on exactly this command and is the one that can fix the conflict, but it only learns about the
problem from the owner.

Seen 2026-11-21: PR #198 (tickets 1255–1257) conflicted with main in `issues/README.md` after
other ticket PRs merged, and the owner had to report "pr 198 has conflicts".

## 2. Technical Specification / Findings

- Forgejo and GitHub both expose a `mergeable` field on the pull request
  (`GET /repos/{o}/{r}/pulls/{n}`); for #198 the Codeberg API returned `"mergeable": false`
  while `state` stayed `open`. GitHub may return `null` while it computes the value.
- `internal/prwait/wait.go` has outcomes 0–6; no outcome covers "open but unmergeable".
- A conflict can appear after the wait starts, when another PR merges first, so the check belongs
  in every poll, not only at start.

## 3. Implementation & Verification Plan

/goal Make `harnez pr wait` exit with its own code and a clear message when the PR is open but
has merge conflicts, so the waiting agent can rebase or merge main and push again; or stop and
report when blocked on a user decision or denied permission.

- New outcome (e.g. exit 7, `Conflicted`): the summary names the PR, URL and base branch and says
  to merge or rebase on the base and push to the same topic.
- Treat `mergeable: false` as conflicted only on an open PR; `null` or missing (GitHub still
  computing) keeps polling. Decide whether a conflict must persist for two polls before exiting,
  to avoid exiting on a transient value.
- Update the exit-code table in `cmd/harnez/pr.go` help, the help golden, and the docs that list
  exit codes (`docs/AGit.md` and its sources, `docs/commands/issue.md`, `sprint.md`,
  `HarnezLand.md`, the issue skill's AGit route).
- Verify with fake-forge tests for both forge kinds: conflicted exits 7, null keeps polling, and a
  conflict that appears mid-wait is caught on the next poll.

## Filing Notes

- Updating an AGit PR needs its topic, which neither the Forgejo API nor the PR page shows (the
  page says `ubunatic/ubunatic`). Fixing #198 by pushing to a guessed topic opened #205 instead.
  If the exit message tells agents to "push to the same topic", the topic must be discoverable,
  for example recorded by harnez when it pushes, or printed by `pr wait`.
