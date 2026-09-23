# Reverse Sprinter — Delegate a Reverse Sprint

Use this skill to delegate a reverse sprint to a **low-cost developer agent** (a `developer` row from `harnez agent models`). You are the delegator, not the executor.

1. Capture the task context exactly as given — ticket numbers, scoped task or bug text, target files, and acceptance criteria.
2. Select a low-cost coder model (the cheapest fitting `developer` row of `harnez agent models`, or a user-specified model).
3. Spawn exactly one subagent on that low model with this instruction:

   > Read the installed `reverse-sprint` skill completely before doing anything else. You are the Lead Developer driving this task. Execute that skill's complete reverse-sprint workflow yourself: implement the code, author tests, perform frequent auto-compaction (every 100-150k tokens), request milestone reviews from a `reviewer` model of another vendor, and escalate to an `advisor` model (both chosen from `harnez agent models`) with strict limited context only if stuck. Report back when teardown and tracker sync are complete.
   >
   > Task: <verbatim task context>

4. Do not restate or summarize the reverse workflow — the `reverse-sprint` skill is the single source of that policy.
5. Stay responsive after dispatch. Report the handoff and subagent ID to the user.
