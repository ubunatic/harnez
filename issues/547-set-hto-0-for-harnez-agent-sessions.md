# 547 — Clean 60s foreground turn detach and HTO=0 wait reattach for agent sessions

**Status**: Closed
**Priority**: P1 (High)
**Severity**: Major
**Category**: Agentic Ergonomics
**Related**: #506, #581, #608, #611

---

## 1. Problem & Motivation

Agents executing `harnez agent start` or `harnez agent resume` frequently run into the 60-second execution window enforced by `harnez exec` (`defaultExecTimeout = 60s`). When a turn takes longer than 60s (which is typical for non-trivial edits, tests, or multi-step reasoning):
1. `harnez exec` sends `SIGKILL` to the process group after 60s (`exit status 137`), terminating the turn abruptly.
2. The partial work in the provider session may be disrupted or left in an inconsistent state.
3. Host agents panic, repeatedly poll logs, or try complex workarounds (e.g. ad-hoc `HTO=0` prefixes, `manage_task` polling loops, or timer/cron schedules).

For regular shell commands (`git`, `make`, `npm`), a 60-second limit with SIGKILL makes sense because normal commands have no reassembly or session model. But for `harnez agent`, commands represent conversational, stateful session turns.

Rather than letting `harnez agent` turns be killed after 60 seconds:
- Foreground agent turns (`start` and `resume`) should run with the 60-second budget to allow fast synchronous turns to finish in one turn.
- If a turn runs longer than 60 seconds without an explicit user timeout, `harnez agent` should cleanly **detach** the running session to the background instead of being killed.
- The command should exit cleanly with unmistakable, directive instructions telling the host agent how to reattach.
- The reattach command (`harnez agent wait <session>`) must imply `timeout = 0` (`HTO=0`) by default so waiting never hits a 60-second kill window.
- The host must be explicitly instructed to run `harnez agent wait` as a host background task, **never poll**, and **never schedule timers/crons**, relying entirely on the host environment's automatic reactive notification.

## 2. Technical Specification & Design

### A. Regular Commands Remain Unchanged
- `harnez exec` continues to enforce the 60-second default timeout (`defaultExecTimeout = 60s`) for general shell commands.

### B. `harnez agent wait` Defaults to No Timeout (`timeout = 0`)
- In `cmd/harnez/exec.go:resolveExecTimeout`, any invocation containing `harnez agent wait` (whether invoked directly or wrapped in `bash -c`) resolves to `timeout = 0` (unlimited), unless the caller explicitly passed a non-zero `--timeout` flag or `HTO=<duration>`.
- This guarantees host agents can launch `harnez agent wait <session>` in a host background task without being killed at 60s.

### C. Clean 60s Foreground Turn Detach for `start` & `resume`
- `resume` is treated symmetrically with `start`: both represent prompt turns with a 60-second foreground budget when no explicit timeout is specified.
- If an agent turn is still running as the foreground window expires (e.g. at 60s):
  1. The running worker process is preserved and transferred to a detached background worker session (recording process PID, session ID, name, stdout/stderr logs, and status `running`).
  2. The foreground command exits cleanly with an actionable host instruction block:
     ```
     [session info: id=<id> name=<name> status=running]
     Agent turn exceeded 60s and has been cleanly detached to the background.
     Do NOT poll. Do NOT schedule timers or cron jobs.
     Reattach by launching a background job:
       harnez agent wait <name>
     When the background job finishes, your environment will automatically notify this session.
     ```

### D. Zero-Polling & Zero-Scheduling Invariant
- The host agent must not run loops checking `manage_task status`, `harnez agent status`, or task log files.
- The host agent must not set timers (`schedule` / cron).
- The host agent launches the background wait command and yields its turn. The host harness (e.g. Antigravity, Claude Code) automatically wakes the host session when the background task completes.

## 3. Implementation & Verification Plan

1. **Exec Timeout Exemption for Wait (`cmd/harnez/exec.go`)**:
   - Detect `harnez agent wait` in `resolveExecTimeout` (including unwrapped `bash -c` commands) and default its timeout to `0`.
   - Add unit tests in `cmd/harnez/exec_test.go` verifying that direct and wrapped `harnez agent wait` invocations resolve to `0` timeout while regular commands retain 60s.

2. **Foreground Turn Detach Coordination (`cmd/harnez/agent_run.go` & `cmd/harnez/agent_async.go`)**:
   - In `runStart` and `runResume`, when running foreground without an explicit `--timeout` or `HTO` override, monitor the 60s turn deadline.
   - When the deadline is reached, cleanly detach worker execution, persist the session record as `running`, print the standardized background-wait instruction, and exit cleanly.

3. **Stream Protocol Guidance (`cmd/harnez/agent_stream.go`)**:
   - Update `[wait: ...]` and turn stream messages so they explicitly instruct the caller that long turns detach and reattachment is done via a host background task without polling or schedules.

4. **Integration Testing (`cmd/harnez/agent_test.go`)**:
   - Test that a simulated slow turn (exceeding foreground timeout) cleanly transitions to a detached session, emits the zero-polling / zero-scheduling background reattach instruction, and leaves a valid session resumable/waitable via `harnez agent wait`.
   - Verify `make test-q1` passes under full Quota-1 enforcement.

---

## 4. Progress & Milestones

- [x] **M1 (exec timeout exemption for wait)**: Delivered in `46610db`. Implemented `isHarnezAgentWait` in `cmd/harnez/exec.go` and comprehensive unit tests in `cmd/harnez/exec_test.go`. Direct and bash-wrapped `harnez agent wait` resolve to timeout 0 while explicit flags/HTO and regular commands are preserved.
- [x] **M2 (clean 60s foreground turn detach & reattach guidance)**: Delivered in `2559134` and `355bec5`. Implemented foreground worker detachment, independent process groups via `Setsid`, zero-polling/scheduling guidance output, and test-mode guards.
- [x] **M2 Refinements (Pre-Commit Review Gate Findings)**: Delivered in `e156192`. Implemented JSON detach guidance output and real CLI worker-session test coverage.

