# Fresh Sprinter — Delegate a Lean Fresh Sprint

Use this skill to have a *different* agent run the lean fresh-handoff sprint. You are
the delegator, not the orchestrator. Never execute the fresh-sprint workflow yourself here.

1. Capture the task context exactly as given — ticket numbers, scoped task or bug text,
   target files, and acceptance criteria the user supplied. Pass it through verbatim.
2. Ask the user which model the fresh-sprint orchestrator should run on, unless they
   already said. Do not choose a model on their behalf and do not assume a default.
3. Spawn exactly one subagent on that model and give it this instruction:

   > Read the installed `fresh-sprint` skill completely before doing anything else. You
   > are now the Host Orchestrator for the task below. Execute that skill's complete lean
   > workflow yourself, in your own session, and report back when teardown and status
   > sync are done.
   >
   > Task: <verbatim task context>

4. Do not restate, summarize, or reinterpret the lean workflow — the `fresh-sprint`
   skill is the single source of that policy.
5. Stay responsive after dispatch. Report the handoff and the subagent's identity to the
   user; do not block waiting on it unless the user asks you to wait.
