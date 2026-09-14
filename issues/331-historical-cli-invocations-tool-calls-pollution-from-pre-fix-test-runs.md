# 331 — Historical cli_invocations/tool_calls pollution from pre-fix test runs

**Status**: Closed — resolved
**Priority**: P3 (Low)
**Severity**: Minor
**Category**: Data Quality
**Related**: issues/326 (`cli_invocations`), commit `486211c` (the isolation fix), `docs/studies/2026-09-13-cli-invocation-log-exit-code-anti-pattern-and-self-pollution.md`

---

## 1. Problem & Motivation

`cmd/harnez/exec_test.go`'s `TestGearMulticallExecution` builds the real `harnez` binary
and runs it as a subprocess. Its `HARNEZ_DB_PATH`/`HARNEZ_STATE_DIR` env vars were never
read by any production code — `telemetry.DefaultDBPath()` and `resolve.DefaultStateDir()`
resolve purely from `os.UserHomeDir()` — so every `go test ./...` run on a developer
machine was writing real rows into that developer's actual `~/.harnez/tool_catalog.sqlite`,
attributed to project `"harnez"` (the subprocess's cwd is the `cmd/harnez` package
directory), indistinguishable from genuine usage.

This was fixed going forward in `486211c` (`t.Setenv("HOME", ...)` isolation). This ticket
is only about the **pre-existing pollution already sitting in real databases** as of that
fix, on any machine where this test suite has been run repeatedly (at minimum the primary
dev workstation).

## 2. Findings

- Before issue 326, this only affected `tool_calls` (three rows added per test run: one
  plain `exec`, one exit-42 `exec`, one `--tool custom-tool --expect-failure exec`).
- After issue 326 landed (this session), the same three subprocess calls also each wrote
  a `cli_invocations` row, so the pollution rate roughly quadrupled per test run once
  `sessionTipHook`'s `PersistentPreRunE`-driven session-state JSON writes are counted too.
- The polluted rows are **not distinguishable from real usage by any existing field**:
  `project_name` is `"harnez"` (genuine, since the subprocess's cwd really is this repo),
  `command` is `exec` (a real subcommand name), and `session_id` is a normal `ppid-`-derived
  id. The only tell is that these invocations happened during a `go test` run rather than
  interactive/agent use, and that information isn't captured anywhere.
- Unknown scope: how many historical rows this accounts for, and on how many machines
  (anyone who has run `go test ./...` or `make check` in this repo prior to `486211c`).

## 3. Proposed scope (not yet decided — this ticket is for triage, not a committed plan)

Options, roughly in order of invasiveness:

1. **Do nothing.** The absolute row count from this source is small relative to real usage
   (observed: single-digit rows per local `go test` run, not per CI run since this project
   has no CI pipeline that runs `go test` today) and `harnez log`/`harnez stats` are
   diagnostic tools, not billing-grade — minor noise may not be worth a special-case query.
2. **Best-effort heuristic quarantine**: a one-off `harnez` maintenance query (not a
   permanent feature) identifying rows plausibly from this bug (project `harnez`, command
   `exec`, tool_name `custom-tool` or matching the three known argv shapes, timestamps
   before `486211c`'s commit date) and reporting or deleting them. Risky: no reliable way to
   distinguish real `harnez exec --tool custom-tool` usage (if any exists) from test noise
   after the fact.
3. **Leave historical data alone; add a data-quality caveat** to `harnez log`'s/`harnez
   stats`'s `--help` or `docs/CLIDesign.md` noting that pre-`486211c` history may contain
   test-run noise for the `harnez` project specifically.

## 4. Verification

Not applicable until a scope is chosen — this ticket starts in Draft for the user to decide
whether any action is warranted at all.
