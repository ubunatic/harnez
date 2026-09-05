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

---

## Implementation Plan

This is an assessment ticket, so the plan below is a plan for *running the
assessment*, not for building the box. It ends with a filed follow-up ticket,
not with code.

### Grounding: what the data model actually supports (verified 2026-09-04)

- `QuotaWindow` (`internal/usage/types.go:9`) carries `Name`, `UsedPercent`,
  `RemainingPercent`, `ResetAt *time.Time`, `DurationLeft`, `IsActive`,
  `Severity`. **There is no capacity field** — no "tokens allowed in this
  window", no dollar limit. So percentages are the only quantity every agent
  reports for a window, and percentages of *different, unknown, unequal*
  denominators cannot be summed or averaged into anything honest.
- `AgentUsage` (`types.go:36`) has `Session`, `Weekly`, `ModelGroups[].Windows`,
  and `ExtraWindows` — AGY's per-model-group windows mean "an agent" is not
  even a single window pair.
- `TokenBreakdown` (`types.go:20`) has real token counts and `CostUSD`, but per
  [[030-agy-codex-missing-local-token-counts]] (**still Open**) Codex and AGY
  do not populate it reliably. So the one comparable unit is missing for 2 of 3
  agents.
- [[082-agent-usage-collector-daemon]] is **Closed** — `RunCollector`
  (`internal/usage/collector.go:26`) already writes one JSON snapshot per agent
  on a cadence, and `AgentUsage.LastRefreshed` records per-agent freshness. The
  input side this ticket was waiting on exists.
- `internal/usage/history.go` already persists a time series
  (`~/.claude/harnez/usage-history/`), which is what any *rolling* framing
  would have to be computed from.

**Preliminary conclusion to test, not assume**: a fixed-window aggregate
("combined weekly remaining: 62%") is not computable honestly from today's
data. A rolling framing ("no window resets for the next 2h10m; the binding
constraint is AGY session at 8%") is computable, because it needs only
`ResetAt`/`DurationLeft`/`RemainingPercent` per window — all of which exist.

### Steps

1. **Data audit (half a session, read-only).** Dump `harnez usage --json` on
   the real machine and, for each agent and each window, record: is
   `ResetAt` populated? is it a real wall-clock reset or a rolling estimate?
   are `UsedPercent`/`RemainingPercent` both present and consistent? Record
   the table in this ticket. This is the fact base every later question needs
   and it does not exist yet.
2. **Answer "what does aggregate mean" against that table.** Evaluate at least:
   - *(a) Binding-constraint framing* — show the single most-constrained window
     across all agents, plus its reset time. Trivially computable, always
     honest, and matches how the user actually gets blocked (whichever agent
     hits a wall first is what stops work).
   - *(b) Rolling headroom* — "N minutes until the next reset; M agents usable
     right now". Computable from `ResetAt` alone.
   - *(c) Naive percentage sum/mean* — include it explicitly so it can be
     rejected on the record rather than re-proposed later.
   Recommendation to argue for unless the audit contradicts it: **(a) as the
   first pass, (b) as a second line in the same box.** Both avoid inventing a
   denominator that does not exist.
3. **Answer "what unit".** Expected answer given step 0's findings: **not
   tokens, not dollars** — percent-of-plan plus a time-to-reset, because
   percent is the only field every agent populates. Revisit only if 030 lands
   and gives Codex/AGY real token counts.
4. **Answer "how to present uncertainty".** Reuse what exists rather than
   inventing: `QuotaFetchError` already distinguishes "fetch failed" from "no
   window applies" (`types.go:54`), `applyStaleQuota` already has a
   " (stale)" label convention, and `LastRefreshed` gives per-agent age. The
   box should name excluded agents inline ("2 of 3 agents; codex unavailable")
   rather than silently averaging over a hole.
5. **Sketch the box** as literal text (3-6 lines at 80 columns), placed next to
   the existing per-agent boxes in `internal/usage/watch.go`'s panel list — the
   ticket is explicit that per-agent boxes stay. Paste the sketch into this
   ticket; no rendering code yet.
6. **File the implementation follow-up** referencing this ticket, with the
   chosen framing, unit, uncertainty rules, and the sketch fixed. Then close
   084 as "assessment complete".

### Design decisions / tradeoffs

- **Honesty over completeness.** A box that shows one true binding constraint
  beats a combined percentage that averages incomparable denominators. If the
  assessment cannot justify a number, it should say so and ship the framing
  that can be justified.
- **No new data collection.** Everything above is computable from what the
  closed 082 collector already writes. If the assessment concludes otherwise,
  that is itself a finding and should become a dependency ticket, not scope
  creep here.
- **Collapsing per-agent boxes stays out of scope**, as the ticket states.

### Risks / open questions

- The audit (step 1) may find `ResetAt` is unreliable or absent for some
  agents, which would also sink framing (b) and leave only (a).
- 030 (missing Codex/AGY token counts) is a genuine dependency for any
  token- or cost-denominated aggregate. If the assessment picks a
  percent+time framing, 030 stops being blocking — worth stating explicitly in
  the follow-up ticket either way.
- Watch-panel screen budget: the dashboard is already dense; validate the
  sketch against `scripts/canary-watch-pty.sh` at 80 columns before committing
  to a line count.

### Scope

**Small** for the assessment itself (one data dump, one written analysis, one
sketch, one follow-up ticket). The implementation it produces is expected to be
**small-to-medium** — one `build*Box` function plus a panel registration — *if*
framing (a)/(b) is chosen; a true aggregate would be **large** and is expected
to be ruled out.
