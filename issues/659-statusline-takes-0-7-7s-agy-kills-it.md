# 659 — statusline takes 0.7-7s, AGY kills it

**Status**: In Progress — AGY statusline kill recurred at 14:50:40 on 2026-10-10; investigate regression
**Priority**: P2 (Medium)
**Severity**: Minor
**Category**: Bug
**Related**: 657 (usage store migration)

---

## Problem
AGY runs `harnez statusline --agent agy` (`~/.gemini/antigravity-cli/settings.json:44`) on every
render and kills it on timeout. Idle calls take 0.13-0.23 s; under load (`make test-q1`, worker
agents) single calls took 0.67-7.2 s and were killed. Effect: a missing status line for one
render, no data loss. P2, not P1: AGY recovers on the next call (failure 1/30) and nothing else
depends on the output; the fix is still cheap and should not wait long.

## Findings (code at b535de6f)
Rendering itself is pure: `cmd/harnez/statusline.go:34` parses stdin JSON and prints, no
I/O. After printing, the same process synchronously writes to the usage store
(`cmd/harnez/statusline.go:41` → `internal/usage/passive.go:44`), and the process does not exit
until that returns. Per call:

1. `usagestore.OpenWithBusyTimeout` (`internal/usagestore/store.go:93`) opens SQLite with
   `busy_timeout` 25 ms (`spec/usage.yaml:11`) and `journal_mode(WAL)`; the ctx timeout is
   4×busy = 100 ms (`internal/usage/passive.go:52`).
2. `EnsureSchema` runs ~10 `CREATE … IF NOT EXISTS` statements every call
   (`internal/usage/passive.go:59`, `internal/usagestore/store.go:980`).
3. **Main suspect:** `MigrateStableWindowKeys` (`internal/usage/passive.go:62`,
   `internal/usagestore/store.go:222`) opens a write transaction, runs an `UPDATE` over
   `usage_observations` and `SELECT`s **all** `quota_windows` rows on every call. It has no
   "already done" early exit, unlike the summary migration (`store.go:156`), which checks
   `usage_store_migrations` first. Cost grows with the table, and it takes the write lock
   every time, colliding with tests and workers that write the same DB.
4. Whether the 100 ms ctx really cancels a running modernc SQLite statement is unverified;
   if not, steps 2-3 can run long past the budget.
5. Go startup: 32 MB binary plus package `init()` work, e.g. six bitmap fonts built in
   `internal/readcard/font.go:169`, all paid before rendering. Under CPU contention, startup
   alone may account for the idle 0.13 s growing to seconds.

## Goal
/goal `harnez statusline --agent agy` (and `--agent claude`) prints within budget under load:
**p99 < 250 ms, max < 500 ms over 200 calls while `make test-q1` runs plus 2 worker agents**,
idle p50 < 50 ms. Rendering must never wait on the usage store: the passive write is
best-effort, cannot block exit beyond a hard cap, and runs no migrations or schema DDL.
Exit when the budget holds and the tests below pass, or stop and report the measured
breakdown if Go startup alone exceeds the budget under load.

Measure with a small script (keep it as `scripts/statusline-bench.sh`): 200 calls of
`harnez statusline --agent agy < testdata payload` with `/usr/bin/time -f %e` or
`date +%s%N`, print p50/p99/max; run once idle and once during `make test-q1`.

## Milestones
- **M1 Reproduce:** the bench script plus a Go test that holds a write lock on a temp DB
  (open tx, `BEGIN IMMEDIATE`) and a large `quota_windows` table (e.g. 50k rows), then calls
  `ObserveStatuslineAt`; it must fail today on the time limit (e.g. > 150 ms).
- **M2 Fix the store path:** move `EnsureSchema`/`MigrateStableWindowKeys` out of the
  statusline path (run them in `harnez apply`/usage commands, or gate them on a
  `usage_store_migrations` row like `store.go:156`); enforce a hard wall-clock cap on the
  whole passive write (stdout written and flushed first); the M1 test passes.
- **M3 Startup:** measure `harnez --version` vs statusline under load; if startup dominates,
  make font tables lazy (`sync.Once`) and check other `init()` work. Bench meets the budget.

## Edge cases (each needs a test)
- No DB file / no data dir: print the line, exit 0, no error output.
- DB locked by another writer for seconds: line printed, process exits within the cap.
- Empty or malformed stdin JSON: same output as today; no store access.
- Old DB without the migration done: statusline does not migrate; the next usage
  command does.
- Stale cache: the statusline shows only what the payload carries; it never fetches live.

## History
- 2026-09-30: AGY killed it (`~/.gemini/antigravity-cli/log/cli-20260930_163033.log`,
  22:13:25, failure 1/30). Measured 0.67, 2.2, 3.0, 7.2, 7.2 s per call during `make test-q1`.
- 2026-10-10: recurred, `statusline: command failed: signal: killed (stderr: ) (failure 1/30)`
  at 10:31:51 (`cli-20261010_102628.log`, line 290) while agent workers ran. Idle right after:
  0.13, 0.14, 0.23 s.
- 2026-10-10: recurred again, `command failed: signal: killed (stderr: )` at 14:50:40
  (`cli-20261010_145040.log`); user reports the AGY status line is missing again.
