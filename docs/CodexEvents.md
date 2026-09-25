# Codex Events

Issue 420 records the Codex event shapes used by Harnez's analytics adapter.
This is a compatibility note, not a claim that every Codex CLI release emits
every event.

## Observed shapes

- Native hook envelopes use `hook_event_name`, `session_id`, `turn_id`,
  `tool_use_id`, `tool_name`, and a `tool_input` object. Bash input carries its
  command in `tool_input.command`. `PreToolUse` is the interception point;
  `PostToolUse` is the completion observation point.
- Rollout transcripts use `type: "event_msg"` with a nested `payload`.
  `payload.type: "token_count"` carries cumulative provider usage in
  `payload.info.total_token_usage`: `input_tokens`,
  `cached_input_tokens`, `output_tokens`, `reasoning_tokens`, and
  `total_tokens`. The session identifier may be nested in the payload.
- Session-boundary and tool-call records are accepted by the normalizer by
  their event/type names. Unknown fields and unknown event kinds are ignored.

## Compatibility guarantees

The parser is fail-open for malformed or partial records: it returns an empty
or partially populated event rather than preventing the surrounding Codex
operation. Optional provider values remain absent when the provider does not
report them. M2 owns mapping result and failure fields into the shared
`tool_calls` schema; this document does not infer those values from missing
payload fields.

## Context size and compaction (pitfalls, 2026-09-26)

- `turn.completed` usage is cumulative over the session. The real context size is
  `last_token_usage.input_tokens` in the latest rollout `token_count` record
  (`~/.codex/sessions/YYYY/MM/DD/rollout-*.jsonl`).
- Sending `/compact` as a prompt to `codex exec resume` does **not** compact. The model replies
  "Compacted." as plain text. Use `-c model_auto_compact_token_limit=N`; Codex then compacts at
  turn start and writes a `compaction` record to the rollout (canary: context dropped to 13,937
  while `turn.completed` showed 41,691). See issue 594.
- Turn errors appear as rollout `task_complete.error.message` (e.g. `401 Unauthorized`); stderr may
  only show the startup banner "Reading additional input from stdin". `codex login status` can say
  "Logged in" while the token is rejected. See issue 595.
