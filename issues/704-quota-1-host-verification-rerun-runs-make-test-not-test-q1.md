# 704 — Quota-1: host verification rerun runs make test, not test-q1

**Status**: Open
**Priority**: P2 (Medium)
**Severity**: Normal
**Category**: Docs
**Related**: `.harnez/rules/Quota.md`, `docs/AgenticLoop.md`, lean-sprint skill; loom `AGENTS.md` ("the host reruns `make test-q1` before closing the ticket"), loom `docs/LeanSprints.md` (2026-10-04 notes)

---

/goal Let the host's mandated post-developer test rerun work without a bypass flag by making the rules say the host runs `make test`, and propagate that to bundled rules/skills and repos that say "host reruns `make test-q1`"; stop and report if the user prefers a different mechanism.

## Problem

In loom sprints (2026-10-04, tickets 257-263) the host had to rerun the suite after each developer commit (loom AGENTS.md). `make test-q1` refused every time: "no repository source files have been modified since the last test run" — the developer's run already consumed the quota. The host then used `QUOTA_BYPASS=1`, which `.harnez/rules/Quota.md` forbids for agent loops. The rule set has no allowed path for the required host verification.

## Proposal (user)

The host (orchestrator/reviewer) verifies with plain `make test`, which does not go through the Quota-1 guard. Quota-1 (`make test-q1`) stays the developer's test entry point.

## Scope

- `.harnez/rules/Quota.md`: add a rule like "Host verification: the host's single rerun after a developer commit uses `make test` (no quota); developers keep `make test-q1`." Note the duplicated section in that file while editing.
- Lean-sprint / reverse-sprint skills and `docs/AgenticLoop.md` wherever the host rerun is described.
- Harnez-managed repo instructions that say "host reruns `make test-q1`" (e.g. loom AGENTS.md) — regenerate or note for `harnez init`.
- Keep output-to-file + `--- FAIL` grep guidance for the host run too.
