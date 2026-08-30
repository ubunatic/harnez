# 094 — Usage Watch Controls Overlay and Presets

**Status**: Closed — resolved in `internal/usage/watch.go`, `internal/usage/watch_test.go` (2026-08-30)
**Priority**: P2 (Medium)
**Severity**: Moderate
**Category**: Feature
**Related**: [[023-usage-command-token-quota-tracking]], [[049-running-agent-processes-watch-panel]], [[050-remote-host-flag-and-watch-hotkey]], [[083-usage-tui-self-hiding-auto-discovery]], [[084-aggregate-quota-window-box-assessment]], [[093-usage-tui-layout-planner]]

## Problem

`harnez usage --watch` now has enough interactive panel toggles that the control model feels chaotic. The current implementation exposes direct single-key toggles for agent boxes and auxiliary boxes:

- `C`/`G`/`O` and `1`/`2`/`3` toggle Claude, AGY, and Codex boxes.
- `H`/`4`, `T`/`5`, `P`/`6`, and `L`/`7` toggle History, token rows, Processes, and Load.
- `a` toggles the aggregate All Usage box.
- `A` resets to the default section set.
- `r` toggles remote/local when a host was configured.
- `q`, Ctrl-C, and Esc exit.

Some of these keys are visible in box titles and the footer, but the footer only shows `[a]usage`, `[shift-a]ll`, `[r]emote`, and `[q]uit`. Hidden panel hints show badges such as `[P]` and `[a]`, but they do not explain what action is available, whether a panel was manually hidden, hidden by compact mode, absent because no data exists, or dropped because the terminal is too short. The result is a mostly mnemonic key map that is easy to add to but hard to remember, especially as more panels land.

This is not a request for a full TUI rewrite. The narrow problem is that high-friction, low-frequency display controls are all competing for top-level hotkeys and there is no in-TUI place to ask "what can I do from here?"

## btop Comparison

btop does use direct box toggles, so the lesson is not "avoid hotkeys." Its main loop maps numeric keys to coarse box visibility (`1` CPU, `2` MEM, `3` NET, `4` PROC, and GPU keys when supported), and `p`/`P` cycle presets. The important difference is that these direct keys sit inside a broader discoverability and configuration model:

- Global `Esc`/`m` opens the main menu, `F1`/`?`/`h` opens Help, and `F2`/`o` opens Options.
- The Help menu lists mouse support, menu keys, preset cycling, box toggles, process navigation, process filtering, sorting, tree view, and other actions in one grouped reference.
- The Options menu exposes persistent configuration, including `shown_boxes` and `presets`.
- Presets provide named/coherent layout states rather than requiring users to toggle boxes one by one for common views. Preset 0 is all boxes, and custom presets can define visible boxes and graph style.
- When no boxes are shown, btop renders a recovery screen that lists the keys to show boxes, open the menu, or quit instead of leaving the user with a blank or confusing state.
- Many detailed controls are contextual: process filtering/sorting/tree behavior is handled while the process box is shown, and detailed configuration lives in Options rather than the main footer.

For harnez, the practical takeaway is to keep a small set of direct shortcuts for common display modes, but move the growing list of individual toggles behind a Help/Controls overlay and a few presets/modes.

## Proposal

Add a lightweight controls/help overlay for `harnez usage --watch`, plus a smaller top-level command model:

1. Add `?` as the primary "Controls" overlay key, with `h` as an optional alias if it does not conflict with History. The overlay should be drawn in the existing alternate-screen frame and dismissed with `?`, Esc, `q`, or Enter.
2. Group controls in the overlay by intent:
   - View modes: default, compact, aggregate, agents-only.
   - Panels: Claude, AGY, Codex, History, Processes, Load.
   - Data rows: token velocity/details.
   - Session: remote/local, refresh, quit.
3. Reduce footer pressure. Keep the footer to the most likely controls: `[?]controls`, `[m]ode` or `[p]reset`, `[r]emote`, `[q]uit`. Do not try to list every panel toggle in one line.
4. Introduce two or three preset/mode shortcuts before adding more panel toggles:
   - Default: discovered agent boxes, History, Load, Tokens; Processes follows `--proc`.
   - Compact/Aggregate: All Usage plus Load and Tokens.
   - Agents: individual discovered agent boxes plus Tokens, without History/Load/Processes.
5. Keep existing direct panel toggles initially for compatibility, but treat them as secondary controls documented in the overlay. Avoid adding more top-level panel toggles unless the action is common enough to deserve footer space.
6. Make hidden-state language more explicit when cheap: distinguish "hidden by mode/user" from "not discovered" and "dropped: terminal too short." This can build on issue 093's layout planner instead of blocking on it.

## Acceptance Criteria

- `harnez usage --watch` has an in-TUI controls overlay reachable via `?`.
- The overlay lists all active keyboard commands, grouped by purpose, including existing panel toggles and quit/remote behavior.
- The live footer no longer relies on hidden box title badges as the only way to discover direct toggles.
- At least two coherent mode/preset shortcuts exist for common states, so users do not need to toggle several boxes one by one for default-vs-compact-vs-agents workflows.
- Existing panel toggles continue to work unless a deliberate compatibility note and migration path are documented.
- Tests cover overlay rendering and key dispatch for the overlay open/close path plus at least one preset/mode transition.
- The implementation stays within `harnez usage --watch`; no broad TUI framework migration is required.

## Resolution

Implemented directly in `internal/usage/watch.go` (built on issue 093's `internal/uix` layout planner integration, commit e9c09cb):

- `?` opens/closes an in-alternate-screen "Controls" overlay (`controlsOverlayLines`), grouped by View modes/presets, Panels, Data rows, Session — documents every active key, including the pre-existing direct panel toggles as secondary controls. Dismissed via `?`, Esc, `q`, Ctrl-C, or Enter; while open, all other keys are swallowed rather than mutating panel state behind it.
- Footer reduced to `[?]controls  [m]ode  [r]emote  [q]uit` (was a longer list of individual badges).
- `[m]` cycles three presets in a fixed order: default -> compact -> agents-only -> default (`nextWatchPreset`, `agentsOnlyWatchSections`). `--proc` keeps winning across preset changes, matching issue 093's existing `initialWatchSections` guarantee.
- All existing direct toggles (`C`/`G`/`O`/`1`/`2`/`3`, `H`/`4`, `T`/`5`, `P`/`6`, `L`/`7`, `a`, `A`, `r`, `q`) keep working unchanged via the existing `applyWatchSectionKey`.
- Cheap hidden-state wording improvement: the header's "hidden: [x]" hint now appends "(press ? for controls)" so users are pointed at the overlay instead of having to reverse-engineer the badges. Full hidden-reason categorization (mode/user vs not-discovered vs dropped-too-short) was not built out further — the existing distinction between the header's user/mode-hidden hint and the body's "hidden — terminal too short" drop note was judged sufficient for this ticket's "where cheap" scope, per issue 093 not being blocked on.
- Key dispatch (`dispatchWatchKey`) was extracted into a pure, unit-testable function so overlay open/close and preset cycling don't require a live PTY to test.

Verified: `go test ./...`, `make check` (go vet + tests), and a PTY-driven manual run of `harnez usage --watch` sending `?`/`?`/`m`/`q` confirmed the overlay renders and dismisses, the preset cycle changes the hidden panel set, and the process exits cleanly on `q` with no lingering process.

## Sources Consulted

- btop README, Features/Configurability/command-line options: https://github.com/aristocratos/btop
- btop README default config excerpt for `presets`, `shown_boxes`, `vim_keys`, and `--preset`: https://github.com/aristocratos/btop#configurability
- btop input source for global menu/help/options keys, direct box toggles, and preset cycling: https://raw.githubusercontent.com/aristocratos/btop/main/src/btop_input.cpp
- btop menu source for Help menu entries and Options descriptions of `presets` and `shown_boxes`: https://raw.githubusercontent.com/aristocratos/btop/main/src/btop_menu.cpp
- btop main loop source for no-box recovery screen: https://raw.githubusercontent.com/aristocratos/btop/main/src/btop.cpp

## Files and Commands Inspected

- `git status --short`
- `internal/usage/watch.go`
- `internal/usage/watch_test.go`
- `cmd/harnez/main.go`
- `issues/README.md`
- `issues/049-running-agent-processes-watch-panel.md`
- `issues/050-remote-host-flag-and-watch-hotkey.md`
- `issues/083-usage-tui-self-hiding-auto-discovery.md`
- `issues/093-usage-tui-layout-planner.md`
- `docs/studies/2026-08-18-usage-watch-tui-terminal-rendering-postmortem.md`
- `docs/studies/2026-08-23-usage-history-subcommands-process-panel-and-remote-monitoring.md`
- `rg -n "func RunWatch|applyWatchSectionKey|hidden|controls|Press|keypress|readKey|gridColumns|renderWatchFrame|compactWatchSections|defaultWatchSections|watchSections" internal/usage/watch.go internal/usage/watch_test.go cmd docs issues/README.md issues/0*.md`
- `git clone --depth 1 https://github.com/aristocratos/btop.git`
- `rg -n "shown_boxes|presets|help|Help|Menu|menu|show/hide|hide|boxes|preset|filter|tree|proc_sorting|key|Input" <btop checkout>/README.md <btop checkout>/src <btop checkout>/include <btop checkout>/manpage.md`
