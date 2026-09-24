# 534 — Capture agent session tokens continuously, not only at session end

**Status**: Open
**Priority**: P2
**Severity**: Medium
**Category**: Telemetry
**Related**: [[034-hook-triggered-token-extraction]], [[082-agent-usage-collector-daemon]], [[535-review-agent-collector-lifecycle-automation-install-auto-start-keep-current]]

---

## Problem

While a session runs, `harnez agent list` shows TOKENS/CACHED as `0`. Only
completed sessions show real counts. Seen 2026-09-24 in a live
`harnez agent chat` Claude session (`lucky-fox`, `a46e2ba4-…`, status `active`,
tokens 0/0) while completed codex/agy sessions showed millions. The
agent-collector daemon was not running then (see 535), so it is unclear whether
the counts are only written at session end or the daemon is simply missing.

## /goal

Active sessions show token and cache counts that update during the session
(per turn or on a short cadence), for every supported provider. Completing a
session must not be the only point where counts are written.

## Notes

- First find out where session tokens are written today (at session end? by the collector?).
- 034 (hook-triggered transcript seek) may be the mechanism for Claude and AGY.
- Check the live code before starting; this ticket may be stale.
