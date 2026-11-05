# 1134 — Jules backlog batch: agent compaction (merge/close assessment)

**Status**: Closed — Batch applied, reports/jules/1134.md
**Priority**: P3 (Low)
**Severity**: Minor
**Category**: Chore
**Related**: 646, 675, 685, 764, 869, 969, 1026

---

/goal Get a checked per-ticket verdict (keep, merge into another, close as done/obsolete) for the backlog tickets
below and apply it; or stop and report when blocked on an owner decision.

## Tickets (grouped by title)
- 646 claude: jev compaction via session.compact function-hook plugin (canary first)
- 675 agent: claude compaction runs on the default model instead of the session model
- 685 harnez compact --auto for the calling session and --dry-run stats
- 764 apply: user-level jev_compaction_enabled in local.yaml
- 869 agy agents auto-compact at 200k tokens by default, per-session flag to turn auto-compact on or off
- 969 Per-model compaction limit and percent, with re-compact and post-compact fill guards
- 1026 Agent session record shows compaction_count 0 for Pi sessions that compacted

## Plan
1. One Jules audit run over this ticket (report `reports/jules/1134.md`).
2. Check the report's evidence (jules skill Section 4), then close, merge or correct each ticket.

## Jules runs
| kind | session | Jules ID | PR | state |
| --- | --- | --- | --- | --- |
| batch audit | jules-batch-1134 | 10325919561618513719 | GH #91 (closed) | done: all 7 kept. Report wrong on 675: `internal/subagent/claude.go:85` resumes with `Model{}`, so `/compact` runs on the default model; that is the bug, not the fix. 646, 685, 764, 869, 969, 1026 kept |
