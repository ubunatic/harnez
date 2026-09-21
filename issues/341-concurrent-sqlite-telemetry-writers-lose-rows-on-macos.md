# 341 — Concurrent SQLite telemetry writers lose rows on macOS

**Status**: Open — fixed on Linux, awaiting macOS CI confirmation
**Priority**: P2 (Medium)
**Severity**: Data Loss (Telemetry)
**Category**: Cross-Platform / Concurrency
**Related**: [338](338-macos-ci-verification-via-github-mirror-and-homebrew-release-packaging.md), [docs/MacOSPortability.md](../docs/MacOSPortability.md)

---

## 1. Problem & Motivation

`internal/telemetry.TestConcurrentWriters` fails on real macOS (`macos-14` GitHub
Actions runner) but passes on Linux. It exercises two concurrent writers appending
rows to a shared SQLite tool-catalog database and expects 50 total rows after both
finish; on macOS it observed only 25 rows, with `writerB` failing outright:

```
writerB: Open: telemetry: /var/folders/.../tool_catalog.sqlite has schema version 0,
need 2, and this package has no migration framework — delete the file (it's a local
telemetry cache, safe to lose) and it will be recreated on next use
```

This points to a real cross-platform concurrency gap, not a test-environment
artifact: on macOS's filesystem (APFS) two writers appear to race on first-open
schema initialization — writer B seemingly reads (or initializes) the sqlite file
before writer A has committed its schema, landing on `schema version 0`. Linux
(likely ext4/tmpfs under `go test`'s TempDir) doesn't exhibit the same race, whether
due to differing fsync/locking semantics or differing timing.

Discovered via the `macos-hello` CI workflow (issue 338) — first real-macOS test run
surfaced this; it was invisible on Linux CI.

**Confirmed flaky, not deterministic**: a second `macos-hello` run (same code, no fix
applied) passed `TestConcurrentWriters` cleanly. This is consistent with a genuine
race condition rather than a hard macOS incompatibility — do not close this ticket on
the strength of a passing run; it needs to pass reliably across many runs (or under
`-race` with an artificial delay forcing the interleaving) before being considered
fixed.

## 2. Technical Specification

1. Reproduce locally against a real or virtualized macOS host (or narrow the race
   with `-race` and forced scheduling delays) to confirm root cause: concurrent
   schema-creation race vs. a locking/fsync difference vs. something else entirely.
2. Fix likely needs either:
   - A proper `CREATE TABLE IF NOT EXISTS` / schema-init guarded by a single-writer
     lock (e.g. `BEGIN IMMEDIATE` transaction, or a sqlite `busy_timeout` PRAGMA) so
     concurrent first-opens can't race past schema creation, or
   - An actual migration framework (the error message already flags "this package
     has no migration framework") if schema versioning needs to be more robust than
     an equality check.
3. Do not just special-case macOS to skip the test — this is in the critical path
   for telemetry data integrity generally; the race could in principle also occur on
   Linux under different filesystem/timing conditions.

## 3. Implementation & Verification Plan

1. Reproduce `TestConcurrentWriters` failure locally (may require `GOOS=darwin`
   cross-testing constraints, or run via `make macos-ci` against the GitHub mirror).
2. Add locking/transaction guard around first-open schema initialization in
   `internal/telemetry`.
3. Re-run `TestConcurrentWriters` under `-race` on both Linux and via `make macos-ci`
   until the 50-row assertion passes reliably (not just once — race conditions can
   pass by luck).
4. Close only after a real macOS CI run (not just local Linux) confirms the fix.

## Sprint Status (lean-sprint, 2026-09-21)

Root cause reproduced on Linux: the check-then-`ALTER TABLE` migration was not serialized, so
concurrent openers hit `duplicate column name: model` and lost a row (`TestConcurrentMigrationMany`,
32 processes, failed before the fix). Fixed by running schema init and migration inside one
connection-pinned `BEGIN IMMEDIATE` transaction. Remaining before closing: a real macOS CI run
(`make macos-ci`) must pass repeatedly.
