# 1026 — Agent session record shows compaction_count 0 for Pi sessions that compacted

**Status**: Open
**Priority**: P2 (Medium)
**Severity**: Minor
**Category**: Bug
**Related**: 1023, 1024, 1027, lmcoder 263

---

## Symptom
The 1024 reviewer rerun on `lmcoder:coder` (session `r1024`, 2026-10-29 21:42) compacted 13 times according to
Pi's own session log (`2026-10-29T20-42-38-834Z_01a18966...jsonl`, found by lmcoder 263 section 4), but the harnez
session record said `compaction_count: 0`. The 1023 M3 "after" run also compacted twice; its count in harnez
was not checked.

The lmcoder dev agent had to count compactions from Pi's log by hand, because harnez's number could not be
trusted: "I can't see harnez's own compaction count. Harnez recorded compaction_count: 0 last time even though
the Pi log showed 13 compactions. So I'll count compactions from Pi's session log when the job ends."

## Scope (owner, 2026-10-29)
Collect the compaction count for every `harnez agent` session, on every provider, and show it where agents
and the owner look (`harnez agent list` or session status), so nobody has to read provider logs.
Likely cause for Pi: harnez counts compactions from telemetry hook events (`internal/telemetry`,
`compaction_pair.go`, `economics_query.go`), and Pi has no such hook. Pi's session log does record each
compaction. Not checked: whether a killed run (the `r1024` case) loses the count too.

## Acceptance
- Per provider (claude, codex, pi/lmcoder, agy): the source of compaction events, and the count filled from it.
  A provider with no source shows "unknown", not 0.
- A test with a Pi session log that contains compaction entries asserts the right count.
- Live: one `lmcoder:coder` run that compacts shows the same count in harnez as in Pi's log.

/goal Make the compaction count of every `harnez agent` session correct and visible, or stop and report when a
provider gives no compaction signal or a decision is needed. Circuit breaker: on a loop or no progress, clear the
goal and write a failure report. Re-check live code first; this ticket was written on 2026-10-29.
