# 845 — lean-sprint: absolute paths for out-of-repo files in the developer handoff

**Status**: Open
**Priority**: P3 (Low)
**Severity**: Minor
**Category**: Agentic Ergonomics
**Related**: 841 (observed), 699 (teardown rating, same session)

---

/goal Add "absolute paths for any files written outside the repo" to the lean-sprint developer
handoff checklist (skill source, then install), or stop and report when blocked on a user decision.

## Evidence (sprint 841, 2026-10-21)

The ticket said "session scratch dir" for the canary location; the developer cannot know the
host's scratch path, so its plan fell back to shared `/tmp` and the host had to correct it on
resume. Giving the absolute path in the first handoff avoids that turn.
