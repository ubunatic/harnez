# 179 — `harnez rate --ok`: lean periodic heartbeat to confirm tool calls are still fine

**Status**: Closed
**Priority**: P2 (Medium)
**Severity**: Moderate
**Category**: Agentic Ergonomics
**Related**: [[117-harnez-rate-command]], [[122-agent-instruction-tool-feedback-protocol]],
[[181-narrow-tool-feedback-protocol-to-failures]] (docs history for the narrowing this
ticket partially relaxes), [[142-disable-rate-feedback-and-measure-overhead]],
[[183-session-state-assessment-and-reminders]] (`internal/sessionstate` — gap-tip
nudge this ticket's heartbeat would hook into)

## Problem

Issue 181 narrowed `harnez rate` to fire only on tool-call failure or an
unexpected outcome, which fixed the original chattiness/ignored-instruction
problem. But it left a real observability gap: **silence is now ambiguous**.
Zero `harnez rate` calls in a session could mean "everything has been fine"
or "the agent forgot/ignored the instruction entirely" — `harnez stats` has
no way to distinguish the two from current data. The session-state gap-tip
(issue 183) already nudges the agent when there's been no rate call in a
while, but the agent has nothing lean to respond with beyond either silence
or a full per-tool rating that doesn't fit a "nothing went wrong" report.

## Proposal

Add a distinct, minimal call — `harnez rate --ok` (name TBD during
implementation; `harnez heartbeat` is an alternative worth considering) —
that logs a single lightweight confirmation: "N tool calls since the last
rate/heartbeat were fine," not a per-tool-call rating. No target tool name,
no outcome text required for the default case.

Optional timing/scope params to consider during design (lean — don't
overbuild):
- A count or time-window marker (e.g. "covers the last N calls" or "since
  timestamp T") so `harnez stats` can reconstruct a clean-streak timeline
  without the agent needing to enumerate every call.
- Reuse whatever id/sequence `internal/sessionstate` already tracks
  (`Total`/`TotalAtLastTip`) rather than inventing a second counter.

## Constraints

- Must stay lean: this should not reintroduce the chattiness issue 181 was
  filed to fix. The gap-tip that prompts for this heartbeat should fire
  infrequently (e.g. only after a meaningfully large gap), not on every
  handful of calls.
- Must be clearly distinguishable from a failure rating in whatever storage
  backs `harnez stats`/`harnez rate`, so failure-rate metrics aren't diluted
  by heartbeat "ok" entries.
- Update the Tool Feedback Protocol instruction text (`config.yaml`'s
  `agents_md.global.sections`, the `tool-feedback-protocol` skill) to
  document the new heartbeat call alongside the existing failure-only rule
  — two short rules, not a return to the old chattiness.
- Coordinate with issue 142 (opt-out + overhead measurement) so a disabled
  rate-feedback session also suppresses the heartbeat nudge, not just
  failure ratings.

## Acceptance Criteria

1. `harnez rate --ok [params]` (or equivalent chosen name) records a
   heartbeat distinguishable from a failure rating.
2. `harnez stats` can report a session's confirmed-clean-streak / last
   heartbeat time using the new data.
3. The session-state gap-tip (issue 183) can prompt for a heartbeat at a
   low, tuned frequency without regressing to pre-181 chattiness.
4. Tool Feedback Protocol instruction text updated to document both rules
   (failure-only rating + periodic ok-heartbeat) concisely.
5. Unit tests cover: heartbeat recording, its distinctness from failure
   ratings in storage/reporting, and gap-tip frequency behavior.
6. `go test ./...` passes.

## Resolution Note

Implemented as `harnez rate --ok ["<note>"] [<ticket_id>] [--since <n>]`
(flag on the existing command, not a parallel subcommand) — reuses all of
`harnez rate`'s session/ticket/agent resolution instead of duplicating it,
and keeps one CLI surface for both call shapes.

**Storage distinctness**: heartbeat rows write `call_type="heartbeat"`
(`telemetry.HeartbeatCallType`), `tool_name="heartbeat"`, `score=NULL`,
`exit_code=NULL` — a value distinct from the existing `call_type="internal"`
failure/rating rows. Because `score`/`exit_code` are NULL, SQL's own
`AVG`/`COUNT` semantics already exclude heartbeat rows from
`AvgScore`/`ScoredCount`, and `GroupStats.FailureCount`'s definition
(`exit_code != 0 OR score <= 2`) never matches a NULL/NULL row — so
heartbeats can't dilute failure-rate metrics even before considering
`call_type`. `telemetry.RateCallOverhead` (issue 142) now sums both
`call_type`s, since both are the same CLI command's overhead; a new
`telemetry.HeartbeatStats(Filter)` returns `{Count, LastAt, CallsSince}` —
last-heartbeat time plus how many tool_calls rows of any type landed after
it — which `harnez stats` renders unconditionally (not gated behind
`--overhead`) as a "heartbeat (harnez rate --ok, issue 179): N recorded,
last at T, M call(s) since" line, in both table and JSON output.

**Gap-tip frequency (issue 183 integration)**: no new counters — reused
`State.TotalAtLastRate`/`LastRateAt`, since `sessionstate.Record` already
treats any `rate` subcommand invocation (heartbeat or failure rating) as
clearing the gap. Added `heartbeatGapThreshold = 2 * rateGapThreshold`
(40 calls, vs. the existing 20-call plain-reminder threshold) as a second,
more specific check in `GapTip`: once 40+ calls have passed with no rate
call of any kind, the tip switches from the generic "you haven't rated
anything" wording to explicitly suggesting `harnez rate --ok`. Doubling
the existing threshold keeps this rarer than the pre-existing tip rather
than adding a second frequent nag — `tipCooldown` (10 calls) still gates
repeat firing either way, so this doesn't reintroduce issue 181's
chattiness.

**Opt-out (issue 142 integration)**: `GapTip` gained a `feedbackDisabled
bool` parameter; `cmd/harnez/main.go`'s `sessionTipHook` now loads the
embedded config and passes `claude.RateFeedbackDisabled(cfg, nil)` (config
flag OR `$HARNEZ_DISABLE_RATE_FEEDBACK`) through. When true, both the
plain rate-gap tip and the new heartbeat tip are suppressed (the unrelated
`harnez find` underuse tip still fires) — a session that opted out of the
Tool Feedback Protocol doesn't get nagged about either half of it.

**Instruction text**: both the `agents_md.global.sections` "Tool Feedback
Protocol" entry and the fuller `tool-feedback-protocol` skill content now
document the `--ok` rule alongside the existing failure-only rule. The
lean config.yaml section grew from ~55 to ~82 words to fit one added
sentence; `internal/claude/toolfeedback_test.go`'s word-budget check was
extended (not replaced) from a 15-60 to a 15-90 word band to accommodate
the genuinely added second rule, plus a new assertion that the section
mentions `harnez rate --ok`.

**Tests added**: `internal/sessionstate/sessionstate_test.go` (heartbeat
threshold supersedes the plain reminder, below-threshold still uses the
plain wording, a heartbeat call clears the gap, opt-out suppresses both
tips); `internal/telemetry/heartbeat_test.go` (heartbeat rows don't dilute
`AggregateByTool`'s score/failure aggregates, `RateCallOverhead` sums both
call types, `HeartbeatStats` reports last-heartbeat/calls-since correctly);
`cmd/harnez/rate_test.go` (`runRateOk` writes a distinct row, defaults the
note to "ok", folds `--since` into the note, doesn't dilute a sibling
failure rating's aggregate); `cmd/harnez/stats_test.go` (heartbeat info
surfaces in both JSON and table output). `go test ./...` and `go vet ./...`
pass.
