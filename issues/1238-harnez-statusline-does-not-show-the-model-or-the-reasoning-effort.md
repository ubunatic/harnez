# 1238 — harnez statusline does not show the model or the reasoning effort

**Status**: Open
**Priority**: P2 (Medium)
**Severity**: Minor
**Category**: Feature
**Related**: 536 (statusline cwd and drift, closed)

---

## 1. Problem & Motivation

`harnez statusline` prints the current directory, context usage, cache-read percentage and rate-limit usage. It does not show the active model or the reasoning effort. Both matter when a session is running, so the user wants them at a glance. The data is available: Claude Code's statusLine payload already carries it.

## 2. Technical Specification / Findings

- Observed: `echo '{"model":{"display_name":"Opus"},"workspace":{"current_dir":"/tmp"}}' | harnez statusline` prints `/tmp` only. The model is not shown.
- `harnez statusline --help` describes only the directory, context, cache-read and rate-limit output. Its only flag is `--agent`.
- Claude Code statusLine docs (code.claude.com/docs/en/statusline): the payload has `model.id` and `model.display_name` (always present), and `effort.level` (`low`, `medium`, `high`, `xhigh` or `max`; absent when the model does not support effort).
- Workaround in use: a jq command in `~/.claude/settings.json` prints `[Opus high] <dir>`. It drops the context and rate-limit fields, so it is not a replacement for harnez statusline.
- Not verified: whether the Antigravity (`--agent agy`) payload carries an equivalent model or effort field.

## 3. Implementation & Verification Plan

- Add the model display name and the effort level to the Claude statusline output. Keep the directory, context, cache-read and rate-limit fields.
- Show the effort only when the payload has it; a model without effort support shows the model alone.
- Tests: a golden test for a payload with effort, one without, and one missing `model` (must not fail).
- Live check: after `/effort` changes, the status line shows the new level.
