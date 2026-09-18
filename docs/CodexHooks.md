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
