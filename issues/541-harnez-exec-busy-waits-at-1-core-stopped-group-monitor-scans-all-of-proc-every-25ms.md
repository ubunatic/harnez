# 541 — harnez exec busy-waits at ~1 core: stopped-group monitor scans all of /proc every 25ms

**Status**: Open
**Priority**: P0
**Severity**: High
**Category**: Bug / Performance (regression)
**Related**: [[533-harnez-clean-reap-stuck-processes-and-release-stuck-quota-1-state]]

---

## Observed (loom session report, top 3s sample, 2026-09-24 16:12)

- A `harnez agent start ... codex:terra:med` launch process (PID 1198408, shown as `⚙`, i.e. a
  `harnez exec` wrapper) sat at 94% CPU for its whole 6-minute turn (3:26 CPU time) while only waiting
  on the codex child.
- Two `harnez exec` wrappers inside the codex session (1205041, 1205673) were at ~94% each.
- Total load was ~3 cores during one agent turn.

## Cause (checked)

Regression from 533 M4 (`ef0408b`). `monitorStoppedGroup` (`cmd/harnez/stopped.go`) ticks every
`stoppedPollInterval = 25ms`, and each tick calls `processGroupStopped`, which runs
`procs.LinuxProcReader{}.List()`. That reads `/proc/<pid>/stat` **and** `status` for every process on
the machine. That is ~40 full /proc scans per second per wrapper, and nested wrappers multiply it.

## /goal

A waiting `harnez exec` uses about 0% CPU, and stopped-child detection still works (the 533
`sh -c 'kill -STOP $$'` repro returns exit 125 within a few seconds).

## Notes

- Options (pick by measurement): a much longer interval (e.g. 1s; detection within seconds is enough);
  read only the known group members (the child pid's `/proc/<pid>/stat`, or scan only the pids in the
  group) instead of the whole of /proc; or use `waitid`/`Wait4` with `WUNTRACED` on the direct child,
  which reports stops without polling (it covers the leader; grandchildren may still need a slow scan).
- Acceptance: `harnez exec -- sleep 30` shows <1% CPU in `top`/`ps -o %cpu`, the stop repro passes,
  and there is a test asserting that the monitor does not use the full-/proc lister on each tick
  (or a benchmark guard).
- Also check `harnez clean procs`: a one-shot full scan is fine there.
