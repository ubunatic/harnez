# 912 — usage: show swap in the Load and Remote Load boxes

**Status**: Open
**Priority**: P2 (Medium)
**Severity**: Minor
**Category**: Feature
**Related**: 885 (disk swap backstop for zram-only systems), 875 (low memory/swap warnings), 090, 879

---

/goal `harnez usage` shows swap usage (used/total, zram and disk swap together) in the local "⁷ Load" box and
in the "[R] Remote Load" box, verified by a test and a live run of the installed binary on this host and the
remote host; or stop and report when blocked on a user decision. Circuit breaker: on a loop or no progress,
clear the goal and write a failure report here.

## Notes

- Owner request (2026-10-24). Swap matters here because the host runs zram-only swap (885) and tmpfs `/tmp`.
- Remote Load renders in `internal/usage/watch_load_box.go:50`; the remote snapshot must carry swap too,
  so check how remote metrics are collected (879) before adding the row.
- Show nothing extra, or "no swap", when a host has no swap; never a silent 0%.
- Check the row against its `.ansi` sketch with the tui-check skill.
