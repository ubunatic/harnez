# 777 — harnez tui Interactive Input Pickers (@ files, / skills) and Ctrl+G Editor Integration

**Status**: Draft
**Priority**: P2 (Medium)
**Severity**: Feature
**Category**: Interactive TUI / UX
**Related**: #774, #775, #620

---

## 1. Problem & Motivation
When authoring prompts in an interactive coding agent, developers frequently need to attach repository files, select specialized skills/commands, or write extensive multi-paragraph prompts. Standard single-line terminal inputs make this tedious. `harnez tui` needs fluid inline completion pickers (`@` for files, `/` or `$` for skills) and seamless external `$EDITOR` takeover via `Ctrl+G`.

## 2. Technical Specification & Design

### Inline Popover Pickers
1. **File Picker (`@`)**:
   - Typing `@` triggers an inline file search overlay filtered by fuzzy matching against the repo file index.
   - Selecting a file inserts `@path/to/file` into the prompt buffer.
2. **Skill / Command Picker (`/` or `$`)**:
   - Typing `/` or `$` triggers an inline skill menu listing installed Harnez skills (`/sprint`, `/lean-sprint`, `/review`, `/harnez-status`, etc.).
   - Selecting a skill inserts the command slug and parameters into the prompt.

### External Editor Takeover (`Ctrl+G`)
1. **Shortcut Handling**:
   - Pressing `Ctrl+G` suspends Loom TUI raw mode.
   - Opens the temporary prompt buffer in the user's `$EDITOR` (falling back to `nvim`, `vim`, or `nano`).
2. **Buffer Synchronization**:
   - Upon closing the editor (exit code 0), `harnez tui` reads the file content back into the multiline input box and restores TUI raw terminal mode.

## 3. Implementation & Verification Plan
- Implement fuzzy file matcher using existing Harnez index / file discovery helpers.
- Implement Loom popover widget for `@` and `/` inline completions.
- Implement `$EDITOR` process spawn helper in `internal/tui/editor.go` with terminal suspension/restoration.
- Add unit tests for trigger parsing (`@`, `/`, `$`), buffer insertion, and editor file roundtrip.
