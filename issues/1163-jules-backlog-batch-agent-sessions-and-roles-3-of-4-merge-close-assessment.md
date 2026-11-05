# 1163 — Jules backlog batch: agent sessions and roles (3 of 4) (merge/close assessment)

**Status**: Draft
**Priority**: P3 (Low)
**Severity**: Minor
**Category**: Chore
**Related**: 639, 674, 680, 681, 683, 691, 721, 733, 753, 875

---

/goal Get a checked per-ticket verdict (keep, merge into another, close as done/obsolete, taia candidate, deinvest) for the backlog tickets
below and apply it; or stop and report when blocked on an owner decision.

## Tickets (grouped by title)
- 639 Recommend `loom eval` for .ansi Asset Validation in Agent Guidelines
- 674 test: TestAgentDetachedSpawnAndWait flaky under full-suite load (cannot establish ownership of process)
- 680 Probe Codex cross-task messaging for agent send
- 681 Copyable AgenticLoop doc links to non-copyable TUIDesign
- 683 Learning: agent-friendly CLI design rules (TTY, prompts, side effects, shell wrappers)
- 691 track Claude and Agy sessions in agent list
- 721 Support OpenCode as a native Harnez agent
- 733 Add an AGPL-3.0 license rule to the guidance every agent loads
- 753 Audit and document bash heredoc appends vs native edit tool guidelines for agents
- 875 warn the orchestrator when its shells, agents or the host run short on memory, swap or CPU

## Plan
1. One Jules audit run over this ticket (report `reports/jules/1163.md`).
2. Check the report's evidence (jules skill Section 4), then close, merge or correct each ticket.
3. Collect `taia candidate` and `deinvest` verdicts as leads for the owner; do not move or close on them alone.

## Jules runs
| kind | session | Jules ID | PR | state |
| --- | --- | --- | --- | --- |
