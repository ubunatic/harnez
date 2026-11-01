# 1094 — Jules resume returns the old question instead of waiting for the reply

**Status**: Open
**Priority**: P2 (Medium)
**Severity**: Minor
**Category**: Bug
**Related**: 1090, 1092, 1088

---

/goal After `harnez agent resume --model jules ... "<text>"`, harnez waits until Jules reacts to that message (a newer agent activity or a state change) and prints that reaction, never the question that was already there. Stop and report if blocked on a user decision or a denied permission. Circuit breaker: on a loop, a hard blocker or no progress, clear the goal and write a failure report here.

## 1. Problem & Motivation

On 2026-11-01, `harnez agent resume --model jules --id 5832791262383075829 "Confirmed: option 2. ..."` delivered the message (a user activity at 21:19:03Z) but returned after 9 s and reprinted Jules's 18:59 question as if it were the answer. The host then believed Jules had asked again. Jules did not react to that message for over 40 minutes; a later owner message, "submit a draft", made it act. So resume must wait for a real reaction, and a quiet session must read as "still waiting", not as a new question.

## 2. Technical Specification / Findings

- `JulesDriver.Resume` (`internal/subagent/jules.go:556`) passes the pre-send `session.UpdateTime` to `track` as `staleUpdate`.
- `track` (`jules.go:614`) accepts any session whose `updateTime` differs. The send itself bumps `updateTime`, so the still-`AWAITING_USER_FEEDBACK` state passes at once, and `waitingDetail` prints the newest agent message, which is the old question.
- Fix direction: after a send, record the send time, and count the session as answered only on a state change or an agent activity (`agentMessaged`, `planGenerated`, progress, PR output) created after it. Until then keep polling; when the wait runs out, report "message sent, no reply yet" with the resume `wait` hint.

## 3. Implementation & Verification Plan

- Reproduction test first with the fake Jules client in `internal/subagent/jules_test.go`: the session stays `AWAITING_USER_FEEDBACK`, `updateTime` moves after the send, and only an old agent message exists. Today resume returns that old message; after the fix it keeps waiting and times out with "no reply yet".
- Second test: a new agent message after the send is returned.
- `approve` path: same rule (a plan approval must show a state change).
- Done when both tests pass, `make test-q1` has no `--- FAIL`, and `/jules` skill text (`docs/commands/jules.md`) needs no change or is updated.
