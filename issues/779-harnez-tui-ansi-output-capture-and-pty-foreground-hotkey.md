# 779 — harnez tui ANSI Output Stream Capture and External PTY Foreground Hotkey (Ctrl+O)

**Status**: Draft
**Priority**: P2 (Medium)
**Severity**: Feature
**Category**: Interactive TUI / Stream Capture & Terminal Control
**Related**: #774, #775, #776, #559

---

## 1. Problem & Motivation
In Phase 1 of `harnez tui`, agent output is captured as simple plaintext. However, modern coding agents emit rich ANSI formatting (syntax highlighted diffs, progress spinners, color-coded status badges). Additionally, when an external CLI agent (`claude` or `agy`) requests interactive human confirmation (e.g., OAuth re-authentication, complex permission prompts), the user needs a direct way to bring the underlying external PTY session to the full foreground.

## 2. Technical Specification & Design

### 1. Full ANSI Stream Capture & Safe Wrapping
- Capture ANSI escape sequences emitted by external harness forwarders.
- Parse SGR color/style sequences and apply safe line wrapping using `internal/readcard` / `internal/rograph` ANSI utilities without breaking color reset state across terminal line breaks.
- Render rich ANSI output inside the Loom chat thread view.

### 2. External PTY Session Foreground Switch (`Ctrl+O`)
- **Keybinding**: `Ctrl+O` (non-conflicting hotkey).
- **Behavior**:
  - Suspends `harnez tui` Loom raw mode and event loop.
  - Connects terminal standard I/O directly to the active underlying external PTY session (`creack/pty`).
  - Allows full, uninhibited native interaction with `claude` or `agy` CLI (e.g. answering interactive confirmation prompts or viewing native full-screen UI).
  - Detaches and resumes `harnez tui` Loom interface upon user escape hotkey or process completion.

## 3. Implementation & Verification Plan
- Implement ANSI stream parser and wrapping layer in `internal/tui/ansi.go`.
- Implement PTY foreground bridge in `internal/tui/pty_bridge.go` using `creack/pty` and `golang.org/x/term`.
- Add unit tests for ANSI wrapping state preservation.
- Test `Ctrl+O` foreground switching with `claude` and `agy` interactive sessions.
