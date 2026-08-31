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
