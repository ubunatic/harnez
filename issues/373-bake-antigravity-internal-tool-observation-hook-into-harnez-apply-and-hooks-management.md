# 373 — Bake Antigravity internal tool observation hook into harnez apply and hooks management

**Status**: Closed — resolved in 0314dfbd
**Priority**: P1 (High)
**Severity**: Minor
**Category**: Feature

---

## Summary

Antigravity client tools (`schedule`, `generate_image`, `ask_question`, `view_file`, `replace_file_content`, etc.) execute internally as host RPC calls without spawning a shell process running `harnez exec`. Consequently, `harnez stats` was unable to track client-level tool usage.

We proved with a canary hook (`scripts/canary-agy-tool-hook.sh` / `.agents/hooks.json`) that Antigravity's `PreToolUse` lifecycle hook with `matcher: "*"` seamlessly intercepts every client tool call, records the invocation into `~/.harnez/tool_catalog.sqlite` under `agent_id: "agy"`, and allows tool execution to proceed unimpeded via `{"decision": "allow"}`.

This ticket tracks baking the Antigravity tool observation hook directly into `harnez` core:
1. Provide a built-in telemetry hook binary/subcommand (`harnez hook agy` or `harnez internal-hook agy-tool`).
2. Manage the `hooks.json` entry via `harnez apply`, `harnez status`, and `harnez diff`.
3. Support global (`~/.gemini/config/hooks.json`) and workspace-local (`.agents/hooks.json`) lifecycle hooks.

---

## Canary & Verification

- Script: `scripts/canary-agy-tool-hook.sh`
- Configuration: `.agents/hooks.json`
- Verification: Simulated tool invocations (`generate_image`) recorded to SQLite and accurately displayed in `harnez stats --tool generate_image` and `harnez stats --agent agy`.

---

## Acceptance Criteria

- [x] `harnez` provides an internal subcommand/handler to process Antigravity `PreToolUse` stdin and write to `tool_catalog.sqlite`.
- [x] `internal/agy/hooks.go` is updated so `BuildHooksDoc()` registers the wildcard tool observer hook.
- [x] `harnez apply` installs/updates the hook in `~/.gemini/config/hooks.json` when `~/.gemini` exists.
- [x] `harnez status` and `harnez diff` report AGY hook status accurately.
- [x] Unit tests in `internal/agy` and `cmd/harnez` verify apply, drift detection, removal, and idempotency.
