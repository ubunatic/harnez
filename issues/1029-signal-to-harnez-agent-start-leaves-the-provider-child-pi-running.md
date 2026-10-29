# 1029 — Signal to harnez agent start leaves the provider child (pi) running

**Status**: Open
**Priority**: P2 (Medium)
**Severity**: Major
**Category**: Bug
**Related**: 661, 592, 1024

---

## Symptom
Twice on 2026-10-29 (1024 reruns on `lmcoder:coder`), stopping a synchronous `harnez agent start` left its `pi`
child running:
- Session `r1024`: `kill <harnez pid>` (SIGTERM). The `pi` child was reparented (PPID 5008) and kept talking to the
  model until it was killed by hand. `harnez agent list` still showed the session as `running`, and
  `harnez agent delete` needed `--force`.
- Session `r1024b`: `timeout 600 harnez agent start ...` (SIGTERM at 10 min, exit 124). `pi` was still alive
  afterwards and exited on its own shortly after.

A leftover child keeps using the shared local model server and can still edit files, after the caller thinks
the run is over. 661 fixed this for `harnez agent stop` (kills the process tree); the signal path of the
synchronous start itself was not covered.

## Acceptance
- SIGTERM, SIGINT and SIGHUP to `harnez agent start` (and `resume`) stop the provider child and its process group,
  then record the session as stopped, not `running`.
- A test starts a fake provider that sleeps, signals harnez, and asserts that no child process is left.
- Check the other providers (claude, codex, agy) on the same path.

/goal Make a signal to a synchronous `harnez agent start`/`resume` end the provider child and mark the session
stopped, or stop and report when a provider cannot be stopped this way. Circuit breaker: on a loop or no progress,
clear the goal and write a failure report. Re-check live code first; this ticket was written on 2026-10-29.
