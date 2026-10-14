# 738 — harnez tui Loom-based Chat Layout with Thread Display and Multiline Input Box

**Status**: In Progress — shell, chat, prompt and spec keybindings done (e1fbce7); `.ansi` design and loom bump open
**Priority**: P1 (High)
**Severity**: Feature
**Category**: Interactive TUI / Loom Integration
**Related**: #737, #607

---

## 1. Problem & Motivation
`harnez` currently relies on line-oriented batch commands or raw provider terminal sessions. To build a unified `harnez tui`, we need a core TUI engine and view hierarchy built on `ubunatic.com/loom`. This foundational issue provides the main TUI framework, header status bar, scrollable chat thread view, and multiline prompt box.

## 2. Technical Specification & Design

### Components & Layout
1. **Header / Status Line**:
   - Displays active model (`claude:sonnet`, `agy:flash37`, `codex:luna`, etc.).
   - Displays current session ID, token usage / context window percentage, and active model status.
2. **Chat Thread View**:
   - Scrollable history of turns (user prompts, assistant responses, tool execution blocks).
   - Clean plaintext message extraction and formatting in initial phase.
3. **Multiline Input Box**:
   - Multiline prompt entry located below chat thread.
   - `Enter` sends prompt to active agent/model.
   - `Ctrl+J` inserts a newline character into the prompt buffer.
     Terminals send Ctrl+J and Enter as the same byte, so `Alt+Enter` and a trailing `\` + `Enter` also insert a newline.
   - Standard cursor movement, editing, and backspace handling.

### Tech Stack & Dependencies
- Go `ubunatic.com/loom` TUI primitives.
- `golang.org/x/term` for terminal raw mode and size detection.

## 3. Implementation & Verification Plan
- Implement `cmd/harnez/tui.go` and `internal/tui/` package using `ubunatic.com/loom`.
- Wire `harnez tui` subcommand into Cobra CLI tree.
- Create unit tests for prompt buffer manipulation, `Ctrl+J` newline insertion, and chat thread scrolling.
- Create `.ansi` layout spec in `docs/data/tui-chat-design-001.ansi` and verify rendering with `loom eval` or visual test.

## 4. Progress (2026-10-14)
- Done: `harnez tui` command, header, scrollable chat, multiline prompt, history,
  paste, spec-driven keys, `--render` mode, tests.
- Newline: Alt+Enter and trailing `\` + Enter work; Ctrl+J/Shift+Enter wait on loom 307.
- Done: `docs/data/tui-chat-design-001.ansi` (80x24) passes `loom eval`/`measure`/`check-box`; loom bumped v0.2.18 → v0.3.7.
- Open: Ctrl+J/Shift+Enter newline (loom 307).
