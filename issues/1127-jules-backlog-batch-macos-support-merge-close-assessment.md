# 1127 — Jules backlog batch: macOS support (merge/close assessment)

**Status**: Open
**Priority**: P3 (Low)
**Severity**: Minor
**Category**: Chore
**Related**: 334, 336, 337, 338, 339

---

/goal Get a checked per-ticket verdict (keep, merge into another, close as done/obsolete) for the backlog tickets
below and apply it; or stop and report when blocked on an owner decision.

## Tickets (grouped by title)
- 334 Research OS-agnostic process inspection across macOS and Linux
- 336 Research cross-platform shell shim patterns for macOS and Linux
- 337 Research macOS system permissions and CLI whitelist for the config template
- 338 macOS CI verification via the GitHub mirror
- 339 Graceful degradation and gating of hardware telemetry and mic probes on macOS

## Plan
1. One Jules audit run over this ticket (report `reports/jules/1127.md`).
2. Check the report's evidence (jules skill Section 4), then close, merge or correct each ticket.

## Jules runs
| kind | session | Jules ID | PR | state |
| --- | --- | --- | --- | --- |
| batch audit | jules-batch-1127 | 11116387758249995946 | none | failed: "Jules was unable to complete the task", no findings |
| batch audit | jules-batch-1127b | | | pending (retry) |
