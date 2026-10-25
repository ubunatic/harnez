# 923 — usage: btop-style coloured hotkey char in box titles via loom

**Status**: Open
**Priority**: P3 (Low)
**Severity**: Minor
**Category**: Feature
**Related**: loom 385 (prerequisite), 922 (`[L] Load`), 132 (superscript hotkeys), 917, Epic 921

---

/goal `harnez usage --watch` box titles show their toggle key btop-style: the key letter inside the title word in a
semantic key colour (red `m` in `menu`), or a coloured prefix badge when the letter is not in the title; btop's
frame style is not adopted. Verified per cell (glyph, FG) against the `.ansi` sketch and live in tmux; or stop
and report. Circuit breaker: on a loop or no progress, clear the goal and write a failure report here.

## Notes

- Owner request (2026-10-25), reference: btop top bar `¹cpu`, `menu`, `preset` with red key letters.
- Blocked on loom 385 (Box title cannot style a single char today, `loom/box.go:227-238`).
- The watch view does not draw through loom yet (own `renderWBox`, `internal/usage/watch_layout.go:84`), so
  this either waits for the migration (see the loom-usage note in this session, park as its own ticket) or
  adds the colour in `renderWBox` and moves later. Decide when picked up.
- Key colour is a semantic name in `spec/` colours, not a Go default; key per action from `spec/actions.yaml`.
