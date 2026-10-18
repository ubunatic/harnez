# harnez agent send: queue a message for exec-mode workers' next turn

Status: Open
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
