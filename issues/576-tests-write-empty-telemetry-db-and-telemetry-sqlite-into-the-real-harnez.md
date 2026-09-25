# 576 — empty telemetry.db and telemetry.sqlite in the real ~/.harnez

**Status**: Open
**Priority**: P3
**Severity**: Low
**Category**: Tests / Hygiene
**Related**: `docs/Databases.md` (Leftovers)

## Finding (2026-09-25)

`~/.harnez/telemetry.db` (0 bytes, Sep 24 22:31) and `~/.harnez/telemetry.sqlite` (0 bytes,
Sep 16 19:51) exist. No production code opens either name; the real store is
`~/.harnez/tool_catalog.sqlite`.

All test references to `telemetry.sqlite` use `t.TempDir()` (`cmd/harnez/{index,hook,
stats_agents,codexhooks}_test.go`), so those are not the source. Candidates:

- A manual or agent `sqlite3 ~/.harnez/telemetry.{db,sqlite}` call on a guessed name: `sqlite3`
  leaves an empty file behind on a missing DB.
- A test or older code path that resolves a DB path from `$HOME` without isolating `HOME`.

## M1

- Find the source: check `git log -S` for both names, and run the test suite with `HOME` set to a
  temp dir and see whether any file appears under `$HOME/.harnez`.
- If a test leaks, isolate it (`t.Setenv("HOME", t.TempDir())`) and add a guard test. If nothing
  in code creates them, record the finding here and close.
- Delete the two empty files; update `docs/Databases.md` Leftovers.
