# 657 — usage --compact drops claude and codex: writers store label window keys, cross-provider observation links, codex limits not collected

**Status**: Closed — Compact usage now projects normalized Claude, Codex, and AGY quota windows; migration repairs cross-provider/missing observation links, preserves compact errors, and Codex limits continue from the Wham usage endpoint.
**Priority**: P1 (High)
**Severity**: Major
**Category**: Bug / Regression
**Related**: 650-655 (usage collection architecture), docs/UsageCollection.md

---

## 1. Problem

After 655, `harnez usage --compact` shows only the AGY pools (Gemini, Claude/GPT). The Claude Code
and Codex providers are missing. Before 655 they showed as broken `tokens 0%` rows.
Live DB (`~/.local/share/harnez/telemetry.sqlite`, 2026-09-30 ~21:15 CEST) shows three defects:

1. **Writers still store display labels as `window_key`**: new claude rows use `Weekly (7-day)`,
   `Session (5-hour)`; agy uses `Weekly Limit Remaining`. 655 migrated only old rows; readers now
   expose normalized keys only, so the fresh claude rows are dropped.
2. **Cross-provider observation links**: `quota_windows.observation_id` points at an observation of
   another provider (e.g. window provider `claude` -> observation provider `codex`/`agy`). Suspect
   749aef39 "preserve observation links on upsert".
3. **Codex quota windows are never collected**: codex rows are only `tokens/cumulative`; no
   5h/weekly used fractions (true before 650 too).

## 2. Acceptance

- All writers (registry, collect-all, statusline, turn-capture) normalize keys at write time
  (`five_hour`, `weekly`, ...), one shared normalizer; `window_name` keeps the label.
- Each window links to an observation of its own provider; migrate/repair existing rows additively.
- Codex 5h/weekly used fractions are collected (find the real source: codex rate-limit data in its
  session/rollout files or its statusline) and stored.
- `harnez usage --compact` shows Claude, Codex and AGY rows with percentages and reset times;
  no `tokens 0%` pseudo rows.
- Regression test: fixture with label-keyed input from each writer yields normalized keys and a
  compact render that includes claude and codex.

/goal Compact usage shows Claude, Codex and AGY limits from the store again, verified live on the real
DB, or stop and report when blocked on a user decision or denied permission.
