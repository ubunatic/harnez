# 1161 — Jules backlog batch: agent sessions and roles (1 of 4) (merge/close assessment)

**Status**: Closed — Batch applied in the 2026-11-06 Jules backlog cleanup; per-ticket results in this ticket's Jules runs table and docs/studies/2026-11-06-jules-backlog-cleanup.md
**Priority**: P3 (Low)
**Severity**: Minor
**Category**: Chore
**Related**: 128, 176, 274, 281, 285, 304, 317, 352, 370, 382

---

/goal Get a checked per-ticket verdict (keep, merge into another, close as done/obsolete, taia candidate, deinvest) for the backlog tickets
below and apply it; or stop and report when blocked on an owner decision.

## Tickets (grouped by title)
- 128 Per-agent full system-prompt self-audit for repetition and conciseness
- 176 Structured, Capped Subagent Completion-Report Contract
- 274 Add session start hooks to check harnez harness configuration across all agents
- 281 Make advisor discovery opt-in except for highly undecidable problems
- 285 Agent must not claim it "noted" something without writing it to a durable file
- 304 Advisor lifecycle CLI and metadata tracking
- 317 split harnez-advisor per-harness guidance into resources/ reference files
- 352 Tell agents unrelated untracked files from parallel sessions are expected, not a fabrication concern
- 370 Offer useful Go agent capabilities through harnez init
- 382 Make user shell shortcuts available to Codex, Claude, AGY, and other agents

## Plan
1. One Jules audit run over this ticket (report `reports/jules/1161.md`).
2. Check the report's evidence (jules skill Section 4), then close, merge or correct each ticket.
3. Collect `taia candidate` and `deinvest` verdicts as leads for the owner; do not move or close on them alone.

## Jules runs
| kind | session | Jules ID | PR | state |
| --- | --- | --- | --- | --- |
| batch audit | jules-batch-1161 | 16711537219403827925 | github #123 (closed) | done (>25m, one resumed wait): 3/3 refs hold; no closes proposed. All kept |
