# Status Report: `harnez agent send` Command

## Overview

This report documents the status, implementation details, test coverage, and roadmap for `harnez agent send` as of October 2026.

## Command Registration & CLI Surface

- **Entrypoint & Routing**: Registered under `cmd/harnez/agent.go` as a subcommand of `harnez agent`. Alias normalization also accepts `harnez agents send`.
- **Implementation**: Implemented in [`cmd/harnez/agent_send.go`](file:///home/uwe/projects/harnez/cmd/harnez/agent_send.go).
- **Target Resolution**:
  - Target sessions are addressed via `--name` or `--id` (UUID).
  - Scope can be filtered using `--dir`.
  - Target resolution inspects Harnez session metadata and provider session records.
  - Running `harnez agent send` without target arguments lists live named sessions capable of receiving messages.
- **Message Delivery Semantics**:
  - Outgoing messages include framing with sender identity.
  - Delivery reports `accepted` upon dispatch.
  - Optional `--wait` flag waits for reception confirmation, reporting `received` or `unconfirmed`. Confirmation reflects receipt, not downstream task execution or completion.

## Provider Support & Current Limitations

- **Claude (`claude`)**: Fully implemented and verified. Sends to live, opted-in Claude channel sessions via plugin/channel bridge. Reports `accepted` and supports receipt confirmation with `--wait`.
- **Codex (`codex`)**: Fully implemented and verified (issue 680, commit `25c6e837`). Enqueues via `codex queue --thread <id> --message <text>`. Reports `queued`. `--wait` is not supported.
- **Antigravity (`agy`)**: Fully implemented and verified for supervised sessions (issue 758, commits `eb37d24b`, `6a19c3ee`). Enqueues into supervised `harnez-agy --meter` or `harnez agent start -i` sessions via Unix control socket `enqueue` action. Messages are delivered at turn boundaries when idle with typing protection via bracketed paste (`\x1b[200~...\x1b[201~\r`). Reports `queued`. `--wait` is not supported. Non-supervised sessions fail with a clear guidance message.

## Testing & Documentation

- **Unit Tests**: [`cmd/harnez/agent_send_test.go`](file:///home/uwe/projects/harnez/cmd/harnez/agent_send_test.go) covers:
  - Session target resolution and `--dir` scoping.
  - Metadata parsing and payload validation.
  - Message framing with sender attribution.
  - Accepted send workflow.
  - Receipt confirmation with `--wait` and timeout handling.
- **Documentation**:
  - User documentation: [`docs/AgentMessaging.md`](file:///home/uwe/projects/harnez/docs/AgentMessaging.md) (covers operational usage and the proven Claude transport).
  - Prior study: [`docs/studies/2026-10-07-claude-cross-session-sendmessage.md`](file:///home/uwe/projects/harnez/docs/studies/2026-10-07-claude-cross-session-sendmessage.md).

## Ticket Tracking & Next Steps

- **Closed Tickets**:
  - Issue 740: Implementation of Claude cross-session messaging transport.
  - Issues 743 & 747: Plugin packaging and channel hooks.
- **Open Tickets**:
  - Issue 656: Parent ticket for generalized cross-provider agent-to-agent messaging.
  - Issue 680: Codex probe and implementation for `agent send` / MCP send tool.
- **Roadmap / Follow-up**:
  1. Implement and test the Codex queue delivery pathway under Issue 680.
  2. Expose the MCP send tool for cross-agent orchestration.
  3. Reconcile provider parity under Issue 656.
