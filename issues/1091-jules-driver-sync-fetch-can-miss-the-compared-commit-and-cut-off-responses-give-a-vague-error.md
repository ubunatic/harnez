# 1091 — Jules driver: sync fetch can miss the compared commit, and cut-off responses give a vague error

**Status**: Closed — obsolete: Google stops accepting new Jules work on 2026-11-11 and shuts Jules down on 2026-12-31 (see the jules skill); this fixes a Jules start/driver path that will not run again
**Priority**: P3 (Low)
**Severity**: Minor
**Category**: Bug
**Related**: 1074, 1088

---

/goal Both leads below are confirmed and fixed or closed as not real, or stop and report when blocked on a user decision or denied permission. Circuit breaker: on a loop, a hard blocker or no progress, clear the goal and write a failure report here.

## Findings

From the Jules ideas run (GitHub PR 61), read against `internal/subagent/jules.go` at `3cb914df`. Leads, not yet reproduced. Jules stops taking new work on 2026-11-11, so close this as moot if neither is fixed by the shutdown on 2026-12-31.

- **Sync fetch race:** `syncGitHubBranch` (lines 425-432) fetches the branch by name when a commit is missing locally. If the remote moved in between, the compared commit can still be absent, and `merge-base` then reports it as "not strictly behind", which is a misleading warning.
- **Cut-off responses:** the proposal says a response cut at `julesMaxResponseBytes` fails with a generic JSON decode error (`decode Jules API response`, line 189). Its citation of line 179 was off, so check where the limit is applied.
