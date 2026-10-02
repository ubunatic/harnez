# 677 — Show rate-fetch status consistently in usage watch TUI

**Status**: Open
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
