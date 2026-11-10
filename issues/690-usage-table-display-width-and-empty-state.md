# 690 — Usage table display width and empty state

**Status**: Backlog — grooming pass 843 M4 (2026-10-22); promote when needed
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

## Jules audit (2026-11-10, af9688e0)

Still valid; the report's `partly done` does not hold. `visLen` still counts runes
(`internal/usage/watch_layout.go:40`: `utf8.RuneCountInString(stripANSI(s))`), so the width part is not
done. Empty state: `internal/usage/watch_frame.go:383-389` prints "no agent usage detected …" only when
`len(discovered) == 0 && len(summary.Agents) > 0`; `usageTableLines` returns nil for no rows
(`internal/usage/usagetable.go:45-46`) and the table branch (`watch_frame.go:392`) then adds nothing.
Check whether the existing line already covers the table-only case before adding a second message.
Report: `reports/jules/690.md` (github#152).

## Jules runs

| kind | session name | Jules ID | PR | state |
| --- | --- | --- | --- | --- |
| implement | jules-impl-690 | 7897340296191755096 | github#160 | done: kept, host fixed truncation and added the CJK table test |

Host check: Jules switched `visLen` to `runewidth.StringWidth` (already a dependency) and showed the
existing "no agent usage detected" line in the empty table view (`watch_frame.go:383`). Its
`truncateVisible` still overshot by one column when a wide rune straddled the limit; the host fixed that
and added `TestTruncateVisibleKeepsDisplayWidth` plus `TestUsageTableAlignsCJKModelName` (the done-when's
CJK model name). All three tests fail on main and pass with the fix; `make test-q1` passed.
