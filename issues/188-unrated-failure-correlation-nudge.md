# 188 — Nudge specifically when real tool-call failures went unrated

**Status**: Closed
**Priority**: P2 (Medium)
**Severity**: Moderate
**Category**: Agentic Ergonomics
**Related**: [[179-harnez-rate-ok-heartbeat]], [[183-session-state-assessment-and-reminders]]
(`internal/sessionstate` — gap-tip mechanism this adds a sharper signal alongside),
[[122-agent-instruction-tool-feedback-protocol]] (the failure-rating rule this
directly enforces), `internal/telemetry` (`tool_calls` schema — `exit_code`,
`call_type`)

## Problem

Verified gap: `sessionTipHook`/`sessionstate.GapTip` (issues 183, 179, and
186 if landed) are purely **call-count/time based** — they never look at
`internal/telemetry`'s actual `tool_calls` rows. So if 3 tool calls
genuinely failed (non-zero `exit_code`, or another already-recorded failure
signal) in this session and the agent never called `harnez rate` about any
of them, nothing distinguishes that from a session where nothing has failed
at all — both currently produce, at most, the same generic "no rate call in
N calls" tip. This is a real correlation the harness already has the data
for (`internal/telemetry`) but doesn't use.

## Technical Specification

- Query `internal/telemetry` for tool_calls in the current session (or
  since the last rate/heartbeat marker) with a failure signal — non-zero
  `exit_code`, or whatever the existing `GroupStats.FailureCount` definition
  uses (`exit_code != 0 OR score <= 2`) — that have no corresponding
  `call_type="internal"` rating row referencing them.
- This requires linking a rating row back to the tool call(s) it covers —
  check whether the current schema already supports this (a ticket/tool
  reference column) or whether "no rating logged in the window following
  the failure" is an acceptable looser proxy. Document whichever approach
  you take and why; a perfect per-call linkage is not required if a
  reasonable session-window heuristic is far simpler and still correct in
  practice.
- Wire this as a new, distinct check in `sessionTipHook`'s tip selection —
  it should take priority over (or at minimum coexist clearly with) the
  plain count/time-based gap-tips, since "N real failures went unrated" is
  a strictly sharper, higher-value signal than generic silence.
- Suggested wording direction (not final):
  "harnez tip: N tool call(s) failed without a `harnez rate` report — use
  `harnez rate <tool> <score> \"<summary>\"` to record what went wrong (see
  Tool Feedback Protocol)."
- Must respect issue 142's opt-out.

## Acceptance Criteria

1. A session with genuine tool-call failures and zero corresponding rate
   calls surfaces a distinct nudge naming the failure count — not the
   generic silence tip.
2. A session with failures that WERE rated does not surface this nudge for
   those failures.
3. A session with zero failures never surfaces this nudge regardless of
   rate-call silence (that's the existing gap-tips' job).
4. Opt-out (issue 142) suppresses this nudge too.
5. Unit tests cover: unrated-failure trigger, rated-failure non-trigger,
   zero-failure non-trigger, and opt-out suppression.
6. `go test ./...` passes.

## Resolution Note

**Failure-linkage heuristic**: the `tool_calls` schema has no column
linking a `harnez rate` row back to the specific call(s) it covers, and
adding one (plus updating every rate-call site to populate it) was judged
out of scope for a v1 nudge. Instead `internal/telemetry.UnratedFailureCount`
uses the session-window proxy the ticket explicitly sanctioned: the most
recent `call_type="internal"` rate call in the session marks the point up
to which prior failures are presumed addressed, so only failures (per
`GroupStats.FailureCount`'s exact definition — `exit_code != 0 OR
score <= 2`) recorded *after* that point (or all of them, if no rate call
has fired yet this session) count as unrated. This isn't perfect per-call
linkage — an agent could rate one failure while leaving an earlier,
concurrent one unaddressed — but it's far simpler than adding a linkage
column and correct in the common fix-or-rate-then-move-on case this ticket
targets. Rate/heartbeat rows themselves are excluded from the failure scan
so a rating's own row never counts as the failure being reported on.

**Prioritization**: `sessionstate.GapTip` gained a new `unratedFailures int`
parameter (`cmd/harnez/main.go`'s `sessionTipHook` queries
`UnratedFailureCount` scoped to the current session and passes the count
in, best-effort — a missing/unopenable telemetry DB just leaves it at 0).
When `unratedFailures > 0` and feedback isn't disabled, this check is the
first thing `GapTip` evaluates, preempting the spec-driven summary
reminder, the heartbeat nudge, and the plain rate-gap tip in the same
call — "N real failures went unrated" is a strictly sharper signal than
any of those, so it replaces rather than stacks alongside them. It still
shares the same `tipCooldown` gate and issue 142 opt-out as every other tip
in `GapTip`, so a persistent unrated-failure streak doesn't become a new,
more chatty nag channel on top of the existing ones.

**Tests**: `internal/telemetry/unratedfailures_test.go` covers the DB-level
heuristic (trigger, rated-failure exclusion, zero-failure, low-score
failures, post-rate-only counting, session scoping).
`internal/sessionstate/sessionstate_test.go` covers the pure `GapTip`
priority/opt-out/cooldown behavior for the new parameter.
