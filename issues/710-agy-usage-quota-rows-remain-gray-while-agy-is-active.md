# 710 — AGY usage quota rows remain gray while AGY is active

**Status**: Open
**Priority**: P2 (Medium)
**Severity**: Moderate
**Category**: Bug
**Related**: [649](649-harnez-usage-compact-no-longer-shows-agy-gemini-quota-rows.md), [101](archive/101-usage-keep-stale-agents-visible-until-7d.md)

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
