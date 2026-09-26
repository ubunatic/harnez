---
name: next
description: Continue the most recent workflow on its next actionable item
disable-model-invocation: true
---

# Next

Continue the most recent user-directed workflow on its next actionable item.
Use the current conversation as the primary record of the workflow, arguments,
model preferences, and execution constraints. Use recent session history or the
active task list only when available in context; do not claim to have inspected
history that is not available.

## Workflow Detection

- Identify the most recently completed or explicitly continued workflow, such
  as `/lean-sprint`, `/review`, or `/issue`.
- Carry forward its relevant parameters, such as issue scope, selected models,
  and requested verification. Do not carry forward a one-time instruction that
  applied only to the previous item.
- If there is no clear previous workflow, ask which workflow to continue.

## Target Discovery

- Prefer the active task list or milestone sequence when one is in progress.
- Otherwise use the roadmap at `docs/Roadmap.md` when the prior workflow was
  following it.
- For issue work, discover open candidates with
  `harnez find -d . issues -a status:open`; use the prior sequence, roadmap
  ordering, or explicit user criteria to choose the next one.
- Do not assume the numerically lowest open issue is the next item. Exclude the
  item just completed and items that are closed, blocked, or unrelated to the
  established sequence.
- If multiple candidates remain or the ordering is unclear, present the best
  candidate and workflow parameters and ask the user to choose before execution.

## Execution Contract

- State the selected item and the workflow and parameters you will reuse.
- Execute only when the next item and carried-forward instructions are
  unambiguous, or the user has explicitly authorized automatic advancement.
- Follow the invoked workflow's own rules, including its planning, review,
  verification, and commit requirements. `/next` does not relax those rules.
- For a recommendation-only request, report the proposed next item without
  starting its workflow.
- After completion, identify the next item and workflow outcome so `/next` can
  continue the sequence again.
