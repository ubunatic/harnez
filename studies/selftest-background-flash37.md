# Study: `harnez-selftest` background flow — flash37

- Model: flash37 (Gemini 3.7 Flash, `gemini-3-7-flash`), Antigravity CLI (`agy`)
- Date: 2026-10-01
- Project: voxi (harnez-tracked)
- Result: PASS — all steps in correct sequence, background step ran 10s, exit 0

## What the agent had loaded

- The `harnez-selftest` skill text ("summarise, run hello, follow CLI output").
- The project instruction file (`AGENTS.md`), which requires executing `harnez read .harnez/rules/*.md` before any work. The agent ran this single-call read upfront, loading all rules including `HTO=0` background execution guidelines and reactive notification wait requirements (no polling).

## Steps as executed

1. `harnez read` of all six rule files in one call.
2. Briefly summarised the task to the user and ran `harnez agent selftest --step hello` → printed session ID and next actions.
3. `HTO=0 harnez agent selftest --step background` launched as a native background task via `run_command` with a short wait timeout (`WaitMsBeforeAsync: 500`), returning task ID `ba558100-304b-4450-a78e-5c6497abec1c/task-6`.
4. `harnez agent selftest --step confirm` in the next call → "Confirmed background task is currently active".
5. Followed "Inspect your native background job/task list" by invoking the native `manage_task` tool with `Action: "list"`. This returned the active background task metadata, description, start timestamp, and log URI.
6. `harnez agent selftest --step confirm-running` (omitting optional `<duration>`).
7. Stopped calling tools to await the reactive background completion event from the host (`SYSTEM_MESSAGE` with exit code 0 after 10s).
8. `harnez agent selftest --step verify` → PASS.

## Clarity assessment

Clear:
- Step sequence and exact command parameters.
- `HTO=0` was understood from the loaded rules.
- Explicit instruction to await completion before running verify fit the reactive notification model.

Ambiguous or host-dependent:
1. `confirm-running [<duration>]` — the `<duration>` parameter is optional and unspecified; the agent omitted it without issue.
2. "Inspect your native background job/task list" — in Antigravity CLI, `manage_task` (`list`) exists and matches this step directly. In hosts without a task-list tool (e.g. Claude Code), this instruction causes confusion or is skipped.
3. Session ID consistency: In Antigravity CLI, the selftest session ID (`ba558100-304b-4450-a78e-5c6497abec1c`) matched the native host conversation and task ID prefix exactly.

## Comparison across models and hosts

- **Opus (Claude Code)**: Skipped rule loading, read an empty log file for inspection, omitted duration, passed verify.
- **Sonnet (Claude Code)**: Loaded rules, bundled background/confirm in one message, skipped task list inspection due to lack of tool, omitted duration, passed verify.
- **Flash 3.7 (Antigravity CLI)**: Loaded rules, launched background task, ran sequential confirmation, performed native task-list inspection (`manage_task(list)`), awaited reactive notification without polling, passed verify.

## Suggestions

- Clarify or drop `[<duration>]` in `confirm-running`.
- Standardise task inspection expectations across CLI environments (e.g. note that listing tasks is host-dependent, or accept host task ID).
