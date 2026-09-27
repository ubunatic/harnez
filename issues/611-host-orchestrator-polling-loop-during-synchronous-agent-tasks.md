# 611 — Host orchestrator polling loop during synchronous agent tasks

**Status**: Open
**Priority**: P2 (Medium)
**Severity**: Moderate
**Category**: Behavior / Experience
**Related**: 572 (async subagent dispatch), 578 (safely discoverable agent workflow), 589 (background shell guidance)

---

## 1. Problem & Motivation
When the Host Orchestrator in an interactive pair programming environment dispatches or resumes a child agent (via `harnez agent start` or `harnez agent resume`), the command is launched into the background as an async task if it takes longer than a short wait threshold.

In practice, agents often fall into tight polling patterns:
- Checking task status via `manage_task status` or repeatedly reading the task log (`task-xxx.log`).
- This causes high tool call volume, consumes unnecessary context tokens, and risks tripping rate warnings (`harnez rate`).

### Why the Host Chose to Poll (Root Cause Analysis)
1. **Lack of Synchronous Execution Guarantee**: A short default timeout in tools (`WaitMsBeforeAsync` or execution timeouts) causes long-running agent starts to quickly drop into the background.
2. **Ambiguity on Long-Running Agent State**: Without a dedicated event/stream notification protocol or clear detachment convention in the skill instructions, the host agent tries to actively observe the child worker's turn output (e.g. waiting for `[done]` / confirmation banners).
3. **Execution Timeout Traps (`HTO`)**: Child agent turns can get killed by default execution timeouts (e.g. `HTO=1m` killing a resume turn with `exit status 137`), reinforcing the host's instinct to monitor process health closely to detect timeouts early.

---

## 2. Technical Findings & Experience Report

1. **Reactive Wakeup Contract**:
   - The hosting environment (e.g., Antigravity/AGY) already provides reactive wakeups: when the background process finishes, a high-priority notification message with output is delivered automatically into context.
   - Polling loops provide zero additional responsiveness because LLM turns are discrete; active tool loops only churn turns without accelerating worker completion.
2. **Explicit Detachment vs Background Stream**:
   - For interactive orchestrators, commands like `harnez agent start` and `harnez agent resume` either need clear detached semantics (e.g. `--detach` + waiting explicitly on a single wake event) or explicit guidance in `.harnez/rules/Subagents.md` / `LeanSprints.md` instructing the host to yield turns immediately rather than inspecting log files in loops.

---

## 3. Proposed Improvements

1. **Update Lean Sprint & Agentic Rules**:
   - In `docs/practices/AgenticLoop.md` and `.harnez/rules/Subagents.md`, explicitly state the zero-polling invariant: once a subagent or background task is dispatched, the host must not poll logs or loop on status. The host must yield the turn immediately and wait for the reactive completion message.
2. **Auto-set `HTO=0` for Subagent Commands**:
   - Ensure subagent commands (`harnez agent start/resume`) automatically execute without default 1m timeouts, avoiding unexpected SIGKILLs (`exit status 137`) that trigger host panic-checking.
3. **Clear Stream / Progress Feedback**:
   - When detached/backgrounded, provide a clear machine-friendly instruction: `"Agent is executing asynchronously in session <id>. Do not poll. Await completion signal."`

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

