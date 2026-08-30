# 105 — Surface per-collector fetch status in usage UI, including compact view

**Status**: Open
**Priority**: P2 (Medium) — this is observability/UX, not itself a data-correctness bug like
103/104; it doesn't lose or misrepresent quota numbers on its own. But given that 103/104 were
only found by the user cross-referencing manual disk digging (AGY's own conversation DB/log
timestamps) rather than anything `harnez usage` itself said, and that this exact kind of
silent-collector-failure has now recurred at least twice (086, 103/104), there's a case for P1.
Left at P2 because it's additive UI work with no user-facing bug to fix today — bump to P1 if
another silent-collector-gap incident surfaces before this is picked up.
**Severity**: Moderate
**Category**: Feature
**Related**: [[103-agy-missing-from-all-usage-aggregate]],
[[104-agy-quota-collector-requires-live-process-poll-coincidence]],
[[093-usage-tui-layout-planner]], [[094-usage-watch-controls-overlay-and-presets]],
[[085-watch-tui-show-collector-daemon-status]] (adjacent: that ticket is about the *daemon
process* being alive; this ticket is about *per-agent collector fetch outcome*, a different
axis), `internal/usage/types.go` (`AgentUsage.Sources`, `QuotaFetchError`, `LastRefreshed`),
`internal/usage/watch.go` (`buildAllUsageBox`, per-agent panel rendering),
`internal/usage/usage.go:~284-300` (existing verbose "sources:" line)

## Problem

Issues 103 and 104 together describe a real, multi-day AGY quota-collection failure: no live AGY
process happened to be running at any harnez poll tick for 7+ days, the on-disk stale-cache
fallback had nothing to fall back to either, and none of this was ever surfaced anywhere in
`harnez usage` — the per-agent panel instead showed a misleadingly fresh "updated just now" with
an empty quota box, and the `[a] All Usage` aggregate simply omitted AGY's row entirely, with no
indication anything was wrong. The user only discovered the gap by independently inspecting AGY's
own conversation database and log timestamps and noticing they didn't match what harnez was (not)
reporting.

That investigation path — manually diffing on-disk state outside the tool to figure out what the
tool itself isn't saying — is the actual gap this ticket targets. `AgentUsage` already carries the
raw ingredients for a status story (`Sources []string`, `QuotaFetchError string`,
`LastRefreshed time.Time`), and they're partially surfaced today:

- The full/non-compact per-agent panel (`internal/usage/usage.go:~284-300`) prints a raw
  `sources:` line listing `agent.Sources` verbatim (e.g. `~/.claude/harnez-quota-cache.json
  (stale)`, `127.0.0.1 (LanguageServer RPC)`, `<dir> (usage-history, stale)`) — useful, but it's
  an unsynthesized dump of file paths, not a "collector status" a user can scan at a glance, and
  it only exists in the full/verbose summary output, not `--watch`'s per-agent boxes or the
  compact view.
- `internal/usage/watch.go:1036-1055` (per-agent panel) does print a `quota: unavailable
  (<QuotaFetchError>)` line when `QuotaFetchError` is set and no quota data exists, and an
  `updated <FormatAgo(LastRefreshed)>` line — but as issue 103 documents, `QuotaFetchError` is
  never actually set for AGY's "zero live ports found" case (`agy.go:337-382`, `len(ports) == 0`
  skips the assignment), and `LastRefreshed` gets defaulted to "now" when zero
  (`internal/usage/usage.go:~73-75`), so in exactly the failure mode 103/104 describe, both of
  these existing surfaces say nothing useful or actively mislead.
- The `[a] All Usage` aggregate box (`buildAllUsageBox`, `watch.go:592-648`) has **no staleness or
  status concept at all** — it only ever emits a row when `ModelGroups`/`Weekly`/`Session` are
  non-empty (`watch.go:612-636`); when they're empty, the agent's row is just absent, with nothing
  distinguishing "genuinely never used this agent" from "used it constantly, collector's been
  silently failing for a week."

So even once 103/104's underlying bugs (staleness-gate discarding fallback data, live-RPC-only
collection) are fixed, there's still no mechanism in `harnez usage` itself that would have let the
user notice this class of problem *from the tool* rather than from manual disk archaeology — and
no fix to 103/104 changes that; a *future* collector regression (AGY or otherwise) would recreate
the same blind spot.

## Why it matters

Collector-status invisibility doesn't just mean missing data — it actively masks and compounds
correctness bugs like 103/104: without a status indicator, a user has no way to distinguish three
very different conditions that currently all look about the same (nothing shown, or a stale
"updated just now"):

1. "This agent has genuinely stopped answering" (upstream collector broken — 104's case).
2. "This agent has nothing to report because I haven't used it" (expected/benign).
3. "Everything is fine, this is just today's real quota state."

Today's UI cannot tell these apart, so a real collector outage (case 1) reads identically to
"nothing to see here" (case 2) — which is exactly how 103/104 went unnoticed for 7+ days despite
near-daily real AGY use. Fixing 103/104's specific bugs restores AGY today, but doesn't give the
user (or a future maintainer) any way to *notice* the next instance of this pattern without
repeating the same manual-log-diffing investigation.

## Acceptance Criteria

1. **Full/verbose usage view**: a per-agent collector status must be visible, at minimum
   including: last successful fetch time (distinct from the "updated Nd ago" wall-clock label,
   which per 103 can currently be wrong), and whether the current data came from a live poll, a
   history fallback, or a cache snapshot — reusing/synthesizing the tagged strings already present
   in `Sources` (`(stale)`, `(cached)`, `(usage-history, stale)`, live RPC source) into a single
   human-readable status rather than a raw path dump — plus any current `QuotaFetchError`,
   surfaced even when it's a "no data source available" condition rather than an RPC failure
   (which in turn requires the `QuotaFetchError`-not-set-when-`len(ports)==0` gap from 104
   AC #3 to actually be fixed for AGY to have anything to show here — cross-reference, don't
   duplicate that fix in this ticket).
2. **Compact view (`--summary --compact`)**: this is the user's primary/most-used view and must
   not be left out or treated as secondary. A compact, space-efficient collector-health indicator
   must appear per agent row even under the tight width budget the compact layout already commits
   to post-093/094 (box widths capped and packed via `internal/uix`, commits e9c09cb/c313a33) —
   this does not mean the full detail from criterion 1; a single marker/glyph/short suffix per
   row is sufficient, e.g. distinguishing at minimum:
   - live (fresh, this poll cycle)
   - history/cache fallback, stale (with an age, e.g. `7d`)
   - no data / never fetched (with a reason if available, e.g. `no proc found`)
3. **`[a]` All Usage aggregate specifically**: since this box currently omits a row entirely when
   an agent has no quota windows (`watch.go:612-636`), decide and implement one of: (a) still
   render a minimal row for an agent with `HasUsageData()==true` but empty quota windows, carrying
   just the compact status indicator from criterion 2 instead of bars, so the agent's presence and
   problem are visible even with nothing to plot; or (b) if the row is still omitted rather than
   rendered degraded, document why that's the right tradeoff for the aggregate specifically. Don't
   let "no windows to render" silently collapse to "nothing to say" here, which is the exact shape
   of 103's bug.
4. Verify AC #2 and #3 render sanely at realistic terminal widths (not just wide terminals) —
   the compact box in 103's reproduction is already width-constrained (`Claude Code   [░░░░] 7%
   6d4h  [██░░] 56% 4h15m`), so the added indicator needs to fit without pushing the box past
   `internal/uix`'s planner-enforced caps or wrapping.
5. Whoever implements this should decide the concrete glyph/marker vocabulary and where exactly it
   attaches per row (leading marker vs. trailing suffix vs. color-only) as part of the
   implementation, not as a precondition for filing this ticket — this ticket establishes the
   requirement and acceptance bar, not the final visual spec.

## Notes for whoever picks this up

- Don't fold 103/104's actual data-correctness fixes into this ticket's scope — this ticket is
  purely about status *visibility*, and is meaningful independent of whether 103/104 are fixed
  first (arguably more urgent to land before them, precisely so their fix can be verified through
  the UI rather than through manual log/history-file inspection again).
- `Sources` already carries most of the raw signal needed for criterion 1 — this may be more of a
  "synthesize and relocate existing data" task than a "collect new data" task. Check whether
  `QuotaFetchError`/`LastRefreshed`/`Sources` need any additional fields (e.g. an explicit
  enum-like "fetch mode: live | history | cache | none" instead of string-matching `Sources`
  suffixes) to make criterion 2's compact classification robust rather than parsed out of
  free-text source strings.
- Issue 085 (daemon status in `--watch`) is related but distinct: that's "is the background
  collector daemon process itself alive," this ticket is "did each agent's most recent collection
  attempt actually succeed, and with what data provenance." Both could plausibly share UI real
  estate but shouldn't be conflated in implementation.
