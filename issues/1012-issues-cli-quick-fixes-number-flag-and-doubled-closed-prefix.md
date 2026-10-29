# 1012 — issues CLI quick fixes: `--number` for `new` and the doubled "Closed —" prefix

**Status**: Closed — issues new --number and status-word dedup landed
**Closed-By**: claude:sonnet:med, session h1012-dev
**Priority**: P3 (Low)
**Severity**: Minor
**Category**: Bug
**Related**: 991, 1010

---

/goal Both quick fixes below are in, each locked by a test, and `make test-q1` passes apart from
known pre-existing failures. Stop and report when blocked on a user decision or denied permission.
Circuit breaker: on a loop, a hard blocker or no progress, clear the goal and write a failure report
here.

Group ticket for small `harnez issues` fixes found while filing tickets on 2026-10-29.

## 1. `issues new --number <n>`

`harnez issues new` always takes the next free number (`NextNumber`). When the owner has reserved
a number remotely (1009 was being written on another machine), there is no way to skip it; 1010
had to be created by hand. Add `--number <n>`: use that number, refuse it when a ticket with that
number already exists locally, and otherwise behave exactly like today. Proper number management
(reservations across machines) is a later, separate ticket.

## 2. `issues close` doubles the status word

`harnez issues close 991 "Closed — items 1-3 merged"` wrote
`**Status**: Closed — Closed — items 1-3 merged`. When the reason already starts with the verb's
status word and a dash ("Closed —", "Closed -", case-insensitive), strip it before writing. Apply
the same rule to the other verbs that take a reason (open, start, block, draft, backlog).
