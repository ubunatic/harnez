# 226 — Intentionally-Failed Tool Calls Pollute the Unrated-Failure Counter

**Status**: Closed — resolved: implemented both proposed directions. Direction 1
(docs-only `|| true`/`|| echo` convention) documented in the `tool-feedback-protocol`
skill content (`config.yaml`). Direction 2 added a `HARNEZ_EXPECT_FAILURE=1` command
prefix that `harnez exec` detects and records as `call_type=shell-expected`
(`telemetry.ExpectedFailureCallType`) — true exit code always preserved/returned,
excluded from `GroupStats.FailureCount` and `UnratedFailureCount`. See section 4 below
for the concrete design decisions.
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

## 4. Resolution — Both Directions Implemented

### Direction 1 (docs-only)

Added a paragraph + two examples to the `tool-feedback-protocol` skill's `content:` block in
`config.yaml` (the single source of truth embedded into the `harnez apply`-installed
`~/.claude/skills/tool-feedback-protocol/SKILL.md`), right after the `--ok` heartbeat
explanation: wrap a command expected to fail so the *wrapping* call's own exit code is 0
(`cmd || echo "expected failure: exit=$?"`), which `harnez exec` records as `exit_code=0`,
keeping it out of `FailureCount`/`UnratedFailureCount` with zero code changes. Cross-referenced
against direction 2 for when the real exit code must still reach the caller.

### Direction 2 (tracker-side escape hatch)

**Storage mechanism — reused `call_type`, not a new column.** `internal/telemetry/schema.go`
has no migration framework by design (`schemaVersion` bump forces a "delete the file" reset, no
in-place `ALTER TABLE` — see its doc comment); a new boolean column would force every existing
user's local telemetry DB to be discarded on upgrade for what is fundamentally a classification
value. `call_type` (`internal`/`heartbeat`/`shell`) is already the established "how should this
row be read" discriminator with an existing exclusion pattern in both
`GroupStats.FailureCount`'s SQL and `UnratedFailureCount`'s `call_type NOT IN (...)` clause, so a
fourth value — `telemetry.ExpectedFailureCallType` = `"shell-expected"` — composes directly into
both without new columns or a schema version bump. `exit_code` is never faked: the row still
stores the command's true exit code, only `call_type` differs from the normal `"shell"` rows.

**Detection mechanism — env var, but detected from command *text*, not process env, for the
hook-rewritten path.** The issue's framing (`cmd/harnez/exec.go`'s execution stage "already has
access to the full environment the child inherits") undersells one real wrinkle discovered while
implementing: because the PreToolUse hook wraps every Bash call as
`harnez exec --tool Bash -- bash -c '<original command>'`, an agent-typed
`HARNEZ_EXPECT_FAILURE=1 <command>` prefix lives *inside* that quoted script string — it is a
shell-level assignment scoped to the inner `bash -c`'s own execution, and never reaches `harnez
exec`'s own `os.Environ()`. `detectExpectFailure` (`cmd/harnez/exec.go`) therefore checks two
independent signals: `opts.Getenv(HARNEZ_EXPECT_FAILURE)` for direct/scripted `harnez exec`
invocations where the var genuinely is in the process's own env, and a regex
(`expectFailureCmdRE`) matching a leading `HARNEZ_EXPECT_FAILURE=1`/`=true` assignment (allowing
other leading `VAR=value` assignments first) against each spawned arg, which is what actually
catches the normal agent-typed-command path. Both are documented inline so a future reader isn't
surprised the "environment" half of the mechanism is the minority case.

**No parallel `--expected` flag added to `harnez rate`.** `harnez rate`'s score is already a
first-person, at-call-time judgment the agent enters directly — there is no mechanical
after-the-fact reclassification step to guard against the way there is for a shell exit code
(the agent choosing to score something a 4 instead of a 1 *is* the "declare intent at call time"
mechanism already). AC2's retroactive-laundering concern is specific to direction 2's exec path;
extending it to `rate` would add surface area without closing a real gap.

**Files changed**: `internal/telemetry/query.go` (`ExpectedFailureCallType` const,
`aggregateGroupedBy`/`UnratedFailureCount` exclusion), `cmd/harnez/exec.go`
(`expectFailureEnv`/`expectFailureCmdRE`/`detectExpectFailure`, `execCall.ExpectFailure`,
`recordExecTelemetry` call_type selection), `internal/sessionstate/sessionstate.go` (doc comment
sync), `config.yaml` (direction 1 doc + direction 2 mention). Tests added in
`internal/telemetry/telemetry_test.go`, `internal/telemetry/unratedfailures_test.go`,
`cmd/harnez/exec_test.go`.
