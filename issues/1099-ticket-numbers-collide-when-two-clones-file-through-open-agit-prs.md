# 1099 — Ticket numbers collide when two clones file through open AGit PRs

**Status**: Open
**Priority**: P2 (Medium)
**Severity**: Minor
**Category**: Bug
**Related**: 1016, 1018, 194

---

/goal `harnez issues new` does not hand out a number that a ticket in another clone's open PR already uses, and a collision that still lands on `main` is caught before the merge, not after it. Stop and report when blocked on a user decision or denied permission. Circuit breaker: on a loop, a hard blocker or no progress, clear the goal and write a failure report here.

## 1. Problem & Motivation

On 2026-11-02 two clones of this repo filed tickets 1095 and 1096 at the same time, each through an AGit PR. One clone's PRs (#77/#79, Jules resume and `pr wait`) merged first. Then PR #76 from the other clone merged its own 1095 and 1096 research tickets. After that merge `harnez index` failed with "duplicate ticket number", and PR #82 renumbered the later pair to 1097 and 1098.

`harnez issues new` checks only fetched remote branches, and the `/issue` skill checks only `origin/main`. A ticket that waits in an open AGit PR lives under `refs/pull/<n>/head` (Forgejo) or `refs/for/...`, which a plain `git fetch` does not bring in. So neither clone could see the other's number.

## 2. Technical Specification / Findings

- Forgejo exposes open PR heads as `refs/pull/<n>/head`. `git fetch origin 'refs/pull/*/head:refs/remotes/origin/pr/*'` makes them visible. A cheaper alternative is the PR file list from the API, `/pulls?state=open` plus `/pulls/<n>/files`. `internal/prwait` and `internal/forge` (1096) already parse remotes and call the Forgejo and GitHub APIs.
- Options:
  1. Have `issues new` also scan open PR heads.
  2. Add a pre-merge check, `harnez issues lint --against origin/main`, that the publish step or CI runs.
  3. Both.
- `harnez issues reconcile` (1016) repairs collisions after a merge. This ticket is about preventing them.

## 3. Implementation & Verification Plan

- Test: a fake remote with an open PR ref carrying `issues/<n>-x.md`; `issues new` skips `n`.
- Test: a branch whose ticket number is already taken on `origin/main` fails the pre-merge check with both paths named.
- Update the `/issue` skill's number-collision step to use the new check.

## Decision

- Owner signed off (2026-11-02) on the dev-1099 plan: both prevention and backstop.
  - M1 `internal/forge` gains the shared forge types and token source (moved from `internal/prwait`), `ListOpenPulls` with paging, and the `open_pr_scan` spec (timeout 5s, page_size 50, max_pages 4) with schema.
  - M2: `issues new`, `find next` and `issues mv` share one `Elsewhere` set that includes ticket files in open PRs. Adds `--offline`.
  - M3 `issues lint --against <ref>`, wired into `scripts/git-hooks/pre-push` and the `/issue` number-collision step (`docs/commands/issue.md`).
  - M4 docs, `make install`, one `make test-q1`, close.
- Use the forge API (`/pulls?state=open`, then `/pulls/<n>/files`), not a `refs/pull/*` fetch. `ls-remote` lists all 81 PR refs, merged ones included, with no state.
- Scan the default remote plus every other remote whose host is in `spec/pr.yaml` `forges`, with duplicate numbers counted once.
- Closed-unmerged PRs do not reserve numbers.
- Offline or an API error only warns ("open PRs not checked: ...") and never blocks filing. The lint fails only on a confirmed collision.
- No PR was open at plan time, so acceptance is fixture-based. Add one live check once a PR is open.
