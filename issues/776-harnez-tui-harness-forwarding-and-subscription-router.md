# 776 — harnez tui External Harness Forwarding and Subscription-Aware Model Router

**Status**: In Progress — router, streaming (codex/claude/pi), cancel, `/model`, model picker done; agy streaming, direct API, quota indicators open
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

## 4. Progress (2026-10-16)
- Done: `internal/tui/router` runs turns through `subagent.DriverFor` (shared with `harnez agent`);
  one backend session per provider model, resumed on the next prompt (kept after cancel; a failed
  resume starts a new session with a note). Backend session ids persist per tui `--session` id
  (`sessions.dir` of `spec/tui.yaml`), so a restart resumes them; header shows `[resumed]` /
  `[new session]` (#791).
- Streaming: codex streams messages and activity via `StreamingDriver`. The router picks
  `subagent.StreamDriverFor`, which wraps claude and pi in `ClaudeStreamDriver` /
  `PiStreamDriver`: same args and final `TurnResult`, plus live events from their JSONL
  (claude stream-json: init/result `session_id` -> session, main-thread text -> message,
  `tool_use` -> activity, usage -> tokens; subagent traffic and malformed lines skipped;
  pi `--mode json`: session, completed assistant messages). `harnez agent` keeps
  `DriverFor`, whose claude/pi drivers stay batch. agy has no streaming mode in the driver
  (`--output-format json` is one object), so its messages arrive when the turn ends.
- TUI: chunks reach the chat via `Pane.Post`; status busy/idle/error; Esc/Ctrl+C cancel a running
  turn (`keybindings.cancel_turn`); errors render as `Error:` turns; `/model <spec>` validates
  against `spec/agent.yaml`.
- Cancel kills the provider process group for claude/pi (`providerOutput`), codex via `startProcess`.
- Open: real-CLI check of claude/pi live streaming; text deltas
  (`--include-partial-messages`) not used, chunks are whole messages; agy cancel signals only its leader (agymeter `runChild`); direct API
  clients (`local`, `ollama`, `openrouter`) return a "not supported yet" error turn; quota headroom
  indicators; real-CLI quota check.
- Model picker: Ctrl+T / Alt+M or bare `/model` (Ctrl+M is Enter in terminals); spec/agent.yaml
  models by provider, one row per tier, active and stored-session markers, type-to-filter; Enter
  switches via the `/model` path, also mid-turn (applies to the next turn). Decisions: docs/HarnezTUI.md.
