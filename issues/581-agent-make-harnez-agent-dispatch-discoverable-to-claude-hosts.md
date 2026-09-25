# 581 agent: make harnez agent dispatch discoverable to Claude hosts

Status: Open
Priority: P1
Category: harness

## Problem

In a freshly `harnez init`-ed repo (`~/projects/search`, neus), a Claude host told to "delegate to
luna:med, reviews to terra:med" did not know how. What the host did not know or did wrong:

1. **Model names.** `luna:med` / `terra:med` appear nowhere in AGENTS.md or bundled docs. The host
   searched its native agent types and model list, found nothing, and asked the user. Only
   `harnez agent models` (run by the user) revealed them.
2. **Dispatch command.** Nothing says "delegation means `harnez agent start --model <m> --role <r>`".
   AgenticLoop.md Invariant 9 mentions `harnez agent --role`, but not as the way to delegate.
3. **Blocking.** `harnez agent start` is synchronous. The host learned this by hitting the 120s tool
   timeout, and ran 001-003 sequentially by accident.
4. **Backgrounding.** To run agents in parallel, the host detached them with `( ... &)` inside one
   shell call. That made them invisible to the user and unmanaged (a Zero Zombie risk). Correct: one
   native background shell call per agent (Claude `run_in_background`), several calls in one message.
5. **Logs.** The host wrote agent logs to the repo root instead of the session scratchpad.
6. **Roles and writers.** Developer agents were started without `--role developer`, and three writers
   ran in parallel on one workspace (Invariant 1, "Parallel Writing" anti-pattern). Separate ticket
   files avoided conflicts, but the rule was not followed.
7. **Peer sessions.** The user had told the harnez/lmcoder sessions to assist; nothing in the repo
   tells the host that peer sessions exist or when to message them instead of reading their code.

## Proposal

Add a short "Delegation" section to the Harnez Managed Conventions block (AGENTS.md):

- To delegate, use `harnez agent start --name <n> --model <alias:tier> --role <role> -d <repo>`;
  `harnez agent models` lists aliases (luna, terra, ...) with their roles.
- It blocks until done: run each call as its own native background shell (Claude:
  `run_in_background`), never with `&`, `nohup` or subshell detach.
- Parallel only for read-only roles (advisor, reviewer); developers write one at a time, or in
  disjoint files with the user's OK.
- Agent output goes to the session scratchpad, never into the repo.
- `ListAgents` / peer sessions: ask them about their own repo before reading it cold.

Consider also a `harnez agent start --background` that returns a session name at once, plus
`harnez agent wait`, so hosts need no shell tricks.

## Evidence

neus session 2026-09-25: tickets 001-003 dispatched, then review by terra:med, then fixes;
`harnez agent list` shows neus-001..003 and neus-review.
