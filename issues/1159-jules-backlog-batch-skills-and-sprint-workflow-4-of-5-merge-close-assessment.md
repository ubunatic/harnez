# 1159 — Jules backlog batch: skills and sprint workflow (4 of 5) (merge/close assessment)

**Status**: Draft
**Priority**: P3 (Low)
**Severity**: Minor
**Category**: Chore
**Related**: 744, 885, 888, 902, 925, 926, 958, 961, 962

---

/goal Get a checked per-ticket verdict (keep, merge into another, close as done/obsolete, taia candidate, deinvest) for the backlog tickets
below and apply it; or stop and report when blocked on an owner decision.

## Tickets (grouped by title)
- 744 Skill or workflow instruction for reproducible user verification probes and visual demo scripts
- 885 Add optional disk swap backstop for zram-only systems so tmpfs /tmp can spill
- 888 Move agent run/resume orchestration out of cmd/harnez (884 remainder)
- 902 first live run of the epic and new-project skills
- 925 spec: allow user overrides of spec values from ~/.config/harnez
- 926 Support installing custom repository-provided skills into agent environments
- 958 agent review subcommand for second-vendor diff audits
- 961 Agents write scratch files to ./.harnez/scratch by default; harnez init creates it and gitignores it
- 962 harnez agent isolated mode: inference only, no files, network, skills or tools

## Plan
1. One Jules audit run over this ticket (report `reports/jules/1159.md`).
2. Check the report's evidence (jules skill Section 4), then close, merge or correct each ticket.
3. Collect `taia candidate` and `deinvest` verdicts as leads for the owner; do not move or close on them alone.

## Jules runs
| kind | session | Jules ID | PR | state |
| --- | --- | --- | --- | --- |
