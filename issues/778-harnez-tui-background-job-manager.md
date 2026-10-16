# 778 — harnez tui Background Job Manager

**Status**: Draft
**Priority**: P2 (Medium)
**Severity**: Feature
**Category**: Interactive TUI / Background Execution
**Related**: #774, #775, #049, #533

---

## 1. Problem & Motivation
In interactive coding workflows, developers frequently launch long-running background tasks (such as test suites `make test`, builds, or subagent runs) while continuing to converse with the main agent. Without a dedicated job manager in `harnez tui`, background tasks become invisible, difficult to monitor, or impossible to cancel safely.

## 2. Technical Specification & Design

### Background Job Panel & Drawer
1. **Visual Drawer**:
   - TUI drawer/overlay listing all active and recent background jobs (`[1] make test (Running)  [2] harnez agent (Completed)`).
   - Shows job ID, command string, runtime duration, status (Running, Succeeded, Failed, Cancelled), and PID.
2. **Live Log Inspector**:
   - Selecting a background job (or pressing its numeric hotkey `[1..N]`) opens a split/overlay view displaying the live stdout/stderr log stream of the job.
3. **Job Cancellation & Process Hygiene**:
   - User can cancel single or multiple background jobs with a dedicated key (e.g. `Ctrl+K` or `d` in job manager).
   - Process cancellation sends `SIGTERM` followed by a bounded `SIGKILL` to ensure clean process tree teardown without leaving orphaned background processes.

## 3. Implementation & Verification Plan
- Implement `internal/tui/jobs/` background runner and job registry.
- Connect job events to Loom TUI reactive model updates.
- Unit test job spawning, output buffer streaming, process tree cancellation, and state transitions.
- Verify that cancelling a background task cleanly terminates all child processes.
