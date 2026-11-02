# 1099 — Ticket numbers collide when two clones file through open AGit PRs

**Status**: Closed — Fixture-based acceptance passes: make test-q1 exit 0, no --- FAIL, 68 ok packages incl. internal/forge, internal/issues, internal/prwait, cmd/harnez (help golden, lint --against). Commits d9e6af47 (M1), 20b21774 (M2), 6b05afcb (M3), 2c2238e3 (M4). Pending: one live check of the open-PR scan once a real PR is open on the forge; no PR was open at test time.
**Closed-By**: claude:haiku:med, session dev-1099
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

## Host review (2026-11-02)

- M1 (forge open-PR list), M2 (shared number reservation), M3 (lint --against + pre-push + /issue step) and M4 (docs): diffs reviewed. Targeted `go test` passes for forge, issues, prwait and find. The developer's `make test-q1` exited 0 with no `--- FAIL`.
- Live checks with the installed binary:
  - `harnez find issues next` scans both forges and returns 1100 in 0.77s.
  - With an unreachable proxy, it warns "open PRs not checked on github/origin" and still returns 1100.
  - In a scratch clone, `lint --against` fails on a reused number and names both paths. A missing ref only warns, with exit 0.
- Still pending: the live open-PR scan with a PR actually open. This sprint's own PR is the first case.
