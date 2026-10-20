# harnez agent send: queue a message for exec-mode workers' next turn

Status: Closed — Implemented durable exec-worker message queues consumed once on successful explicit resume; concurrent sends preserved, live routes retain priority; checks passed (b0b62e5e)
Priority: P3
Category: Feature

## Problem
`harnez agent send --name q-loom ...` against a running `codex exec` worker printed the list
of live TUI sessions instead of delivering. New CI findings had to wait in a host-side round
file until the worker finished (2026-10-17 sprint).

## Proposal
Store the message on the session; `resume` prepends queued messages to the next prompt, and
`send` says "queued for next turn" for exec-mode sessions.

## Acceptance
- `send` to an exec-mode session exits 0 with a queued notice; the next `resume` prompt
  contains the message once; tested.

## Delivery
- Implemented in `b0b62e5e`: exact-name registry fallback after live routes, durable FIFO messages
  with sender labels, atomic successful-turn acknowledgement under the registry flock, and
  preservation across failures and concurrent sends/session saves. Completed/deleted targets fail.
- Queue consumption is explicit-resume only: batch exec providers have no turn-boundary hook.
- Tests cover queue/resume once, ordered sends, failed resume retention, arrivals during resume,
  stale saves, unknown/ambiguous/completed/deleted names, store/directory scope, and live priority.
- Passed: `make lint`, `go test ./internal/subagent/ ./cmd/harnez/`, `make test-short`, `make install`.
  Updated `docs/AgentMessaging.md` and help goldens. Dedicated self-review completed; no delegation.
- Host owns the live agent check; no live agent sessions were started by this leaf worker.
