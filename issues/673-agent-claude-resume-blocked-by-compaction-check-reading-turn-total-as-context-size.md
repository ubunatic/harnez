# 673 — agent: claude resume blocked by compaction check reading turn total as context size

**Status**: Open
**Priority**: P1 (High) — breaks the lean-sprint resume loop for every Claude developer
**Severity**: Major
**Category**: Bug
**Related**: [[594-agent-codex-compaction-never-acknowledged-over-limit-resume-always-fails]], [[644-agent-agy-compaction-never-acknowledged-resume-always-fails]], [[656-harnez-agent-send-opt-in-messaging-to-detached-agents-after-host-session-agent-tracking]]

---

## 1. Problem & Motivation

`harnez agent resume` of a Claude worker after one long turn always fails:

```
Error: refusing to send resume prompt: compaction completed without a verified context drop (before 441684, after 0 full input tokens)
```

Seen twice in the 656 sprint on 2026-10-01 (`dev-656-sonnet`, before 849070; `dev-656-fix`,
before 441684), both `claude:sonnet` with exactly one turn. The work was lost to the loop: a new
developer had to be started to finish each step.

## 2. Technical Specification / Findings

- `harnez agent status --name dev-656-fix` shows `Tokens: 449682 (turn 449682)`. That's the
  cumulative input over every tool step of one turn, not the context size. Sonnet's context can't
  hold 449k, so the "over threshold, compact first" decision is wrong. The same class as 644 (agy)
  and 594 (codex).
- After `/compact`, `VerifyCompaction` (`internal/subagent/compact.go`) gets `ContextTokens == 0`
  for Claude and refuses. Either Claude's compact turn reports no usage, or the parser drops it.

## 3. Implementation & Verification Plan

- Canary first: resume a Claude session with a long multi-step turn, record the raw usage of the
  last API call vs. the turn total, and of the `/compact` turn.
- Use the last call's input (input + cache read + cache creation) as the context size for Claude;
  mark it unknown (negative, which skips compaction) when it isn't available, as 644 did for agy.
- Verify: resume a Claude worker twice after a 400k+ cumulative turn; both resumes are delivered.
