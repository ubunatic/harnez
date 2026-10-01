# 665 — Add /harnez-selftest skill and agent selftest CLI command for background task execution verification

**Status**: Open
**Priority**: P2 (Medium)
**Severity**: Minor
**Category**: Feature
**Related**: [agent command](../cmd/harnez/agent.go)

---

## 1. Problem & Motivation

Agents operating across different harnesses (Claude, Codex, Antigravity, etc.) have distinct native background job/task management mechanisms.
We need an automated verification mechanism and self-test skill (`/harnez-selftest`) along with a CLI command (`harnez agent selftest --step <name>`) to verify that agents can correctly schedule, monitor, and await background execution in their native environments without polling or failing the background handoff.

To prevent agents from guessing or shortcutting the step sequence, hidden step names should not be leaked to agents (e.g., in headless non-TTY help output), so agents must follow runtime step instructions.

## 2. Technical Specification / Findings

1. **CLI Command (`harnez agent selftest`)**:
   - Accepts `--step <name>` (and optional args like `--step confirm-running [<duration>]`).
   - Hidden step names / flag values are only shown to human users in an interactive TTY; hidden from non-interactive agent inspection so agent instructions guide the progression.
   - Tracks session state per host agent identity (e.g., using agent identifier/session ID) writing step sequence records to a `/tmp` state file (e.g., `/tmp/harnez-selftest-<id>`).
   - Validates correct progression and ordering during `--step verify`:
     - Clean exit (code 0) if step sequence and background execution order are valid.
     - Non-zero exit with descriptive error diagnostic if out-of-order, missed steps, or premature completion occurred.

2. **Step Progression Flow**:
   - **Step 1 (Intro)**: Briefly summarize the task.
   - **Step 2 (`--step hello`)**: Instruct agent to:
     1. First dispatch `harnez agent selftest --step background` in the agent's **native** background as Task/Job (waking on completion).
     2. Second run `harnez agent selftest --step confirm` to confirm that the job now runs as a native Task/Job in the agent's background system.
   - **Step 3 (`--step confirm`)**: Completes before the background step finishes; instructs agent to inspect its native background job/task list, check if/how long the job has been running, and confirm with `harnez agent selftest --step confirm-running [<duration>]`.
   - **Step 4 (`--step background`)**: Runs in the background for a duration, completes, and notifies/wakes the agent to run `harnez agent selftest --step verify`.
   - **Step 5 (`--step verify`)**: Validates the recorded `/tmp` progression log and exits clean on success or reports errors.

3. **Skill (`/harnez-selftest`)**:
   - Skill definition guiding agents through running the initial step and executing each phase according to runtime instructions.

## 3. Implementation & Verification Plan

- **/goal**: `/goal Implement the /harnez-selftest skill and harnez agent selftest CLI command supporting step progression, host agent identity tracking, /tmp sequence verification, and TTY-only hidden step visibility; stop and report when blocked on a user decision or denied permission.`
- Implement `harnez agent selftest` subcommand and step handler in `cmd/harnez/` / `internal/agent/`.
- Add test coverage for step transitions, valid sequence, out-of-order errors, and TTY-conditional output.
- Add `/harnez-selftest` skill definition and verify end-to-end execution.
