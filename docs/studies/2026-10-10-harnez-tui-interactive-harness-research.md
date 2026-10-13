# 2026-10-10 — Harnez TUI Interactive Harness Research & Architecture Study

**Author**: Jules (Software Engineer)
**Status**: Completed Research & Specification
**Scope**: `harnez tui` interactive chat interface design, harness forwarding, subscription routing, and TUI interaction patterns.

---

## 1. Executive Summary

`harnez tui` is a planned interactive, terminal-based chat harness for `harnez` built on top of the `ubunatic.com/loom` Go module. It aims to provide a unified, developer-friendly chat interface comparable to top-tier AI coding agents (`Claude Code CLI`, `Antigravity/AGY CLI`, `Pi Agent CLI`, `OpenAI Codex CLI`, `Aider`, `OpenCode`), while leveraging Harnez's cross-harness subagent orchestration, telemetry, and quota management.

Key architectural goals for `harnez tui`:
1. **Loom-based TUI**: Built natively in Go using `ubunatic.com/loom` for high-performance, non-flickering, terminal-native user experience.
2. **Subscription-Aware Model Routing**:
   - Forwarding via external CLI wrappers (`claude`, `agy`, `codex`) to preserve end-user subscription quota (Claude Pro, Google AI Pro / Gemini Advanced, ChatGPT Pro).
   - Direct API client usage for models/providers where direct API access has no subscription drawdowns (e.g. OpenAI API, OpenRouter, local models via `lmcoder` / Ollama).
3. **Phased Output Capture**:
   - Phase 1: Clean plaintext message extraction and structured chat thread rendering.
   - Phase 2: Full ANSI stream capture, safe text wrapping, and rich terminal formatting.
4. **Ergonomic Keyboard Interaction & Pickers**:
   - Multiline prompt box: `Enter` to send, `Ctrl+J` for newline.
   - `Ctrl+G` to launch external `$EDITOR` (e.g. `nvim`, `vim`, `nano`), capturing prompt text on close.
   - Inline `@` file picker and `/<cmd>` / `$<cmd>` skill/command pickers.
   - Foreground session hotkey (`Ctrl+O`) to temporarily bring the underlying external PTY CLI session into full foreground view.
5. **Interactive Background Job Manager**:
   - Real-time job listing overlay.
   - Live command output streaming/inspection.
   - Multi-job cancellation and background task controls.

---

## 2. Research: External Coding Harnesses & Interaction Patterns

A comparative survey of modern CLI coding harnesses reveals key usability features and technical implementations:

| Harness / CLI | Terminal Engine | Interaction Highlights | Subscription / API Access Model | Subagent / Job Handling |
|---|---|---|---|---|
| **Claude Code CLI** | Custom TS/Node PTY | Compact chat thread, tool approval prompts, `@` file inclusion, `/` slash commands, `Ctrl+G` external editor. | Claude Pro / Max OAuth subscription quota or Anthropic API key. | Background tool calls with notification wakeups. |
| **Antigravity (AGY)** | Go / PTY runner | Slash menu, task monitor, background process management, multi-agent dispatch. | Google AI Pro / Gemini Advanced OAuth subscription quota. | Asynchronous background tasks (`manage_task`). |
| **OpenAI Codex** | Rust / C++ core | Session rollouts (`.jsonl`), `exec resume`, MCP tools, thinking levels. | ChatGPT Plus / Pro subscription OAuth or OpenAI API key. | `spawn_agent` subagents with stdio/MCP communication. |
| **Pi Coding Agent** | TS / Node CLI | Minimal JSON/interactive mode, extension TypeScript hooks, skills registry. | Configured provider API keys or local endpoints. | Subprocess management; session transcript store. |
| **Aider** | Python / Prompt Toolkit | Inline file add (`/add`), git commit auto-generation, voice-to-text, diff previews. | OpenRouter / Anthropic / OpenAI API keys. | Synchronous model execution. |

### Key Takeaways for `harnez tui`:
1. **Developers love `$EDITOR` integration (`Ctrl+G`)**: Long prompts, complex refactoring instructions, and code snippets are cumbersome in single-line or basic multiline inputs. TUI tools that suspend raw mode and open `$EDITOR` provide superior prompt editing.
2. **Inline Autocompletion (`@` files, `/` skills)**: Context file attachment and command dispatch must be fluid inline operations without leaving the chat thread.
3. **Background Job Visibility**: Developers need to run test suites, builds, or subagents in background jobs without blocking the conversation thread, while being able to enter and inspect output streams on demand.

---

## 3. Subscription Access vs Direct API Access Model Routing

End-user subscriptions (Claude Pro, Google AI Pro, ChatGPT Pro) offer high usage limits for a fixed monthly cost, but their quota is accessible **only** via their respective official CLI tools / OAuth authentication flows. Direct API access (e.g. calling `https://api.anthropic.com` or `https://generativelanguage.googleapis.com`) uses separate pay-as-you-go API balances and ignores subscription plans.

### Routing Policy Matrix in `harnez tui`:

```
                             ┌─────────────────────────────────────────┐
                             │               harnez tui                │
                             └────────────────────┬────────────────────┘
                                                  │
                                       Model Selection Request
                                                  │
                        ┌─────────────────────────┴─────────────────────────┐
                        ▼                                                   ▼
         Subscription-Bound Providers                        Direct API / Local Providers
     (Claude Pro, Google AI Pro, ChatGPT Pro)                (OpenAI API, OpenRouter, Ollama)
                        │                                                   │
                        ▼                                                   ▼
          External CLI Forwarder Driver                       Direct Go Model Client
        ┌───────────────────────────────┐                   ┌───────────────────────────────┐
        │ claude -p / agy / codex exec  │                   │ native HTTP / API client      │
        └───────────────┬───────────────┘                   └───────────────┬───────────────┘
                        │                                                   │
                        ▼                                                   ▼
          Subscription Quota Preserved                         Pay-as-you-go / Local Free
```

1. **Claude Models (`claude:sonnet`, `claude:opus`, `claude:haiku`)**:
   - Access Route: `claude` CLI forwarder driver (`claude -p` / PTY wrapper).
   - Subscription: Consumes **Claude Pro / Max** plan quota.
2. **Gemini / Google Models (`agy:flash37`, `agy:flash38`, `agy:sonnet`, `agy:opus`)**:
   - Access Route: `agy` CLI forwarder driver.
   - Subscription: Consumes **Google AI Pro / Gemini Advanced** plan quota.
3. **OpenAI / Codex Models (`codex:luna`, `codex:sol`, `codex:astra`)**:
   - Access Route: `codex` CLI forwarder driver or direct OpenAI API.
   - Subscription: Consumes **ChatGPT Plus / Pro** subscription quota via CLI wrapper.
4. **Direct / Local Models (`local:lmcoder`, `ollama:*`, `openrouter:*`)**:
   - Access Route: Direct Go API client or local HTTP endpoint.
   - Subscription: Pay-as-you-go API key or zero-cost local GPU execution.

---

## 4. TUI Component Architecture (Loom Engine)

`ubunatic.com/loom` will drive the interactive terminal rendering and event loop:

```
┌─────────────────────────────────────────────────────────────────────────────┐
│ harnez tui — [Model: claude:sonnet] [Session: vibrant-falcon] [Tokens: 24k]  │
├─────────────────────────────────────────────────────────────────────────────┤
│ User: Please refactor internal/statusline to reduce latency.                │
│                                                                             │
│ Agent (claude:sonnet):                                                      │
│ I have inspected `internal/statusline` and identified two blocking syscalls.│
│ Here is the planned optimization:                                           │
│  1. Cache CWD path lookups.                                                 │
│  2. Run Git status checks asynchronously.                                   │
│                                                                             │
│ [Tool Call: harnez exec "make test"] -> Completed (exit 0)                  │
├─────────────────────────────────────────────────────────────────────────────┤
│ BG Jobs: [1] make test (Running)  [2] harnez agent (Done)                   │
├─────────────────────────────────────────────────────────────────────────────┤
│ > Replace blocking CWD call with cached getter @internal/statusline/cwd.go  │
│                                                                             │
│ [Enter] Send  [Ctrl+J] Newline  [Ctrl+G] Editor  [@] File  [/] Skill  [Ctrl+O] PTY│
└─────────────────────────────────────────────────────────────────────────────┘
```

### Component Details:
- **Header / Status Bar**: Displays active model, session ID, context window utilization, and current status indicator.
- **Chat Thread View**: Scrollable message history containing user prompts, agent replies, tool call blocks, and compaction markers.
- **Background Jobs Drawer**: Collapsible panel showing active/completed background tasks with hotkey toggles (`[1..N]`).
- **Multiline Input Box**: Textarea supporting cursor navigation, multiline editing (`Ctrl+J`), and interactive popover overlays (`@` file picker, `/` skill picker).
- **External PTY Bridge (`Ctrl+O`)**: Suspends TUI raw mode and attaches stdout/stdin directly to the underlying `claude` or `agy` process, enabling full native CLI control when needed, and returning to Loom TUI upon detachment or exit.

---

## 5. Technical Stack & Invariants

- **Language**: Pure Go (>= 1.24). No Python, no JS/TS.
- **TUI Framework**: `ubunatic.com/loom`.
- **PTY Handling**: `github.com/creack/pty` or `golang.org/x/term` for raw terminal management and size resynchronization.
- **IPC / Process Management**: `internal/subagent` and `internal/exec` for child process supervision and signals.
- **Testing**: Native Go tests (`go test ./...`), `.ansi` layout mockups, and terminal snapshot verification.
