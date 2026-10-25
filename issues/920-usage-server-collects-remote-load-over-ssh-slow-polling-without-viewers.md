# 920 — usage: server collects remote load over ssh, slow polling without viewers

**Status**: Open
**Priority**: P2 (Medium)
**Severity**: Minor
**Category**: Feature
**Related**: Epic 921, 919, 879, 912

---

/goal The local harnez server, not the UI, holds the ssh connection to each configured remote and collects its
load (CPU, RAM, swap, all GPUs); it polls slowly while no UI is attached and switches to the normal streaming
rate when a client connects. Verified with a test for both rates and a live run against x600; or stop and report
on an owner decision. Circuit breaker: on a loop or no progress, clear the goal and write a failure report here.

## Notes (owner, 2026-10-25)

- Transport stays ssh streaming; a server-to-server protocol is a later idea, out of scope.
- Check what the UI does today (watch-side stream, `remote_load_poll_interval`, `remote_load_staleness`) and
  move it behind the collector registry (docs/UsageCollection.md §registry).
- Slow and fast rates are configurable values: they go in `spec/` with schema (docs/Spec.md).
