# 1123 — Jules backlog batch: mic and audio level (merge/close assessment)

**Status**: Closed — merged into 247 / batch applied (Jules batch 1123, reports/jules/1123.md)
**Priority**: P3 (Low)
**Severity**: Minor
**Category**: Chore
**Related**: 247, 250, 251, 253, 264, 278, 330

---

/goal Get a checked per-ticket verdict (keep, merge into another, close as done/obsolete) for the backlog tickets
below and apply it; or stop and report when blocked on an owner decision.

## Tickets (grouped by title)
- 247 Third mic graph: amplitude-over-time audiogram while recording
- 250 Research Linux desktop mic indicators and live input meters
- 251 Best-effort suppression of desktop mic indicators for the live meter stream
- 253 Mic view (preset 8) triggers the desktop privacy indicator: separate watcher process
- 264 ALSA/arecord live mic-level backend for amixer-only systems
- 278 Mic box: "recording n/a" wording and a real recording-active probe
- 330 Adopt voxi audiolevel's rolling Braille timeline next to the live mic bar

## Plan
1. One Jules audit run over this ticket (report `reports/jules/1123.md`).
2. Check the report's evidence (jules skill Section 4), then close, merge or correct each ticket.

## Jules runs
| kind | session | Jules ID | PR | state |
| --- | --- | --- | --- | --- |
| batch audit | jules-batch-1123 | 14459146956897976025 | GH #81 (closed) | done: 7/7 refs held; 330 merged into 247, 6 kept |
