# 661 — harnez agent stop reports success but the agent process keeps running

**Status**: Open
**Priority**: P2 (Medium)
**Severity**: Minor
**Category**: Bug
**Related**:

---

## 1. Problem & Motivation
Describe the problem and why it matters.

## 2. Technical Specification / Findings
Record relevant technical details and findings.

## 3. Implementation & Verification Plan
Describe the implementation and how it will be verified.

Status: open · Priority: P1 · Severity: high · Category: bug

## Problem
2026-09-30, host Claude Code, dev `dev-order` (codex:gpt-6.1-sol:med) running a turn from a
host background job (`harnez agent resume ... --stream stats`). `harnez agent stop --name
dev-order` exited without error, yet the Codex process kept running. The next
`harnez agent resume` failed with `thread-store conflict: thread 01a0f3ee-... already has an
active writer`. Only killing the host's background job (Claude `TaskStop`) freed the thread.
So `stop` is unsafe: the host believes the agent stopped, and a follow-up resume or delete
then races a live writer.

Related: 538 (leftover processes at turn end), 630 (backgrounding guides), 656 (agent send).

## Goal
/goal `harnez agent stop` is safe and truthful:
- it terminates the whole agent process tree (provider CLI and children), not just a
  wrapper, with SIGTERM then a bounded SIGKILL escalation;
- it confirms the exit before reporting, and prints one of:
  "stopped: <name>, no process left, safe to resume/delete" or
  "stop requested: <pid> still exiting; run `harnez agent wait --name <name>`" (non-zero exit);
- it tells the host that the native background task/job running the turn may take a few
  seconds to report exit, and that this is expected, not a failure;
- resume refuses early with a clear message while a writer is still alive.
Verify live with a Codex dev and a Claude dev started from a host background job; add tests
for the process-tree kill and the reporting. Stop and report when blocked on a user decision.
