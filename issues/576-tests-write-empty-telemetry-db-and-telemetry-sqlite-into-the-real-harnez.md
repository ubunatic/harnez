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

## Research findings (2026-09-25)

- **Premise verified on HEAD:** `/home/uwe/.harnez/telemetry.db` and `telemetry.sqlite` exist as 0-byte files; `tool_catalog.sqlite` is the populated store. `internal/telemetry/DefaultDBPath` resolves only to `~/.harnez/tool_catalog.sqlite` ([telemetry.go](/home/uwe/projects/harnez/internal/telemetry/telemetry.go:28)).
- **No production source or template names either decoy path.** Go test references to `telemetry.sqlite` pass paths under `t.TempDir()`, including [index_test.go](/home/uwe/projects/harnez/cmd/harnez/index_test.go:52), [hook_test.go](/home/uwe/projects/harnez/cmd/harnez/hook_test.go:338), and [stats_agents_test.go](/home/uwe/projects/harnez/cmd/harnez/stats_agents_test.go:106). Repository source/template search found no `telemetry.db` references.
- `cmd/harnez`’s `TestMain` sets `HOME` to a temporary directory for the package test process ([main_env_test.go](/home/uwe/projects/harnez/cmd/harnez/main_env_test.go:14)). The inspected explicit DB-path tests are also temporary-directory isolated. This makes those tests unlikely sources; no suite run was performed.
- `git log -S` finds no production introduction of either name; the matches are historical docs/ticket references and temporary test paths. `docs/Databases.md` already records the files as empty leftovers and says tests are the only references ([Databases.md](/home/uwe/projects/harnez/docs/Databases.md:52)).
- **Conclusion:** the ticket’s suspected test leak is unsupported by current source. A manual SQLite invocation or older external code remains plausible, but cannot be established from HEAD.

**Minimal resolution:** correct `docs/Databases.md` Leftovers to say the files have no identified creator and are safe to delete; retain the canonical-store inventory. No code change or new test appears warranted.

**Acceptance checks:** confirm no production source/template references either decoy name; confirm all test DB-path references remain temporary-directory based; confirm the leftovers note no longer attributes creation to tests.

## Host decision (2026-09-25)

Docs-only. No code source found; delete the two empty files, correct `docs/Databases.md` Leftovers (no identified creator, likely a manual `sqlite3` on a guessed name), close.
