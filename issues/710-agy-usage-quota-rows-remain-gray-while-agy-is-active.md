# 710 — AGY usage quota rows remain gray while AGY is active

**Status**: Open — agy statusline payloads carry quota in a shape passiveWindows drops (24 drops live)
**Priority**: P2 (Medium)
**Severity**: Moderate
**Category**: Bug
**Related**: 714, [649](649-harnez-usage-compact-no-longer-shows-agy-gemini-quota-rows.md), [101](archive/101-usage-keep-stale-agents-visible-until-7d.md)

---

## 1. Problem & Motivation
User report (2026-10-05): `Agy-Gemini` and `Agy-Claude` usage often remain grayed out in the `harnez usage` TUI even while AGY is active. The user cannot tell whether the displayed quota data is fresh.

/goal Investigate and resolve persistent gray AGY quota rows while AGY is active, and make the freshness of displayed data clear; verify the resulting behavior, or stop and report when blocked on a user decision, required authentication action, or denied permission.

## 2. Technical Specification / Findings
The cause is unconfirmed. Check whether quota refresh is failing, stale fallback is being shown correctly, or the TUI incorrectly marks fresh data as stale. AGY process activity alone does not establish quota freshness. Related closed issues addressed missing rows and old meter readings; recheck live code and recent commits before implementing.

## 3. Implementation & Verification Plan
- Reproduce with active AGY and inspect the actual quota timestamps and refresh outcome for both model groups.
- Correct any confirmed refresh or stale-rendering defect and add meaningful regression coverage.
- Ensure the TUI communicates when quota data was last refreshed and why stale fallback remains, so color alone does not leave freshness ambiguous.
- Verify fresh and stale states for both rows; record the cause and any remaining upstream limitation.

## 4. Recurrence (2026-10-22)

User reports rows "often" turning gray in the live `harnez usage` view, together with the
`unavailable (signal: killed)` errors tracked in 714. Likely the same root cause (collector
killed → stale snapshot → gray); fix 714 first and re-check this one.

## Live check (2026-10-22, lean sprint): not reproducible today

`harnez usage --raw` showed fresh Claude, Codex and both AGY pools. Gray comes from either a `QuotaFetchError`
(`internal/usage/types.go:160`) or snapshot age (`internal/usage/freshness.go:5`); the display does not say
which. Way forward: show the reason on a gray row (fetch error text or age), keep the last good quota on a
failed refresh, test fresh/stale/failed rendering. Then park until the next live occurrence, which the reason
text will explain.

## Live data (2026-10-23, after 714 M5)

`harnez server status --json` shows `passive_dropped.agy: 24` (since server start) while the user's AGY
sessions display quota in their status line: `45k/1049k (95% cached) · 5h 31% left · argon-wtu-weekly 43.3%
left · weekly 19.6% left`. harnez code does not render that text (no "% left" in cmd/ or internal/), so agy
has the quota, but every payload sent to `harnez statusline --agent agy` is dropped: `passiveWindows`
(`internal/usage/passive.go:108-120`) reads only `quota.<model>.remainingFraction`, so agy most likely sends
quota in another shape. Next: capture one real payload (e.g. an env-gated dump of stdin in
`harnez statusline --agent agy` to the state dir), map its fields, and test with the captured JSON.
