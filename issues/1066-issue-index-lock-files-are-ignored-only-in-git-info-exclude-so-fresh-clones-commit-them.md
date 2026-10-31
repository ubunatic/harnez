# 1066 — Issue-index lock files are ignored only in .git/info/exclude, so fresh clones commit them

**Status**: Open
**Priority**: P2 (Medium)
**Severity**: Moderate
**Category**: Bug
**Related**: https://codeberg.org/ubunatic/harnez/issues/41, 672, 703, 719, 1064

---

## 1. Problem & Incident (Imported from Codeberg #41)

*Source: Reported by `flashcoder` on Codeberg (2026-10-31)*

`harnez init` ignores `/issues/README.md.lock` and `/issues/.reserve.lock` only in `.git/info/exclude` (`internal/claude/init_agent_file.go`, `initGitIntegration`, issue 672). That file is local to the clone and is not tracked or copied by `git clone`.

**Incident on 2026-10-31**:
A Forgewerk job ran `harnez issues` in a fresh clone of `flashcoder/forgewerk`, where `harnez init` had never been run. The job's report commit staged untracked files, and `issues/README.md.lock` was committed into Pull Request #13 (forgewerk ticket 024; worked around there with a tracked `.gitignore` entry).

Any automated tool, CI pipeline, or secondary developer environment running in a fresh clone encounters this issue whenever `harnez issues` executes.

## 2. Technical Specification & Solutions

Two complementary remediation paths:

1. **Tracked `.gitignore` Addition in `harnez init`**:
   `harnez init` should add `/issues/README.md.lock` and `/issues/.reserve.lock` directly to the tracked `.gitignore` file (matching Harnez's own repository configuration), rather than relying solely on the clone-local `.git/info/exclude`.
2. **Process Teardown Cleanup in `harnez issues`**:
   When `harnez issues` (e.g. `new`, `open`, `close`, `index`) finishes its execution, it should clean up ephemeral lock files if they are empty and unheld, leaving no dirty sidecars behind in uninitialized repositories.

## 3. Goal & Acceptance Plan

/goal Ensure issue-index lock files are never left untracked or committed in fresh clones by adding them to tracked .gitignore during init and ensuring issue commands clean up unheld lock files upon exit, verified with fresh clone tests.

1. Update `internal/claude/init.go` to ensure `.gitignore` contains the lockfile ignore patterns.
2. Update lock management in `internal/issues` to safely remove zero-byte lock files on graceful command exit.
3. Test in a fresh clone without `.git/info/exclude` to verify `harnez issues new` leaves a clean `git status`.
