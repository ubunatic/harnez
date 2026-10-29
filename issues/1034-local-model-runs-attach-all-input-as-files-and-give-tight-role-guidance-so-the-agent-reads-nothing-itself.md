# 1034 — Local-model runs: attach all input as files and give tight role guidance, so the agent reads nothing itself

**Status**: Open
**Priority**: P2 (Medium)
**Severity**: Minor
**Category**: Feature
**Related**: 1024, 1027, 1033

---

## 1. Problem & Motivation
In 1024, `lmcoder:coder` (64k window) was asked to review a 98-line diff. It made 73-80 tool calls, most of them
`grep` across the repo, and gave no answer in 10 minutes, even with a prompt that limited it to 4 more calls.
A small local model does not converge when it has to gather its own input. The owner's learning (2026-10-30):
such runs need rails. Two rails already have tickets: a harnez-enforced tool-call limit (1033) and stripped
tools (1027). This ticket covers the other two:
- **All input attached up front.** harnez puts everything the task needs (diff, the changed files or excerpts,
  the ticket) into the run as files or inline context, so the agent does not read or search anything itself.
- **Tight role guidance.** A short, fixed instruction per role for this mode: what to produce, in what shape,
  and that all input is already given.

## 2. Technical Specification / Findings
- Not checked: how `harnez agent start` passes files today (`-f` prompt file) and whether Pi accepts attached
  files or only one prompt text.
- Open: who assembles the input (the caller, or a harnez helper such as "review this diff" that collects the
  diff and changed files); how it stays inside the window (1030 shows the live window per run).
- Fits 1027: an input-only mode is naturally a `plain` mode with no read tools at all.

## 3. Implementation & Verification Plan
/goal Let a local-model run get all its input attached and a short role guidance, with no read or search tools,
and verify it on the 1024 review. Stop and report when Pi offers no way to attach input or remove the tools, or
when an owner decision on the mode's shape is needed. Circuit breaker: on a loop or no progress, clear the goal
and write a failure report. Re-check live code, 1027 and 1033 first; written 2026-10-30.
- Acceptance: the 1024 review (diff `38190e98^..f27aeebd`) on `lmcoder:coder`, with input attached and no read
  tools, gives an answer within a few minutes, with 0 tool calls reported.
- Tests with a fake provider: attached input arrives in the prompt; the run has no read or search tools.
