# 084 — Aggregate Quota-Window Box: UX & Feasibility Assessment

**Status**: Open — deferred, needs assessment before implementation
**Priority**: P3 (Low)
**Severity**: Minor
**Category**: Feature
**Related**: [[082-agent-usage-collector-daemon]], [[023-usage-command-token-quota-tracking]]

## Problem

Once [[082-agent-usage-collector-daemon]] gives us all agents' usage data collected in one place
on a regular cadence, we're positioned to compute *aggregate* remaining quota across agents/models,
not just per-agent numbers. Today `harnez usage` shows each agent's window (5-hour, weekly, etc.)
separately; there's no combined view of "how much room is left overall."

Keep the existing per-agent/per-model boxes as-is — the user runs dedicated agents per task and
wants to see dedicated numbers per model. An aggregate view would be an *additional* box, not a
replacement.

## Desired End State (rough shape, not committed)

- A separate box in the terminal UI showing aggregated quota headroom across agents, at minimum:
  - Remaining weekly limit (aggregated).
  - Remaining 5-hour window (aggregated).
- Later iterations might make the per-agent boxes collapse to on-demand/expandable detail once the
  aggregate box is trustworthy — explicitly out of scope for the first pass.

## Why This Needs an Assessment First (not a straight implementation ticket)

Aggregating quota windows across agents is not just "sum the percentages" — each agent's
5-hour/weekly windows have independent, non-aligned start times, so windows overlap in a way that
resists naive combination. Before writing code, we need to answer:

- **What does "aggregate" even mean here?** A straight sum of remaining tokens/percent across
  agents' windows can be misleading when windows don't align — is a rolling combined-headroom
  number (e.g. "tokens available in the next 2 hours" / "tokens available in the next 3 days")
  a better mental model than restating each agent's fixed 5h/weekly window?
- **What's the right unit** — tokens, dollars, percent-of-plan, or a normalized "capacity" score
  that's comparable across differently-priced/rate-limited agents?
- **What's actually computable** from the data we have — Codex and AGY currently have no reliable
  local token counts (see [[030-agy-codex-missing-local-token-counts]]), only quota percentages,
  which constrains what an honest aggregate can claim.
- **How do we present uncertainty** — e.g. partial data for one agent shouldn't silently distort
  a combined number; the box needs to be honest about what it can and can't account for.

## Next Steps

1. UX assessment: sketch 2-3 candidate presentations (e.g. fixed-window restatement vs. a rolling
   "available in next N hours/days" framing) and pick a direction, informed by what's actually
   computable (see below).
2. Feasibility assessment: given non-aligned reset windows across agents, determine a sound
   aggregation method (or conclude a rolling/normalized framing is required instead of a naive
   sum) before any implementation ticket is written.
3. Only after both assessments land, split out a concrete implementation ticket.

**Explicitly deferred** — do not implement until the assessment above is done and a follow-up
ticket is filed.
