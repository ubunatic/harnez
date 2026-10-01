# Study: `harnez-selftest` background flow — opus

- Model: opus (Claude Opus 5.5, `claude-opus-5-5`), Claude Code CLI
- Date: 2026-10-01
- Project: voxi (harnez-tracked)
- Result: PASS — all steps in correct sequence, background step ran 10s, exit 0

## What the agent had loaded

- The `harnez-selftest` skill text only ("run hello, then follow CLI output").
- No harnez rules. The project's instruction file asks to `harnez read .harnez/rules/*.md`
  before any work; the agent skipped that. All guidance came from runtime CLI output.

## Steps as executed

1. `harnez agent selftest --step hello` → printed next actions.
2. `HTO=0 harnez agent selftest --step background` as a native background task.
3. `harnez agent selftest --step confirm` (in parallel with step 2) → "active".
4. Read the task output file (empty), then `--step confirm-running` with no duration.
5. Waited for the host's completion notification (no polling).
6. `--step verify` → PASS.

## Clarity assessment

Clear: step order, exact commands, and the instruction to use native background execution.

Ambiguous (agent had to guess):

1. `confirm-running [<duration>]` — meaning of `<duration>` and when to pass it is undefined.
   Agent omitted it.
2. "Inspect your native background job/task list" — Claude Code has no task-list tool; the agent
   read the task's output file instead, which was still empty, so the check proved little.
3. `HTO=0` — purpose unexplained; passed through verbatim.
4. "Await background task completion" — no guidance on how (notification, poll, monitor). Agent
   briefly loaded a Monitor tool, then relied on the completion notification.

## Suggestions

- Define `<duration>` in the `confirm-running` output, or drop it.
- Phrase the inspect step per host, or accept the output file as the inspection.
- Explain `HTO` in one clause where it is printed.
- Say "wait for your host's completion notification; do not poll" in the await step.
