# 1033 — Tool-call limit per agent turn, enforced by harnez, not by the prompt

**Status**: Open
**Priority**: P2 (Medium)
**Severity**: Minor
**Category**: Feature
**Related**: 1024, 1027, 592, 241, 662

---

## Problem
A small model can loop on tool calls instead of answering. In 1024 (2026-10-29, `lmcoder:coder`, 64k window) a
review of a 98-line diff made 73 tool calls with an open prompt and 80 with a prompt that said "at most 4 more
tool calls" (51 `grep`, 30 of them repo-wide). Both runs ended at the 10-minute cap without an answer. A limit in
the prompt is not followed, so it must be enforced by the harness.

## Ask
A per-turn tool-call limit for `harnez agent start`/`resume`: when a turn reaches the limit, harnez stops further
tool calls and asks the model to answer now with what it has; if it still does not answer, the turn ends with a
clear "tool-call limit reached" status, not a bare timeout.

## Open questions
- Where it can be enforced per provider: a Pi extension (`tool_call` can block calls, 1023 findings), Claude and
  Codex hooks or settings, or harnez counting stream events and stopping the turn (as the 592 token watchdog does).
- Defaults per role or model (in `spec/` with a schema), and a flag to override for one run.
- How it combines with 1027 (`plain` mode limits which tools exist; this limits how many calls).

## Acceptance
- The 1024 review run on `lmcoder:coder` with a limit (e.g. 10) ends with an answer or the clear limit status
  within a few minutes, with the call count in the run output.
- Tests with a fake provider that keeps calling tools.

/goal Enforce a per-turn tool-call limit at least for Pi/lmcoder, or stop and report when a provider gives no way
to block or count calls. Circuit breaker: on a loop or no progress, clear the goal and write a failure report.
Re-check live code and Pi's extension API first; written 2026-10-29.
