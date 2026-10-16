# 774 — Research and Spec Harnez TUI Interactive Harness

**Status**: In Progress — research done, children #775–#779 filed; #775 shell built; #776 router and streaming in
**Priority**: P1 (High)
**Severity**: Feature
**Category**: Interactive TUI / Agent Harness
**Related**: #439, #607, #620

---

## 1. Problem & Motivation

Users and developers currently interact with Harnez either through batch/dispatch CLI commands (`harnez agent start`, `harnez agent resume`) or through host-native interactive wrappers (`claude`, `agy`, `codex`). While `harnez agent start -i` and `resume -i` provide basic interactive launch capabilities, Harnez lacks a dedicated, full-featured, interactive terminal chat interface (`harnez tui`) comparable to top-tier AI coding agents (`Claude Code CLI`, `Antigravity/AGY CLI`, `Pi Agent CLI`, `OpenAI Codex CLI`).

A native `harnez tui` built with `ubunatic.com/loom` will unify interactive multi-model chat, subscription-aware model routing, background job management, and rich terminal ergonomics under a single Go binary.

## 2. Technical Specification & Architecture

### Overview & Sub-Issues
This issue serves as the overarching Epic tracking the implementation of `harnez tui`. The work is decomposed into five focused, modular issues:
1. **#775**: `harnez tui` Loom-based interactive chat layout with thread display and multiline prompt box.
2. **#776**: `harnez tui` External harness forwarding & subscription-aware model router (Claude Pro, Google AI Pro, ChatGPT Pro).
3. **#777**: `harnez tui` Input box interactive pickers (`@` files, `/` skills) and `Ctrl+G` external `$EDITOR` integration.
4. **#778**: `harnez tui` Background job manager (job panel, live log inspector, task cancellation).
5. **#779**: `harnez tui` ANSI output stream capture, safe wrapping, and external PTY session foreground hotkey (`Ctrl+O`).

### Architecture Highlights
- **Engine**: Built with `ubunatic.com/loom` in Go (>= 1.24). No Python, no JS/TS.
- **Model Routing**: Forwards requests to `claude`, `agy`, and `codex` CLIs to preserve end-user subscription quota (Claude Pro, Google AI Pro / Gemini Advanced, ChatGPT Pro), while utilizing direct API clients for OpenAI / OpenRouter / local models (`lmcoder` / Ollama).
- **Interaction**:
  - Multiline prompt box: `Enter` sends prompt, `Ctrl+J` inserts newline.
  - `Ctrl+G` launches `$EDITOR` (e.g. `nvim`/`vim`/`nano`), restoring text on close.
  - Inline `@` file picker and `/<cmd>` / `$<cmd>` skill pickers.
  - `Ctrl+O` hotkey to temporarily switch the underlying external PTY session to full foreground.
- **Background Jobs**:
  - Visible background task drawer for long-running builds, tests, or subagents.
  - Real-time command log inspection and multi-job cancellation.

## 3. Status (2026-10-14) and Loom Gaps
- #775 shell runs (`cmd/harnez/tui.go`, `internal/tui/`); #776 routes submits to the harness CLIs (2026-10-16).
- harnez pins loom v0.3.8 (`Pane.Post`, `Pane.Suspend`).
- Loom gaps, fixed in loom, not worked around here:

| Loom | Gap | Blocks |
|---|---|---|
| 319 | `Pane.Suspend/Resume` to hand the tty to a child | #777 Ctrl+G, #779 Ctrl+O |
| 320 | `Pane.Post` to mutate widgets on the event loop | #776 streaming, #778 job logs |
| 321 | Opt-in kitty keyboard protocol (Shift+Enter, Ctrl+J) | #775 newline key |

- Order: loom 320 → bump loom → #776; loom 319 → #777, #779; #778 after 320.

## 4. Implementation & Verification Plan

- Complete research study in `docs/studies/2026-10-10-harnez-tui-interactive-harness-research.md`.
- File modular child issues #775, #776, #777, #778, #779 using `harnez issues new`.
- Update `docs/Roadmap.md` to sequence `harnez tui` epic and child tasks.
- Verify ticket index, linting (`harnez issues lint`), and project build/test suite (`make check`).
