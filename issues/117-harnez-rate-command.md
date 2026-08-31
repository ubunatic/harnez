# 117 — `harnez rate`: positional internal-tool rating command

**Status**: Closed — resolved in a3a67ba
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

- [x] Parses positional args per the signature above; rejects score
      outside 1–5 with a non-zero exit and message naming the valid
      range.
- [x] `--agent`/`--session` overrides work and are covered by tests.
- [x] Omitted `ticket_id` resolves via [[121]]'s resolution chain rather
      than silently writing NULL.
- [x] End-to-end latency benchmark (cold and warm DB) documented and
      under 20ms on the dev machine, or the constraint is explicitly
      renegotiated here if the storage engine chosen in 115/116 can't
      hit it.
- [x] `harnez rate --help` documents the command per this repo's
      self-documenting CLI convention.

## Notes

Do not start before [[116]] lands — this command is a thin CLI wrapper
around that storage layer, not a second place to open the DB.

## Implementation

`cmd/harnez/rate.go` implements `harnez rate <tool_name> <score>
"<description>" [<ticket_id>]` as a thin Cobra command wrapping
`internal/telemetry.DB.Insert` and `internal/resolve.Session`/`Ticket`.

- Score is parsed client-side (`strconv.Atoi`) only to build the `*int`
  Insert needs — the 1-5 *range* is not re-validated in Go; the DB's own
  CHECK constraint (schema.go) is the single enforcement point, per
  docs/other/Spec.md. A CHECK-constraint failure mentioning "score" is
  caught and re-wrapped with a message naming the 1-5 range, so the UX
  requirement is met without a second hardcoded range.
- `--agent` > `$HARNEZ_AGENT` > env-var agent auto-detect (mirrors
  `internal/resolve`'s `SessionEnvVars` signals: `CLAUDE_CODE_SESSION_ID`
  etc.) > `"unknown"`. No prior "current agent" auto-detection existed
  elsewhere in the repo (`internal/usage`'s `isAgentName` only classifies
  *other* running processes, not this process's own identity).
- `--session` overrides `resolve.Session()`; omitted `ticket_id` resolves
  via `resolve.Ticket()` (git-repo-root + ticket-shaped branch name, or
  last-used-ticket-for-session fallback) rather than writing NULL/"".
- Latency measured via `cmd/harnez/rate_test.go`'s
  `TestRunRate_LatencyBudget` and `BenchmarkRunRate` (mirrors
  `internal/telemetry`'s `BenchmarkInsert`/`TestInsertLatencyBudget`
  pattern): cold end-to-end (arg parse → resolve → Open incl. schema
  create → Insert) **1.36ms**, warm max **0.78ms** over 50 calls — both
  well under the 20ms budget, on this dev machine.
- Manually verified `harnez rate --help` output and one real
  `harnez rate test-tool 4 "smoke test" smoketest/117-rate --session
  smoke-session-cleanup-me` run against the real
  `~/.harnez/tool_catalog.sqlite` (no `--db-path` flag exists/was
  requested); the row was verified then deleted, and no lock-file/ticket
  state was left behind (the run used `--session`/explicit ticket_id, so
  `resolve`'s lock-file and last-ticket-history paths were never
  exercised).
