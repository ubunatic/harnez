# 667 — Lite IssueTracking doc points to itself in consumer repos

**Status**: Closed — lite doc points to 'harnez docs variant issue-tracking full'; init copy log prints the actual (lite) source
**Priority**: P3 (Low)
**Severity**: Minor
**Category**: Bug
**Related**: [lite doc](../docs/practices/IssueTracking.lite.md), [ramp](../internal/assess/ramp.go)

---

## 1. Problem

`harnez init` in voxi (2026-10-01) installed the lite variant as `docs/IssueTracking.md`.
Its line 4 now says "Full doc in `docs/IssueTracking.md`" — the file itself. Before, it
pointed to `docs/practices/IssueTracking.md`, which does not exist in consumer repos.
Either way the reader has no reachable full doc.

The init log also says `copied docs/practices/IssueTracking.md → ./docs/IssueTracking.md`,
although the lite variant (`IssueTracking.lite.md`) was what got copied.

## 2. Fix ideas

- Lite doc: name a reachable source for the full doc (e.g. `harnez read` of the bundled
  full doc, or the upstream URL), or drop the pointer.
- Init log: print the actual source file (`IssueTracking.lite.md`).
