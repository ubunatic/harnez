# 117 — `harnez rate`: positional internal-tool rating command

**Status**: Open
**Priority**: P2 (Medium)
**Severity**: Moderate
**Category**: Feature
**Related**: [[116-tool-telemetry-schema-and-storage-layer]], [[121-multi-repo-session-and-ticket-id-resolution]], [[122-agent-instruction-tool-feedback-protocol]]

## Problem

Agents need a near-zero-overhead way to record a 1–5 quality rating for
an internal tool call (file read, edit, semantic scan, web search, …)
immediately after using it, without paying MCP/JSON-RPC schema overhead
in the system prompt. The spec calls for a single ultra-compact
positional command.

## Scope

Command signature:

```
harnez rate <tool_name> <score> "<description>" [<ticket_id>]
```

- `tool_name` (required): free-form tool identifier string.
- `score` (required): integer 1–5; reject out-of-range with a clear
  error and non-zero exit.
- `description` (required): 1-line quoted summary.
- `ticket_id` (optional): `<project_folder>/<ticket_name>` composite —
  resolution when omitted is [[121]]'s responsibility, not this ticket's.
- Flags: `--agent <name>` (override; default `$HARNEZ_AGENT` or
  auto-detect), `--session <id>` (override auto-resolved session ID).
- Writes one row via the [[116]] storage layer with `call_type =
  'internal'`, `exit_code = NULL`.
- Performance: sub-20ms end-to-end (process start to write complete) —
  this is the command most sensitive to the spec's latency budget since
  it fires many times per agent turn.

## Acceptance Criteria

- [ ] Parses positional args per the signature above; rejects score
      outside 1–5 with a non-zero exit and message naming the valid
      range.
- [ ] `--agent`/`--session` overrides work and are covered by tests.
- [ ] Omitted `ticket_id` resolves via [[121]]'s resolution chain rather
      than silently writing NULL.
- [ ] End-to-end latency benchmark (cold and warm DB) documented and
      under 20ms on the dev machine, or the constraint is explicitly
      renegotiated here if the storage engine chosen in 115/116 can't
      hit it.
- [ ] `harnez rate --help` documents the command per this repo's
      self-documenting CLI convention.

## Notes

Do not start before [[116]] lands — this command is a thin CLI wrapper
around that storage layer, not a second place to open the DB.
