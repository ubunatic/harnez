# 206 — Confirm `tool_input.command` field name against a live Codex PreToolUse call

**Status**: Draft
**Priority**: P2 (Medium)
**Severity**: Major
**Category**: Bug risk / Verification
**Related**: [[200-codex-native-hooks-preTooluse-wiring]] (introduced
`cmd/harnez/codexhooks.go`'s `codexPreToolUseInput`), [[199-research-codex-hook-surface-for-transparent-exec-distill]]
(source of the schema assumption)

## Problem

`codexPreToolUseInput` (`cmd/harnez/codexhooks.go`) decodes the Bash
command string from `tool_input.command`, by analogy with Claude Code's
own PreToolUse schema. Ticket 199's research never observed a real,
live Codex PreToolUse invocation — the field name is inferred from
binary-string similarity to Claude Code's schema, not confirmed.

If the real field name differs, `runCodexHooksHook` silently treats every
command as empty and always emits `{"permissionDecision":"allow"}` with
no rewrite — i.e. the whole `harnez codex-hook` handshake becomes a
silent no-op. No test currently exercises this against real Codex output,
only synthetic JSON matching our own assumption.

## Scope

- Trigger a real Codex Bash tool call with the `[hooks.harnez]` PreToolUse
  hook installed and capture the actual stdin payload (e.g. via a debug
  wrapper that tees stdin to a file before decoding).
- Confirm or correct the `tool_input.command` field name (and check
  other assumed field names/shapes while at it).
- Add a regression test fixture built from the captured real payload.

## Non-goals

- No behavior change beyond fixing the field name if it's wrong.
