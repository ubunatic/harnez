# 107 — Indicate data staleness via dimming/marker in usage UI (compact + full views)

**Status**: Open
**Priority**: P2 (Medium)
**Severity**: Moderate
**Category**: Feature
**Related**: [[106-verify-offline-derivability-of-quota-state]] (follow-up to this audit),
[[103-agy-missing-from-all-usage-aggregate]],
[[104-agy-quota-collector-requires-live-process-poll-coincidence]],
[[105-surface-per-collector-fetch-status-in-usage-ui]],
[[093-usage-tui-layout-planner]], [[094-usage-watch-controls-overlay-and-presets]],
[[101-usage-keep-stale-agents-visible-until-7d]], `internal/usage/types.go` (`AgentUsage.Sources`,
`QuotaFetchError`, `LastRefreshed`), `internal/usage/usage.go:~285-316` (existing `\033[2m` dim
sources note), `internal/usage/watch.go` (`dim()` helper, `formatCompactGroupLineWithLabelWidth`,
`visLen`/`stripANSI`), `internal/rograph/options.go`, `internal/rograph/sparkline.go`.

Note: this ticket was requested as a UX follow-up to a separate "verify" audit ticket (an
Architecture-category ticket auditing whether the storage model lets harnez derive
quota/limit/reset-time state fully offline from historical data). As of filing, no such ticket
exists in `issues/` — the most recent filed tickets are 103, 104, and 105, none of which matches
that description. If/when that audit ticket is filed, it should be added here as a Related link.

## Problem / originating idea

103/104/105 establish that `harnez usage` can show quota data that is actually stale (sourced from
a historical/cache fallback rather than a live poll) without saying so anywhere in the UI. 105
addresses this at the "collector fetch status" level (a status line/marker describing *why* data
looks the way it does). This ticket captures a narrower, complementary proposal focused
specifically on the displayed *values themselves*: make it visually obvious, at the point where a
percentage or time is shown, whether that particular number is fresh or stale — without adding new
columns, since screen space (especially in the compact view, the primary daily-use view) is
already tight post-093/094.

The proposed mechanism, as originally sketched:

- Use color/dimming instead of new columns. When data is live/recently-refreshable (an active
  session exists and the value could genuinely still change on the next poll), render the
  percentage/time in the normal/bright color. When data is stale (sourced from a historical
  fallback, no live refresh happened recently), render the same numbers dimmed.
- ANSI SGR "faint" (code `2`) is the natural mechanism for this, but it is terminal/theme-relative
  by design — not every terminal renders it distinctly, and dimming can hurt contrast/accessibility
  for some users. This should be treated as an open question, not a settled design choice; the
  proposal explicitly should not assume the author's own dark-background/bright-foreground
  terminal is representative.
- An alternative/complementary idea: a single-character marker (e.g. `*`) next to stale values,
  plus a compact footnote/legend line at the bottom of the view (e.g. `* stale data`) instead of
  spelling out staleness per row. This was floated as "just an idea," not a decision.

This ticket is a UX assessment and proposal, not a decided design — see Acceptance Criteria.

## Investigation findings

**Current styling approach**: there is no styling/color library in this codebase (no `lipgloss`,
`termenv`, or similar dependency in `go.mod`) — all color/emphasis is done via hand-written raw
ANSI escape sequences directly in `internal/usage/*.go` and `internal/rograph/*.go`. This matters
because any staleness convention has to compose with hand-rolled `fmt.Sprintf("\x1b[...m%s\x1b[0m",
...)` calls scattered across the rendering code, not a themed style system.

**An existing "dim" convention already exists, but it is inconsistent**:
- `internal/usage/usage.go:314` wraps the full-view "sources:" note in literal SGR faint,
  `\x1b[2m...\x1b[0m` — this is genuine ANSI dim/faint (code 2), terminal-interpreted.
- `internal/usage/watch.go` instead uses `\x1b[90m...\x1b[0m` (explicit bright-black/gray
  foreground) pervasively for de-emphasized text: the `dim()` helper (`watch.go:1167-1168`),
  "not installed" / "installed, not logged in" panel bodies, "quota: unavailable (...)",  the
  "updated Nd Nh ago" caption already printed per-agent (`watch.go:1055`, this is the caption
  referenced in issue 101's work), footer hints, "history: N files" stats, hidden-panel notes.

Both read as "dim" but are different SGR mechanisms with different terminal behavior (faint is a
true dim/opacity attribute many terminals render as such; `90` is a fixed gray color substitution,
which does not adapt to light-on-dark vs dark-on-light in the same way). Any staleness-dimming
convention introduced by this ticket needs to pick one of these and reconcile the other usage, not
introduce a third scheme alongside both.

**No existing quota-bar-fill color convention to reuse**: checked `internal/rograph/options.go`
(`RenderBar`) and `sparkline.go` — quota bars are rendered with plain block glyphs (`░`/`█`-style)
and no threshold-based red/yellow/green coloring; the sparkline's only color use is a fixed
`\x1b[100m` background, unrelated to fill level or freshness. So the "reuse the color already
applied to quota-bar fill" alternative mentioned as a possible simpler option is not actually
available today — there is no existing fill-color convention to reuse. Introducing one would be
new scope, not a free reuse, if pursued as the mechanism.

**Staleness signal granularity**: `AgentUsage.Sources []string`, `QuotaFetchError string`, and
`LastRefreshed time.Time` (the fields 103/104/105 already key off of) live on `AgentUsage` —
i.e. per-agent, not per-`QuotaWindow`. Today there is no per-window/per-value freshness signal;
if one quota window's data came from a live poll and another (within the same agent) came from a
history fallback, that distinction isn't currently captured anywhere. This is a real constraint on
the "dim individual numbers" idea: as the data model stands, dimming can only be decided
per-agent-row (all values in a panel share one freshness verdict), not per-individual-percentage,
unless `QuotaWindow` (or a parallel structure) gains its own provenance field — which would
overlap with 105's note about a possible explicit "fetch mode: live | history | cache | none" enum.
Worth deciding together with 105 rather than inventing a second, incompatible provenance model.

**Compact view feasibility — width budget**: `internal/usage/watch.go`'s
`formatCompactGroupLineWithLabelWidth` (~line 1067) already does exact-content-width formatting
with a fallback that *drops* the reset-time field entirely when the line doesn't fit (`if
visLen(line) > contentW { line = ... without resetStr }`, ~line 1080-1082). This confirms the
compact-row budget is not just visually tight but has an active truncation/field-dropping
mechanism today — i.e. there is no headroom to spend on a literal extra character per value.
However, `visLen`/`stripANSI` (`watch.go:152-154`) explicitly strip ANSI escape sequences before
measuring width, so wrapping a value in `\x1b[2m...\x1b[0m` (or any SGR pair) costs **zero**
visible-width budget under the existing layout engine (093/094's `internal/uix` planner and the
`maxPanelContentWidth` cap included) — the escape sequences are invisible to every width
calculation in this codebase. A literal marker character (e.g. `*`) is not free in the same way:
it is a real, counted rune and can push a line into the existing drop-a-field fallback path.

**Full/non-compact views**: `watch.go:1055` already prints an "updated Nd Nh ago" caption per
agent panel (the caption issue 101's work is built around), and there is more horizontal/vertical
room generally (per-agent boxes, not single dense rows). So per-value or per-panel dimming is
comfortably feasible here with room to spare — this view is much closer to "already mostly solved"
than compact is, and a footnote/legend line is also cheap here since panels already have blank
lines and secondary caption rows.

## UX assessment

- **Dimming as the primary mechanism is the more promising of the two ideas specifically because
  it's free under the current compact layout engine**, per the `visLen`/ANSI-stripping finding
  above — it can be added to compact rows without touching the box-width math issues 093/094 just
  finished stabilizing. The marker+footnote idea is not obviously wrong, but it spends real,
  counted width in the view most likely to already be at its cap, and (per the compact repro shown
  in issue 103) rows are already dense enough that a footnote line risks feeling like a third kind
  of annotation stacked on top of existing hidden-panel/footer hint lines.
- Dimming alone is weaker as a *sole* signal than marker+footnote for accessibility/discoverability
  reasons the original proposal itself flagged: SGR faint (code 2) is genuinely not rendered
  distinctly by every terminal, and reduced contrast is an active accessibility concern for some
  users, not just a cosmetic terminal-compat detail. A silent color-only signal that some
  terminals render identically to normal text is a regression risk for anyone on such a terminal —
  worth deciding whether dimming should be the sole signal, or paired with something
  terminal-independent (a marker, but only in views with width to spare; or a difference already
  present in the text, like the "unavailable"/"stale" wording issue 105 is introducing).
- The per-agent (not per-value) granularity limit is probably the most important open question
  functionally, independent of which visual mechanism is chosen: if only whole-panel/whole-row
  freshness is knowable today, "dim just the stale number, leave the live one bright" as literally
  described in the originating idea may not be achievable without a data-model change that
  overlaps with issue 105. Worth resolving that scope question (row-level vs. value-level) jointly
  with 105 before committing to an implementation, rather than each ticket inventing its own
  provenance granularity.
- The existing `\x1b[2m` vs `\x1b[90m` inconsistency is a small but real pre-existing debt this
  ticket's implementation will step on regardless of which mechanism is chosen — worth reconciling
  as part of this work rather than adding a third convention.

None of the above is meant to force a final design — flagged explicitly as open questions for
whoever implements this, with more user input likely warranted before locking in the exact visual
spec.

## Acceptance Criteria

1. Stale values are visually distinguishable from live values in both the compact view
   (`--summary --compact`, including the `[a] All Usage` aggregate) and the fuller views
   (`--summary`, `--watch` per-agent panels), without adding new columns or growing any box beyond
   its current `internal/uix`-enforced width budget (093/094).
2. The mechanism degrades gracefully on terminals that don't render the chosen ANSI attribute
   distinctly — i.e. a user on such a terminal doesn't lose information, only the visual emphasis
   (this likely means: don't rely on dimming alone if a terminal-independent complement is cheap
   enough to include; document the decision either way).
3. The staleness signal used is decided jointly with (or reuses the vocabulary from) issue 105's
   per-collector fetch-status work, rather than introducing a second, incompatible notion of
   "stale" — resolve whether the granularity is per-agent-row or per-value as part of
   implementation, informed by whether `QuotaWindow`-level provenance is added for 105.
4. The exact visual mechanism (dim-only, marker+footnote, marker-only, reuse of a to-be-introduced
   quota-bar color convention, or something else) is decided by whoever implements this, not fixed
   by this ticket — this ticket establishes the requirement, the constraints found above, and the
   open questions, not the final spec.
5. Verify rendering at realistic (not just wide) terminal widths, consistent with issue 105 AC #4,
   since compact-row width is the primary constraint this ticket has to respect.

## Notes for whoever picks this up

- Read issue 105 first — it already establishes the practical fetch-status vocabulary and
  per-agent status story; this ticket is specifically about how displayed *values* reflect that
  story visually, not about collecting new status data.
- The `visLen`/`stripANSI` free-width finding above is the strongest argument for dimming being
  viable in compact view at all; re-verify it still holds if `internal/uix`'s width measurement
  changes in the future (093/094 just landed, so this is current as of filing).
- Consider whether the "verify"/offline-derivability audit ticket referenced in this ticket's
  origin (not found in the repo as of filing — see the Related note above) should land first,
  since it may reshape what "stale" even means at the data-model level.
