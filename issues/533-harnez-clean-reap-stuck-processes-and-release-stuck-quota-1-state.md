# 533 — harnez clean: reap stuck processes and release stuck quota-1 state

**Status**: Open
**Priority**: P1
**Severity**: Medium
**Category**: Feature

---

## Problem

Agents leave zombie/stopped test runners and agent processes behind (Invariant 3 violations).
The quota-1 state then blocks the next run (see 532: a stopped `go test` tree held the quota for
about 10h, and only the forbidden bypass could release it). There is no sanctioned, verifiable way
for an agent to recover.

## Design decisions

- `clean` becomes the "remove stale leftovers" verb. The current `clean` (remove managed blocks
  written by `apply`) moves to `revert --managed`. There is a single user, so no deprecation alias.
- An agent's belief that it is stuck is **not** enough to release quota-1. `clean q1` only releases
  the state when harnez can verify it: the last run has no recorded result **and** its process
  group is gone (or was just killed by `clean procs`). Anything else refuses with the reason.
- Dry-run by default. Acting needs `--kill`. Only processes of the current uid are touched, and the
  start time (`/proc/<pid>/stat` field 22) is checked before signalling, to guard against PID reuse.
- `clean` targets are subcommand-style positional args (`harnez clean procs q1 --kill`), so later
  targets (`sessions`, `locks`, `sockets`, `cache`, `worktrees`, `tickets`, `telemetry`,
  `scratch`) fit without redesign. They are out of scope here.

## Milestones

### M1 — free the verb: `clean` → `revert --managed`
- Move the current `clean` behaviour to `harnez revert --managed` and remove `clean`'s old meaning.
- Update `docs/CLIDesign.md` (command table, scope notes, #009 note), the man pages (generated, plus
  SEE ALSO), `scripts/smoke-test.sh` and any other callers (`grep -rn "harnez clean\|clean\b"`).
- Tests: `revert --managed` removes managed blocks, as `clean` did.

### M2 — checkable quota-1 run records (fixes 532 Expected item 1)
- The quota-1 state holds `{started, pgid, pid_starttime, finished, exit}` instead of a bare
  timestamp. It still reads the old timestamp-only format.
- `exec --quota-1` records the start before running and the result after. Only a normal exit
  (an exit status, not a signal or stop) consumes the quota. If the run was killed or stopped,
  record it as incomplete and allow one retry without a source change.
- Repro test: `harnez exec --quota-1 -- sh -c 'kill -KILL $$'` must not consume the quota.

### M3 — process records + `harnez clean procs q1`
- `exec` writes `$XDG_RUNTIME_DIR/harnez/procs/<pgid>.json` (pgid, pid start time, argv, cwd,
  owner pid, quota-1 marker, started) and removes it when the command exits. Fall back to
  `~/.harnez/run/procs` if `XDG_RUNTIME_DIR` is unset.
- `harnez clean [procs] [q1] [-d dir] [--kill] [--json]`; with no target it covers all targets.
  - `procs`: recorded groups that are stopped (state `T`) or whose owner is dead. SIGTERM the
    group, wait a grace period, then SIGKILL. Remove stale records whose group is gone.
  - `q1`: release as described under Design decisions; otherwise explain why not
    (e.g. "run finished normally; modify source to rerun").
- The quota-1 "blocked" message tells agents about this: "if the previous run hung or was killed:
  `harnez clean procs q1 --kill`".
- Add the one-liner to `docs/practices/AgenticLoop.md` Invariant 3 and `/sprint` phase 4
  (edit the source, then sync the root copy per CLAUDE.md).
- Tests use fake `/proc` readers and injected signalling. No real kills in unit tests except one
  guarded integration test on a child the test itself spawns.

### M4 — `exec` detects a stopped child (532 Expected item 2)
- While waiting, `exec` polls the child group state. When it is stopped (`T`) for longer than a
  short bound, send SIGCONT once. If it stays stopped, kill the group and report it clearly on
  stderr with a distinct exit code. Record the quota-1 run as incomplete (M2).
- Repro test: `harnez exec --quota-1 -- sh -c 'kill -STOP $$'` returns promptly, does not hang,
  and does not consume the quota.

## Done when

- M1–M4 are committed with tests, and `make install` has been run.
- 532 is closed. Its item 3 (agy runner wall-clock timeout and teardown) is outside harnez: it is
  recorded in 532 as handed off to agy/loom.
