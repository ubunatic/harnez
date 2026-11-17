# 1240 — Native Read guard blocks files whose content is no longer in context, and Edit then has no path

**Status**: Open
**Priority**: P2 (Medium)
**Severity**: Minor
**Category**: Bug
**Related**: [405](issues/405-implement-strict-pretooluse-hook-to-intercept-native-view-file-and-read-tool-calls-with-harnez-read.md) (introduced the hook), [1079](issues/1079-harnez-read-guard-allow-native-read-with-offset-limit-harnez-read-l-range-failure-from-haiku-worker.md) (ranged reads)

---

## 1. Problem & Motivation

The PreToolUse guard refuses a native `Read` of a file the session "already read natively". The check looks at the session, not at the current context window. After `/compact`, the agent no longer holds the file's content, but the guard still refuses the native `Read`. The `Edit` tool needs a native `Read` first, so the agent has no way to edit that file through the normal tool. It falls back to Bash writes, which skip `Edit`'s exact-match check.

This costs turns in every long session and pushes agents toward a less safe write path.

## 2. Technical Specification / Findings

Observed in session `21fc03f9-e6cb-4496-a4d5-c318cd88d005` (goself repo, 2026-11-17):

- The native `Read` of `/home/alfred/.claude/skills/focus/SKILL.md` was refused with `harnez guard: this session already read this file natively, so a second native Read of it is denied`. The file was read natively in an earlier context window, before a `/compact`, and its content was not in the current window. The refusal named `harnez read --` as the alternative.
- A native `Read` of `issues/028-report-systemd-unit-state-and-configured-binary-in-serverstatus.md` was refused in the same way. That file had been read through `harnez read` in the current window. Whether it had been read natively earlier was not checked.
- The `Edit` tool requires a native `Read` of the file first. With that `Read` refused, `Edit` cannot run on the file, and the agent appended to the ticket through Bash instead.

Not verified: how the guard records "read natively" (per session, per file, or per window), and whether `harnez read` counts as a native read for it.

## 3. Implementation & Verification Plan

Expected: the guard should refuse a native `Read` only when the file's content is still in the current context window. After compaction, or when the agent has only seen the file through `harnez read`, the native `Read` should be allowed, or the refusal should name a route that also permits `Edit`.

Design (owner, 2026-11-17):
- Clear the session's read markers in a Claude `PreCompact` hook (`harnez hook compact-reset`, installed from `spec/claude-compaction.yaml`). Markers are one directory per session, so one `RemoveAll` clears them.
- Deny a repeat only when that PreCompact hook is installed in Claude's `settings.json`. Otherwise the repeat is allowed with a warning: `File already read in this session: <path>. Please avoid re-reading it; its content may no longer be in context.` The warning goes in `additionalContext` and `systemMessage`.
- The repeat guard is Claude-only: the agy tool hook checks only size, and the Codex hooks only see Bash. Codex has `PreCompact` and agy has no compaction hook (`docs/AutoCompaction.md`, `docs/JevCompaction.md`), so neither needs a repeat guard.

Verify: read a file natively, run `/compact`, then `Read` the same file and then `Edit` it. Both should succeed, or the refusal should name a working edit path.

/goal Make the guard match what is in the current context, and keep `Edit` usable after a `harnez read`, or stop and report when blocked on a user decision or denied permission.
