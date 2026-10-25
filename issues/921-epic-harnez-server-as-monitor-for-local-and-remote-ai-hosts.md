# 921 — Epic: harnez server as monitor for local and remote AI hosts

**Status**: Open
**Priority**: P2 (Medium)
**Severity**: Normal
**Category**: Epic
**Related**: 912, 917, 919, 920, 922, 879

---

/goal The local harnez server is the monitoring system for this host and its configured remotes (CPU, RAM, swap,
all GPUs): it keeps remote binaries current, collects remote load itself, and the UI only renders. Stop and report
when a ticket needs an owner decision. Circuit breaker: on a loop or no progress, clear the goal and write a
failure report here.

## Tasks

| # | Ticket | Pri | What | Status |
|---|---|---|---|---|
| 1 | 919 | P2 | remote host config, version check, upgrade older remotes on server start | Open |
| 2 | 920 | P2 | server collects remote load over ssh; slow polling without viewers | Open |
| 3 | 922 | P3 | Load box hotkey `L` instead of `7` | Open |
| 4 | 917 | P3 | server-driven boxes and items from spec | Open, after 920 |
| 5 | — | — | live check of 912's swap row on x600 (after 919) | Pending |

## Out of scope

- Server-to-server protocol instead of ssh (later idea).
- Downgrading a remote that runs a newer harnez.
