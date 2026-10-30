# 1013 — `issues new` checks fetched remote refs for the next free number

**Status**: Closed — issues new and --number skip numbers on fetched remote branches (1f70988e)
**Closed-By**: claude:haiku:high, session gentle-otter
**Priority**: P3 (Low)
**Severity**: Minor
**Category**: Feature
**Related**: 1014, 1009, 1010

---

/goal `harnez issues new` (and `--number`) never hands out a number that already exists on any
fetched remote branch, locked by tests, with `make test-q1` green apart from known pre-existing
failures. Stop and report when blocked on a user decision or denied permission. Circuit breaker: on
a loop, a hard blocker or no progress, clear the goal and write a failure report here.

## Problem

The next number is max+1 over the local `issues/` only. On 2026-10-29 the owner reserved 1009 on
another machine; the fork clone would have handed out 1009 again. 1014 added `--number` as a manual
escape hatch, which still checks only the local tree.

## Proposal (advisor h1012-adv, claude:opus:med)

Next number = max over the working tree and `git ls-tree --name-only <ref> issues/` for every
`refs/remotes/*` ref. `--number` gets the same existence check. Optionally `git fetch --all` with a
short timeout first, skippable by a flag and when offline. No infrastructure; works the same for
owner and fork clones. Gap: tickets filed elsewhere but never pushed (rule: push tickets soon).

## Later options (not in scope)

- **Forge issue as the id source:** extend `--remote` to Forgejo/Codeberg so every machine
  allocates from upstream's issue counter. The only race-free option; needs auth and network and
  adds a second id namespace. Opt-in for many parallel filers.
- Rejected: a reservation commit pushed first (forks cannot push upstream) and per-machine ranges
  (messy numbering, no age order).
