# 830 — Hooks and statusline scripts hand usage updates to the data collector instead of writing the store

**Status**: Open
**Priority**: P2 (Medium)
**Severity**: Minor
**Category**: Architecture
**Related**: 829, 828, 653, 560

---

/goal Hooks, statusline scripts and other passive observers never open a write transaction on the usage store. They hand each update to the data collector, which stores it or drops it (duplicate, stale, collector busy), and the observer never blocks on a DB lock. Stop and report when blocked on a user decision or denied permission.

## 1. Problem & Motivation
Statusline observations write to the store directly (`internal/usage/passive.go`, 25ms busy timeout),
so they compete with the collector and with `harnez usage` for the write lock (see 828). Observers run
on every prompt or render and must stay fast. With one writer, the collector, locks are never contended.

## 2. Technical Specification / Findings
- The current direct writers include the statusline passive observation path (`internal/usage/passive.go`,
  `WriteCurrent` in `internal/usagestore/store.go`). Find any other hook-side writers in the plan.
- The collector already decides what to keep (passive dedupe interval in `spec/usage.yaml`).
- Decided (user, 2026-10-20): observers send updates to the running collector service, on its fixed
  per-host port from 829. When no collector answers, the observer drops the update (decided by the
  user, 2026-10-20); it never starts a collector and never blocks.

### Pre-Work from 829 (host review 2026-10-20)
- After 829 M3 (b9e63900) the `harnez usage` header shows `19:42:08 UTC`; before it showed local time
  (`21:19:52 CEST`). Show local time again; add a test that pins the timezone.

## 3. Implementation & Verification Plan
- Plan first: list every observer that writes, and choose the hand-off with its no-collector behaviour.
- Test: an observer completes within its time budget while a blocker holds the DB write lock, and the
  collector later stores (or deliberately drops) that update.
- Docs: record the single-writer rule in the usage architecture doc.
