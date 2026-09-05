# 172 — AGY "Claude/GPT" Row Drops Its Second Window at 100%, Breaking All Usage Grid Alignment

**Status**: Blocked — rendering bug (§2/§3.2) fixed; upstream AGY-CLI question (§1/§3.1) and
n/a-placeholder design question (§3.3) remain open pending live account verification
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

## 5. Resolution Note (rendering bug only)

The §2/§3.2 alignment bug is fixed: `formatAllUsageTableLine` (`internal/usage/watch.go:733`) now
dispatches a single-window row to a new `formatAllUsageSingleWindowLine` helper (same file,
directly below the two-window branch) that reuses the two-window branch's
`prefix + bar + midStr` layout and renders the missing second bar as a blank placeholder bracket
(`blankBarPlaceholder()`, `"[    ]"`) rather than omitting it or rendering a misleading real 0%
gauge. Covered by `TestAllUsageBoxSingleWindowRowAligns` in `internal/usage/watch_test.go`, which
asserts the single-window row's second bracket lands in the same column as its two-window box-mate
and is the blank placeholder, not a real bar.

Still open and NOT addressed by this fix:
- §1/§3.1: whether AGY's own CLI genuinely omits the five-hour line at 100% weekly (upstream
  behavior) vs. a harnez-side parsing gap — needs live-account or captured-fixture verification.
- §3.3: whether harnez should render an explicit "n/a"/"exhausted" placeholder (distinct from the
  blank alignment placeholder added here) when a window is genuinely and permanently absent upstream.

---

## 6. Implementation Plan (remaining open items only)

The §2/§3.2 rendering bug is fixed and tested (`formatAllUsageSingleWindowLineWithMidWidth`,
`internal/usage/watch.go:877`; `TestAllUsageBoxSingleWindowRowAligns`). Nothing below re-opens that.
What remains is §1/§3.1 (upstream cause) and §3.3 (whether an explicit "n/a" placeholder is
warranted) — and §3.3 is genuinely undecidable until §3.1 is answered, so the plan is sequenced, not
parallel.

### Step 1 — Answer §3.1 with captured evidence, not reasoning (blocking)

This cannot be resolved by reading `parseAGYUsageOutput` again; it needs the bytes AGY actually
prints. Two acceptance paths, either is sufficient:

- **(a) Live capture.** Next time an AGY Claude/GPT weekly window is at or near 100%, run
  `agy -p "/usage" > /tmp/agy-usage-capped.txt` and commit the sanitized text as a fixture under
  the existing AGY test fixture convention used by `internal/usage/agy_test.go`. Then assert against
  it directly.
- **(b) Opportunistic capture.** Add a debug escape hatch — an env var (e.g.
  `HARNEZ_AGY_DUMP=<path>`) that makes the AGY collector write the raw `agy -p "/usage"` bytes to a
  file before parsing. Roughly 5 lines in `internal/usage/agy.go` near the exec call (~line 450).
  This makes the capture possible whenever the condition happens to occur, instead of requiring the
  user to notice and act during the window.

Prefer (b) if the capped state is not currently reproducible on demand — the whole ticket has been
stalled on "needs live verification", and (b) converts that from a lucky-timing problem into a
passive one.

While capturing, also record `QuotaFetchError`: a transient fetch failure and a genuinely-absent
line are indistinguishable from the rendered row alone, and §2 explicitly flags this as an
alternative hypothesis that must be ruled out.

### Step 2 — Branch on the answer

- **If AGY genuinely omits the five-hour line at 100% weekly** (upstream behaviour): implement §3.3.
  Add a distinct placeholder — semantically "no data upstream", not the blank alignment bracket —
  rendered from the indicators spec (`internal/usage/indicatorsspec.go`), not hardcoded in
  `watch.go`, per this project's spec-driven `--watch` styling convention. The distinction matters:
  the current blank bracket means "this row has fewer windows"; the new state means "this window
  exists but upstream told us nothing". Extend `TestAllUsageBoxSingleWindowRowAligns` with a case
  asserting the two render differently while occupying identical column width.
- **If harnez's parser is dropping a line AGY does print**: this becomes a plain parser bug in
  `parseAGYUsageOutput` (`internal/usage/agy.go:189`) — most likely the `len(fields) < 4` or the
  `strconv.ParseFloat` guard silently skipping a row whose percentage renders as something other
  than a bare number at the 0%-remaining boundary (e.g. `"0"`, `"<1"`, `"—"`). Fix with the captured
  fixture as the regression test. §3.3 then does not apply at all.

### Key Decisions / Tradeoffs

- **Do not implement §3.3 speculatively.** Building an "exhausted/n-a" placeholder before knowing
  whether the window is actually absent risks shipping a permanent visual state for a transient
  fetch failure — strictly worse than today's honest blank bracket.
- **Debug dump over live-watching**: a 5-line env-gated capture hook is cheaper than keeping this
  ticket blocked indefinitely on catching a rare account state by hand.

### Risks / Open Questions

- The capped state may never recur on this account, leaving §3.1 permanently unanswerable. If Step 1
  yields nothing after a reasonable window, the honest close is: mark §3.1/§3.3 **Won't Fix —
  unreproducible**, and close the ticket on the already-shipped alignment fix. That is a legitimate
  outcome, not a failure.
- A fixture captured from a live account needs scrubbing (reset timestamps are fine; account
  identifiers are not).

### Scope

**Small.** Step 1 is ~5 lines plus a fixture; Step 2 is either a small parser fix or a small spec-
driven placeholder. The cost here is calendar time waiting for the upstream state, not engineering.
