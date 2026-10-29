# 1026 — Agent session record shows compaction_count 0 for Pi sessions that compacted

**Status**: Open
**Priority**: P3 (Low)
**Severity**: Minor
**Category**: Bug
**Related**: 1023, 1024, lmcoder 263

---

## Symptom
The 1024 reviewer rerun on `lmcoder:coder` (session `r1024`, 2026-10-29 21:42) compacted 13 times according to
Pi's own session log (`2026-10-29T20-42-38-834Z_01a18966...jsonl`, found by lmcoder 263 section 4), but the harnez
session record said `compaction_count: 0`. The 1023 M3 "after" run also compacted twice; its count in harnez
was not checked.

## Acceptance
- Find where harnez fills `compaction_count` for Pi sessions and why compaction events are not counted (possibly
  because the run was killed before the turn ended, possibly because Pi's compaction entries are not parsed).
- A test with a Pi session log that contains compaction entries asserts the right count.
