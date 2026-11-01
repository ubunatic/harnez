# 1078 — harnez quota fails in agy (Gemini) sandbox: read usage store unable to open database file (14)

**Status**: Closed — Fixed in 3edbe6bb: OpenReadOnly falls back to immutable=1 when the WAL -shm cannot be created
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

## 4. Resolution

Root cause: `usagestore.OpenReadOnly` opened the WAL-mode database with `mode=ro`. In a read-only data dir SQLite cannot create the `-shm` file, so the first read failed with CANTOPEN (14). `Ping` did not touch the file, so the failure surfaced only at query time.

Fix (`internal/usagestore/store.go`): the open is now verified with a real read (`SELECT count(*) FROM sqlite_master`). On failure the store reopens the same path with `immutable=1`, which reads the main file without WAL or `-shm`. Limitation: WAL frames not yet checkpointed into the main file are invisible in that fallback, so the newest observations may be missing until the writer checkpoints. Writes stay rejected (`query_only`).

Test: `TestOpenReadOnlyReadsWALDatabaseInReadOnlyDir` (skipped as root). Quota-1 `make test-q1` run once after the change.
