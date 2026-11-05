# 1168 — Jules backlog batch: docs, search, TUI and testing (merge/close assessment)

**Status**: Draft
**Priority**: P3 (Low)
**Severity**: Minor
**Category**: Chore
**Related**: 297, 300, 310, 361, 426, 427, 505, 513, 771, 778

---

/goal Get a checked per-ticket verdict (keep, merge into another, close as done/obsolete, taia candidate, deinvest) for the backlog tickets
below and apply it; or stop and report when blocked on an owner decision.

## Tickets (grouped by title)
- 297 Add linked subdocuments for detailed language documentation
- 300 TUIDesign.md gap: raw-mode input has no guidance, SetReadDeadline-on-pty trap is undocumented
- 310 Check for ambient enclosing go.work in harnez status and lint
- 361 harnez docs variant --check CLI verb for lite-doc structural gate
- 426 Make find output text-first for human users
- 427 Preserve ANSI colors in stdin render path
- 505 Copyable-doc reference rule is undiscoverable in CLIDesign
- 513 Real-terminal (TTY) run before a CLI feature is called done
- 771 harnez exec: truncate large bash tool output with spill file log message and spec yaml limit
- 778 harnez tui Background Job Manager

## Plan
1. One Jules audit run over this ticket (report `reports/jules/1168.md`).
2. Check the report's evidence (jules skill Section 4), then close, merge or correct each ticket.
3. Collect `taia candidate` and `deinvest` verdicts as leads for the owner; do not move or close on them alone.

## Jules runs
| kind | session | Jules ID | PR | state |
| --- | --- | --- | --- | --- |
