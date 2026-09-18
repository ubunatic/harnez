# Codex Hooks — Reference Schema

Reference material for Codex lifecycle hooks. Source: [OpenAI Docs — Hooks](https://developers.openai.com/docs/hooks).

## PreToolUse rewrite

For a `PreToolUse` hook that allows a compatible tool call and replaces its
input, Codex expects `permissionDecision` and `updatedInput` inside
`hookSpecificOutput`, together with the event name:

```json
{
  "hookSpecificOutput": {
    "hookEventName": "PreToolUse",
    "permissionDecision": "allow",
    "updatedInput": {
      "command": "echo rewritten"
    }
  }
}
```

For Bash and `apply_patch`, `updatedInput.command` must be a string. The
top-level form below is valid JSON but is not valid Codex hook output because
the required `hookSpecificOutput` wrapper and `hookEventName` are missing:

```json
{
  "permissionDecision": "allow",
  "updatedInput": {
    "command": "echo rewritten"
  }
}
```

## PreToolUse denial

```json
{
  "hookSpecificOutput": {
    "hookEventName": "PreToolUse",
    "permissionDecision": "deny",
    "permissionDecisionReason": "Destructive command blocked by hook."
  }
}
```

Codex also accepts the legacy blocking form:

```json
{
  "decision": "block",
  "reason": "Destructive command blocked by hook."
}
```

## Input and output constraints

- Bash and `apply_patch` inputs use `tool_input.command`.
- `updatedInput` is only valid with `permissionDecision: "allow"`.
- Plain text on stdout is ignored; hook JSON must be written to stdout.
- `permissionDecision: "ask"`, `decision: "approve"`, `continue: false`,
  `stopReason`, and `suppressOutput` are currently unsupported for
  `PreToolUse` and cause the hook execution to be reported as failed.

## Harnez implementation note

`harnez codex-hook` must emit the wrapped `hookSpecificOutput` form above. Its
rewritten command is routed through `harnez exec`; the hook itself does not
execute the command or write telemetry.

## Lifecycle schemas and telemetry findings

The official Codex hook contract documents these common input keys for command
hooks:

```json
{
  "session_id": "thread-id",
  "transcript_path": "/path/to/rollout.jsonl",
  "cwd": "/workspace",
  "hook_event_name": "PostToolUse",
  "model": "model-slug",
  "permission_mode": "default",
  "turn_id": "turn-id"
}
```

The event-specific tool keys are `tool_name`, `tool_use_id`, and `tool_input`.
For Bash and `apply_patch`, `tool_input.command` contains the command string.
`PreToolUse` and `PostToolUse` can also observe MCP and other local function
tools. Hosted tools such as `WebSearch` do not use this hook path.

The documented hook payload does not promise provider token usage or a
`tool_output` field. Harnez has observed rollout records containing
`event_msg`/`token_count` data with this shape:

```json
{
  "type": "event_msg",
  "payload": {
    "session_id": "thread-id",
    "type": "token_count",
    "info": {
      "total_token_usage": {
        "input_tokens": 10,
        "cached_input_tokens": 4,
        "output_tokens": 3,
        "reasoning_tokens": 2,
        "total_tokens": 15
      }
    }
  }
}
```

This rollout shape is useful for analytics but is not a stable hook interface;
unknown fields and future changes must be tolerated.

## Recommended telemetry design

- Use `PreToolUse` and `PostToolUse` for reliable tool-call rows, tool names,
  IDs, and commands. `harnez` also parses `success`/`exit_code`/`duration_ms`
  off `PostToolUse` payloads (`internal/codex/events.go`), but a live session
  confirmed these are not fields Codex actually sends: real `PostToolUse`
  rows land with `exit_code`/`duration_ms` NULL/zero (issue 420 M4). The
  parsing exists only in case a future Codex version starts sending them.
  The `PreToolUse` rewrite through `harnez exec` is the permanent,
  confirmed source of exit code and duration, not an interim one.
- Use `SessionStart` to record the session ID and transcript path.
- Use `Stop` for a turn boundary and `SessionEnd` for final reconciliation.
  `SessionEnd` is synchronous and can read the transcript while it runs.
- Use `PreCompact` and `PostCompact` to mark compaction boundaries and avoid
  double-counting token snapshots.
- Use `SubagentStart` and `SubagentStop` for subagent lifecycle attribution;
  subagent hooks expose the parent session ID.
- Read `token_count` rollout records from the transcript at `Stop` or
  `SessionEnd`, calculate deltas from cumulative totals, and associate the
  result with the session and latest turn.
- Treat transcript parsing as best-effort and versioned. The official docs
  provide `transcript_path` for convenience but explicitly do not guarantee
  the transcript format as a stable hook API.

Theoretical alternatives are continuously tailing Codex rollout files, or
using a future first-class usage field if Codex adds one to the hook schema.
Neither should replace lifecycle reconciliation: tailing risks partial
records, while undocumented fields can disappear between CLI releases.

Reference: [OpenAI Codex Hooks](https://developers.openai.com/es-419/docs/hooks).
