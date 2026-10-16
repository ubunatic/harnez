# 776 — harnez tui External Harness Forwarding and Subscription-Aware Model Router

**Status**: Draft
**Priority**: P1 (High)
**Severity**: Feature
**Category**: Interactive TUI / Model Orchestration
**Related**: #774, #775, #501, #603

---

## 1. Problem & Motivation
End users subscribe to Pro plans (Claude Pro, Google AI Pro / Gemini Advanced, ChatGPT Pro) that provide generous usage limits through official CLI tools (`claude`, `agy`, `codex`). Directly invoking vendor REST APIs bypasses subscription quotas and incurs per-token API charges. `harnez tui` must route prompts intelligently: forwarding to external harness CLIs for subscription-bound models, and using direct Go API clients for models/providers where direct access has no subscription drawdowns (e.g. OpenAI API keys, OpenRouter, local `lmcoder` / Ollama).

## 2. Technical Specification & Design

### Model Routing Matrix
- **Claude Pro (`claude:sonnet`, `claude:opus`, `claude:haiku`)**:
  - Route through `claude` CLI driver (`claude -p` / PTY execution wrapper) to consume Claude Pro subscription quota.
- **Google AI Pro (`agy:flash37`, `agy:flash38`, `agy:sonnet`, `agy:opus`)**:
  - Route through `agy` CLI forwarder driver to consume Google AI Pro / Gemini Advanced subscription quota.
- **ChatGPT Pro / Plus (`codex:luna`, `codex:sol`, `codex:astra`)**:
  - Route through `codex` CLI forwarder driver to consume ChatGPT subscription quota.
- **Direct API / Local Models (`local:lmcoder`, `ollama:*`, `openrouter:*`)**:
  - Route directly via native Go HTTP/API client.

### Interactive Model Switcher
- Model selection popup / shortcut (`Ctrl+M` or `/model`) in `harnez tui`.
- Displays available models from `spec/agent.yaml`, subscription plan indicators, and quota headroom status.
- Switches active backend driver dynamically during a session.

## 3. Implementation & Verification Plan
- Implement `internal/tui/router/` model dispatcher bridging `harnez tui` to `internal/subagent` drivers.
- Implement model switcher modal overlay in Loom TUI.
- Add unit tests verifying prompt routing per provider spec.
- Test subscription quota retention when routing through `claude`, `agy`, and `codex`.
