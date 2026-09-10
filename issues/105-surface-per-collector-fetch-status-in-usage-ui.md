# 105 — Surface per-collector fetch status in usage UI, including compact view

**Status**: Open — partial staleness and splash visibility; collector provenance remains missing
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

### Audit — 2026-09-10

- **Conclusion: partially solved.** `AgentUsage.IsValueStale`,
  `allUsageLinesAt`, `buildAgentBox`, and `RenderText` already dim stale
  values; full views annotate age/staleness. Splash stages and badges expose
  initial fetch completion/failure. These supersede the historical claim
  below that the aggregate has no staleness concept at all.
- Remaining: concise live/cache/history/no-source classification and last
  successful fetch provenance across full and compact views. The aggregate
  still adds rows from quota windows; it has no diagnostic-only row for an
  agent with usage evidence but no windows. Per-agent watch errors are shown
  only when windows are absent, so a fallback can still hide the error detail.
- **Measured:** `go test ./...` passes. Existing assertions include
  `TestAllUsageLinesAtDimsStaleAgentRow`,
  `TestBuildAgentBoxDimsStaleQuotaAndAnnotatesUpdatedCaption`, and
  `TestSplashStatusLineFormatsEachStage`; they verify these partial surfaces,
  not the complete provenance vocabulary and narrow-width acceptance cases.

### Implementation progress — 2026-09-10

- Added `AgentUsage.CollectorStatus()` and surfaced the derived status in the
  verbose usage view, reusing `Sources`, `QuotaFetchError`, and quota/token
  presence without changing the JSON shape (`c645ed8`).
- The compact per-row marker and diagnostic no-window aggregate row remain
  open, so this ticket stays Open/partial.

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

## Note from issue 107 (dimming/de-emphasis for stale values, implemented)

107 shipped without waiting for this ticket (per its own instructions), using only the per-agent
signals available today. Before designing this ticket's fetch-status vocabulary/enum, be aware of
what 107 already introduced so this ticket reuses/reconciles it rather than inventing a second,
incompatible notion:

- `AgentUsage.IsValueStale()` (`internal/usage/types.go`): a per-agent-row (not per-`QuotaWindow`)
  "should this row's numbers read as live right now" verdict — true when `QuotaFetchError != ""`,
  or a `Sources` entry contains the substring `"stale"` (the existing `"(stale)"`/`"(cached,
  stale)"`/`"(usage-history, stale)"` tagging convention), or `LastRefreshed` is older than
  `DefaultCacheStaleness` (30 min). This is a coarser, string-matched, per-agent boolean —
  exactly the kind of thing this ticket's proposed explicit `fetch mode: live | history | cache |
  none` enum would supersede. If that enum lands, `IsValueStale` should be re-derived from it
  (fetch mode `history`/`cache`/`none` implying stale, `live` implying not) rather than kept as a
  second, parallel heuristic.
- A rendering convention (`internal/usage/colorsspec.go`'s `staleValueANSI`) that dims a value with
  `\x1b[90m` ("dim-grey", now the sole de-emphasis/staleness color — see 107's Resolution Note for
  why `\x1b[2m`/"dim-faint" was retired from active use) and, in views with room to spare (full
  `--summary`, `--watch` per-agent panels — NOT the compact `[a]` All Usage aggregate, which stays
  dim-only for width reasons), appends a plain-text `" · stale"` suffix to the existing "updated
  ... ago" caption. If this ticket adds a distinct glyph/marker vocabulary for collector-fetch
  status (criterion 2's "live / history-fallback / no-data" markers), keep it visually distinct
  from the `" · stale"` suffix or fold the two together deliberately — don't let a row end up with
  two independent-looking staleness annotations that actually mean overlapping things.
- Granularity is still per-agent, not per-`QuotaWindow`, in both tickets' current state — 107's
  investigation flagged this same data-model gap as the reason it couldn't do finer-grained
  dimming; this ticket's provenance-enum idea is the natural place to fix that for both, if pursued.

---

## Implementation Plan

### Core decision: add an explicit `FetchMode` enum, retire the string-matching

The ticket's own note answers its main open question. `IsValueStale()`
(`internal/usage/types.go:137`) currently decides staleness by
`strings.Contains(source, "stale")` over free-text `Sources` strings — that heuristic is
what this ticket must replace, not extend. Everything else follows from having the enum.

```go
// internal/usage/types.go
type FetchMode string

const (
    FetchModeUnknown FetchMode = ""        // collector predates the field / not threaded through
    FetchModeLive    FetchMode = "live"    // fresh answer from the agent this cycle
    FetchModeCache   FetchMode = "cache"   // daemon snapshot (statecache.go)
    FetchModeHistory FetchMode = "history" // usage-history fallback (fillFromHistoryIfNoQuotaWindows)
    FetchModeNone    FetchMode = "none"    // nothing available; see QuotaFetchError for why
)
```

Added to `AgentUsage` as `FetchMode FetchMode \`json:"fetch_mode,omitempty"\`` plus
`LastSuccessfulFetch time.Time \`json:"last_successful_fetch,omitempty"\`` (AC #1's
"last successful fetch time, distinct from the wall-clock `updated` label"). Both are
`omitempty` so existing on-disk snapshots deserialize unchanged as `FetchModeUnknown`.

`IsValueStale()` is then re-derived, exactly as the ticket's 107 note prescribes:
`FetchModeCache/History/None` → stale, `FetchModeLive` → not stale, `FetchModeUnknown` →
**fall back to today's string/timestamp heuristic verbatim** so no behavior regresses for
snapshots or collectors that haven't been updated.

### Steps, in order

1. **`internal/usage/types.go`** — add `FetchMode`, the constants, the two `AgentUsage`
   fields, a `FetchStatus() (mode FetchMode, age time.Duration, reason string)` helper
   that synthesizes the one human-readable status string every view will render, and the
   rewritten `IsValueStale()` with the unknown-mode fallback. Keep `Sources` untouched —
   it stays the raw provenance dump for the verbose view; the enum is the *classification*
   layered on top, not a replacement.

2. **Set the mode at each origin point** (this is the bulk of the work, ~1 line each):
   - `internal/usage/claude.go`, `codex.go`, `agy.go`: `FetchModeLive` +
     `LastSuccessfulFetch = time.Now()` on a successful live fetch; `FetchModeCache` on
     their internal stale-cache fallback paths (the `(stale)`/`(cached)` `Sources` tagging
     sites — grep `"(stale)"` / `"(cached"` to find them all).
   - `internal/usage/statecache.go` `cacheOrLive` (~line 205): when it serves the daemon
     snapshot rather than calling `collect`, stamp `FetchModeCache` and carry
     `LastSuccessfulFetch` from the snapshot's `FetchedAt`.
   - `internal/usage/usage.go` `fillFromHistoryIfNoQuotaWindows` (~line 148-150): stamp
     `FetchModeHistory`.
   - `internal/usage/usage.go` collectAll's zero-`LastRefreshed` defaulting (~line 167-176):
     this is the line 103 called out as actively misleading. Keep defaulting `LastRefreshed`
     (it means "when we last looked"), but do **not** default `LastSuccessfulFetch` — a zero
     value there is the honest "never" signal, and `FetchModeNone` should be stamped when a
     collector returns with neither data nor a `QuotaFetchError`.

   Cross-reference only, do not fix here: 104 AC #3's `len(ports)==0` → `QuotaFetchError`
   gap in `agy.go`. This plan's `FetchModeNone` gives that case somewhere to land, but the
   AGY-specific fix belongs to 104.

3. **Verbose/full view — `internal/usage/usage.go` `RenderText` (~line 360-430).** Replace
   nothing; *add* one line above the existing raw `sources:` dump:
   ```
   collector: live · fetched 2m ago
   collector: cache · last live fetch 6d ago
   collector: no data · no agy process found (last live fetch 7d ago)
   ```
   Reuse `staleValueANSI` for the non-live cases so it matches 107's dim-grey convention.
   Keep the existing `" · stale"` suffix on the `updated` caption **or** drop it in favour
   of this line — pick one; two annotations meaning overlapping things is the exact failure
   the 107 note warns about. Recommendation: **drop the `" · stale"` suffix in the verbose
   view only** (this new line is strictly more informative there) and keep it everywhere
   107 put it that this ticket doesn't touch.

4. **`--watch` per-agent panel — `internal/usage/watch.go:~1036-1055`.** Same line as
   step 3, one row, already has the vertical budget.

5. **Compact / `[a] All Usage` — `internal/usage/watch.go` `allUsageLinesAt` (~line 676+).**
   This is AC #2 and #3 and the width-sensitive part:
   - The `allUsageRow` struct already carries `stale bool`; add `mode FetchMode`.
   - **AC #3: choose option (a)** — render a minimal row for any agent with
     `HasUsageData() == true` but zero quota windows, instead of `continue`-ing past it.
     The row is `label` + the compact marker + a short reason, no bars. Rationale: "no
     windows" collapsing to "no row" is literally 103's bug shape; the aggregate is the
     view where an agent's *absence* is most misleading.
   - **Marker vocabulary (AC #5 decision)**: a single trailing suffix, no new glyph
     alphabet, no color-only signalling (fails on non-color terminals and duplicates 107):
     | mode | suffix | width |
     |---|---|---|
     | live | *(nothing)* | 0 |
     | cache / history | `·6d` (age of last successful fetch) | 3-4 |
     | none | `·—` plus, if it fits, a short reason | 2+ |
     Dim-grey (`staleValueANSI`) applied to the suffix only. Absence-of-marker means live,
     which keeps the common case at zero width cost — the only way to satisfy AC #4's
     width budget without touching `internal/uix`'s planner caps.
   - Compute the suffix **before** label-width padding so `labelWidth` accounts for it;
     if `contentW` can't fit label + bars + suffix, drop the reason text first, then the
     age, then the suffix entirely (never the bars).

6. **Tests** (`internal/usage/types_test.go`, `watch_test.go`, `usage_test.go`):
   - `IsValueStale` truth table across all five modes **plus** the unknown-mode fallback
     asserting byte-identical behavior to today for legacy inputs.
   - Round-trip: an `AgentUsage` with `FetchMode`/`LastSuccessfulFetch` through
     `WriteAgentSnapshot`/`ReadAgentSnapshot`; and a legacy snapshot JSON without the
     fields deserializing to `FetchModeUnknown` without error.
   - `allUsageLinesAt` with a fabricated agent that has `HasUsageData() == true` and zero
     `ModelGroups`/`Weekly`/`Session`: assert a row *is* emitted and carries the marker
     (this is the direct 103-regression test).
   - `allUsageLinesAt` at a narrow `contentW` (e.g. 40): assert no rendered line exceeds
     `contentW` — AC #4, asserted mechanically rather than eyeballed.

### Tradeoffs / risks

- **Enum threading is the risk, not the rendering.** There are several places a collector
  can return, and missing one leaves `FetchModeUnknown` — which degrades to today's
  behavior rather than to a wrong claim. That fallback is deliberate and should not be
  removed even once every site is covered.
- **Per-`QuotaWindow` provenance stays out of scope.** Both 105 and 107 flagged it; doing
  it means changing `QuotaWindow` and every collector's parse path. The per-agent enum
  already satisfies every AC here. Note it as follow-up, don't build it.
- **`Sources` stays.** Deleting it would break the verbose view's usefulness and any
  downstream JSON consumer; the enum sits alongside.
- Ordering vs. 103/104: this can and probably should land **first**, per the ticket's own
  note, so those fixes are verifiable through the UI.

### Scope

**Medium** — one data-model addition, ~8 small collector call-site changes, three render
sites, and a focused test batch. Contained to `internal/usage`; no CLI flags, no new files
strictly required (`FetchMode` can live in `types.go`).
