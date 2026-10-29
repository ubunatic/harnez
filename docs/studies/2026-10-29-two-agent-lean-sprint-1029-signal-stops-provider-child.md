---
kind: sprint
date: 2026-10-29
issue: 1029
---

# Lean sprint 1029: a signal to agent start stops the provider child

## Goal
A SIGTERM or SIGINT to a synchronous `harnez agent start`/`resume` left the provider child (`pi`) running and the
session marked `running`. Seen twice in the 1024 reruns on `lmcoder:coder` (a plain `kill`, then `timeout`).

## Agents
- Searcher `haiku:low` (35 s, 279k tokens incl. cached): confirmed the premise and the cause (no signal handler on
  the start path, so Go's default exit skips the existing `cmd.Cancel` group kill; no `Pdeathsig`), and named
  the status writes and test templates.
- Developer `opus:low`: plan turn (58 s, 174k), write turn (7 min 53 s, 2.2M). Claude models only (OpenAI and
  Gemini quota exhausted).

## Milestones
- M1 `e0a35fed`: `agentSignalContext` with a cancel cause, wired in `withAgentTimeout` and the detached runners;
  `stopped` status on signal; tests failed first, then passed; `--timeout` stays a deadline.
- M2 (host, live): `timeout 25` on `lmcoder:coder` left no `pi` and status `stopped`.

## Observations
- The searcher's audit was precise enough that the developer's plan needed only small host additions (second
  signal, `--timeout` still a timeout, bounded codex test).
- The write turn used 2.2M tokens (mostly cached) for a 358-line change; the plan turn was cheap.
- The developer used `sed` once for a test fix, against the edit rule, and reported it itself.
- `harnez agent list` hides stopped sessions; the live check needed `harnez agent status --name`.
