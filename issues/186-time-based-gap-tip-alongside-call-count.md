# 186 — Time-based reminder alongside the call-count gap-tip

**Status**: Open
**Priority**: P2 (Medium)
**Severity**: Moderate
**Category**: Agentic Ergonomics
**Related**: [[179-harnez-rate-ok-heartbeat]] (heartbeat + call-count gap-tip thresholds
this ticket adds a time dimension to), [[183-session-state-assessment-and-reminders]]
(`internal/sessionstate` — `State`, `GapTip`)

## Problem

`sessionstate.GapTip` (issue 183, extended by issue 179's
`heartbeatGapThreshold`) triggers purely on call counts: `Total -
TotalAtLastRate`/`TotalAtLastTip` crossing a fixed number of harnez
subcommand invocations. In a slow-paced or idle session — long gaps between
tool calls, a human pausing mid-conversation, a session with genuinely few
Bash/tool calls per unit time — the count-based threshold may rarely or
never trigger even though a long *wall-clock* period has passed with no
rate/heartbeat confirmation. Silence over time is itself worth surfacing,
independent of call volume.

## Technical Specification

- `internal/sessionstate.State` already has per-call timestamps or can
  derive one (`LastRateAt` / equivalent — check current field set,
  `sessionstate.go`, before adding new ones; reuse rather than duplicate).
- Add a wall-clock check to `GapTip` alongside the existing call-count
  checks: if more than some tuned duration (e.g. 15-30 minutes — pick a
  sensible default, make it not-hardcoded-forever if a config knob is
  cheap) has passed since `LastRateAt`/`LastHeartbeatAt` with no
  intervening rate/heartbeat call, surface a tip — even if the call-count
  threshold hasn't been crossed.
- Must compose cleanly with the existing count-based thresholds (issue 179's
  20-call plain reminder / 40-call heartbeat-specific reminder) rather than
  firing redundantly — e.g. whichever condition (count or time) fires first
  wins, and `tipCooldown` still gates repeat firing.
- Must respect issue 142's opt-out (`RateFeedbackDisabled`) exactly like the
  existing gap-tips do.

## Acceptance Criteria

1. A session with very few but well-spaced-out tool calls (e.g. 5 calls
   over 30+ minutes) still receives a gap-tip, where today it would not
   (count threshold never crossed).
2. A fast, bursty session's existing count-based tip timing is unchanged —
   no regression to issue 179/183's tuned frequencies.
3. Opt-out (issue 142) suppresses the time-based tip exactly like the
   count-based ones.
4. Unit tests cover: time-only trigger (count under threshold, elapsed time
   over), count-only trigger (existing behavior unchanged), both-under
   (no tip), and opt-out suppression.
5. `go test ./...` passes.
