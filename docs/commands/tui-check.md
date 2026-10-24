# Check a Terminal UI Change

Validate a TUI change against its `.ansi` sketch per cell: glyph, foreground, and background. Use this for layout or colour checks; it covers tests and a live tmux view.

## Prerequisites

This workflow assumes `loom` and `tmux` are installed. Stop and report if `command -v loom` fails; do not switch tools. Locate the widget's `.ansi` sketch, when present, under `docs/data/`; see `docs/TUIDesign.md` §“.ansi visual mockups”.

## 1. Sketch first

Before implementation, inspect and validate the sketch:

```sh
loom eval docs/data/<Widget>.ansi
loom measure docs/data/<Widget>.ansi
loom check-box docs/data/<Widget>.ansi
```

Fix row geometry, display widths, and borders in the sketch before changing code.

## 2. Canvas tests per cell

Compare rendered output with the sketch at picks at the start, inside, and past every layout boundary. Assert glyph, FG, and BG for every relevant cell. Pin the colour environment (for example, `LOOMCOLOR=truecolor` in `TestMain`). Locate text by rune column or display width, never byte offset.

## 3. PTY tests

Exercise key paths through a real pseudo-terminal when terminal lifecycle or styled output matters. Cover Shift+arrow in PTY tests; tmux `send-keys` does not deliver those keys to the app. Use the full test tier for PTY coverage; see `docs/TestTiers.md`.

## 4. Live tmux check

The host runs a private socket and fixed terminal size. Use a wait helper, not `sleep`; `timeout 0.8 tail -f /dev/null` waits, while `timeout 0.1 true` does not. Capture rendered colours as text, compare the relevant cells against the sketch, and kill only the private socket you started.

```sh
tmux -L tNNN new-session -d -x 80 -y 24 '<app command>'
timeout 0.8 tail -f /dev/null
tmux -L tNNN capture-pane -pe | perl -pe 's/\e\[([0-9;]*)m/<$1>/g'
tmux -L tNNN kill-server
```

Never print raw ANSI escapes into the agent output; convert them to readable text first.

## 5. Goldens

Render the widget with `loom widgets --show <Name>` and save or update its diffable golden at `docs/progress/<Widget>.ansi`. Use the sketch-versus-capture cell diff when loom ticket 362 provides it.

## 6. Final visual check

Ask the user to inspect a screenshot for font-dependent glyph rendering, such as `▂` versus `🮂`. Treat this as the final look check after cell tests and live comparison.

## Pitfalls

- Green glyph assertions do not prove FG or BG are correct.
- Do not use byte offsets to locate terminal cells.
- Do not use `sleep` or `timeout 0.1 true` as a wait.
- Follow `docs/Bash.md` and `docs/ProcessHygiene.md` for `HTO=0` and process cleanup.
