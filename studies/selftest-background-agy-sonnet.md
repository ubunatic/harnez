# Study: `harnez-selftest` background flow — agy:sonnet

- Model: agy:sonnet (Claude Sonnet 4.6 Thinking, `claude-sonnet-4-6`), Antigravity CLI (`agy`)
- Date: 2026-10-01
- Project: voxi (harnez-tracked)
- Result: PASS — all steps in correct sequence, background step ran 10s, exit 0

## What the agent had loaded

- The `harnez-selftest` skill text ("summarise, run hello, follow CLI output").
- The project instruction file (`AGENTS.md`), which requires executing `harnez read .harnez/rules/*.md`
  before any work. The agent attempted this using the `harnez_command` MCP tool, but the MCP call
  failed twice due to an incorrect argument schema (first omitting `action`, then sending `action:
  "read"` which the tool rejected as not a valid action). The rules were never successfully loaded.
- Despite the rules not loading, the agent proceeded with the selftest correctly, relying on the
  `harnez-selftest` skill instructions alone.

## Steps as executed

1. Attempted `harnez read` of all six rule files via `harnez_command` MCP tool — failed twice
   (wrong argument schema); rules not loaded.
2. Briefly summarised the task and ran `harnez agent selftest --step hello` → returned session ID
   `1607df2b-787b-4c75-8ad8-fa6e2ddc1e00` and next actions (FIRST: background, SECOND: confirm).
3. `HTO=0 harnez agent selftest --step background` launched as a native background Daemon task via
   `run_command` with `IsDaemon: true` and `WaitMsBeforeAsync: 3000`, returning task ID
   `1607df2b-787b-4c75-8ad8-fa6e2ddc1e00/task-4`.
4. `harnez agent selftest --step confirm` in the next sequential call → "Confirmed background task
   is currently active."
5. Followed "Inspect your native background job/task list" by invoking the native `manage_task`
   tool with `Action: "list"`. Simultaneously, the system delivered a reactive completion
   `SYSTEM_MESSAGE` from task-4 (exit 0, "Background task completed after 10s") alongside the
   list result. The list itself reported no running tasks (the task had already finished).
6. `harnez agent selftest --step confirm-running 10s` — the agent correctly extracted the 10s
   duration from the background task's completion message and passed it as the optional argument.
7. `harnez agent selftest --step verify` → PASS.

## Clarity assessment

Clear:
- Step sequence and exact command parameters understood from skill instructions alone.
- FIRST/SECOND ordering respected (background launched before confirm).
- Awaited completion reactively (the `SYSTEM_MESSAGE` arrived during the `manage_task list` call,
  so no extra wait step was needed).

Ambiguous or host-dependent:

1. `HTO=0` prefix — without the rules loaded, the agent used `IsDaemon: true` on `run_command`
   instead. This works differently: a daemon task is intentionally long-lived, whereas `HTO=0` is
   about disabling the human-turn timeout for a one-shot background command. The intent was
   achieved (the command ran in background), but the mechanism differed from what the rules specify.
2. `confirm-running [<duration>]` — the agent correctly inferred the duration (10s) from the task
   completion message output rather than leaving it blank. This is the best observed behaviour
   across all runs so far.
3. "Inspect your native background job/task list" — `manage_task(list)` exists in Antigravity CLI
   and was called correctly. However, by the time the call was made the task had already completed,
   so the list was empty. The inspection was nominal rather than substantive.
4. Session ID consistency: The selftest session ID matched the host conversation ID prefix exactly
   (`1607df2b-...`), making task linkage unambiguous.

## Comparison across models and hosts

| Model | Host | Rules loaded | Task list inspected | Duration passed | Result |
|---|---|---|---|---|---|
| Opus | Claude Code | No | Attempted (read empty log) | No | PASS |
| Sonnet 5.5 | Claude Code | Yes | No (no tool) | No | PASS |
| Flash 3.7 | Antigravity CLI | Yes | Yes (task still running) | No | PASS |
| **Sonnet 4.6** | **Antigravity CLI** | **No (MCP error)** | **Yes (task finished)** | **Yes (10s)** | **PASS** |

Notable: agy:sonnet is the first run to pass a concrete duration to `confirm-running`, and the
first where the harnez_command MCP tool failed to load the rules due to schema mismatch — the
correct invocation via `harnez find` or `run_command harnez read ...` was not attempted as a
fallback.

## Suggestions

- Document or fix the `harnez_command` MCP tool's argument schema so that `action: "read"` (as
  used for `harnez read`) maps to a supported action, or expose a dedicated MCP tool for reading
  files so agents don't have to guess.
- Clarify or drop `[<duration>]` in `confirm-running`; extracting it from the completion message
  (as done here) is reasonable but not documented.
- The `IsDaemon: true` vs `HTO=0` distinction should be noted in the skill or rules: Daemon keeps
  a task alive indefinitely; `HTO=0` is a one-shot background timeout override.
