# 611 — Host orchestrator polling loop during synchronous agent tasks

**Status**: Open
**Priority**: P2 (Medium)
**Severity**: Moderate
**Category**: Behavior / Experience
**Related**: #547 (clean 60s turn detach and HTO=0 wait reattach), 572 (async subagent dispatch), 578 (safely discoverable agent workflow), 589 (background shell guidance), 608 (CLI backgrounding)

---

## 1. Problem & Motivation
When the Host Orchestrator in an interactive pair programming environment dispatches or resumes a child agent (via `harnez agent start` or `harnez agent resume`), the command is launched into the background as an async task if it takes longer than a short wait threshold.

In practice, agents often fall into tight polling patterns:
- Checking task status via `manage_task status` or repeatedly reading the task log (`task-xxx.log`).
- Setting recurring timers (`schedule` / cron) to poll child workers.
- This causes high tool call volume, consumes unnecessary context tokens, and risks tripping rate warnings (`harnez rate`).

### Why the Host Chose to Poll (Root Cause Analysis)
1. **Lack of Synchronous Execution Guarantee**: A short default timeout in tools (`WaitMsBeforeAsync` or execution timeouts) causes long-running agent starts to quickly drop into the background.
2. **Ambiguity on Long-Running Agent State**: Without a dedicated event/stream notification protocol or clear detachment convention in the skill instructions, the host agent tries to actively observe the child worker's turn output (e.g. waiting for `[done]` / confirmation banners).
3. **Execution Timeout Traps (`HTO`)**: Child agent turns get killed by default execution timeouts (e.g. `HTO=1m` killing a resume turn with `exit status 137`), reinforcing the host's instinct to monitor process health closely to detect timeouts early.

---

## 2. Technical Findings & Experience Report

1. **Reactive Wakeup Contract**:
   - The hosting environment (e.g., Antigravity/AGY) already provides reactive wakeups: when the background process finishes, a high-priority notification message with output is delivered automatically into context.
   - Polling loops provide zero additional responsiveness because LLM turns are discrete; active tool loops only churn turns without accelerating worker completion.
2. **Explicit Detachment vs Background Stream**:
   - See issue #547: rather than killing agent turns at 60s or running into ambiguous states, `harnez agent start` and `resume` allow synchronous completion under 60s, but cleanly detach after 60s with explicit instructions to reattach via `harnez agent wait <session>` in a background task.
   - Reattaching via `harnez agent wait` implies no timeout (`timeout = 0`), eliminating SIGKILL traps.
   - The host instruction explicitly mandates: **Do NOT poll. Do NOT schedule timers or cron jobs.**

---

## 3. Proposed Improvements

1. **Consolidated with Issue #547**:
   - Issue #547 defines the engine-level detach mechanism at 60s and the default `timeout = 0` for `harnez agent wait`.
2. **Host Rule Guidance (Zero-Polling & Zero-Scheduling Invariant)**:
   - In `docs/practices/AgenticLoop.md` and `.harnez/rules/Subagents.md`, explicitly state the zero-polling and zero-scheduling invariant: once a subagent or background task is dispatched, the host must not poll logs, loop on status, or set timers. The host must yield the turn immediately and wait for the reactive completion message.
3. **Clear Stream / Progress Feedback**:
   - When detached/backgrounded, emit the directive instruction:
     `"Agent turn exceeded 60s and has been cleanly detached to the background. Do NOT poll. Do NOT schedule timers or cron jobs. Reattach by launching a background job: harnez agent wait <name>. When the background job finishes, your environment will automatically notify this session."`

---


## 4. Field Feedback from AGY Host Session (2026-09-27)

During lean sprints on Loom issues 138 and 140, an interactive AGY host orchestrator tested this workflow in practice. Key observations and working patterns:

1. **Zero-Coding Invariant & Leaf Worker Delegation**:
   - The host maintains context discipline: plans milestones in the issue ticket, reviews commit diffs (`git log -n 1 --stat`, `git diff HEAD~1`), and runs verification commands (`go test ./...`, `make install`).
   - All source code edits, unit tests, and bug fixes are delegated to a low-cost leaf worker (`codex:luna:low`).

2. **Crucial Role of `HTO=0` for Non-Trivial Sprints**:
   - `harnez agent start` initially timed out on M1 after 1m0s with code 137.
   - Setting `HTO=0` on all agent commands (`HTO=0 harnez agent start ...`, `HTO=0 harnez agent resume ...`) allowed long test runs and multi-file edits to complete reliably without hitting arbitrary kill boundaries.

3. **Clean Reactive Wakeup without Busy Polling**:
   - When `run_command` sends a long-running agent command into the background, the hosting environment automatically delivers the completion output as a high-priority system notification.
   - Yielding the turn immediately without polling `manage_task` or reading `.log` files kept context small and token usage minimal across multi-milestone sprints.

4. **CLI & Native Tool Synergy**:
   - Combining Harnez CLI lifecycle commands (`harnez issues new/open/close`, `harnez agent start/resume/stop`, `harnez find`, `harnez apply`) with native shell (`run_command`), targeted file viewing (`view_file`), and ticket editing (`write_to_file`) provided a smooth, fully autonomous execution pipeline.

5. **Verified Cross-Session Peer Messaging (`send_message`)**:
   - Direct asynchronous peer messaging between separate host sessions (e.g. `loom` assistant and `cati` media browser) was proven end-to-end using native `send_message` targeting the peer's conversation ID (`fa46cc11-...`).
   - The recipient session successfully received the handoff notice and replied with an instant reactive `pong`, proving that multi-session peer coordination works smoothly without shared polling loops or file locks.


