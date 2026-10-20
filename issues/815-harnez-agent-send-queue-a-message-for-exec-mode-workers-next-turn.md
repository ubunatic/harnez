# harnez agent send: queue a message for exec-mode workers' next turn

Status: Closed — M2 accepts resumable registry sessions and caps unknown-target candidates; regression coverage and requested checks pass (46b82388)
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
  preservation across failures and concurrent sends/session saves. M2 below corrects M1's
  mistaken rejection of resumable completed sessions.
- Queue consumption is explicit-resume only: batch exec providers have no turn-boundary hook.
- Tests cover queue/resume once, ordered sends, failed resume retention, arrivals during resume,
  stale saves, unknown/ambiguous/completed/deleted names, store/directory scope, and live priority.
- Passed: `make lint`, `go test ./internal/subagent/ ./cmd/harnez/`, `make test-short`, `make install`.
  Updated `docs/AgentMessaging.md` and help goldens. Dedicated self-review completed; no delegation.
- Host owns the live agent check; no live agent sessions were started by this leaf worker.

## M2 Pre-Work / Required Refinements (host live check 2026-10-20)

Live: `harnez agent start --name ping-815 --model flash38:low ...` then
`harnez agent send --name ping-815 "..."` -> exit 1: `session "ping-815" cannot queue messages: status
completed; unknown agent send target ...` followed by ~50 live Codex threads from all projects.

1. Host task error (my spec said completed -> error): a batch session is `completed` between every turn;
   that is exactly the state a queued message targets. Accept any resumable session (completed, idle,
   running, failed-but-resumable); refuse only deleted/missing sessions or ones whose provider
   session can no longer be resumed. Test with a completed batch session record.
2. Unknown-target error: list at most ~10 candidates, prefer the current dir/store, and print
   "(N more; use harnez agent list)" instead of every Codex thread on the machine.
3. Then the host reruns: send, resume asks for the word (must contain it), second resume must not
   see it again.

## M2 Delivery
- Implemented in `46b82388`: accept resumable completed, idle, running, failed, and stopped
  registry records; refuse blocked, quarantined, or provider sessions that fail `CheckResumable`.
  Active interactive resumptions also prepend and acknowledge queued messages on successful send.
- Unknown and ambiguous live-target errors show at most ten candidates, prioritize current-dir
  entries, and point to `harnez agent list` for the full list.
- Regression coverage verifies completed-session queueing, non-resumable refusal, current-dir
  priority, candidate truncation, and interactive resume delivery/acknowledgement. Existing tests
  cover FIFO order, failures, and messages arriving during batch resume.
- Passed: `make lint`, `go test ./internal/subagent/ ./cmd/harnez/`, `make test-short`, and
  `make install`. Updated `docs/AgentMessaging.md` and help goldens.
