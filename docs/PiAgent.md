# Pi Agent Driver

Harnez can run the Pi coding-agent CLI (1.0+) as a batch `harnez agent` provider.
The adapter uses Pi's documented JSON mode and persistent session support; install
Pi separately and make the `pi` executable available on `PATH`.

## Models

`--model pi` (or `pi:default`) uses Pi's configured default model. A Pi model can
also be selected directly with `--model pi:<provider/model-id>[:low|med|high]`,
for example:

```sh
harnez agent start --name pi-task --model pi:openai/gpt-4o:high -d . -p "Inspect the tests and report gaps"
```

Model IDs are passed to Pi unchanged. Pi must already have the corresponding
provider/model configured and authenticated. Thinking tiers map to Pi's
`--thinking` option (`med` becomes `medium`). The generic Pi default entry has
no comparable Harnez quota-cost measurement; model-specific cost metadata is not
inferred from Pi's provider catalog.

## Sessions and limitations

Each turn runs Pi in JSON mode with Harnez's selected working directory. Pi's
session header supplies the provider session ID; Harnez uses that ID with Pi's
`--session` option for follow-up turns. Pi persists the transcript in its own
session store, grouped by working directory. Harnez's `--name` labels its own
session record and does not rename Pi's session.

Harnez requests manual compaction through Pi's documented RPC `compact`
command, not by sending `/compact` as a prompt. Pi returns an estimated
post-compaction context usage; that is a heuristic, not an exact provider token
count. Pi's own automatic compaction and overflow recovery also remain active.

Pi's CLI invocation is a foreground process. Harnez's process controls bound or
terminate that process; Pi has no separate stop request in this adapter. Deleting
a Harnez session does not delete the corresponding Pi transcript. External Pi
sessions are not imported into Harnez's session list.

JSON events provide per-turn input/output/cache usage when Pi reports it. Pi's
JSON protocol does not provide Harnez with a validated context-window size, so
context-window capacity and reasoning-token counts remain unknown.

## Interactive Project Extensions and Background Tasks

Pi has separate extension scopes: user extensions under
`$PI_CODING_AGENT_DIR/extensions/` (default `~/.pi/agent/extensions/`) and
project extensions under `.pi/extensions/`. Project resources load after project
trust is granted. `/reload` reloads extensions after an edit; replacing the
extension runtime runs `session_shutdown` before the new runtime starts.

This repository includes `.pi/extensions/background-tasks.ts`, a project-local
extension for general shell commands (not Pi prompt/subagent execution):

- The `background_task` model tool and `/task <command>` start the supplied
  command through `$SHELL -lc` in the current project directory, inheriting the
  Pi process environment.
- The `background_task_list` model tool and `/tasks` report in-memory tasks,
  including running/completed status, elapsed time, exit code, and command.
- `/task-result <id>` shows captured output; `/task-cancel <id>` sends a
  termination signal to a running task.
- The status bar shows `Tasks: N` for running tasks. The count is local to this
  extension runtime, not a cross-session/global process count.
- When a background command exits, its command, exit status, and captured output are sent
  to the agent as a follow-up message so the agent can inspect the result. The
  extension retains up to 16 KiB of combined stdout/stderr per task.
- Task state is in memory only. Session shutdown—including extension reload—
  sends running tasks a termination signal and clears the list; a process that
  ignores the signal may survive (issue 1003). Do not reload while work must continue.

For Harnez's `/harnez-selftest`, the generic tool runs the command provided by
the CLI, for example `HTO=0 harnez agent selftest --step background`. Then
follow the CLI's confirmation instructions and wait for the completion
notification before running its verification step. The `background_task_list`
tool is available for checking whether a job is still active; completion
notifications avoid polling merely to wait.

## Skills and Hooks Integration

`harnez apply` manages skills and extension hooks for Pi instances, exactly like
it manages Claude, Codex, Antigravity, and Prime Agent.

- **Skills target**: Pi reads skills from `<pi home>/skills/<name>/SKILL.md`.
  `harnez apply` copies Harnez skills to this path. Pi's home defaults to
  `~/.pi/agent` (configured via `pi_agent_target` in `config.yaml`), or
  `$PI_CODING_AGENT_DIR` when set in the environment.
- **Hooks & Extensions**: Harnez installs its Distill extension hook to
  `<pi home>/extensions/harnez-distill.ts`.
- **Status & Revert**: `harnez status` reports installed Pi skills and extension
  hooks; `harnez revert --managed` (or `CleanAll`) cleans up managed Harnez
  artifacts across all Pi targets.
- **Verified live** with Pi 1.0.4 by `make smoke-container` (issue 723): every
  Harnez skill appears in Pi's system prompt under `<available_skills>` in
  `--mode json` output, and the Distill hook fires after a `bash` tool call.
  The bundled third-party skills are explicit-only and not copied to Pi.
- **No telemetry hook yet:** Pi sessions started outside `harnez agent` are
  not recorded in harnez telemetry (issue 725).
