# 601 — Add /wrap (or /handoff) invoke-only session close and handoff skill

**Status**: Closed — wrap skill shipped: docs/commands/Wrap.md registered in config.yaml as 'wrap'
**Priority**: P2 (Medium)
**Severity**: Minor
**Category**: Skills / Workflow / Handoff
**Related**: `docs/commands/evergreen.md`, `docs/commands/Next.md`, `docs/practices/AgenticLoop.md`

---

## Goal

`/goal`: Add an invoke-only `/wrap` (or `/handoff`) skill that cleanly finishes a session, updates docs/issues, files observed non-nitpick issues, commits working tree state ("as clean as possible", or with a clear handoff ticket if work is partial/broken), and ensures what was learned in the current session is preserved for the next session.

## 1. Problem & Motivation

When concluding an interactive session or preparing for context resets / harness restarts, agents often leave dangling context in the chat transcript that is lost to subsequent sessions.

Developers need a single command to:
1. Ensure all background helpers and subagents are terminated (Zero-Zombie guarantee).
2. Sync tracker status (`issues/`, `issues/README.md`) and evergreen architecture docs (`docs/*.md`).
3. File distinct, non-nitpick issues for observed bugs, broken assumptions, or unfinished threads.
4. Commit repository state as clean as possible (or commit with a dedicated handoff issue ticket if partially completed or broken).
5. Produce a high-signal handoff summary to easily pick up in a fresh session.

## 2. Assessment: Distinguishing `/wrap` vs `/evergreen`

| Dimension | `/evergreen` | `/wrap` (Session Close / Handoff) |
| :--- | :--- | :--- |
| **Primary Scope** | Knowledge curation & architecture synthesis into durable PascalCase `docs/*.md`. | Session lifecycle closure, operational handoff, process hygiene, and git/issue sync. |
| **Trigger Timing** | Any time substantial architectural or design insights occur. | End of a session, before context reset, or before handoff to another agent/human. |
| **Code & Git State** | Focuses on documentation editing; does not manage git commits or broken worktree handoffs. | Commits working tree cleanly (or commits with a handoff ticket), stops subagents/daemons. |
| **Issue Handling** | Updates issues index and cleans stale docs. | Closes completed sprint tickets, files newly observed non-nitpick blockers/bugs, updates tracker. |
| **Output Artifact** | Updated `docs/*.md` evergreen files. | Clean git commit + high-signal session handoff summary for the next session. |

## 3. Technical Specification & Skill Contract

### Skill Attributes
- **Name**: `wrap` (or `handoff`)
- **Mode**: User-invocable only (`disable-model-invocation: true`).
- **Location**: `docs/commands/Wrap.md` (and registered in `config.yaml` / skill templates).

### Execution Checklist
1. **Hygiene & Subagent Teardown**: Stop all active background subagents and helpers (`harnez agent stop`, `manage_task`).
2. **Issue Tracker Sync**: Verify all completed tickets in the session are marked `Closed` with reasoning and indexed (`harnez index`).
3. **Filing Observed Issues**: File any non-nitpick findings, broken assumptions, or unfinished follow-ups discovered during the session via `harnez issues new`.
4. **Git Commit & State Preservation**:
   - If working tree is clean: verify `git status`.
   - If working tree has working changes: commit with a descriptive message.
   - If working tree has partial/broken state: commit state and explicitly reference the handoff ticket number so the next agent knows where to resume.
5. **Durable Learnings**: If major architectural or tooling insights were gained, update the relevant `docs/*.md` (or cross-invoke `/evergreen` rules).
6. **Handoff Report**: Output a compact summary (Current HEAD, active open issues, recommended next command).

## 4. Implementation Plan

- **M1**: Author `docs/commands/Wrap.md` with skill frontmatter, execution checklist, and distinction from `/evergreen`.
- **M2**: Register in `config.yaml` and add unit test in `internal/claude`.
- **M3**: Run `make test-q1` and `make install`.

## 5. Acceptance Criteria

- `/wrap` skill registered and installed.
- Clear distinction between `/wrap` (session close & commit handoff) and `/evergreen` (architecture knowledge curation).
- Unit tests verify presence in all configured skill targets.
