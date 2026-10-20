# 836 — Stop asking to push other repos mid-session; check unpushed repos in wrap

**Status**: Open
**Priority**: P2 (Medium)
**Severity**: Minor
**Category**: Feature
**Related**: `docs/commands/Wrap.md`, `docs/Git.md` ("don't push unless asked"), issues 834, 835

---

## 1. Goal

/goal Agents no longer end replies with "say push <repo>" for commits in other repos; instead the wrap skill lists every repo touched in the session with unpushed commits and pushes them (or reports why not) as one checklist step; or stop and report when this conflicts with the "don't push unless asked" rule and needs a user decision.

## 2. Findings (user, 2026-10-20)

The user finds it very annoying that Claude keeps asking to push commits in other repos. Example: after filing harnez issues 834/835 from an ubunatic.com session, the reply ended with "say push harnez", because another session's agent was working in harnez.

Cause: `docs/Git.md` says "don't push unless asked", so every cross-repo commit turns into a question.

## 3. Plan

1. Agent guidance (`docs/Git.md` or the managed AGENTS.md section): commits in other repos are left unpushed without asking. At most one line names them, and only in the final report.
2. `docs/commands/Wrap.md`: add a step that lists repos touched this session with commits ahead of their upstream. The step pushes them, unless another session is active in that repo or the push fails; then it reports which repos are left.
3. Reinstall the skills and verify the installed copies.
