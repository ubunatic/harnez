# 139 - Codex Usage Window Rollover Keeps Stale Limit State

**Status**: Open  
**Priority**: P2  
**Severity**: Medium  
**Category**: Usage / Quota UI

## Problem

`harnez usage` can show stale Codex quota-window state when an existing
watch/UI session spans a limit-window reset. The observed UI started
before the window reset and later showed Codex with the short window at
100% even though the reset timer had reached the rollover point:

```text
OpenAI Codex  [▋   ] 16% 6d19h  [████] 100% 1m
```

The expected behavior is that once the limit window rolls over, usage
and reset-time state refresh coherently rather than leaving the old
window pinned at 100% until the user restarts or triggers some unrelated
collector refresh.

## Impact

- The user cannot trust the Codex quota panel around rollover time.
- A stale 100% bar can make Codex look unavailable when the new window
  should have started.
- Long-running `harnez usage --watch` sessions become less useful for
  planning agent work near quota resets.

## Expected Behavior

- When a Codex usage window reaches or passes its reset timestamp, the
  collector/UI should treat the quota snapshot as expired.
- The next display update should either fetch fresh Codex quota data or
  clearly mark the data stale/degraded.
- The UI should not keep showing a fully exhausted old window with a
  near-zero or expired countdown as if it were current.

## Investigation Notes

- Check whether Codex quota cache records include enough reset-time
  metadata to expire a single window independently.
- Check whether `usage --watch` recomputes rollover state from cached
  data every frame or only when a collector writes a new snapshot.
- Check whether stale fallback logic from previous quota-cache fixes is
  masking rollover expiry.
- Reproduce with a fixture/cache snapshot whose reset timestamp is in
  the past, then render the Codex panel without live network access.

## Acceptance Criteria

- Add a test fixture or unit test for a Codex quota snapshot whose reset
  time has passed while the UI/watch process remains alive.
- The UI no longer renders the expired window as current 100% usage.
- The behavior is deterministic without requiring a real Codex quota
  window reset.
- If fresh live fetch fails after rollover, the panel marks the data as
  stale/degraded instead of presenting the old exhausted window as fresh.
