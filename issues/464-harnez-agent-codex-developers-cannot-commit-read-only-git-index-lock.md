# 464 — harnez agent: codex developers cannot commit, read-only .git/index.lock

**Status**: Open — filed from loom session
**Priority**: P2
**Severity**: Moderate
**Category**: Bug
**Related**: 288 (/harnez-agent skill), 108

## Goal

A developer started with `harnez agent start codex:luna:low|codex:sol:low` can run `git add`
and `git commit` in the workspace like any developer, so the lean-sprint rule "developer commits
at each milestone" holds without the host committing on its behalf.

## Observed

In the loom repo (2026-09-20/21, 8 sprints, luna:low and sol:low) every developer commit failed
because `.git/index.lock` was not creatable or was read-only inside the agent's sandbox. The host
staged and committed after each review. Haiku native subagents did not hit this.

## Discovery

- Reproduce with a minimal `harnez agent start codex:luna:low` task that commits.
- Find the cause: codex sandbox write scope (`.git` excluded?), a stale lock, or a harnez wrapper
  mount. Decide the fix in harnez (sandbox config, writable `.git` path, or an approved
  commit helper), not in each project.
- Add a regression probe (canary) that starts an agent, commits a trivial file, and checks the log.

## Related annoyance

Developers also left built binaries in the repo root; consider a hint or `git status` check in
the agent prompt or handoff.
