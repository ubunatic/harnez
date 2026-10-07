# 749 — Prune stale channel registry entries

**Status**: Open
**Priority**: P3 (Low)
**Severity**: Minor
**Category**: Hygiene
**Related**: 740 (channel registry), 747

---

/goal Registry files in `~/.harnez/channels` whose PID is dead or whose endpoint fails the
authenticated health check are removed (by `harnez channel list`/`send` or `harnez clean`),
with a test; or stop and report when removal could hit a live session.

## 1. Problem & Motivation
Entries are removed only on a clean server exit. Killed or crashed Claude sessions leave files
behind: on 2026-10-07 the dir held 10 files with one live session. `list` already hides them.

## 2. Technical Specification / Findings
- Never remove an entry whose PID is alive and whose endpoint answers; a PID reused by another
  process fails the identity check and is stale.
- Fits `harnez clean` (stale process records) or a prune step in `channel.List`.

## 3. Implementation & Verification Plan
- Test: a dead-PID fixture is removed, a live fixture is kept.
