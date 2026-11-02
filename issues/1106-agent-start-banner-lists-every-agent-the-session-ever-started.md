# 1106 — agent start banner lists every agent the session ever started

**Status**: Open
**Priority**: P2 (Medium)
**Severity**: Minor
**Category**: Bug
**Related**: taia (codeberg.org/harnez/taia), 1083 (grooming for taia focus)

---

/goal `harnez agent start` output stays short no matter how many agents the session started, with a test. Stop and report when blocked on a user decision or denied permission; on a loop or no progress, clear the goal and write a failure report here.

## Problem

Reported by the taia session (2026-11-03): every `harnez agent start` prints a banner listing all agents the session ever started (62 in taia's session), which floods tool output. Show only live agents, or a count plus the newest few, and keep the full list in `harnez agent list`.
