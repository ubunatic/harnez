# 332 — Consider a real HARNEZ_DB_PATH/HARNEZ_STATE_DIR production override

**Status**: Draft
**Priority**: P3 (Low)
**Severity**: Minor
**Category**: Feature (idea, not yet scoped)
**Related**: issues/326, issues/331, commit `486211c`, `docs/studies/2026-09-13-cli-invocation-log-exit-code-anti-pattern-and-self-pollution.md`

---

## 1. Problem & Motivation

While fixing `TestGearMulticallExecution`'s real-DB pollution (issue 331, commit
`486211c`), the user asked whether harnez should instead properly support
`HARNEZ_DB_PATH`/`HARNEZ_STATE_DIR` as real, production-respected overrides — the test had
been setting env vars of exactly those names, but neither `telemetry.DefaultDBPath()` nor
`resolve.DefaultStateDir()` has ever read them; both resolve purely from
`os.UserHomeDir()`.

This ticket exists to record that idea for deliberate triage, **not** as a commitment to
build it. The assessment at the time leaned against building it just to fix the test:

- The fix that was actually needed (test isolation) is already solved, more robustly, by
  `t.Setenv("HOME", ...)` — matching the repo's one other subprocess-boundary test
  (`internal/claude/bash_shim_test.go`). A `HOME` override isolates *everything* rooted
  under the user's home directory in one call, including any future feature that lands
  there without remembering to also wire a path-specific override; a `HARNEZ_DB_PATH`-only
  fix only ever covers what it explicitly names.
- No `HARNEZ_*` env var in this codebase today is a location override — every existing one
  (`HARNEZ_DISABLE_RATE_FEEDBACK`, `HARNEZ_AGENT`, `HARNEZ_EXPECT_FAILURE`,
  `HARNEZ_DISTILL_AUTOPIPE`, `HARNEZ_DISABLE_CLI_LOG`) is a feature toggle or identity flag.
  Adding a path-override convention would be a new category of configuration surface, not
  an extension of an existing one.
- Every existing in-process test isolates DB/state via Go struct-field injection
  (`testExecOptions.DBPath/StateDir`, `findHistoryOptions.DBPath`, `statsOptions.DBPath`,
  `cliLogOptions`/`logOptions.DBPath`/`StateDir`) — env vars aren't how this repo does test
  isolation anywhere except the one now-fixed subprocess test, which needed them only
  because Go-level injection is unreachable across a process boundary.

## 2. Why this might still be worth doing someday (the case *for*)

- A real user might legitimately want their telemetry DB or session state on a different
  disk/path (multi-user machines, XDG compliance, a containerized or ephemeral `$HOME`,
  syncing telemetry to a shared location deliberately).
- If ever built, it should almost certainly be a `harnez`-wide `--state-dir`/`$HARNEZ_HOME`
  concept (relocating the whole `~/.harnez` root) rather than two independent
  `HARNEZ_DB_PATH`/`HARNEZ_STATE_DIR` variables, to avoid the exact "only covers what it
  names" gap noted above. Would need to interact sensibly with `-d`/`--dir` flags that
  `find`/`issues`/`index` already use for *repository* directories (different concept, same
  letter — naming collision risk to design around explicitly).

## 3. Proposed scope

Not scoped. If the user decides this is worth pursuing, the next step is a design pass
(possibly an advisor dispatch, given the naming-collision risk with existing `-d` flags)
before any implementation ticket is written.

## 4. Verification

Not applicable — Draft, pending a decision on whether to pursue this at all.
