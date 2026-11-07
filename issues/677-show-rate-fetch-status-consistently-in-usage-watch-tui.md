# 677 — Show rate-fetch status consistently in usage watch TUI

**Status**: Backlog — grooming pass 843 M4 (2026-10-22); promote when needed
**Priority**: P2 (Medium)
**Severity**: Moderate
**Category**: Bug
**Related**:

---

/goal Make rate-fetch cooldown indicators consistent and show each source's last fetch outcome, or stop and report when blocked on a user decision or denied permission.

## 1. Problem & Motivation
In `harnez usage --watch --compact`, pressing `!` shows cooldown spinners, but the number displayed varies (sometimes three of four, none, or one). The display also does not reveal whether the last fetch succeeded, failed, or used a fallback, and it does not explain when fetching is disabled.

## 2. Technical Specification / Findings
Show a stable indicator for every applicable rate source during cooldown. Color each spinner by its latest outcome: red for a failed fetch, green for a successful fetch, gray for a successful fetch with no rate change, and yellow when the normal fetch failed but a fallback (such as `agy -p "/usage"`) supplied the result. Show a red `!` when fetching is deactivated, with the reason available in the TUI.

## 3. Implementation & Verification Plan
Verify all applicable source indicators are consistently present and animate during cooldown. Exercise successful changed and unchanged results, fetch failure, fallback success, and deactivated fetching; confirm the color and disabled marker match each state.

## Audit 2026-11-08 (Jules)

Jules audit (GitHub PR 142, `reports/jules/677.md`, checked against d8c61da; 4/4 references hold) said `obsolete`, but its evidence shows the feature is missing, not superseded, so the host reads it as still valid. Today `!` toggles a text debug overlay (`internal/usage/watch_keys.go:151`, `watch_debug.go:47`); no cooldown spinner per rate source exists (spinners only in `watch_splash.go:276`); `watch_usage_box.go:70` returns a static `!` for unavailable/unknown, not a deactivated-fetch state. Owner decision needed: keep the spinner idea or close in favour of the debug overlay.
