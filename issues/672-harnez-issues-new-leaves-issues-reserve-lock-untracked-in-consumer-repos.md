# 672 — harnez issues new leaves issues/.reserve.lock untracked in consumer repos

**Status**: Draft
**Priority**: P3 (Low)
**Severity**: Minor
**Category**: Bug
**Related**: psync 017

---

## 1. Problem & Motivation
`harnez issues new` creates the empty flock file `issues/.reserve.lock` and keeps it (deleting a
lock file under flock is racy). In repos without a `*.lock` gitignore rule, such as psync, it shows
up as untracked after every new ticket.

## 2. Technical Specification / Findings
`harnez init` already adds `/issues/README.md.lock` to `.git/info/exclude` when `issues/` exists.

## 3. Implementation & Verification Plan
- Exclude `/issues/.reserve.lock` the same way; no project `.gitignore` changes.
- Test: `TestRunInit_IgnoresIssuesReadmeLockWithoutChangingGitignore` checks both locks are ignored
  and each exclude line appears once after two runs.
- Verify: re-run init in psync; `git status` no longer lists the lock.
