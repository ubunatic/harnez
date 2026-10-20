# 829 — harnez usage is read-only: only the data collector writes the usage store

**Status**: Open
**Priority**: P2 (Medium)
**Severity**: Minor
**Category**: Architecture
**Related**: 828, 830, 560, 653

---

/goal `harnez usage` (compact, `--watch`, every view) opens the usage store read-only and never writes it; schema setup, migrations, compat imports, observation and history writes happen only in the data collector. A test proves `usage` works against a read-only database file. Stop and report when blocked on a user decision or denied permission.

## 1. Problem & Motivation
Issue 828 found that every `harnez usage` run wrote to the store (schema DDL, key migration, compat
import, observations, snapshots), and those writes caused `database is locked` failures. 828 only
made the writes cheaper and best-effort (ab180545). A reader that writes still contends with writers
and hides who owns the data. One writer, the data collector, keeps locking simple.

## 2. Technical Specification / Findings
- Write paths to move out are in `storeCompactSummary` (`internal/usage/store.go`): `EnsureUsageSchema`,
  `MigrateStableWindowKeys`, `importCompatibility`, `importAgent`, and the history snapshot.
- Open the store read-only (e.g. `mode=ro` / `query_only`) in the `usage` path, so any leftover write
  fails loudly in tests.
- Uncertain: when `usage` collects fresh provider data itself (no collector running), how does that data
  reach the store? Options: hand it to the collector (see 830), or show it unstored. Decide in the plan.
- Uncertain: who runs schema setup and migrations on a fresh install before the first collector run.

## 3. Implementation & Verification Plan
- Plan first: list every write reachable from `harnez usage` and where each one moves.
- Test: `usage` against a read-only (chmod) store renders rows, and no write is attempted.
- Live check: `harnez usage` and `--watch` while the collector writes; no busy note, no lock errors.
