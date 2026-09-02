# 186 — Time-based reminder alongside the call-count gap-tip

**Status**: Closed
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

## Resolution Note

Implemented in `internal/sessionstate/sessionstate.go`:

- Added `rateGapIdle` (20 minutes) as the wall-clock counterpart to
  `rateGapThreshold` (20 calls), with `heartbeatGapIdle` derived as `2 *
  rateGapIdle` (40 minutes), mirroring `heartbeatGapThreshold`'s 2x
  relationship to `rateGapThreshold`. The idle default is overridable via a
  cheap config knob — the `HARNEZ_RATE_GAP_IDLE_MINUTES` env var
  (`rateGapIdleDuration()`), read at check time rather than baked into a
  compiled constant, so it can be tuned without a rebuild.
- Added a `FirstCallAt` field to `State`, set by `Record` on a session's
  very first invocation. This gives the time-based check a wall-clock
  anchor for a session that has never called `harnez rate`, mirroring the
  existing count-based fallback (`callsSinceRate = s.Total` when
  `LastRateAt` is zero) rather than leaving the never-rated case with no
  time signal at all.
- `GapTip` gained a `now time.Time` parameter. Each threshold check became
  an OR of the count and time conditions (`callsSinceRate >=
  heartbeatGapThreshold || elapsedSinceRate >= heartbeatIdle`, similarly for
  the plain rate-gap check) — whichever fires first wins, there is no
  separate/redundant time-tip. Both conditions sit inside the same
  `!feedbackDisabled` block and behind the same `tipCooldown` gate at the
  top of the function, so issue 142's opt-out and issue 181's cooldown
  cover the time-based path exactly like the count-based one, with no new
  suppression logic needed.
- Call sites (`cmd/harnez/main.go`'s `sessionTipHook`) now capture one
  `now := time.Now()` and pass it to both `Record` and `GapTip`, so the
  recorded call timestamp and the tip's elapsed-time calculation use the
  same instant.
- New tests in `sessionstate_test.go`: `TestGapTip_TimeOnlyTriggersRateGap`,
  `TestGapTip_TimeOnlyTriggersHeartbeatGap`,
  `TestGapTip_CountOnlyTriggerUnchangedByTimeCheck`,
  `TestGapTip_BothUnderThresholdNoTip`,
  `TestGapTip_TimeBasedTipRespectsOptOut`, and
  `TestGapTip_TimeGapFromFirstCallWhenNeverRated`. All existing tests were
  updated only for the new `now` parameter — their assertions and threshold
  values are unchanged, confirming no regression to the tuned issue
  179/183 frequencies.

Note: this working tree had a second, concurrent Claude session active
during implementation (touching unrelated `internal/usage`/`spec` files),
which repeatedly reset the uncommitted sessionstate.go/main.go edits back to
HEAD mid-task. The final implementation was re-applied and committed
(27e2d05) immediately after a clean `go build`/`go vet`/`go test` pass to
close that window; the diff was verified scoped to only the intended lines
before committing.
