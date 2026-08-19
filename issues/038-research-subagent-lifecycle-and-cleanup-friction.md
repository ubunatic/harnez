# 038 — Research: Automated Subagent Lifecycle Hooks & Teardown Friction

**Status**: Closed (Research Complete)  
**Category**: Architecture / Agentic Orchestration  
**Related**: [Feedback: 2026-08-19](../docs/feedback/2026-08-19-orchestrated-subagents-process-hygiene-and-review-loops.md), [Study: 2026-08-19](../docs/studies/2026-08-19-subagent-lifecycle-management-and-teardown-friction.md), [AGENTS.md](../../AGENTS.md), [Git.md](../docs/lang/Git.md)

---

## 1. Problem Statement & Motivation

In multi-agent architectures (e.g. Antigravity, Claude Code harnesses), agents frequently spawn subagents, background watch commands, schedule timers, and test subprocesses. When parent tasks finish or fail, background entities often linger as zombies—consuming memory, holding open file handles, exhausting API concurrency quotas, and triggering spurious reactive wakeups.

While automatic lifecycle teardown hooks (e.g., killing all child subagents/timers upon parent turn end or session exit) solve zombie accumulation, naive or aggressive cleanup introduces severe operational friction:

- **Premature Interruption of Long-Running Tasks**: Tasks that legitimately outlive a single prompt turn (e.g. test suites, fuzzing runs, container builds, model inference checks) get killed mid-flight when cleanup is tied naively to turn boundaries.
- **Workspace Eviction & Data Loss**: In systems with workspace isolation (`Workspace: "branch"`), terminating a subagent deletes its branched directory. If unmerged code edits or artifacts exist, aggressive kills cause irreversible data loss.
- **Conversational Amnesia & Spawn Overhead**: Killing idle subagents destroys their in-memory session. Follow-up inquiries require re-spawning and re-ingesting context, adding significant latency and token costs.
- **Signal Races & Dropped Messages**: When a subagent completes and sends its final report concurrently with a teardown hook, message deliveries can fail or result in double-kill error noise.
- **Cascading Subagent Orphans**: Subagents spawning nested children or background processes require orderly tree-teardown to prevent orphaned grandchild processes.

---

## 2. Subagent & Background Entity Taxonomy

To prevent destructive cleanup, teardown policies must distinguish between different entity archetypes:

| Dimension | Ephemeral Advisor / Auditor | Stateful Branch Worker | Background Daemon / Watcher |
|:---|:---|:---|:---|
| **Role & Purpose** | Code review, spec audit, web lookup, read-only analysis | Feature implementation, refactoring, test fixes | File watchers, test runners, local dev servers |
| **Workspace Mode** | `Workspace: "inherit"` (read-only) | `Workspace: "branch"` or `"share"` | `Workspace: "inherit"` (process daemon) |
| **Persistence Need** | Turn-scoped (deliver findings & yield) | Milestone-scoped (survives turns until merge) | Session-scoped (persists until user stops) |
| **Teardown Policy** | **Auto-reap immediately** once idle after message delivery | **Preserve workspace**; require explicit merge/drain handshake | **Drain on session close**; monitor via heartbeat lease |
| **Failure Mode** | Low risk on premature kill (idempotent) | High risk (lost code / uncommitted branch work) | Medium risk (lingering zombie process/port lock) |

---

## 3. Lifecycle State Machine Design

```
               ┌───────────────┐
               │   SPAWNING    │
               └───────┬───────┘
                       │ (initialize context / branch workspace)
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
      │                │ (no dirty state)            │ (flush branch / export diff)
      │                ▼                             ▼
      │        ┌───────────────┐             ┌───────────────┐
      │        │  TERMINATED   │             │   COMPLETED   │
      │        └───────────────┘             └───────────────┘
```

### 3.1 State Definitions & Invariants

- **`SPAWNING`**: Resource allocation, conversation ID registration, and ownership tagging (`ephemeral`, `worker`, `daemon`).
- **`RUNNING`**: Active computation or tool execution.
- **`IDLE_REUSABLE`**: Subagent has messaged findings and yielded. The subagent is paused; no active CPU or token spend occurs.
- **`AUTO_DRAINING`**: Triggered automatically for `ephemeral` agents upon message delivery or parent turn completion. Reaped cleanly without user prompt.
- **`GRACEFUL_DRAIN`**: Triggered for `worker` and `daemon` entities. The harness checks for dirty workspaces or pending file diffs:
  - If unmerged diffs exist, an automatic patch artifact is saved to `<appDataDir>/brain/<id>/artifacts/` before workspace eviction.
  - Background processes receive `SIGTERM`, wait for a 5s drain window, and escalate to `SIGKILL` only if unresponsive.
- **`TERMINATED` / `COMPLETED`**: Workspace and resources released; transcript and execution logs preserved.

---

## 4. Architectural Patterns for Robust Teardown

### 4.1 Lease-Based Heartbeat & Self-Reaping
- **Lease Mechanism**: Background tasks and long-running subagents acquire a time-to-live lease (e.g., TTL = 300s).
- **Heartbeat Renewal**: Active communication or parent turns renew the lease.
- **Self-Termination**: If a background process or worker subagent receives no heartbeat or task input within $2 \times \text{TTL}$ and has no active child processes, it gracefully self-terminates to prevent zombie accumulation after parent crashes.

### 4.2 Workspace Preservation Guard
- **Pre-Kill Diff Check**: When `manage_subagents` receives a `kill` command for a branched workspace:
  - Check `git status --porcelain` in the branched directory.
  - If uncommitted changes exist, generate a patch file in the parent's artifact directory before deleting the branch.

### 4.3 Turn-Scoped vs. Session-Scoped Cleanup Hooks
- **Turn-Scoped Hook**:
  - Automatically queries `manage_subagents` (`list`) and `manage_task` (`list`).
  - Kills all `idle` agents with role `research` or marked `ephemeral`.
  - Cancels completed one-shot timers (`schedule`).
- **Session-Scoped Finalizer (`harnez cleanup`)**:
  - Dispatched when the entire session concludes.
  - Drains all remaining background command tasks, daemons, and worker subagents.

---

## 5. Case Study Evaluation Criteria

To benchmark and evaluate automated lifecycle management in future case studies, measure the following metrics:

| Metric | Definition | Target Goal |
|:---|:---|:---|
| **Zombie Leakage Rate** | Number of lingering background processes / orphan subagents per 100 turns | $0\%$ |
| **False Interruption Rate** | Percentage of active test runners / builds killed prematurely | $< 0.1\%$ |
| **Workspace Data Loss Count** | Occurrences of uncommitted branch work lost to subagent kill | Exactly $0$ |
| **Context Re-spawn Overhead** | Extra tokens spent re-initializing identical subagents due to over-eager kills | $< 5\%$ overhead |
| **Teardown Signal Race Rate** | Percentage of teardown operations resulting in error codes or dropped messages | $0\%$ |

---

## 6. Action Items & Roadmap

- [x] Publish case study findings in `docs/studies/2026-08-19-subagent-lifecycle-management-and-teardown-friction.md`.
- [ ] Prototype turn-end ephemeral advisor cleanup hook.
- [ ] Implement pre-teardown diff preservation guard for branch workspaces.
- [ ] Add lease/heartbeat timeout logic for detached background command tasks.

