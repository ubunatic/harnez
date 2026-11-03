# 1112 — Auto mode refuses Bash calls the gear-shim hook rewrote, doubling tool calls

**Status**: Open
**Priority**: P2 (Medium)
**Severity**: Minor
**Category**: Bug
**Related**:

---

## Problem
Reported 2026-11-03 by the lmcoder session. The harnez PreToolUse hook rewrites Bash calls to `⚙ ...` (for example `HTO=0 ⚙ bash -c 'cd <repo>; ...'`); see `formatGearRewrite` in `internal/exec/exec.go`. In Claude Code auto mode, the call is then refused with "a hook changed this call's input after auto mode reviewed it". The agent must resend the rewritten command verbatim, so every rewritten command costs two tool calls. Five background calls hit this in one batch.

`⚙` is the harnez multicall symlink installed by `harnez apply` (`GearSymlinkTargets`, `internal/claude/apply.go`). It runs the command through `harnez exec`, which records telemetry and can distill output.

## Goal
/goal In auto mode, Bash calls go through ⚙ without a refused first attempt. Stop and report if Claude Code offers no supported way to do this.

## Options
- Detect auto mode in the hook and skip the rewrite. The bash shim (`~/.harnez/shims/bash`) may already cover telemetry.
- Tell agents to write `⚙ <cmd>` themselves, so the hook has nothing to rewrite. Check that the hook passes such commands through unchanged.

## Done when
- A hook test shows no `updatedInput` for the chosen auto-mode path, or for an already-routed `⚙` command.
- The agent docs say what agents should type.

## Findings 2026-11-03 (lmcoder)
- A call that already starts with `⚙ bash ...` passes through unchanged when it is the only call in the response.
- If the hook rewrites even one plain call in a response with several Bash calls, auto mode refuses every other call in that response ("another call in this response was changed by a hook"). Each one must be resent alone.
- Writing `⚙` on every call works, but one forgotten prefix breaks the whole batch. A per-agent habit is fragile, so the hook-side fix (skip the rewrite in auto mode) is the better route.
