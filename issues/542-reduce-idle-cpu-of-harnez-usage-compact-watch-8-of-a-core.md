# 542 — Reduce idle CPU of harnez usage --compact --watch (~8% of a core)

**Status**: Open
**Priority**: P2
**Severity**: Medium
**Category**: Performance
**Related**: [[541-harnez-exec-busy-waits-at-1-core-stopped-group-monitor-scans-all-of-proc-every-25ms]], [[535-review-agent-collector-lifecycle-automation-install-auto-start-keep-current]]

---

## Observed (loom session report, 2026-09-24)

`harnez usage --compact --watch` (PID 526524) had used 74 CPU minutes over 14h, a steady ~8% of a
core, while mostly idle.

## /goal

The watch view uses well under 1% of a core while idle, and the refresh behaviour the user sees stays
the same.

## Notes

- Profile first (`pprof` or `perf top -p`), then pick the fix: a longer interval for the expensive
  parts, event-driven refresh (file watches on session stores, telemetry DB), caching of parsed
  transcripts or quota readings, or skipping redraws when nothing changed.
- Suspects to check: per-tick full re-reads of transcripts or JSONL, /proc scans (see 541), and the live
  mic meter (245/265) if it is enabled in compact mode.
- Acceptance: `ps -o %cpu` over 10 minutes idle stays <1%.
