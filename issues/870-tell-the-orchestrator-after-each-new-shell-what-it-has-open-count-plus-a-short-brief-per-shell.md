# 870 — tell the orchestrator after each new shell what it has open: count plus a short brief per shell

**Status**: Open
**Priority**: P2 (Medium) — orchestrators of every vendor forget their open subtasks and shells
**Severity**: Minor
**Category**: Feature
**Related**: 611 (host polling loop), 581/589 (one background shell per agent run), 630 (HTO=0 session
shells), 868, 869

---

/goal After an orchestrator agent opens a new background shell (subagent, tests, server preview, ...), it is
told how many shells it has open and what runs in each, in a brief that shrinks as the count grows; verified
by tests for each brief size and one live Claude Code session, or stop and report when a host gives no
reliable way to see its open shells.

## Problem (user, 2026-10-23)

Agents from all vendors tend to forget the subtasks and shells they started. Seen live in sprint 714: the host
believed one shell was running while two were (an old `harnez agent resume dev-714` job plus the new
`dev-714b`); the user had to point it out, and the forgotten agent kept editing the shared tree.

## Behaviour

- Trigger: right after the agent starts a new background shell. Not on every tool call.
- Content depends on what can be detected **safely**; never guess. Unknown commands show as "shell".
- Brief sizes:
  - 1-3 shells: one line each, two or three words plus start time, e.g. `make test-q1 (since 12:47)`; a harnez
    agent shows name, model and session length, e.g. `agent dev-714b sol:med, 12m`.
  - More than 3: one short line, count by type, e.g. `5 shells open: 3 agents, 1 test, 1 other`.
- Thresholds and wording in spec (docs/Spec.md), not Go literals.

## Open questions (record answers, do not invent)

1. Where to hook per host: Claude Code PostToolUse on Bash with `run_in_background` can add context; check
   what Codex and AGY offer. A host without a hook gets nothing rather than a wrong brief.
2. How to list a session's open shells: host task list if exposed, else the shells harnez itself started
   (harnez already prints "this session started N agents (M running)" after agent runs).
