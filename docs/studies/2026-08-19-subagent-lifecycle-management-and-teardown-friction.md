# Study: Subagent Lifecycle Management and Teardown Friction

**Date**: 2026-08-19  
**Scope**: Multi-agent orchestration, subagent lifecycle state machines, workspace preservation, and process teardown hygiene across Claude Code and Google Antigravity (AGY)  
**Related Issues**: [Issue 038](../../issues/038-research-subagent-lifecycle-and-cleanup-friction.md)  
**Related Docs**: [Orchestrated Subagents Feedback](../feedback/2026-08-19-orchestrated-subagents-process-hygiene-and-review-loops.md), [Git Conventions](../lang/Git.md), [Canary Development](../other/Canary.md)  
**Status**: Completed research and architectural synthesis  

---

## 1. Problem Statement: Zombie Accumulation vs. Premature Teardown

In agentic coding platforms (such as Google Antigravity, Claude Code, and Prime Agent harnesses), parent orchestrators spawn subagents, background watch commands, schedule timers, and test subprocesses. Managing the lifecycle of these secondary entities presents an acute trade-off:

1. **Zombie Entity Accumulation**: If teardown is manual or omitted, background processes linger indefinitely. They consume memory, hold open file descriptors and port locks, exhaust upstream LLM API rate limits, and trigger spurious reactive wakeups in parent orchestrators.
2. **Aggressive Premature Teardown**: If teardown hooks are applied naively (e.g., blanket termination at the end of every prompt turn), system reliability degrades:
   - **Interrupted Long-Running Work**: Compilation, fuzzing runs, container builds, and test sweeps extending beyond a single turn are killed mid-execution.
   - **Workspace Eviction & Code Loss**: In isolated branch workspaces (`Workspace: "branch"`), terminating a subagent deletes its isolated working directory. Uncommitted edits or pending refactors are permanently lost.
   - **Conversational Amnesia & Token Overhead**: Terminating reusable idle subagents discards their in-memory session context, forcing expensive re-spawning and context re-ingestion for follow-up questions.
   - **Signal Races & Dropped Messages**: When a subagent delivers its final findings concurrently with an automated reap trigger, race conditions cause message drops or double-kill errors.
   - **Cascading Orphan Trees**: Terminating a subagent without recursively draining its subprocesses leaves orphaned grandchild daemons running detached.

A robust lifecycle model must replace naive turn-based kills with an **archetype-aware, state-driven lifecycle framework**.

---

## 2. Subagent Archetype Taxonomy

Teardown policies must be parameterized by entity archetype. Entities fall into three core categories:

| Dimension | Ephemeral Advisor / Auditor | Stateful Branch Worker | Background Daemon / Watcher |
|:---|:---|:---|:---|
| **Role & Purpose** | Code review, spec audit, web research, read-only analysis | Feature implementation, refactoring, bug fixes, test creation | File watchers, test runners, dev servers, proxy sidecars |
| **Workspace Mode** | `Workspace: "inherit"` (read-only) | `Workspace: "branch"` or `Workspace: "share"` | `Workspace: "inherit"` (process daemon) |
| **Lifecycle Scope** | **Turn-scoped**: deliver findings and yield | **Milestone-scoped**: survives across turns until merged or explicitly discarded | **Session-scoped**: persists across the entire CLI session until explicitly halted |
| **Teardown Policy** | **Auto-reap immediately** once idle after message delivery | **Preserve workspace**; require explicit merge handshake or patch export before kill | **Graceful drain on session close**; monitor runtime health via heartbeat lease |
| **Premature Kill Risk** | Low (idempotent research; no dirty state) | High (uncommitted branch edits destroyed on directory removal) | Medium (stale lockfiles, port binding collisions) |

---

## 3. Lifecycle State Transitions & Signal Flow

Subagent and task execution transitions through well-defined lifecycle states:

```
               ┌───────────────┐
               │   SPAWNING    │ (allocate conversation ID, branch workspace, tag archetype)
               └───────┬───────┘
                       │
                       ▼
               ┌───────────────┐
      ┌───────►│    RUNNING    │◄─────────────────────────┐
      │        └───────┬───────┘                          │
      │                │                                  │
      │                │ send_message (yield findings)    │ parent send_message
      │                ▼                                  │ (follow-up task)
      │        ┌───────────────┐                          │
      │        │ IDLE_REUSABLE ├──────────────────────────┘
      │        └───────┬───────┘
      │                │
      │                ├─────────────────────────────┐
      │                │                             │
      │                ▼ (Auto-reap Ephemeral)       ▼ (Parent Kill / Finalizer)
      │        ┌───────────────┐             ┌───────────────┐
      │        │ AUTO_DRAINING │             │ GRACEFUL_DRAIN│
      │        └───────┬───────┘             └───────┬───────┘
      │                │ (clean workspace)           │ (SIGTERM -> 5s -> SIGKILL; export diff)
      │                ▼                             ▼
      │        ┌───────────────┐             ┌───────────────┐
      │        │  TERMINATED   │             │   COMPLETED   │
      │        └───────────────┘             └───────────────┘
```

### 3.1 State Semantics

1. **`SPAWNING`**: Conversation registration, workspace provisioning, and archetype tagging (`ephemeral`, `worker`, `daemon`).
2. **`RUNNING`**: Active LLM inference, tool execution, or subprocess execution.
3. **`IDLE_REUSABLE`**: Subagent completed its prompt turn and sent its message to the parent. In this state, the agent consumes 0 CPU and 0 tokens, awaiting reuse.
4. **`AUTO_DRAINING`**: For `ephemeral` advisors, transitions automatically to reap when idle at parent turn boundaries.
5. **`GRACEFUL_DRAIN`**: For `worker` and `daemon` entities:
   - Sends `SIGTERM` to the process group.
   - Starts a 5-second graceful drain timer.
   - If the process or subagent has not exited after 5s, escalates to `SIGKILL`.
   - Worktree state is checked prior to directory deletion.
6. **`TERMINATED` / `COMPLETED`**: Workspaces cleaned up, lingering file descriptors closed; transcript logs preserved in `<appDataDir>/brain/<id>/.system_generated/logs/`.

---

## 4. Core Invariants & Safety Mechanisms

### 4.1 Pre-Kill Workspace Git Diff & Patch Export Guard

When `manage_subagents` receives a `kill` command for an agent with `Workspace: "branch"`, destroying the branch directory risks deleting uncommitted work. 

**Invariant**: *No isolated workspace may be deleted without verifying working tree cleanliness.*

**Protocol**:
1. Execute `git status --porcelain` in the branched directory.
2. If modified, untracked, or staged files exist:
   - Generate a unified diff: `git diff HEAD` and `git status --porcelain` untracked files.
   - Save the patch to `<appDataDir>/brain/<parent-id>/artifacts/patches/<subagent-id>.patch`.
   - Log an explicit warning in the parent's conversation transcript containing the artifact path.
3. Proceed with branch directory removal (`rm -rf <branched-dir>`).

### 4.2 Lease-Based Heartbeats & Self-Reaping Daemons

Background processes and test runners often outlive disconnected parents (e.g. parent agent crashed or terminated by user).

**Invariant**: *Background daemons must not run detached without an active supervisor lease.*

**Protocol**:
- Background daemons acquire an ephemeral lease with a time-to-live (TTL, e.g. 300s) written to `~/.harnez/leases/<task-id>.json`.
- Active parent turn executions touch/renew the lease file timestamp.
- Background watchdog routines check lease age; if $\Delta t > 2 \times \text{TTL}$ with no parent renewal, the background task executes a graceful self-drain (`SIGTERM` followed by exit).

### 4.3 Message Boundary Synchronization

A common failure mode is reaping an idle subagent while an in-flight `send_message` payload is still traversing the message broker.

**Invariant**: *Teardown hooks must synchronize against message delivery receipts.*

**Protocol**:
- Teardown hooks must check subagent state via `manage_subagents` (`list`).
- If an agent is in state `waiting_for_input` or actively flushing a message, the reap operation blocks until the message is acknowledged and the state settles to `idle`.

---

## 5. Benchmarking & Evaluation Criteria

To evaluate lifecycle management implementations and guard against regressions in future harness releases, measure the following five metrics:

| Metric | Formula / Measurement | Target Objective |
|:---|:---|:---|
| **Zombie Leakage Rate** | Lingering background tasks or orphan subagents per 100 finished sessions | **$0\%$** |
| **False Interruption Rate** | Percentage of active test runners or multi-turn builds killed prematurely | **$< 0.1\%$** |
| **Workspace Data Loss Count** | Total occurrences of uncommitted code lost to subagent workspace eviction | **Exactly $0$** |
| **Context Re-spawn Overhead** | Extra tokens consumed re-initializing destroyed subagents for follow-ups | **$< 5\%$ overhead** |
| **Teardown Signal Race Rate** | Percentage of teardowns resulting in dropped messages or double-kill error codes | **$0\%$** |

---

## 6. Recommendations for Harnez Architecture

1. **Turn-End Ephemeral Reaper**: Implement an automatic turn-end sweep in `harnez` orchestration that queries active subagents and terminates idle `research` / `ephemeral` advisors while leaving `branch` workers untouched.
2. **Workspace Safety Guard**: Add pre-kill patch extraction into the harness subagent teardown pipeline.
3. **Session-Scoped Finalizer**: Provide `harnez cleanup` as a deterministic session finalizer that gracefully terminates all daemons, clears stale watch commands, and cleans orphaned leases.
