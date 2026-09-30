# 649 — harnez usage --compact no longer shows agy/Gemini quota rows

**Status**: Open
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

## 2. Suspects (unverified)
- The agy collector stopped refreshing (`agy -p "/usage"` failing, reauth — see 112, 104).
- A staleness or exhaustion rule from 603 / 515 / 560 hides old Gemini windows but keeps Claude/GPT.

## 3. Acceptance
- Root cause named in this ticket.
- Compact view shows a Gemini row (marked stale if the data is old, per 103) instead of dropping it.
- Test covering a cache with both model groups.
