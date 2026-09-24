# 532 — quota-1 counts suspended/killed test runs; agent turns wait forever on stopped commands

**Status**: Open
**Priority**: P2
**Severity**: Medium
**Category**: Bug

---

## Observed (loom issue 113, 2026-09-24)

- An agy/flash37 session ran `make test-q1` → `harnez exec --quota-1 -- make test` → `go test ./...`.
- The process tree was left **suspended** (state `T`), not killed. Log ended at `go test ./...`.
- Quota-1 still counted the run, so the next run was blocked until a source change was made.
  Only the reserved bypass could have unblocked it.
- The agy turn waited about 30 minutes on the dead command. The stopped tree survived about 10 hours.
  `-test.timeout=10m` never went off because stopped processes don't run their timers.
  Later `go test` runs in the repo appeared to hang ("tests run forever") until the process group was killed manually.

## Expected

1. `harnez exec --quota-1` counts a run only when the child exits normally (exit status, not a signal or stop).
   If the run is killed or stopped, record it as "incomplete" and allow one retry.
2. `harnez exec` watches the child for a stopped state (`T`). When it sees one, it sends SIGCONT or kills the process group,
   then reports that clearly instead of blocking.
3. The agent runner (agy) sets a wall-clock timeout on commands and cleans up the child process group when a turn ends
   (AgenticLoop Invariant 3, Zero Zombie Guarantee).

## Repro sketch

`harnez exec --quota-1 -- sh -c 'kill -STOP $$'` should not consume the quota and should not hang.
