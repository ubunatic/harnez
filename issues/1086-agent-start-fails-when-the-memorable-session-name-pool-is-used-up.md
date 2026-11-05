# 1086 — agent start fails when the memorable session name pool is used up

**Status**: Open
**Priority**: P2 (Medium)
**Severity**: Major
**Category**: Bug
**Related**: 439, 479

---

/goal `harnez agent` without `--name` never fails because the name pool is used up, and finished sessions do not hold names forever, or stop and report when blocked on a user decision or denied permission. Circuit breaker: on a loop, a hard blocker or no progress, clear the goal and write a failure report here.

## Problem

Reported by the sitegen session on 2026-11-01. In `~/projects/sitegen`, this exited with code 1 before doing any work:

```
HTO=0 harnez agent --model claude:haiku:med --role developer -d . -p "..."
Error: all memorable session names are in use
```

Passing `--name` worked. `harnez agent list` there showed no sessions and "1 more agents in other sessions; use --all-sessions", so it gives no hint about what holds the names. `--all-sessions` and the store in `~/.harnez/agents` were not inspected. The error comes from `internal/subagent/name.go`.

## Want

- Automatic naming falls back, for example to a numeric suffix, instead of failing.
- Old finished sessions stop holding names. Check first what holds them, and whether resuming a finished session by name must keep working.
- The error or `list` output shows what holds the names, if a failure remains possible.

## Jules experiment findings (GitHub PR 62, 2026-11-01)

A numeric suffix fallback in `GenerateSessionName` is a small change, and tests matching `Name` in `internal/subagent` pass at the PR head. Do not copy it as is: its test only marks names under 15 characters as taken, so it may never reach the fallback, and the loop never ends when every name counts as taken. It also leaves finished sessions holding names. The store is read from `.json` files, per the report; not independently checked.

## Audit 2026-11-03

Jules audit (PR GH#68, report `reports/jules/1086.md`, checked against d8904997; one line off by one): still valid. `internal/subagent/name.go:26` fails with a static "all memorable session names are in use"; `cmd/harnez/agent_run.go:408-410` reserves every session's name, finished ones included. Smallest step: fall back to a numeric suffix when the pool is exhausted, so finished sessions keep resumable names.

## Jules runs

| kind | session name | Jules ID | PR | state |
| --- | --- | --- | --- | --- |
| implement | jules-implement-1086 | 9431202035978934268 | github #98 (open, not mergeable as is) | done; tests pass, new test fails without fix. Suffix fallback (name.go) OK. Freeing names of completed/failed/stopped sessions (agent_run.go:445) skips the resume-by-name check: Find returns the first name match (session.go:159), so a reused name can resume the old session. Take the fallback only. |
