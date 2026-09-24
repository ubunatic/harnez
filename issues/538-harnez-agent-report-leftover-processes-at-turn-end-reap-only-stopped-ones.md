# 538 — harnez agent: report leftover processes at turn end, reap only stopped ones

**Status**: Open
**Priority**: P2
**Severity**: Medium
**Category**: Feature / Agents
**Related**: [[532-quota-1-counts-suspended-killed-test-runs-agent-turns-wait-forever-on-stopped-commands]], [[533-harnez-clean-reap-stuck-processes-and-release-stuck-quota-1-state]], [[537-agy-route-shell-commands-through-harnez-exec-via-hooks-json]]

---

## Problem

`internal/subagent` starts `claude`, `codex` and `agy` with `exec.CommandContext` and no process
group of their own. Processes a turn leaves behind are invisible to harnez once the turn ends. In
532 a stopped `go test` survived for 10h.

## Decisions (2026-09-24, with the user)

- **No turn time limit.** Turn length varies widely (5–17 min in the 533 sprint), and no turn
  durations are recorded to base a limit on. 537 already bounds single commands for all agents.
- **No blanket kill at turn end.** A turn may leave a server running on purpose (e.g. a bug-fix
  agent starts a dev server with `./server &` for the user to try). Leftovers are reported, not killed.
- Only processes that cannot be doing useful work are reaped: stopped (`T`) processes, and ones
  whose owner is dead. These are the existing `harnez clean procs` rules.

## /goal

1. `harnez agent` starts each provider in its own process group (Setpgid), recorded in the session.
2. At turn end, list the group members still alive (pid, command, age, state) in the turn output
   and in `agent status`, e.g. `leftover: 2 processes (./server :8080, 3m)`.
3. Reap stopped members with the `internal/procs` clean logic (uid + start-time checks). Leave
   running ones alone.
4. Docs (AgenticLoop Invariant 3): when an agent means to leave something running, it detaches it
   explicitly with `setsid` or `systemd-run --user`, so it is not counted as a leftover.

## Notes

- systemd units (`systemctl --user start`) and `setsid`/`systemd-run` processes never join the
  group, so they are unaffected by design.
- Check first how Setpgid interacts with interactive `agent chat` (terminal job control, Ctrl-C):
  the agent needs the foreground terminal group, so `chat` may need a different approach from `-p` runs.
