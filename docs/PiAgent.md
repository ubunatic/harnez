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
extension for general shell commands (not Pi prompt/subagent execution). Its
logic lives in `.pi/lib/background-task-runner.ts`; Pi autoloads only
`.pi/extensions/`, so the library and its tests stay out of autoload.

Tools (model-callable):

- `background_task` starts a command through `$SHELL -lc` in the current project
  directory, inheriting the Pi process environment. Each command runs in its own
  process group, which is what cancellation targets.
- `background_task_list` lists in-memory tasks with status, elapsed time, exit
  code or signal, and command.
- `background_task_result` takes `id` and returns status, exit code or signal,
  elapsed time, and the captured output so far (running or finished).
- `background_task_cancel` takes `id` and stops a running task. It returns
  `cancelled`, or reports that the task already ended, or returns an error when
  termination cannot be confirmed (the task then stays `running`).

Commands: `/task <command>`, `/tasks`, `/task-result <id>`, `/task-cancel <id>`.
The same IDs are accepted by the tools and commands: a positive safe integer, or
its canonical decimal string. `0`, negative values, fractions, leading zeros
(`01`), `NaN`, empty strings and unknown IDs are rejected without sending any signal.

Lifecycle and reporting:

- A task is `running`, then exactly one of: `completed` (exit 0), `failed`
  (non-zero exit, or killed by a signal such as SIGKILL, shown as `signal SIGKILL`
  with a null exit code), or `cancelled`.
- When a task completes or fails, its command, status, exit code or signal, and
  captured output are sent to the agent as one follow-up message. A confirmed
  cancellation and session shutdown send no follow-up. A cancel that was not
  confirmed leaves the task running; when it later ends, it settles like any other
  task and sends its follow-up.
- An unknown end (for example a spawn error with no exit code) is shown as
  `exit unknown`, never `exit null`.
- Elapsed time is frozen when the task leaves `running`.
- Output is decoded per stream (stdout and stderr each have their own UTF-8
  decoder), so a multibyte character split across chunks is not corrupted.
- The status bar shows `Tasks: N` for running tasks. The count is local to this
  extension runtime, not a cross-session/global process count.

Bounds live in `spec/pi-background-tasks.yaml` (schema
`spec/schemas/pi-background-tasks.schema.json`). The file is JSON-compatible YAML so
the extension loads it strictly without a YAML dependency; unknown, missing or
non-positive values fail extension load instead of falling back to defaults.

- `output_limit_chars` (16000): combined stdout/stderr characters kept per task; the newest text is kept.
- `finished_retention_limit` (50): finished tasks kept; the oldest finished tasks
  are evicted first. Running tasks are never evicted.
- `stop.grace_ms` (1000): wait after SIGTERM to the process group before SIGKILL.
- `stop.kill_wait_ms` (2000): bounded wait after SIGKILL, and bounded wait for
  output streams to close after the shell exits.
- `stop.poll_ms` (25): interval for re-checking that the process group is gone.

Termination (POSIX): cancellation sends SIGTERM to the process group, waits up to
`grace_ms`, then sends SIGKILL and waits up to `kill_wait_ms`. A task is reported
`cancelled` only when the shell leader has been reaped and its process group holds no
live member. Each task is in one phase at a time: `live` (a leader exit is a natural
end), `cancelling` (an attempt is in flight; only a confirmed cancel can settle it), or
`unconfirmed` (the attempt failed; the task is watched and settles with its real exit).
Concurrent cancels share one attempt, so one SIGTERM and one SIGKILL are sent. A cancel
on a task whose group has already drained reports `already-ended`, and the natural end
is then settled once, with its follow-up.

- Liveness on Linux is read from `/proc`. A zombie (exited, not yet reaped) is not
  live: it runs nothing and holds no output pipe. A zombie still answers a signal-0
  group probe, which is why the probe alone is not used there. A group member that
  is zombie only because PID 1 does not reap orphans therefore does not block
  confirmation. If `/proc` cannot be read, or a stat line is malformed or unreadable,
  liveness is unknown: the signal-0 probe decides, and an unreadable entry is never
  read as an empty group.
- Elsewhere there is no `/proc` reader, so liveness is unknown and the signal-0 probe
  decides: any member, zombie or not, keeps the task `running`. Whether macOS
  reaps orphans in practice is not verified here.
- Signals after the leader is reaped go only to a group in which a live member is
  seen at that moment. The group ID stays reserved while a member exists, so it
  cannot have been reused by another group. An empty group never receives a signal.
- If the group is not confirmed empty, the tool returns an error and the task stays
  `running`. The task is then watched until its group is empty, and settles with the
  status its real exit gives (with a follow-up), because the cancel request did not
  take effect. A later `background_task_cancel` retries termination.

Limitations (not hidden by the tools):

- **Shell exit is not group death.** A shell that exits early, for example after
  its own children are still running, is `completed` or `failed`; those children
  are not stopped by that status. Cancellation waits for the whole group, but only
  for the group it started.
- **Escaped processes.** A child that calls `setsid`/`setpgid` or otherwise leaves
  the process group is not reached. `cancelled` then describes the group only.
- **Uninterruptible processes.** A process in uninterruptible sleep (D state, for
  example stuck I/O) cannot be stopped by any signal; cancellation reports
  `not-confirmed`, and the task stays `running` and watched until the process exits.
- **Zombies outside Linux.** Without `/proc`, an unreaped zombie in the group is
  indistinguishable from a live process, so cancellation is `not-confirmed` until
  the zombie is reaped by its parent or PID 1.
- **Zombie evidence.** The zombie exclusion is tested with an injected group probe and
  with a fixture `/proc`. A real unreaped zombie with an exited leader was not
  reproduced: this environment's PID 1 is `systemd` (`/proc/1/comm`), and whether it
  reaps orphans was not tested here.
- **Windows.** There are no process groups in this runner: only the shell process is
  signalled (`process.kill`, which terminates it), descendants may survive, and
  `cancelled` is reported once the shell has exited. This is weaker than POSIX group
  cancellation and is not verified on Windows.
- **Stopping rule for escaped output.** If an escaped child keeps stdout or stderr
  open after the shell exits, output is finalized after `kill_wait_ms` with a note;
  later output is discarded.
- Task state is in memory only. Session shutdown (including extension reload) awaits
  cancellation of every running task, then releases every task: output is finalized,
  its streams are destroyed, timers are cleared, and later events from the child are
  ignored. Shutdown sends no follow-ups. Tasks not confirmed stopped are named on
  stderr; their processes may survive. Do not reload while work must continue.

Tests (outside the autoloaded extension directory):

```sh
make test-pi    # scripts/test-pi.sh; runs in make test-short and make check
```

CI runs `make test-pi` as its own step before `make test-short`. Inside `make test-short`
the TAP output would land in the JSON report file, which keeps only a 40-line tail on
failure, so the failing test's name would be lost.

The script requires node 22.18 or newer (TypeScript type stripping) and fails when
node is missing or older; it never skips the tests. The register preload maps the
`typebox` import to a test stub, so the test run does not depend on a Pi installation
in the home directory. Real-process tests use `/bin/sh` and bounded waits on output or
exit events; they send signals only to their own spawned processes. The escaped-child
test needs Linux and `setsid`; elsewhere it is reported as skipped with its reason.

The spec schema is checked by the Go test `TestPiBackgroundTasksSpecMatchesSchema`
(`pi_background_spec_test.go`), which runs in `make test-short`.

For Harnez's `/harnez-selftest`, the generic tool runs the command provided by
the CLI, for example `HTO=0 harnez agent selftest --step background`. Then
follow the CLI's confirmation instructions and wait for the completion
notification before running its verification step. The `background_task_list`
tool is available for checking whether a job is still active; completion
notifications avoid polling merely to wait.

### Global background-task runner (`harnez apply`)

The same runner is also installed for every project, not only this repository.
`harnez apply` mirrors `.pi/extensions/background-tasks.ts`, its library, and
`spec/pi-background-tasks.yaml` into the Pi agent dir as one directory extension:
`<pi agent dir>/extensions/background-tasks/`, with a `package.json` whose
`pi.extensions` names the entry point. The Pi agent dir is `$PI_CODING_AGENT_DIR`
when set, otherwise `pi_agent_target` (default `~/.pi/agent`). The runner is part
of the `skills` component: with that component selected, apply installs it; without
it, apply reverts a managed install.

Commands:

- `harnez apply` installs or updates the runner. It decides everything from the
  disk first and refuses before writing when a file is modified or unrecorded.
  Re-running with no changes writes nothing.
- `harnez status` lists one row per managed path with its state (for example `ok`
  or `drifted`).
- `harnez diff` reports drift without writing anything.
- Revert runs when apply is given a component set without `skills`, for example
  `harnez apply --components telemetry-only`. It removes the unmodified files it owns
  and keeps the changed ones and their marker, naming each one it kept.

**Ownership by digest.** The directory's `.harnez-managed` file is a ledger. It
records the SHA-256 digest of each file harnez last committed, and, while an apply
is interrupted, the digest of the bytes it is writing. A file is harnez's only while
its bytes match one of those digests. A modified file, or a file without a ledger
record, is a drift: apply refuses to overwrite it, and revert keeps it. A symlink, a
special file, or an extra entry point (`index.ts`/`index.js`) is unsafe: apply refuses
and deactivates an unmodified `package.json`. Revert keeps a changed package and its
marker until nothing it did not write remains.

**Pretrust load.** The runner lives in Pi's user extension scope, which Pi loads
without project trust. Project extensions under `.pi/extensions/` load only after
the project is trusted. Both copies run in one Pi runtime, so the event-bus claim
below covers them (see the limits list).

**Event-bus dedup.** A copy that loads first emits a claim probe on the channel
`harnez:background-tasks:claim`. An earlier copy marks the probe as claimed; a later
copy sees the claim and does not register its tools, commands, or session hooks.
The claim is released on session shutdown.

Limits of this dedup (not hidden by the code):

- **Broken foreign extensions.** A foreign listener on the channel that throws makes
  the emit throw. This copy then fails to load and records no claim; the other copy
  may still load. This runner does not repair or disable the foreign extension.
- **Spoofable events.** Any extension on the same bus can emit the claim channel and
  mark the probe as claimed, which makes this copy yield. The bus is not
  authenticated; it is coordination, not a security boundary.
- **Missing event API.** If `pi.events` is missing or lacks `emit`/`on`, the runner
  prints a warning on stderr and registers unconditionally; duplicate protection is
  off.
- **Same-user concurrent race.** Two Pi processes started at the same moment do not
  share a bus. Each registers its own runner, so the tasks are not deduplicated
  across processes.

**Write order and hard kill.** Apply writes in this order: the ledger, staged (its
digests describe the bytes about to be written); the dependency files; the
`package.json` that activates the runner; and finally the ledger, committed. Each
write is a temp file renamed into place. Nothing is deleted automatically.

- **Before the first ledger rename** (no `.harnez-managed` marker yet). A kill can
  leave an empty `extensions/` and `extensions/background-tasks/`, or a ledger temp
  file `.harnez-managed.<hex>.tmp` in the managed directory. An empty directory is
  not "nothing to do". Without a marker, apply and revert classify the directory as a
  clash, print `clash ... (no .harnez-managed marker); kept it`, and install nothing.
  To recover, inspect it with `ls -la`. Move a temp file aside, then remove the
  directory with `rmdir`, which refuses to delete anything still inside. Do not use
  `rm -rf`, and do not delete unknown files.
- **After the staged ledger, before `package.json`.** The install is in progress. Some
  dependency files may be missing or partial. Pi has no manifest to activate, so the
  runner does not load. Apply refuses unrecorded files and lists them. Move any
  unknown file aside, then run `harnez apply` again to finish.
- **After `package.json`, before the final commit.** The payload is complete and the
  runner is active, but the ledger still holds the staged record. Running
  `harnez apply` again writes the final ledger and changes nothing else.

An unrecorded temp file inside the payload, such as `.background-tasks.ts.<hex>.tmp`,
is preserved: apply refuses and names it, and revert keeps it.

**Recovery without deleting data.** Stop Pi. Run `harnez status` and `harnez diff`
to see the state. Move unknown or temp files aside (or inspect them), then run
`harnez apply` again. Do not remove the ledger by hand; apply and revert read it.

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
