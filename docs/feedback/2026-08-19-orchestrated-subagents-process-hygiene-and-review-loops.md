# Agentic Retrospective: Parallel Advisors, Sequential Dev Orchestration, and Subagent Hygiene

**Date:** 2026-08-19  
**Category:** Agentic Coding Patterns & Harness Evolution  
**Related:** [Issue 003](../../issues/003-mergedocs-dedup-bug.md), [Issue 004](../../issues/004-diff-exit-code-swallowed.md), [Issue 011](../../issues/011-autodetect-nondeterministic-order.md), [AGENTS.md](../../AGENTS.md)

---

## 1. Executive Summary

This session executed a multi-ticket maintenance sprint on the `harnez` CLI (Issues 003, 004, and 011) and established three operational patterns in `AGENTS.md` and `docs/lang/Git.md`:
1. **Parallel Advisor Audits vs. Sequential Dev Execution**: Parallel read-only subagents efficiently audited and refined tickets without risk, while implementation tasks ran strictly in sequence to eliminate code churn and race conditions.
2. **Background Process & Zombie Hygiene**: Identified a widespread agentic failure mode where long-running watch tasks, timer schedules, and spawned test subprocesses are abandoned in the background.
3. **Log Browsing Traps**: Identified an LLM trap where agents exhaustively browse historical JSONL logs and transcripts instead of formulating hypotheses or asking for guidance.
4. **Pre-Commit Review Gaps**: Formalized a review gate requiring non-trivial changes to undergo a review pass (via host orchestration or a fresh subagent) before committing.

---

## 2. Agentic Workflow Learnings

### 2.1 Parallel Discovery vs. Sequential Execution
- **Parallel Advisors**: Launching 3 subagents concurrently to review Issues 003, 004, and 011 completed in under 45 seconds. The advisors discovered that Issue 011 was already solved and tested in code (`444c29f`), while pinpointing the exact missing lines in Issues 003 and 004.
- **Sequential Implementation**: When development agents modified shared files (`internal/claude/apply.go`, `internal/claude/docs_test.go`, `issues/README.md`), executing sequentially ensured:
  - No merge conflicts across file edits.
  - Test suites (`go test ./...`) remained green at every step.
  - Clean attribution of changes per issue.

### 2.2 Background Process & Zombie Management
LLM agents frequently start background jobs (e.g. `schedule`, `run_command` with async waits, file watchers, or test servers) and fail to terminate them when tasks finish or fail:
- **Impact**: Lingering watch commands and orphan subagents consume memory, lock file descriptors, and trigger false wakeups.
- **Fix**: Codified explicit hygiene rules in [`AGENTS.md`](../../AGENTS.md) requiring agents to audit `manage_task` and `manage_subagents` and kill all idle/completed tasks before ending turns.

### 2.3 The "Log Deep-Seek" Trap
When agents encounter errors or explore agent histories, they often ingest massive raw JSONL transcript files into their context window:
- **Impact**: Massive token consumption, context pollution, and loss of high-level problem-solving focus.
- **Rule Added**: If an issue is not diagnosable after 1–2 targeted `grep` or `tail` checks, agents must stop reading logs, reason from first principles, or ask the user for direction.

### 2.4 Review Before Commit
Agents acting autonomously often commit immediately after tests pass without verifying if:
- The tests are rigorous (or merely passing due to weak assertions).
- Associated issue tickets and evergreen docs are in sync.
- The code is readable and maintainable for future agent sessions.
- **Rule Added**: Established a rule in [`AGENTS.md`](../../AGENTS.md) and [`docs/lang/Git.md`](../lang/Git.md) requiring non-trivial changes to undergo an explicit review pass before commit.

---

## 3. Issues Closed in this Session

| Issue | Title | Solution & Verification |
|:---|:---|:---|
| **003** | `mergeDocs` deduplication bug | Delegated `mergeDocs` to `jsonc.UnionStrings` and enhanced `UnionStrings` to deduplicate both slices. Added `TestMergeDocs` in `docs_test.go` and `jsonc_test.go`. |
| **004** | `diff` exit code 2 silently swallowed | Wrapped `cmd.Run()` with `errors.As(err, &exitErr)` to separate exit code 1 (files differ) from fatal exec failures (exit code ≥ 2). Added unit tests in `markdown_test.go` and `integration_test.go`. |
| **011** | `autoDetectDocs` non-deterministic order | Verified deterministic sequence via `docNamesInOrder` and verified 10-iteration stability test `TestDocNamesInOrder`. Marked closed. |

---

## 4. Harness Recommendations for the User

1. **Automated Subagent Cleanup Hooks**:
   - Introduce an automatic lifecycle cleanup hook in `harnez` that terminates child subagents and background timers when the parent task finishes.
2. **`harnez diff --exit-code` CLI Flag**:
   - Now that `diff` error handling is fixed (Issue 004), consider adding `--exit-code` to `harnez diff` to return exit status 1 on differences (for CI drift checking), mirroring `git diff --exit-code`.
