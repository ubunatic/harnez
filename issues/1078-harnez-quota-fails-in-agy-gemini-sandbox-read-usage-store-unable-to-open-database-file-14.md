# 1078 — harnez quota fails in agy (Gemini) sandbox: read usage store unable to open database file (14)

**Status**: Open
**Priority**: P2 (Medium)
**Severity**: Major
**Category**: Bug
**Related**: 

---

## 1. Problem & Motivation
`harnez quota` fails inside the agy (Gemini) sandbox with `read usage store: unable to open database file (14)`. A reviewer worker could not verify quota figures, so Quota-1 reviews in agy sandboxes go unchecked.

## 2. Technical Specification / Findings
SQLite code 14 is SQLITE_CANTOPEN: the sandbox likely cannot reach or create the usage store path (read-only home, missing XDG dir, or WAL/-shm sidecar files blocked).

Reported by the taia session from horg worker runs (epic horg#6), 2026-11-01. Source log: codeberg harnez/horg docs/studies/2026-11-01-agent-performance-log.md, section "Tool Experience".

## 3. Implementation & Verification Plan
- Reproduce under agy sandbox; find the resolved store path.
- Open the store read-only (`mode=ro`/`immutable=1`) for read paths, or degrade with a clear message naming the path and sandbox.
- Test: unwritable store dir still lets `harnez quota` report or fail with an actionable error.
