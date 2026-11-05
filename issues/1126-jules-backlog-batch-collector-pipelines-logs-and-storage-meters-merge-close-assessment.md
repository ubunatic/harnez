# 1126 — Jules backlog batch: collector pipelines, logs and storage meters (merge/close assessment)

**Status**: Open
**Priority**: P3 (Low)
**Severity**: Minor
**Category**: Chore
**Related**: 085, 111, 113, 152, 161, 255, 256, 343, 404

---

/goal Get a checked per-ticket verdict (keep, merge into another, close as done/obsolete) for the backlog tickets
below and apply it; or stop and report when blocked on an owner decision.

## Tickets (grouped by title)
- 085 Briefly show agent-collector daemon status in harnez usage --watch
- 111 Per-agent collector pipelines: independent cadence, stricter timeout, cancellation
- 113 Record per-collector roundtrip times; expose via harnez usage --meta
- 152 Move agent-collector under usage instead of top-level
- 161 Collector: absorb remote-load control host + opt-in Prometheus exposition
- 255 Collector resilience and diagnostics: startup retries, error hints, TUI logs overlay
- 256 Persist rolling watch and collector launch logs to disk
- 343 Display host disk usage and storage meters in the Load TUI
- 404 HDD/SSD storage and I/O usage metrics in the usage watch TUI

## Plan
1. One Jules audit run over this ticket (report `reports/jules/1126.md`).
2. Check the report's evidence (jules skill Section 4), then close, merge or correct each ticket.

## Jules runs
| kind | session | Jules ID | PR | state |
| --- | --- | --- | --- | --- |
| batch audit | jules-batch-1126 | | | pending |
