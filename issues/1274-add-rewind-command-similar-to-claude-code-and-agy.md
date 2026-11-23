# 1274 — Add `/rewind` command similar to Claude Code and AGY

**Status**: Open
**Priority**: P2 (Medium)
**Severity**: Moderate
**Category**: Feature
**Related**: `skills/`, `pi/`

---

## Goal

`/goal Add a /rewind command in harnez providing checkpoint/rewind ergonomics similar to Claude Code and AGY, or stop and report when blocked on a user decision (scope of rewind targets/surface) or denied permission. On a loop, hard architectural blocker, or no progress, clear the goal and write a failure report explaining why it failed.`

## 1. Problem & Motivation

Claude Code and AGY provide a `/rewind` command that lets users step back to an earlier turn or checkpoint when a session takes a wrong turn. Harnez currently has no equivalent `/rewind` command or skill.

## 2. Scope & Open Questions

Before starting implementation, check live skills, Pi extensions, and session/checkpoint plumbing in the codebase, and clarify with the user:
- **Target surface**: Should `/rewind` be a Pi slash command, a Harnez skill (`skills/rewind/`), a CLI subcommand (`harnez rewind`), or a combination?
- **Rewind scope**: Should it rewind conversation history, workspace/file state (e.g. git checkpoints), or both (with a choice like Claude Code's "restore code / restore conversation / restore both")?
- **Safety**: How should dirty working-tree changes, commits already made, and subagent sessions be handled?

## 3. Acceptance Criteria

- Users can invoke `/rewind` on the agreed surface(s) to list recent checkpoints/turns and safely restore the selected state.
- Destructive workspace restores require clear preview/confirmation or safe backup.
- Covered by unit/integration tests and `make test-q1` passes.
