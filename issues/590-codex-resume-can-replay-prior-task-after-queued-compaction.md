# 590 — Codex resume can replay prior task after queued compaction

**Status**: Open
**Priority**: P1 (High)
**Severity**: Major
**Category**: Bug
**Related**: `cmd/harnez/agent_run.go`, `internal/subagent/codex.go`

---

## 1. Problem & Motivation
When `harnez agent resume` crosses the automatic compaction threshold, a Codex worker can repeat its previous task and report that no work is needed instead of handling the new prompt. In the neus report, the requested new work was lost in practice, making resume unsafe for handing off a new task.

## 2. Technical Specification / Findings
`cmd/harnez/agent_run.go:321-328` calls `driver.Compact` when the threshold is reached, then immediately proceeds to `Resume`/`ResumeStream` with `req.Prompt` at lines 347 or 356. `internal/subagent/codex.go:97-101` implements Codex compaction with `codex queue --thread <id> --message /compact` and returns as soon as the command queues it; it does not wait for the compaction turn to finish. Thus Harnez queues compaction and submits the new resume prompt without a completion barrier. The log's `[compaction ack: 6s]` followed by the prior task is consistent with an ordering/handoff race, although it cannot establish whether Codex dropped the prompt after accepting it. Harnez is the likely source of the unsafe ordering; provider behavior remains a hypothesis.

Targeted log excerpt (from `/tmp/claude-1000/-home-uwe-projects-search/9d24bd58-d824-420a-87e7-6783ed7ab/tasks/bxzh13byt.output`):

```text
[session info: id=01a0d96b-bc1e-7e22-a09b-0590574c26f9 agent=codex:gpt-6-luna action=resume resolved=name]
[wait: synchronous turn, wait for [done]; no polling, no re-sending the prompt]
[compact: queued /compact at 2.2M new tokens since the last compaction; the agent acknowledges it before its reply]
[compaction ack: 6s]
Ready for the next task.
[warning: no confirmation after 10s; caller may stop the agent: harnez agent stop neus-dev]
[confirmation: 11s]
I’ll inspect the Go chunking/index path and ticket 013, add natural-language retrieval context to embedded Go declaration text while leaving display snippets unchanged, then build and evaluate a scratch index under `/tmp` using the x600 Nomic tunnel only. I’ll run `make test-q1` within the three-run limit, install, record comparative results, and commit.
The requested fix is already present in commit `7f9c16e`: eval loads saved embedding settings, rejects model mismatches, and applies saved prefixes.
No new commit was needed; the working tree is clean.
```

## 3. Implementation & Verification Plan
Make Codex compaction a completed, observable turn before sending the caller's new prompt, or otherwise re-inject the prompt after compaction is confirmed. If the driver cannot verify that ordering, fail loudly instead of reporting a successful resume. Add a regression test that asserts the compaction completion barrier precedes prompt submission, then verify with the existing Codex driver and resume tests.

## 4. Implementation Record
Codex compaction now runs as its own `codex exec resume ... /compact` turn and waits for the JSONL `turn.completed` event, with a two-minute timeout. If the stream ends without completion or the timeout expires, resume fails before submitting the caller's prompt. Regression tests use a fake Codex event stream to check ordering and missing completion.
