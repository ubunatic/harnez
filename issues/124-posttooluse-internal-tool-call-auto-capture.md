# 124 — `PostToolUse` hook: auto-capture internal-tool call counts without a score

**Status**: Open
**Priority**: P3 (Low)
**Severity**: Minor
**Category**: Feature
**Related**: [[117-harnez-rate-command]], [[116-tool-telemetry-schema-and-storage-layer]], [[119-harnez-hook-agent-hook-management]], `docs/HookRewritePattern.md`, `docs/other/Canary.md`

## Problem

[[117]]'s `harnez rate` is an agent self-report: it only produces a
`tool_calls` row when the model actually chooses to call it after an
internal tool use (Read, Edit, Grep, semantic scan, …), per the
instruction snippet [[122]] injects. That's a deliberate design choice,
not an oversight — see the 2026-08-31 discussion this ticket comes
from: a quality *score* is inherently a judgment call only the agent (or
user) can make, so it can't be captured mechanically the way [[118]]'s
`harnez exec` captures objective shell-call metrics (`duration_ms`,
`raw_bytes`, `exit_code`).

But call *frequency* — which internal tool ran, how often, in which
session — is an objective, mechanical fact, same as `harnez exec`'s
metrics. Nothing currently captures that for internal tools; today
`tool_calls` only has rows for `call_type='shell'` (always, via the
`PreToolUse`/`Bash` hook) and `call_type='internal'` (only when the
model happens to rate). This ticket adds an automatic, code-enforced
floor under that — not a replacement for `harnez rate`'s scoring, a
complement to it.

**Three use cases motivating this** (from the 2026-08-31 discussion):

1. **Rating-coverage gap detection.** Since `harnez rate` depends on the
   model following an injected instruction (no code enforcement), there's
   currently no way to tell whether it's actually being followed
   consistently within a session, or silently drops off (e.g. after
   context compaction pushes the instruction out of the effective
   context). An objective internal-tool-call count gives a denominator:
   "40 internal tool calls this session, only 3 rated" is a visible,
   queryable coverage gap instead of an invisible one.
2. **Objective session-shape data for retros.** `docs/AgenticLoop.md`'s
   phase-5 retrospectives currently reconstruct "what did the agent
   actually do" from git log and memory. Per-tool call counts (Read ×40,
   Edit ×3, Grep ×12, …) captured automatically would feed `harnez stats`
   directly as real session-activity data, with no self-reporting
   dependency.
3. **Thrashing/inefficiency detection.** This repo's own CLAUDE.md
   already has a rule against exhaustively re-grepping/re-reading past
   1-2 tries when a root cause isn't apparent. An unusually high call
   count for one tool within a short window (e.g. 15 greps hunting the
   same symbol) is a mechanical thrashing signal, detectable without any
   voluntary score.

## Scope

1. **Canary first** (per `docs/other/Canary.md`) — this needs a probe
   before any implementation, because the two-stage rewrite pattern
   `harnez exec`/`harnez rate` rely on doesn't apply here in the same
   way:
   - Confirm exactly what fields Claude Code's `PostToolUse` hook payload
     actually contains (tool name, tool input, tool *response*/result —
     does it include anything error/success-shaped that's useful without
     a score?).
   - Confirm whether `duration_ms` is obtainable at all. `PreToolUse` and
     `PostToolUse` are separate hook invocations with no shared process
     state — if there's no correlating call ID in both payloads, wall-
     clock duration for internal tools may not be capturable this way at
     all (unlike `harnez exec`, which owns the whole child-process
     lifecycle itself). If duration isn't available, scope this ticket
     down to call-count/frequency only and say so explicitly rather than
     inventing an approximate timing.
   - Confirm `PostToolUse` supports a matcher covering "every tool"
     (not just `Bash`) — the spec's own hooks-guide should say whether an
     empty/omitted matcher or a wildcard is the right config shape; don't
     assume.
2. If the canary confirms a usable mechanism: add a `PostToolUse` hook
   (matcher: everything, or everything except `Bash` since that's
   already covered by [[118]]'s `PreToolUse` capture) writing one
   `tool_calls` row per call with `call_type='internal-auto'` (a new,
   distinct value from `harnez rate`'s `'internal'`, so coverage-gap
   queries in use case 1 can compare the two counts directly) and
   `score=NULL` (no judgment attached — this is the mechanical floor,
   not a replacement for rating).
3. Wire installation through `apply`'s existing managed-hooks merge
   (same mechanism [[119]] extended), not a new command — follow
   `docs/HookRewritePattern.md` and [[119]]'s 2026-08-31 decision on
   command shape.
4. `harnez stats` ([[120]]) should be able to report the coverage-gap
   number from use case 1 directly (e.g. internal-auto count vs. rated
   count per tool/session) — check whether this needs a new query method
   or is derivable from existing `Filter`/`GroupStats` by `call_type`.

## Acceptance Criteria

- [ ] Canary findings recorded here (payload fields available, whether
      duration is capturable, matcher behavior for "all tools") with an
      explicit go/no-go before implementation starts.
- [ ] If duration isn't obtainable, the shipped scope is call-count-only
      and that limitation is documented, not silently worked around with
      an approximation.
- [ ] `PostToolUse` hook writes exactly one row per internal tool call,
      `call_type='internal-auto'`, no double-counting against the
      existing `PreToolUse`/`Bash` shell capture.
- [ ] Idempotent install/removal via `apply`/`clean`, matching every
      other managed hook in this repo.
- [ ] `harnez stats` can show rated-vs-auto-captured coverage per
      tool/session (use case 1) without a raw SQL query.

## Notes

Explicitly a complement to [[117]]'s `harnez rate`, not a replacement —
score stays a self-report because it's a judgment call no hook can
compute. This ticket only closes the "did it even happen" gap under
that, the same way [[118]] closes it for shell calls' objective metrics.

## Implementation Plan

### Step 0 — Canary first (blocking gate, no code changes)

Follow `docs/other/Canary.md`; put it in a top-level `canary-posttooluse-payload/`
dir (same convention as `canary-watch-pty/`, `canary-remote-load-stream/`).

1. Install a throwaway `PostToolUse` hook in a scratch project's
   `.claude/settings.json` whose command is `cat >> /tmp/posttooluse.jsonl` (no
   harnez code involved), run a short `claude -p` session that exercises Read,
   Edit, Grep and Bash, then inspect the captured payloads for:
   - field names: `tool_name`, `tool_input`, `tool_response`, `session_id`,
     `cwd`, and whether any correlating call/invocation id appears in **both**
     `PreToolUse` and `PostToolUse` payloads (install both hooks in the same
     probe so the comparison is direct);
   - whether `tool_response` carries anything success/error-shaped worth storing
     in `exit_code`;
   - matcher behaviour: run one probe with `matcher` omitted and one with `"*"`
     to confirm which shape actually fires for all tools.
2. Record the findings verbatim in this ticket (a `## Canary Findings` section)
   with an explicit go/no-go before any Go code is written. If no correlating id
   exists in both payloads, **ship call-count-only** and state that here; do not
   approximate `duration_ms` from hook wall-clock.

### Step 1 — telemetry constant

`internal/telemetry/query.go` (where `rateCallType` / `HeartbeatCallType` /
`ExpectedFailureCallType` already live): add
`AutoInternalCallType = "internal-auto"` with a doc comment saying it is the
mechanical floor under `harnez rate` and carries `score = NULL`.

Audit every existing aggregate for the new call_type:
- `UnratedFailureCount` (query.go:437) — its predicate needs `exit_code != 0 OR
  score <= 2`, so score-NULL/exit-NULL auto rows can never match; leave the
  `NOT IN (...)` list alone unless the canary shows `tool_response` gives a
  usable non-zero `exit_code`, in which case auto rows **must** be added to the
  exclusion list or every failed Grep becomes an "unrated failure".
- `HeartbeatStats.CallsSince` (query.go:388, "any call_type") — decide
  explicitly whether auto rows inflate it. Recommended: exclude
  `AutoInternalCallType` there, since that counter is a proxy for *agent-driven*
  activity between heartbeats, and the auto floor would swamp it.
- `Aggregate`/`aggregateGroupedBy` avg-score math already ignores NULL scores.
- `internal/sessionstate` is unaffected: its gap tips count `harnez` subcommand
  invocations from the state file, not `tool_calls` rows.

### Step 2 — the hook endpoint

New `cmd/harnez/posthook.go` exposing a hidden `harnez exec posthook`
subcommand registered next to `newExecHookCmd()` (`cmd/harnez/exec.go:424`) —
keeping it under `exec` groups it with the existing hook endpoint and avoids a
new top-level noun.

- Decode the payload with the existing `hookInput` struct, extended with the
  fields the canary confirmed.
- Skip `tool_name == "Bash"` (already captured by the `PreToolUse` rewrite →
  `harnez exec`); this is the no-double-counting guarantee.
- Resolve `session_id`/`ticket_id`/`agent`/`project` exactly the way
  `cmd/harnez/rate.go`'s `insertRateRow` path does (`resolve.Session`,
  `resolve.Ticket`, `detectAgent`) — prefer the payload's own `session_id`/`cwd`
  when present, since the hook process has them for free.
- Insert one row via `telemetry.DB.Insert` (reuse
  `defaultInsertExecRow`-style plumbing at exec.go:411) with
  `CallType = AutoInternalCallType`, `Score = nil`, `ExitCode = nil`,
  `DurationMs = 0`.
- Best-effort semantics: never fail the tool call. Print nothing on stdout
  (a `PostToolUse` hook has no rewrite envelope to emit), log via `debugLog`,
  and always exit 0 — a telemetry write failure must not surface as a hook error
  in the user's session.

### Step 3 — install path

`config.yaml` `hooks:` — add one entry:

```yaml
  - event: PostToolUse
    command: "harnez exec posthook"
```

`Hook.Matcher` is `omitempty` and `buildSettingsDoc` (`internal/claude/apply.go:123`)
already omits the key when empty, so an all-tools matcher needs no code change
*if* the canary confirms an omitted matcher matches everything; if it requires
`"*"`, set `matcher: "*"` and nothing else changes either. Idempotency and
`clean` removal come free from the existing managed-hooks merge — add a
`PostToolUse` assertion to `internal/claude/telemetry_hook_test.go` mirroring
its existing `PreToolUse`/`Bash` idempotency test.

### Step 4 — coverage reporting in `harnez stats`

`Filter` already has `CallType`, and `AggregateByTool` groups by `tool_name`, so
coverage is derivable with two filtered calls (one `call_type='internal'`, one
`'internal-auto'`) — no new query method needed. Add a `--coverage` flag to
`cmd/harnez/stats.go` (`newStatsCmd`, `buildStatsReport`, `renderStatsTable`,
`renderStatsJSON`) printing per-tool `auto / rated / coverage%`. Keep it a
separate report block, like the existing rate-overhead block, rather than adding
columns to the main table.

### Design decisions / tradeoffs

- **New call_type value, not a new column.** Follows the precedent set by
  `ExpectedFailureCallType` (query.go:297) — `call_type` is already the
  established "how was this row meant to be read" axis, and it needs no schema
  bump (`CREATE TABLE IF NOT EXISTS` stays valid).
- **Skip Bash rather than dedupe after the fact.** Cheaper and verifiable in a
  single test; the `PreToolUse` rewrite already owns Bash end-to-end.
- **Fail silent.** Unlike `harnez exec`, this hook is on the critical path of
  every single tool call, so an error path that can break a session is not
  acceptable for a purely observational row.

### Risks / open questions

- **Row volume.** One row per tool call is a large multiple of today's write
  rate. Check `internal/telemetry/telemetry_bench_test.go` for the per-insert
  cost and confirm the synchronous `Insert` plus process spawn stays in the
  low-milliseconds range; if not, this ticket needs a batching/append design
  before shipping, not after.
- **DB growth / retention.** No pruning exists today. Worth a follow-up ticket
  rather than in-scope work, but measure and record the per-day row growth here.
- **`duration_ms` may simply be unavailable** — accept call-count-only scope and
  say so, per the ticket's own acceptance criterion.
- **Export/privacy**: `internal/telemetry/export.go` and the privacy levels
  (issue 204) now see a new call_type; confirm the export tests don't assume a
  closed set of values.

### Scope: **medium** (canary + ~2 small Go files + config entry + stats block),
gated behind a genuinely blocking canary step.
