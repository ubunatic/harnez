# 690 — Usage table display width and empty state

**Status**: Open
**Priority**: P3 (Low)
**Severity**: Minor
**Category**: Bug
**Related**: [689](689-replace-default-usage-boxes-with-a-plain-table.md), [usage table](../internal/usage/usagetable.go)

---

## 1. Problem & Motivation
Two follow-ups from the 689 review:
- `visLen` counts runes, not terminal display width, so a CJK or emoji model or account name
  shifts the table columns. `docs/Go.md` asks for display width in terminal layout.
- With the table view, no other panels and no discovered agents, the frame shows only header and
  footer. It should say that no agent usage was found.

## 2. Done when
Table columns stay aligned for wide characters (test with a CJK model name), and the empty table
view shows a short "no agent usage found" line.
