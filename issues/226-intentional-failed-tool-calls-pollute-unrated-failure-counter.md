# 226 — Intentionally-Failed Tool Calls Pollute the Unrated-Failure Counter

**Status**: Open
**Priority**: P3 (Low)
**Severity**: Minor
**Category**: Feedback / Tool Feedback Protocol
**Related**: `internal/sessionstate/sessionstate.go` (`GapTip`, `UnratedFailures`), `internal/telemetry`
(`GroupStats.FailureCount`, `UnratedFailureCount`), `cmd/harnez/rate.go`, issue 179, issue 188, issue 186

---

## 1. Problem & Motivation

`GapTip` (`internal/sessionstate/sessionstate.go:223`) surfaces `harnez tip: N tool call(s) failed
without a \`harnez rate\` report` whenever `UnratedFailureCount` is positive. A "genuinely-failed
tool call" is defined purely mechanically (`internal/telemetry`'s `GroupStats.FailureCount`):
`exit_code != 0 OR score <= 2`.

This definition can't distinguish an *accidental* failure (a command broke, a tool misbehaved,
something the agent should record via `harnez rate` so the friction is visible) from a command an
agent ran *expecting* a nonzero exit — e.g. probing whether a server is down (`curl ... ; echo
$?`), deliberately reproducing a bug to observe its exact failure mode, or a test asserting a
negative case. During a 2026-09-03/04 debugging session (issues 216/225), several such intentional
probes tripped the counter, and the resulting tip was correctly judged as not warranting a
`harnez rate` call — but there was no way to *tell the tracker that* short of doing nothing and
letting the tip re-fire, or firing a rating that would misrepresent the call as an unexpected
failure worth flagging in future tool-quality analytics.

This matters beyond the nag itself: `GroupStats.FailureCount`/`UnratedFailureCount` presumably also
feed whatever downstream analytics (session summaries, tool-quality trends) treat failure counts as
a signal of agent or tool trouble. An agent that runs many *expected*-to-fail diagnostic commands in
one session (normal during debugging) will look worse in that signal than one that avoided
diagnosis entirely — the opposite of what the metric should reward.

## 2. Proposed Directions (either or both)

1. **Agent-side convention**: document (Tool Feedback Protocol / AgenticLoop.md) that a command run
   expecting failure should make that legible to whatever inspects its exit code, e.g.
   `<command> || echo "expected failure: exit=$?"` or similar, so the wrapping shell call's own
   exit code is 0 and it never enters the failure counter to begin with. Cheapest to ship (docs
   only), but relies on agents remembering to do it consistently, and doesn't help for tools other
   than raw shell commands where the agent doesn't control the exit-code semantics as freely.
2. **Tracker-side escape hatch**: extend `harnez rate` (likely alongside `--ok`, see
   `cmd/harnez/rate.go`) to let an agent mark a specific already-logged failed call — or the most
   recent N — as an *expected/intentional* failure: excluded from `UnratedFailureCount` (so the
   `GapTip` nag doesn't fire for it) and tagged distinctly enough that downstream analytics can
   exclude it from "failed agent behavior" aggregates rather than silently counting it as a plain
   failure. Needs a schema/semantics decision: a new `rate` flag (e.g. `--expected`), a new
   `call_type`, or a dedicated marker column — see `cmd/harnez/rate.go` and whatever table
   `GroupStats.FailureCount` reads from before picking.

## 3. Acceptance Criteria

Not yet scoped — pick a direction (or both) in a follow-up pass. At minimum:
1. An agent that runs a command it expects to fail has some documented, low-friction way to keep
   that call out of `UnratedFailureCount` without either lying (rating it as if it were a genuine
   problem) or ignoring the tip.
2. Whatever mechanism is chosen doesn't let an agent quietly launder a *real* failure into "expected"
   after the fact — the intent should be declared at (or very near) the time of the call, not
   retroactively during cleanup, so it can't become an escape hatch from Tool Feedback Protocol
   accountability.
3. Downstream analytics that currently read failure counts as an agent/tool-quality signal should
   be able to exclude calls marked this way, if direction 2 is chosen.
