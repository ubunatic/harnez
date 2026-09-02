# 187 — Periodic "Summary: X tool calls in `<dur>`" reminder (spec-first)

**Status**: Open
**Priority**: P2 (Medium)
**Severity**: Moderate
**Category**: Agentic Ergonomics
**Related**: [[179-harnez-rate-ok-heartbeat]] (the `harnez rate --ok` call this reminder
points agents at), [[186-time-based-gap-tip-alongside-call-count]] (the time-dimension
this reminder's trigger condition likely shares), [[183-session-state-assessment-and-reminders]]
(`internal/sessionstate` — existing gap-tip mechanism this composes with)

## Problem

The existing gap-tips (issues 183/179/186) are short single-line nudges
("no rate call in N calls/T time — consider `harnez rate --ok`"). The user
wants a distinct, occasional **summary-style** reminder instead of/alongside
that — something closer to:

> Summary: X tool calls in `<dur>`; call `harnez rate --ok` to confirm
> healthy tool usage.

i.e. giving the agent a concrete count + duration, not just "it's been a
while," to make the ask more concrete and the agent's compliance easier to
self-check ("have I actually confirmed the last X calls or not").

## Task — spec first, then implement

This ticket is deliberately spec-first: **before writing implementation
code, draft a first spec** for this reminder and get it into the `spec/`
directory following this repo's existing spec conventions (`docs/other/Spec.md`
— YAML authoring + a companion JSON Schema under `spec/schemas/`, embedded
into the binary, no hardcoded shadow copies in Go source). Look at the
existing `spec/actions.yaml` / `spec/colors.yaml` / `spec/indicators.yaml` +
their schemas for the house style before drafting a new one (e.g.
`spec/reminders.yaml` or fold into an existing file if genuinely a better
fit — your call, document the reasoning).

The spec should define, at minimum:
- The exact condition(s) under which this summary reminder fires — "X
  unconfirmed calls" needs a concrete definition: is X the same threshold
  as issue 179/186's gap-tips, a distinct (larger?) one, or does this
  reminder replace the plain gap-tip's wording once a bigger threshold is
  hit? Decide and document the reasoning in the spec and the ticket's
  Resolution Note.
- The message template itself (with placeholders for call count and
  duration), so wording changes don't require a Go recompile-and-redeploy
  cycle for pure copy edits.
- How it composes with the opt-out (issue 142's `RateFeedbackDisabled`) and
  the existing gap-tip cooldown (`tipCooldown`) so it doesn't create a third
  independent nag channel that stacks with the other two.

Then wire `sessionTipHook`/`sessionstate.GapTip` (or a clearly-scoped new
function, your call) to read the spec and render this summary reminder per
its defined condition, computing X (call count since last
rate/heartbeat) and `<dur>` (wall-clock elapsed since last
rate/heartbeat — reuse issue 186's timestamp if that ticket has already
landed when you pick this up; otherwise compute your own) from
`internal/sessionstate.State`.

## Acceptance Criteria

1. `spec/` gains a validated (schema-backed) definition of this reminder's
   condition and message template — not hardcoded Go string literals for
   the wording/threshold.
2. The reminder fires per the spec's defined condition, with the actual
   call count and duration correctly substituted.
3. It does not stack with/duplicate the existing plain and heartbeat-specific
   gap-tips in the same nudge slot — composes as one coherent nudge system.
4. Opt-out (issue 142) suppresses this reminder too.
5. Unit tests cover: spec loading/validation, condition-trigger logic,
   message rendering with real numbers, and opt-out suppression.
6. `go test ./...` passes.

## Resolution Note

(To be filled in by the implementing agent — document the spec's final
shape, the threshold/condition decision, and why.)
