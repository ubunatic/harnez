# 125 — Long-running agent runs should use async feedback instead of chat polling

**Status**: Open
**Priority**: P2 (Medium)
**Severity**: Moderate
**Category**: Agentic Ergonomics
**Related**: [055](055-no-long-sleep-use-scheduled-wakeups.md), [056](056-agenticloop-buffered-long-running-output-antipattern.md), [122](122-agent-instruction-tool-feedback-protocol.md)

---

## 1. Problem & Motivation

Found in a downstream `lmcoder` benchmark session on 2026-08-31: a long-running
Pi agent canary was launched with a 40 minute budget, and the host agent polled
the attached tool session every 30 seconds while it produced no intermediate
output. The user correctly called this out as wasteful and weird.

This is the same family of failure as [055](055-no-long-sleep-use-scheduled-wakeups.md)
and [056](056-agenticloop-buffered-long-running-output-antipattern.md), but the
specific bad habit is different: the agent stayed attached to a long job and
used repeated empty polls as a substitute for an asynchronous completion signal
or durable job record.

Effects:

- Wastes chat/context budget on no-op status checks.
- Produces noisy user-facing updates with no new information.
- Encourages agents to keep the foreground turn occupied for work that should be
  supervised externally.
- Makes the interface look broken even when the underlying task is running
  normally.

## 2. Technical Specification / Findings

Agent instructions and/or harness behavior should teach the following rule for
long-running agent tasks, benchmark jobs, CI runs, deploys, and remote jobs:

1. Prefer async feedback methods when available: harness-tracked background
   tasks, job logs, benchmark history files, result artifacts, completion
   notifications, or scheduled wakeups.
2. If the interface has no real completion callback, start the long-running work
   in a durable/detached form that writes a log/result file, then return control
   to the user with the job id, log path, and expected budget.
3. Poll only a few times when polling is unavoidable, and use intervals sized to
   the job's expected duration. For a tens-of-minutes task, repeated 30 second
   chat-visible polling is too frequent unless the process is known to emit
   useful progress.
4. User-facing progress updates should be eventful: process started, first
   output observed, status file changed, process exited, artifact written,
   timeout reached, or cleanup performed.
5. Before finishing, check for and report any lingering background process so the
   zero-zombie invariant still holds.

## 3. Implementation & Verification Plan

1. Update `docs/practices/AgenticLoop.md` with an anti-pattern covering
   chat-visible empty polling of long-running tool sessions.
2. Cross-link it with the existing no-long-sleep and buffered-output
   anti-patterns.
3. If there is a command/template that launches agent benchmark jobs, adjust its
   recommended invocation to produce durable logs/history and avoid foreground
   polling.
4. Verify with `harnez status` or the issue tracker linter, and run the relevant
   doc tests if available.
