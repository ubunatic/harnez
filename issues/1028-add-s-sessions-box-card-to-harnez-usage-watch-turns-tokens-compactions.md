# 1028 — Add [S] Sessions box card to harnez usage --watch (turns, tokens, compactions)

**Status**: Open
**Priority**: P2 (Medium)
**Severity**: Moderate
**Category**: Feature
**Related**: 997, 049, 132

---

## 1. Problem & Motivation
While agents run (Pi on the local coder, Pi on OpenAI, Claude, agy), there is no single place to see how long a
session has run, how many turns and tokens it used, and how often it compacted. Compactions are the signal that a
context window is too small (lmcoder 263: a reviewer job compacted 13 times in 14 minutes, while the harnez session
record said `compaction_count: 0`). A `[S] Sessions` box card in `harnez usage -w` (a hotkey card like the existing
boxes) would show this for all active LLM sessions.

/goal Ship the Sessions card for all four agent families with real data per row (turns, tokens, compactions), each
milestone verified against a live session, or stop and report when blocked on a user decision or denied permission.

## 2. Technical Specification / Findings
Per active session, one row: agent, model, age, turns, tokens (used / window), compactions. Auto-hides when there is
no active session (as Mic and GPU do). Data comes from each agent's own session log, not from the harnez session
record, whose `compaction_count` is wrong for Pi (a separate bug; fix it where the source is shared).

Uncertainties to settle in each milestone, not now: where each agent's log lives, how an "active" session is
detected, and whether tokens are reported or must be estimated.

## 3. Implementation & Verification Plan
- **M1: design.** Draw the card as an `.ansi` file (follow the existing watch-card sketches) with typical, narrow and
  empty states. Ask the user to approve it before any code. Gate: explicit approval.
- **M2: Pi + lmcoder.** Read Pi session logs for sessions on the lmcoder proxy (context window from the lmcoder
  proxy `/v1/models` `lmcoder.live_n_ctx`). Verify against a running local-coder job, including its compaction count.
- **M3: Pi + OpenAI.** Same Pi collector, OpenAI models and their windows.
- **M4: Claude + Claude models.** Claude Code session data and model windows.
- **M5: agy + Gemini models.** agy session data and Gemini windows.
Each milestone: tests, `make install`, and a live check with a real session before closing.
