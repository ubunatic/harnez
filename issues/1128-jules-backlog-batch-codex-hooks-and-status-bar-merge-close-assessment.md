# 1128 — Jules backlog batch: Codex hooks and status bar (merge/close assessment)

**Status**: Closed — Batch applied, reports/jules/1128.md
**Priority**: P3 (Low)
**Severity**: Minor
**Category**: Chore
**Related**: 141, 143, 144, 205, 206, 207, 383

---

/goal Get a checked per-ticket verdict (keep, merge into another, close as done/obsolete) for the backlog tickets
below and apply it; or stop and report when blocked on an owner decision.

## Tickets (grouped by title)
- 141 Show the running-agent count in the Codex status bar
- 143 Show Git status in all agent status bars
- 144 Ensure Codex uses the correct model for subagents
- 205 harnez status/diff parity for the Codex hooks entry
- 206 Confirm the toolinput.command field name against a live Codex PreToolUse call
- 207 Determine the real Codex timeoutSec default for PreToolUse hooks
- 383 Disable queued question tool prompts in Codex sessions

## Plan
1. One Jules audit run over this ticket (report `reports/jules/1128.md`).
2. Check the report's evidence (jules skill Section 4), then close, merge or correct each ticket.

## Jules runs
| kind | session | Jules ID | PR | state |
| --- | --- | --- | --- | --- |
| batch audit | jules-batch-1128 | 12352351152984640649 | GH #83 (closed) | done: 8/8 refs held; 206, 207 closed done; 383 kept (report wrong: auto_reply is Jules-only, not Codex); 141 kept, not merged (Codex agent count and Claude git status are separate features); 143, 144, 205 kept |
