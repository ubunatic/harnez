# 1284 — init: EnsureGitExclude walks into ancestor .git outside target directory

**Status**: Open
**Priority**: P2 (Medium)
**Severity**: Moderate
**Category**: Bug
**Related**: #488, #320, #1280

---

## Problem

Running `harnez init --quota-1 --variant lite` in a non-repo workspace directory (`/home/uwe/projects`, which contains `AGENTS.md` and therefore passes `ValidateInitTarget`) fails inside a Taia sandbox with:

```
Error: ignore .harnez/rules/Local.md: open /home/uwe/.git/info/exclude: permission denied
```

`internal/claude/init_agent_file.go:78` calls `fsutil.EnsureGitExclude(dir, ...)` (`internal/fsutil/fsutil.go:142-176`), which uses `FindGitDir(startDir)` and walks up parent directories past `dir` into an unrelated ancestor `/home/uwe/.git`. Under sandbox confinement that fails with `EACCES` before `.harnez/rules/` is written; on an unconfined host it silently mutates an unrelated parent/home `.git/info/exclude`.

## Goal

/goal Prevent `harnez init` (and `EnsureGitExclude` when initializing a directory) from modifying or failing on an ancestor `.git` directory outside the target repository, with regression tests covering both an unreadable/stray ancestor `.git` and legitimate repository/worktree layouts; or stop and report when blocked on a user decision or denied permission. On a loop, hard architectural or OS blocker, or no progress, clear the goal and record a failure report with resume point.

Check live code and recent history first before implementing (`internal/fsutil/fsutil.go`, `internal/claude/init_agent_file.go`, and callers of `EnsureGitExclude`).
