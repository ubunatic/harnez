# 172 — AGY "Claude/GPT" Row Drops Its Second Window at 100%, Breaking All Usage Grid Alignment

**Status**: Open
**Priority**: P2 (Medium)
**Severity**: Minor
**Category**: Bug
**Related**: `internal/usage/agy.go` (`parseAGYUsageOutput`), `internal/usage/watch.go`
(`formatAllUsageTableLine`, `formatCompactGroupLineWithLabelWidth`), issue
[[103-agy-missing-from-all-usage-aggregate]], issue [[104-agy-quota-collector-requires-live-process-poll-coincidence]]
(prior AGY/All-Usage integration work)

---

## 1. Problem & Motivation

User-reported, with screenshot of `harnez usage`'s "All Usage" box:

```
 ¹ All Usage ────────────────────────────────────
Claude Code   [    ] 5% 3d20h   [    ] 48% 2h12m
Gemini        [    ] 0% 6d23h   [    ] 1% 4h39m
Claude/GPT    [████] 100% 2d13h
OpenAI Codex  [    ] 46% 5d12h  [    ] 66% 2h21m
```

Every other row shows two bars (weekly window + 5-hour/session window). The AGY "Claude/GPT" row
shows only one bar (weekly, at 100%) and the row is visibly shorter than the others — the second
bracket column is simply missing, breaking the box's grid alignment (the weekly-window brackets
above/below it don't line up in the same column as they normally would if every row had a
consistent two-bar shape).

User's own hypothesis, which matches the code (see §2): "we don't get the five hour thing probably
because it's at 100% for the weekly and this removes the second part of the row."

## 2. Technical Findings

- `parseAGYUsageOutput` (`internal/usage/agy.go:189`) parses `agy -p "/usage"`'s own CLI text
  output line-by-line, one line per `(group, window)` pair (e.g. `"Claude and GPT models  Weekly
  Limit Remaining  31%  <reset>"`). It has no fallback/synthesis for a missing window — if AGY's
  own CLI doesn't print a "Five Hour Limit Remaining" line for a group, harnez never sees it and
  that `ModelGroup` ends up with just one `QuotaWindow`. This part is scraping AGY's own CLI text
  output (not a versioned API), so if AGY genuinely stops printing the five-hour line once the
  weekly window is fully consumed, harnez has nothing to parse — that would make this an
  **upstream AGY behavior**, not a harnez parsing bug. Not yet confirmed against a live capped
  account; needs live verification of what `agy -p "/usage"` prints when weekly is at 100% before
  concluding this branch (vs. e.g. a transient fetch failure, which would show a fetch error, not a
  quietly missing window — check `QuotaFetchError` handling too).
- Regardless of the above, there is a **confirmed real rendering bug**:
  `formatAllUsageTableLine` (`internal/usage/watch.go:733`) falls back to
  `formatCompactGroupLineWithLabelWidth` (`internal/usage/watch.go:1225`) whenever
  `len(windows) < 2`. That fallback's single-window branch
  (`internal/usage/watch.go:1229-1243`) renders `label + one bar + percent + duration` with **no
  padding to the two-bar column width** other rows in the same box use. When a box mixes one-window
  and two-window rows (exactly this AGY case), the shorter row visibly breaks the grid the user
  sees in the screenshot — this part is unambiguously a harnez layout bug independent of whatever
  AGY's CLI does or doesn't print.

## 3. Suggested Direction (not yet implemented — filed as a bug report only)

1. Confirm the upstream cause: capture `agy -p "/usage"`'s raw text output on an account with a
   fully-consumed weekly Claude/GPT window (or otherwise reproduce/confirm the missing line) before
   assuming it's unfixable upstream data loss vs. a harnez-side parsing gap.
2. Fix the alignment bug regardless of #1's outcome: pad `formatCompactGroupLineWithLabelWidth`'s
   single-window branch (or `formatAllUsageTableLine`'s dispatch) so a row with fewer windows than
   its box-mates still occupies the same column width — e.g. render a blank/placeholder second
   bracket rather than omitting it outright, matching how other "no data" states are already
   handled elsewhere in this renderer (check for prior art before inventing new placeholder
   styling — spec-driven, not hardcoded, per this project's usual convention for `--watch`
   styling).
3. If #1 confirms AGY's CLI simply omits the five-hour line at 100% weekly, decide whether harnez
   should render an explicit "n/a"/"exhausted" placeholder for that window instead of silently
   collapsing to a single-bar row, so the row shape stays predictable even when upstream data is
   genuinely absent.

## 4. Verification Plan (for whoever picks this up)

- Live-verify against a real capped AGY account (or a captured/replayed fixture of AGY's CLI output
  at 100% weekly) that the row renders with consistent column width in `harnez usage --compact` /
  `--watch --compact`.
- Add a unit test fixture: a `ModelGroup` with exactly one `QuotaWindow` alongside other rows with
  two, asserting the rendered lines all share the same bracket-column position (this project has an
  existing pattern of `formatCompactGroupLineWithLabelWidth`-style pure-function tests to extend —
  check `internal/usage/watch_test.go`).
