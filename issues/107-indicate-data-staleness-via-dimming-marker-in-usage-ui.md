# 107 — Indicate data staleness via dimming/marker in usage UI (compact + full views)

**Status**: Closed — resolved in b7295cd
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

## Resolution Note

Implemented without waiting for issue 105 (still open/unimplemented as of this work), using only
the per-agent signals that already exist today (`Sources`, `QuotaFetchError`, `LastRefreshed`) —
per this ticket's own instruction not to block on 105. A note pointing 105 at the vocabulary
introduced here has been added to `issues/105-surface-per-collector-fetch-status-in-usage-ui.md`.

**Staleness definition** — new `AgentUsage.IsValueStale()` (`internal/usage/types.go`), a per-agent
verdict (the data model still has no per-`QuotaWindow` provenance field, so granularity stays
row-level, matching the ticket's own investigation finding). True whenever any of:
- `QuotaFetchError != ""` (a live fetch was attempted this cycle and failed);
- any `Sources` entry contains the substring `"stale"` (matches the pre-existing issue 032/086
  tagging convention: `"(stale)"`, `"(cached, stale)"`, `"(usage-history, stale)"` — deliberately
  does NOT match a plain `"(cached)"` tag, which is a fresh, non-stale cache hit);
- `LastRefreshed` is non-zero and older than `DefaultCacheStaleness` (2× the collector interval,
  30 min) — reusing the same duration `cacheOrLive` itself uses to decide "old enough to attempt a
  live recollect," rather than inventing an unrelated threshold. A zero `LastRefreshed` is
  "unknown," not stale, matching `IsStale`'s existing contract. This is intentionally a much
  tighter/more sensitive threshold than `DefaultDisplayStaleness` (7d, issue 101's unrelated
  auto-hide gate) — the two answer different questions ("should this row still be shown at all" vs.
  "should this row's numbers read as live right now") and both now coexist without collision.

**Dim mechanism** — dim-only (SGR `\x1b[90m`, the "dim-grey" named color in `spec/colors.yaml`),
reconciling the pre-existing `\x1b[2m`/"dim-faint" vs. `\x1b[90m`/"dim-grey" split onto dim-grey
everywhere: dim-grey was already the dominant convention (10+ call sites in `watch.go` vs. one),
and being an explicit color substitution rather than reliance on the SGR "faint" attribute, it
degrades more consistently across terminal emulators (some render faint identically to normal
text; dim-grey visibly changes color on nearly all of them). `RenderText`'s "sources:" note
(`usage.go`, previously the sole `dim-faint` call site) was migrated to dim-grey. `dim-faint`
itself is left defined in `spec/colors.yaml`/`colorsspec.go` rather than deleted, in case a future
feature wants the distinct "true faint" semantic rather than "de-emphasized."

New `staleValueANSI`/`ansiWrapPreservingResets` helpers (`internal/usage/colorsspec.go`) wrap a
whole already-built line rather than an individual token: a plain SGR wrap would be silently
canceled partway through by a bar glyph's own embedded `\x1b[0m` reset (`rograph.RenderBar` always
closes its background-color wrap), so the helper re-asserts dim-grey immediately after every
embedded reset. Confirmed zero-width-cost via `visLen`/`stripANSI` (both strip ANSI before any
layout math), matching the ticket's own free-width finding — see
`TestStaleValueANSIPreservesResetsAndCostsNoWidth`.

Per AC #2 (terminal-independent complement, not dimming alone): the compact `[a]` All Usage
aggregate (tightest width budget, active field-dropping fallback) gets **dim-only**, no text — a
literal marker would cost real, counted width there. The fuller per-agent panel/box (`--watch`
non-compact panels, `buildAgentBoxAt`) and the full/verbose `--summary` view (`RenderText`) both
have comfortable room to spare, so both additionally append a plain-text `" · stale"` suffix onto
the existing "updated ... ago"/"Updated:" caption line — a terminal-independent signal that
survives on any terminal regardless of how (or whether) it renders dim-grey distinctly.

**View-by-view treatment**:
- Compact `[a]` All Usage aggregate (`allUsageLinesAt`/`formatAllUsageTableLine`/
  `formatAllUsageSingleWindowLine`, `watch.go`): whole row dimmed when the contributing agent's
  `IsValueStale()` is true (per-agent, shared across all of that agent's model-group rows). No text
  marker.
- Fuller per-agent panel (`buildAgentBoxAt`, `watch.go`, used by non-compact `--watch` and
  individual per-agent panels): quota line(s) dimmed + `" · stale"` appended to the "updated ...
  ago" caption.
- Full/verbose `--summary` (`RenderText`, `usage.go`): Session/Weekly/ModelGroup value lines dimmed
  + `" · stale"` appended to the "Updated:" caption. Also fixed a latent width-measurement bug this
  change would otherwise have triggered: the box-sizing/padding math there used a raw
  `utf8.RuneCountInString` on each line (no prior line ever carried embedded ANSI), which would
  have mis-sized/mis-padded the box once dimmed lines were introduced; switched to the
  ANSI-stripping `visLen` already used everywhere in `watch.go`.

Both compact and full-view dimming were verified with unit tests
(`TestAllUsageLinesAtDimsStaleAgentRow`, `TestBuildAgentBoxDimsStaleQuotaAndAnnotatesUpdatedCaption`,
`TestRenderTextDimsStaleQuotaLineAndAnnotatesUpdated`) and manually against the real installed
binary at realistic terminal widths (`COLUMNS=100 harnez usage`, plus a scratch scenario forcing a
stale AGY row) — output stayed correctly aligned, embedded bar colors survived the dim wrap intact,
and a genuinely-fresh agent in the same view rendered with no dimming and no `"stale"` text.

Fixed one latent test bug surfaced by this change: `TestBuildWatchFrameAtDebugOverlayUsesWatchFetchInterval`
(`watch_test.go`) hardcoded an absolute past `LastRefreshed` date instead of a `time.Now()`-relative
one, which crossed the new (much shorter) staleness threshold as real time passed and started
tripping the new dimming. Updated it to a relative timestamp, matching the pattern already used by
the other staleness-adjacent tests in the same file.

`go build ./...`, `go vet ./...`, and `go test ./...` all pass.
