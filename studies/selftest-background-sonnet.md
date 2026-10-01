# Study: `harnez-selftest` background flow — sonnet

- Model: sonnet (Claude Sonnet 5.5, `claude-sonnet-5-5`), Claude Code CLI
- Date: 2026-10-01
- Project: voxi (harnez-tracked)
- Result: PASS — all steps in correct sequence, background step ran 10s, exit 0

## What the agent had loaded

- The `harnez-selftest` skill text ("summarise, run hello, follow CLI output").
- The project instruction file, which says to run one `harnez read .harnez/rules/*.md` call
  before any work. This agent did that first (unlike the opus run), so it had the rules,
  including the `HTO=0` and "do not poll, wait for the notification" background guidance.

## Steps as executed

1. `harnez read` of all six rule files (one call).
2. `HTO=0 harnez agent selftest --step hello` → printed next actions.
3. `HTO=0 harnez agent selftest --step background` as a native background task, and
   `harnez agent selftest --step confirm` in the same message (the CLI said "FIRST ... SECOND",
   so this ordering was a loose reading: both were issued together, not one after the other).
   Confirm answered "active".
4. `--step confirm-running` with no duration. The agent did NOT inspect any task list or output
   file first; it just ran the step. Claude Code has no task-list tool, so the instruction could
   not be followed literally, and the agent did not say so at the time.
5. Stopped calling tools and waited for the completion notification (no polling, no Monitor).
6. `--step verify` → PASS.

## Clarity assessment

Clear: step order, exact commands, background execution, "await completion then verify".
With the rules loaded, the `HTO=0` prefix and the no-polling behaviour needed no guessing.

Ambiguous or weak:

1. "Inspect your native background job/task list" — no such tool exists in Claude Code. The agent
   skipped the inspection and ran `confirm-running` anyway, so the step recorded a confirmation
   the agent had not itself checked. The CLI cannot tell the difference.
2. `confirm-running [<duration>]` — `<duration>` is undefined; the agent omitted it.
3. "FIRST ... SECOND" with `confirm` implies sequence, but `confirm` only passes if the
   background task has already started. Issuing both in one message worked here, but it is a race
   the instructions do not acknowledge.
4. The background task's output file lives under a different session directory
   (`84bf25e3-...`) than the selftest session id the CLI printed (`ec7938cd-...`). Harmless here,
   but it makes it hard to link the CLI's session to the host's task record.

## Differences from the opus run

- Sonnet read the rules first; opus skipped them.
- Sonnet never read the (empty) task output file or loaded a Monitor tool; opus did both.
- Both omitted `<duration>`, both hit the "inspect task list" gap, both relied on the
  completion notification.

## Suggestions

- Make `confirm-running` accept evidence from the agent (e.g. a task id) so an uninspected
  confirmation is distinguishable from a real one.
- Define `<duration>` or drop it.
- Phrase the inspect step per host, or accept "the task id the host returned" as enough.
- Say whether `confirm` may be issued in the same message as `background`.
