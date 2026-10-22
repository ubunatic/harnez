# 852 — tui-check skill: validate TUI layout and ANSI colours safely (loom practice)

**Status**: Open
**Priority**: P2 (Medium) — turns a proven loom review practice into a reusable workflow
**Severity**: Minor
**Category**: Workflow
**Related**: 593 (.ansi mockup workflow), docs/TUIDesign.md, 851 (skill ticket shape); loom tickets 359–361 (ColorPicker), loom `.harnez/prompts/developer.md` (per-cell colour rule, 68492ee)

---

/goal A `/tui-check` skill (source `docs/commands/`, mapped in `config.yaml`, installed by `harnez apply`) tells a
host or developer how to validate a TUI change against its `.ansi` design sketch, per cell (glyph, FG, BG), live
and in tests, without hanging or killing the agent's own shell; verified by one real run on a loom widget, or
stop and report when blocked on a user decision.

## Why (loom, 2026-10-22)

In loom tickets 359–361 green tests and a developer's "done" hid three colour bugs (no panel BG; edge glyphs
drawn as solid blocks; a marker leaking its colour along a row and sitting on the wrong BG inside the preview).
All were found only by comparing the screen cell by cell with the sketch. Tests checked glyphs, not FG/BG, and
not picks at layout boundaries.

## Approaches to capture (pick and order in the skill)

1. **Sketch first**: design as `docs/data/*.ansi`; check it with `loom eval` (size, row geometry),
   `loom measure` (display width per line) and `loom check-box` (box borders). Fix geometry before coding.
2. **Canvas tests per cell**: assert glyph, FG and BG per cell against the sketch, with picks at the start,
   inside and past every layout boundary; pin the colour env (e.g. `LOOMCOLOR=truecolor` in `TestMain`);
   locate text by rune column / display width, never byte offset.
3. **PTY tests** for key paths (styled output through a real pseudo-terminal), in the full test tier.
4. **Live tmux check by the host**: private socket (`tmux -L tNNN`), fixed size, wait helper instead of
   `sleep` (`timeout 0.8 tail -f /dev/null` waits 0.8 s; `timeout 0.1 true` does not wait at all, which
   hid the real cause of loom 354), dump colours with
   `capture-pane -pe | perl -pe 's/\e\[([0-9;]*)m/<$1>/g'`, compare per cell; kill only that socket.
5. **Gallery renders**: `loom widgets --show <Name>` and saved `docs/progress/<Widget>.ansi` as a diffable
   golden.
6. **User screenshots** as the final look check for glyph rendering (font-dependent, e.g. `▂`/`🮂`).

## Safety pitfalls to list

- Long runs need `HTO=0`, or harnez kills them after a minute.
- Never `pkill -f` a pattern that matches the agent's own shell; kill the tmux socket or recorded PIDs.
- Shift+arrow keys do not reach the app through tmux `send-keys`; cover them in PTY tests.
- Raw ANSI in agent output can corrupt the terminal; convert escapes to text before printing.

## Decided (user, 2026-10-22)

- The skill recommends `loom` and assumes it is installed; no fallback tooling.
- A sketch-vs-capture cell diff is a loom feature (loom ticket 362), not harnez; the skill uses it once it exists.
