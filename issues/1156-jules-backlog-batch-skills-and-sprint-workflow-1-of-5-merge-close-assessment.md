# 1156 — Jules backlog batch: skills and sprint workflow (1 of 5) (merge/close assessment)

**Status**: Draft
**Priority**: P3 (Low)
**Severity**: Minor
**Category**: Chore
**Related**: 053, 073, 134, 195, 197, 221, 233, 241, 286

---

/goal Get a checked per-ticket verdict (keep, merge into another, close as done/obsolete, taia candidate, deinvest) for the backlog tickets
below and apply it; or stop and report when blocked on an owner decision.

## Tickets (grouped by title)
- 053 Background LSP diagnostics post stale/wrong findings; harness should detect and offer to disable per-agent
- 073 Agent Canary Container: Claude/AGY/Codex With Mounted Credentials (Deferred)
- 134 Add a ConciseMode trigger Skill alongside `harnez mode`'s persistence mechanism
- 195 Implement a PATH-shim wrapper to route agy's shelled-out tool calls through harnez exec/distill
- 197 "Agent shell enhancements": user-invoked aliases/shell funcs feature
- 221 Promote Go for Scripts and Tooling Across Repos and Explicitly Demote/Disallow Ad-Hoc Python
- 233 AGENTS.md note: verify doc edits target the right file (repo-local vs docs/practices, lang, other source)
- 241 Session-Tree Tool-Call Efficiency Audit Command and Agent Skill
- 286 Promote `golang.org/x/term` for terminal operations in Go conventions

## Plan
1. One Jules audit run over this ticket (report `reports/jules/1156.md`).
2. Check the report's evidence (jules skill Section 4), then close, merge or correct each ticket.
3. Collect `taia candidate` and `deinvest` verdicts as leads for the owner; do not move or close on them alone.

## Jules runs
| kind | session | Jules ID | PR | state |
| --- | --- | --- | --- | --- |
| batch audit | jules-batch-1156 | 17128585227537363207 | pending | running (started 2026-11-06 01:23, audited 2cf3be4d) |
