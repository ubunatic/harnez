# 779 — harnez tui ANSI Output Stream Capture and External PTY Foreground Hotkey (Ctrl+O)

**Status**: Done (2026-10-16)
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

## 3. Implementation (2026-10-16)
Turns run non-interactively (`claude -p`, `codex exec`, ...), so there is no live PTY to
foreground; the design changed from a `creack/pty` bridge to a native resume.

- `Ctrl+O` (`keybindings.open_native`, was `toggle_pty`): `router.Native` builds the argv with
  `subagent.InteractiveArgv` (the `agent resume -i` builder): `claude --resume <id> --model <m>`,
  `codex resume <id> -m <m> -C <dir>`, `agy --conversation <id> --model <m> [--effort]`; pi has
  no interactive CLI → note. `Pane.Suspend` runs it on /dev/tty (`runOnTTY`, shared with Ctrl+G;
  agy gets `subagent.InteractiveEnv`). Busy turn, no session, unsupported → notes
  (`native.labels` in spec/tui.yaml). On return: note "returned from <provider>", header and
  stored session id unchanged (a forked native id is not picked up).
- ANSI (`internal/tui/ansi.go`): chat content keeps SGR only; CSI cursor/erase/modes, OSC
  (title, 8, 52), DCS/APC/PM/SOS, other ESC and C1 controls, CR and C0 are dropped; an
  unterminated string drops to end of line. Wrapping uses loom display width and clusters;
  styled lines end with a reset and the next line reopens the style. Tabs expand to 8 columns.
  Job logs keep SGR too (`jobs.Ring`) and pass the same sanitizer.
- Tests: `internal/tui/{native,ansi}_test.go`, `internal/tui/router/native_test.go`; PTY
  check with a fake `claude` on PATH (argv, return note, store unchanged) and a colored job log.
