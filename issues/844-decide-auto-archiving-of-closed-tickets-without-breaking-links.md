# 844 — Decide auto-archiving of closed tickets without breaking links

**Status**: Backlog — grooming pass 843 M4 (2026-10-22); promote when needed
**Priority**: P3 (Low) — parked by user for later discussion
**Severity**: Minor
**Category**: Design
**Related**: 842, 843

---

/goal Reach a user decision on whether and how closed tickets move to `issues/archive/`, or stop
and report when blocked on that decision.

## Context

842 research proposed that `issues close` moves tickets to `issues/archive/` (543 of 562 closed
tickets still sit in `issues/`). The user declined for now (2026-10-21) because file moves break
links in docs, studies, commits and other repos. Discuss later; options from the reports include
a link-rewrite on move, ID-based links/redirect stubs, or never moving and relying on the
active-only index from 843.
