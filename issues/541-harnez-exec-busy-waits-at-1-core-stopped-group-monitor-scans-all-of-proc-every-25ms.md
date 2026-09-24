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

## Resolution & Verification (2026-09-24)

- `processGroupStopped` now reads only `/proc/<child-pid>/stat`; it no longer invokes the
  full `/proc` lister while an exec child is waiting. The monitor polls once per second.
- Guard test: `TestProcessGroupStoppedReadsOnlyLeaderStat` verifies the direct stat-file path
  and rejects reintroducing `LinuxProcReader{}.List` to the stopped-group monitor.
- `make test-q1` passed; its saved output contained no `--- FAIL` marker.
- Stop repro: `harnez exec -- sh -c 'kill -STOP $$'` returned exit `125` after the monitor
  continued the stopped child.
- CPU measurement, `ps -o %cpu` at two seconds into `harnez exec -- sleep 30`:
  before (reported observation) ~94%; after 0.4% (PID 1228331).

## Host review (2026-09-24): regression in descendant stops

- Confirmed: CPU 0.0% while waiting (`harnez exec -- sleep 6`), direct stop → exit 125 in 2s.
- ❌ `harnez exec -- sh -c 'sh -c "kill -STOP \$\$"; echo inner-done'` hangs: a stop that hits only a
  grandchild (e.g. `go test` under `make`, the 532 shape) is no longer detected, because only the
  direct child's stat is read.

### M2 — detect descendant stops cheaply
- Pre-Work / Required Refinements: walk the child's descendants via
  `/proc/<pid>/task/<tid>/children` (recursive, only the tree) each tick and read only those stats.
  Keep the 1s interval. No full /proc scan.
- Tests: grandchild-stop fixture → detected and recovered (exit 125). Keep the guard test.
- Measure again: CPU while waiting on `sh -c 'sleep 30'` (grandchild) stays <1%.

### M2 implementation & verification (2026-09-24)
- The monitor now walks `/proc/<pid>/task/<tid>/children` recursively and reads stat only for
  those descendants; it still polls every second and checks process-group membership.
- The fixture covers a stopped grandchild and an unrelated stopped process. The full `/proc`
  lister guard remains in place.
- `make test-q1` passed with no `--- FAIL` marker; `make install` passed.
- Live grandchild-stop repro returned exit `125` and printed `inner-done` after SIGCONT.
- At two seconds into `harnez exec -- sh -c 'sleep 30'`, `ps -o %cpu` reported `0.0%`
  (wrapper PID 1251710).
