# 649 — harnez usage --compact no longer shows agy/Gemini quota rows

**Status**: Closed — AGY meter collection dropped reset Gemini buckets while Claude/GPT remained active, so it now preserves expired windows for stale-marked compact rows.
**Priority**: P2 (Medium)
**Severity**: Major
**Category**: Usage

---

/goal Make `harnez usage --compact` show current agy Gemini (and Claude/GPT) quota rows again, with a
regression test; or stop and report if the cause is upstream agy/Google auth needing a user action.

## 1. Problem
User report (2026-09-30): the compact view lost the agy/Gemini data. Observed on HEAD `ef2224e0`:

- The All Usage box shows `Claude Code`, a grey (stale) `Claude/GPT` row at 20%, and `OpenAI Codex`.
  No `Gemini` row at all.
- `~/.cache/harnez/quota-cache-agy.json` was last written 2026-09-27, `fetched_at` 2026-09-25. It
  still holds a `Gemini Models` group, so the row is dropped on render or by a staleness rule, not
  only missing from the cache.
- `harnez usage --json` lists `Antigravity (AGY)` with no quota windows.

## 2. Root cause
When recent AGY meter data contained at least one unexpired bucket, the collector replaced
the cached model groups with meter groups after dropping every bucket whose reset time had
passed. If Gemini buckets had expired while a Claude/GPT bucket remained active, the
replacement omitted the Gemini group. The renderer's stale handling was working, and no `agy`
probe was needed to identify the loss. The collector now retains expired meter windows as
stale evidence whenever another bucket remains active; when every meter window has expired,
it still falls back to its normal refresh path. A regression test checks both compact rows,
stale marking, past-reset Gemini windows, and the single-window Claude/GPT case.

## 3. Acceptance
- Root cause named in this ticket.
- Compact view shows a Gemini row (marked stale if the data is old, per 103) instead of dropping it.
- Test covering a cache with both model groups.
