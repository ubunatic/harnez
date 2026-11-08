# 1187 — detached Jules start reports Started agent when Jules creation failed

**Status**: Closed — Detached Jules start now waits for session creation and exits non-zero with the create error (awaitJulesCreate in cmd/harnez/agent_async.go)
**Priority**: P2 (Medium)
**Severity**: Moderate
**Category**: Bug
**Related**: 1185

---

## Problem

On 2026-11-08 at about 03:4x, `harnez agent start --model jules --detach --name taia-jules-audit-94`
printed `Started agent taia-jules-audit-94` and exited 0, although Jules created no session.
The worker ended `failed` with no `jules-id`. A non-detached start at the same time got
`HTTP 400: Precondition check failed`. A host that trusts the detached success counts a run that
does not exist. Reported by the taia session.

Not a duplicate of 1185: that ticket is about `agent list` dropping a run that really exists.

## Want

- A detached Jules start waits until the Jules session is created (or creation fails) before
  printing success, and exits non-zero with the create error when it fails.
- Test: a fake Jules client that returns HTTP 400 on create; assert the detached start fails and
  names the error.

Starting point (unverified): the detach path in `cmd/harnez/agent_run.go` (`foregroundDetachTimeout`,
worker spawn with `--worker-session`) and `cmd/harnez/agent_async.go` `launchDetachedWithPreflight`.

/goal A detached Jules start never reports success for a session Jules did not create; stop and
report if the create step cannot be observed before detaching.
