---
title: Usage Panel Integration and the AGY Collector Cliff
weight: 91
---

# Usage Panel Integration and the AGY Collector Cliff

*2026-08-30 — a retrospective on one `/fresh-sprint` session, continuing from
[2026-08-29-a-day-of-fresh-sprints.md](2026-08-29-a-day-of-fresh-sprints.md)*

## What shipped

Four `internal/usage` tickets landed, sequentially, no conflicts:

- **090** — the `[L] Load` panel in `--host` remote mode now renders from a
  real remote CPU/GPU snapshot carried in the existing SSH JSON payload,
  instead of silently showing the local machine's load while every other
  panel described the remote one.
- **093** — the standalone `internal/uix` box-layout prototype (built the
  day before, never wired in) got integrated into the real
  `--summary`/`--watch`/`--compact` renderer, replacing ad-hoc packing
  logic. Fixed `--compact --proc` silently dropping the Processes panel,
  added keyed hidden-panel hints (`[P] [O] hidden` instead of a bare
  count), and stopped panels from stretching arbitrarily across empty
  terminal width.
- **094** — a `?`-triggered controls overlay for `--watch`, a trimmed
  footer, and an `[m]` preset cycle (default / compact / agents-only),
  replacing a growing pile of single-key toggles nobody could remember.
- **102** — `harnez usage --summary --compact` now works; it was rejected
  by a flag guard that only ever anticipated `--watch --compact`, even
  though `--summary` renders one static frame of the exact same grid.

Each shipped with real tests, `make check`, and a manual render check —
consistent with the discipline the previous day's retrospective argued for.

## The AGY collector cliff

The day's second half was triggered by the user reporting, again, that
Antigravity (AGY) quota data wasn't showing up — this time in the `[a] All
Usage` compact aggregate specifically. Investigation peeled back three
distinct, stacked layers, each real, none sufficient on its own:

1. **Issue 103** — the *display* layer. `fillFromHistoryIfNoQuotaWindows`
   (`internal/usage/history.go:335-349`) fills quota windows from the last
   `usage-history/*.jsonl` entry, but only if that entry is under 7 days
   old (`DefaultDisplayStaleness`). AGY's last real snapshot had just
   crossed that line. Both the aggregate and the per-agent panel went
   blank simultaneously — the per-agent panel's continued visibility
   turned out to be a side effect of a separate "default `LastRefreshed`
   to now when zero" behavior, not evidence the fallback was working.

2. **Issue 104** — the real root cause. `CollectAGY`
   (`internal/usage/agy.go:192-426`) has exactly one source of quota data:
   a live Connect-RPC call to a currently-running AGY `LanguageServer`
   process, found by scanning `/proc`. When the user showed a screenshot
   of AGY's own conversation history with activity as recent as
   yesterday, that contradicted the "just crossed 7 days" framing — real
   usage was happening, but zero fresh quota snapshots had landed in a
   week. The collector's only data path requires a live process to be
   listening on a TCP port at the exact moment harnez polls; AGY is
   invoked per-task rather than run as a persistent daemon, so that
   coincidence apparently hadn't landed once in seven days despite
   near-daily real use. Fixing 103's staleness gate alone would not have
   fixed this — there would still be nothing fresh for the gate to admit.

3. **Issue 106** (audit, closed) — a systematic check of whether the
   storage model even *could* support full offline derivation. Claude Code
   and Codex: yes, fully — session/weekly windows and reset times durably
   persist and survive process exit. AGY: no — it has no durable fallback
   at all; its own on-disk state (conversation DBs, logs) never carries
   quota or reset fields, so once the live RPC stops landing, the data is
   gone until it succeeds again, not just stale. The audit also caught an
   unticketed nuance: `QuotaWindow.DurationLeft` is a frozen relative
   countdown computed at fetch time, never recomputed from the durable
   absolute `ResetAt` — so cached data shows an increasingly wrong
   "resets in Xh" even when the true reset timestamp is still correct.

4. **Issues 105 and 107** — two UX follow-ons filed but not implemented:
   surfacing per-collector fetch status in the UI (including the compact
   view), and using dimming/markers to distinguish live from stale values
   without spending extra column width — the layout engine from 093
   strips ANSI before measuring width, so color is free but an extra
   character is not.

None of 103–107 were implemented this session — investigated, diagnosed,
and ticketed only, per explicit instruction, so the fix work can be
picked up deliberately later with the full picture in hand rather than
patching the first symptom found.

## The ticket-number race

Two ticket-filing-only subagents were dispatched close together — both
read-only investigations with no code changes intended. Each independently
scanned `issues/README.md` for "the next free number" and both landed on
106, producing two files claiming the same number and two duplicate rows
in the tracker table. Caught only because both completion reports happened
to mention issue 106; fixed by hand (renumber, re-link, de-duplicate,
commit `f6d8766`).

This is a different failure mode than the "parallel writes to the same Go
package" risk `docs/practices/AgenticLoop.md`'s Parallel-Read-Sequential-Write
invariant was written for. Both agents genuinely only wrote to files the
other wasn't touching in the same instant — the collision came from a
stale *read* (each computed "next free number" from a README snapshot that
was already out of date by the time it wrote). Any shared, sequentially-
allocated resource is vulnerable to this even when file-level overlap
looks clean. Issue 108 tracks the fix: `harnez status`'s tracker linter
already has a `DiagDuplicateNumber` check that would have caught the
duplicate rows if run between the two commits, but a related check
(`numToFileMap`, file-level duplicate detection) is built and never
actually consulted — a real, if minor, dead-code gap.

The resolution was immediate and unconditional: subagent dispatch is now
sequential by default for *every* task type — code, investigation, and
ticket-filing alike — not just once two tasks are known to converge on the
same package. Parallel dispatch happens only on the user's explicit
request for a specific task, and even then the orchestrator verifies the
run actually was parallel and checks for exactly this class of race
afterward. Saved to persistent memory
(`feedback_no_parallel_agents.md`) so it survives session boundaries.

## What this adds to yesterday's lessons

- **"Resolved" still needs a live-verification story** — reconfirmed by
  094's manual PTY-driven checks and 102's before/after render comparison,
  not just green tests.
- **Investigation-only dispatch is a real, distinct mode** worth using
  deliberately — five of today's subagent dispatches were "find out and
  report, do not fix," which kept a genuinely three-layered bug from being
  patched at the wrong layer.
- **Sequential-by-default now has no exceptions carved out by task type.**
  Yesterday's lesson was "parallel is risky once tickets converge on a
  package." Today's incident showed the risk is broader than code
  conflicts — it's any shared sequential-allocation resource, and
  ticket-filing was assumed safe precisely because it looked read-only.
  It wasn't.

## What's still open

The AGY collector fix itself (104) is unstarted — the highest-value next
step, since 103's staleness-gate fix and 107's UX work are both
downstream of it. 105 (collector status visibility) and 108's mechanical
guard (extending the tracker linter, or making issue-number allocation
atomic) are both real, scoped, and waiting. 051 (multi-host monitoring)
remains too open-ended to dispatch as-is — no acceptance criteria, spans
four separate subsystems — and needs a scoping pass before it's
fresh-sprint-shaped.
