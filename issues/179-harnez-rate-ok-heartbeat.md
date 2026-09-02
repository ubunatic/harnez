# 179 — `harnez rate --ok`: lean periodic heartbeat to confirm tool calls are still fine

**Status**: Open
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
