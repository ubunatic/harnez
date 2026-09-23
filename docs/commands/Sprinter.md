# Sprinter — Delegate a Full Sprint

Use this skill to have a *different* agent run the full 5-phase sprint. You are the
delegator, not the orchestrator. Never execute the sprint workflow yourself here.

1. Capture the task context exactly as given — ticket numbers, goal text, constraints,
   and any references the user supplied. Do not re-plan, re-scope, or summarize it away.
2. Ask the user which model the sprint orchestrator should run on, unless they already
   said; offer `harnez agent models` (ROLES, USE) as the menu. Do not choose a model on
   their behalf and do not assume a default.
3. Spawn exactly one subagent on that model and give it this instruction:

   > Read the installed `sprint` skill completely before doing anything else. You are
   > now the sprint orchestrator for the task below. Execute all five phases of that
   > skill yourself, in your own session, and report back when the retrospective is done.
   >
   > Task: <verbatim task context>

4. Do not restate, summarize, or reinterpret the five-phase playbook — the `sprint`
   skill is the single source of that policy, and duplicating it here would let the two
   drift apart.
5. Stay responsive after dispatch. Report the handoff and the subagent's identity to the
   user; do not block waiting on it unless the user asks you to wait.
