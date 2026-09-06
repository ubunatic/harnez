# 125 — Long-running agent runs should use async feedback instead of chat polling

**Status**: Closed — resolved in 22503de (Chat-Visible Empty Polling anti-pattern added to AgenticLoop.md §6)
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

---

## 4. Implementation Plan

Pure documentation change in one file, plus a propagation step. No Go code.

### Step 1 — Add the anti-pattern

`docs/practices/AgenticLoop.md` §6 "Anti-Patterns to Avoid" (the `- ❌ **Name**: …`
list, currently ending at the "Blocking `sleep` Waits" bullet around line 258).
Add one new bullet immediately after that one, so the two related failure modes
read together:

- ❌ **Chat-Visible Empty Polling** — staying attached to a long-running job
  (benchmark run, canary, CI, deploy, remote agent) and re-checking it on a
  fixed short interval while it produces no new output. Distinct from the
  blocking-`sleep` anti-pattern above: the agent *is* yielding between checks,
  but each check spends context and user attention to report "still running."

Body of the bullet should carry §2's rules 1–4 compressed to prose: prefer a
real completion signal (harness-tracked background task, notification, scheduled
wakeup); if none exists, launch the work detached so it writes a log/result file
and hand the user the job id, log path, and expected budget; when polling is
genuinely unavoidable, size the interval to the job's expected duration (a
40-minute job does not get 30-second polls) and only surface *events* — started,
first output, status file changed, exited, artifact written, timeout, cleanup —
not heartbeats. Rule 5 (report lingering background processes before finishing)
is already covered by Invariant 3 / the "Orphaned Background Tasks" bullet — link
to it rather than restating it.

Match the existing bullets' style: one bold name, then prose that states both the
rule *and why*, no sub-bullets (the surrounding list has none).

### Step 2 — Cross-link

- In the new bullet, name the "Blocking `sleep` Waits" bullet explicitly as the
  sibling case ("that one is about not yielding; this one is about yielding but
  still reporting nothing").
- [[056]] is still Open and adds a *third* bullet to the same list (buffered
  long-running output). Land them in either order but check the other's wording
  first — three near-adjacent bullets about long-running work must not restate
  each other, which is exactly the duplication [[128]] is auditing for. If [[056]]
  is still open when this lands, note in it that the list now has a
  long-running-work cluster.

### Step 3 — Propagate

`docs/practices/AgenticLoop.md` is a bundled copyable doc: run `harnez apply`
(installs to `~/.claude/docs/AgenticLoop.md`) and `harnez index` if the doc index
changes. Then `go test ./...` — `internal/claude/init_test.go` and
`integration_test.go` assert the doc's *presence and reference line*, not its
body, so a content-only edit should not require test changes; if a test does
break, that is a signal the edit touched a managed section header, not prose.

### Step 4 — §2 rule 3 of this ticket ("adjust benchmark-job launch templates")

Nothing in this repo launches agent benchmark jobs — the originating incident was
in downstream `lmcoder`. Drop that sub-item here rather than inventing a harnez
command for it; the doc bullet is the deliverable, and downstream repos pick it
up via `harnez init --docs`/`apply`.

### Risks / open questions

- Anti-pattern-list bloat: this list is already 12+ bullets and is itself a
  candidate for [[128]]'s trimming pass. Keep the new bullet to ~4 sentences.
- Advisory-only enforcement: a doc bullet does not stop the behaviour (same limit
  ConciseMode hit, see [[134]]). Acceptable here — no mechanical signal exists for
  "the agent polled too often," and inventing one is not worth it.

### Scope: **small** (one doc bullet, one cross-link, `apply` + tests).
