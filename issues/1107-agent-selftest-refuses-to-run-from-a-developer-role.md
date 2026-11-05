# 1107 — agent selftest refuses to run from a developer role

**Status**: Open
**Priority**: P2 (Medium)
**Severity**: Minor
**Category**: Bug
**Related**: taia (codeberg.org/harnez/taia), 1083 (grooming for taia focus)

---

/goal A developer-role worker can read the selftest steps, with a test. Stop and report when blocked on a user decision or denied permission; on a loop or no progress, clear the goal and write a failure report here.

## Problem

Reported by the taia session (2026-11-03): `harnez agent selftest` refuses to run from a developer role, so a worker cannot see the real step format it should follow. Allow a read-only form (e.g. print the steps) for developer roles, or explain in the refusal how to get it. Related: 1012, 1008.

## Audit 2026-11-04
Jules audit (session 10604890370916608907, against 70da579e, `reports/jules/1107.md`): references hold;
Jules ran `HARNEZ_AGENT_ROLE=developer go run ./cmd/harnez agent selftest` and got the leaf-worker
refusal. Cause: `guardLeafRole` (`cmd/harnez/agent_run.go:1229-1239`) allows only list, status, models,
help, agent and send for leaf roles. Jules said "partly done", but nothing of the fix exists: still valid.
Jules's next step (add `selftest` to that allow list) is too broad, since selftest steps may start
agents; keep the ticket's read-only form (print the steps) or a refusal that says how to get them.

## Jules runs

| kind | session name | Jules ID | PR | state |
| --- | --- | --- | --- | --- |
| audit | jules-audit-1107 | 2323368347644472118 | github #94 (closed) | done: still valid, 1/1 refs hold (guard now at agent_run.go:1292); repeats 2026-11-04 audit, same too-broad next step |
