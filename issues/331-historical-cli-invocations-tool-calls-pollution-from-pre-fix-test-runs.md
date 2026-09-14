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

## 4. Resolution

Chose option 2 (best-effort heuristic quarantine), scoped narrowly: the user's guidance was
that keeping real usage stats at an estimated ~90%+ correct is sufficient for the "tool usage
story," so any deletion filter needed only to be safe, not exhaustive — ambiguous buckets
(e.g. `tool_name='echo'`, `'sh'`) were left alone rather than risk deleting genuine rows.

Identified two exact-signature matches unique to `TestGearMulticallExecution`'s three known
subprocess invocations (confirmed against the real DB before deleting anything):

- `tool_calls`: `project_name='harnez' AND tool_name='custom-tool' AND call_type='shell-expected'
  AND exit_code=3` — 49 rows (the `--tool custom-tool --expect-failure -- sh -c 'exit 3'` case).
- `tool_calls`: `project_name='harnez' AND tool_name='exit' AND call_type='shell' AND
  exit_code=42` — 44 rows (the `sh -c 'exit 42'` case).
- `cli_invocations`: `command='exec' AND args LIKE 'exec --tool custom-tool --expect-failure%sh
  -c%' AND exit_code=3` and `args LIKE 'exec sh -c%' AND exit_code=42` — 4 rows each.

The third case (`⚙ echo hello from gear`, tool_name `echo`, exit_code 0) was deliberately
**not** touched — indistinguishable from the large volume of genuine `echo` usage across
projects, so removing it would trade real-data loss for marginal cleanup.

Total removed: 101 rows out of 9,647 (7,838 `tool_calls` + 1,809 `cli_invocations`) — about
1.05% of all rows, i.e. the database was already >98.9% clean before this ticket; the fix
brings the known-pollution buckets to 0 rows. Well within the ~90%-correct bar.

Backed up `~/.harnez/tool_catalog.sqlite` to a timestamped `.bak-issue331-<timestamp>` copy
before deleting, then verified both signatures return 0 rows post-delete. No permanent
`harnez` subcommand was added — per this ticket's own framing this was a one-off maintenance
query, run directly via `sqlite3` against the exact confirmed signatures above, not a
heuristic applied blind.

## 5. Verification

- Backup taken: `~/.harnez/tool_catalog.sqlite.bak-issue331-20260914131025`.
- Row counts before: `tool_calls=7838`, `cli_invocations=1809`.
- Row counts after: `tool_calls=7748`, `cli_invocations` unaffected by the same session's
  concurrent writes settling higher (the delete itself removed exactly 8 matching rows).
- Post-delete query for both signatures in both tables returns `0` in each case.
