# 1059 — Interpret slash-separated model picks by skill role

**Status**: Open
**Priority**: P2 (Medium)
**Severity**: Minor
**Category**: Enhancement
**Related**: 981, 942

---

## Problem

A skill invocation such as `/lean-sprint luna/terra/sol`, `/lean-sprint haiku/terra/argon`, or `/lean-sprint cheap/lean/smart` should let the agent select the models for the workflow's respective tasks without the owner spelling out every role. For lean-sprint, the intended order is searcher, developer, reviewer.

## Goal

/goal Define and implement clear slash-separated model/trait selection for applicable skill workflows, verify that each position maps to the intended role and explicit selections are respected, or stop and report when blocked on a user decision or denied permission. On a loop, hard architectural or OS blocker, or no progress, clear the goal and write a failure report with the resume point.

Recheck current skills, model resolution and recent history before implementation. Determine how each applicable skill defines role order; handle missing, extra, unknown or ambiguous picks without silently substituting a different model. Clarify provider/trait resolution and scope rather than assuming all skills have identical roles. Verify examples and negative cases against the actual invocation and dispatch workflow.

Recovered from unpublished commit `b7ced8e7`; its original issue number 1054 was independently allocated on main.

## Audit 2026-11-11

Jules audit (PR GH#186, report `reports/jules/1059.md`, checked against 7332ae6a; 1/1 reference holds): still
valid. The only slash-separated pick parser is `harnez models set <cheap>/<lean>/<smart>`
(`cmd/harnez/models_cmd.go:106`); no skill source defines a positional pick order. Correction: the report's
next step (parse picks in `internal/tui/turn.go`) is wrong. Skills are prose the agent reads, so the change
belongs in the skill sources, starting with `docs/commands/lean-sprint.md`: its invocation list (line 37) and
its role sections (Searcher line 47, developer line 61) are where a `<searcher>/<developer>/<reviewer>` form
would be defined.

## Jules runs

| kind | session name | Jules ID | PR | state |
| --- | --- | --- | --- | --- |
| audit | jules-audit-1059 | 8735768250679803037 | github #186 (closed) | done: still valid, 1/1 ref holds; next step corrected |
