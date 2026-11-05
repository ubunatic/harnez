# 1114 — Jules wait: say a session stalled after its replies instead of offering another wait

**Status**: Closed — report stalled session when waiting for feedback with unanswered replies, omit track-it-again advice, document in docs/commands/jules.md
**Closed-By**: session 74ca1fb9-acc8-489e-8717-30157fb7dbed
**Priority**: P2 (Medium)
**Severity**: Minor
**Category**: Feature
**Related**: 1111, 1113, docs/studies/JulesPeerFeedback.md

---

## Problem
Reported 2026-11-03 by lmcoder for run lmcoder-jules-impl-130 (Jules 9816136848740404229). Jules asked a question at 12:13 and harnez auto-replied at 12:14 (1113). lmcoder then answered the question itself at 12:39. After two 25-minute waits the session was still AWAITING_USER_FEEDBACK, and every wait ended with "message sent, no reply yet" plus "Next: track it again, up to 25m0s more". That advice only costs more time: the run never moved again and produced no PR.

The result comes from `julesStillRunning` in `internal/subagent/jules.go`. The activity log already shows the pattern: the newest agent activity is older than one or more user messages.

## Goal
/goal When a Jules session waits for feedback and has had user replies that got no agent reaction within a wait, the wait result says it stalled ("stalled: N replies since Jules's last message, no reaction") and suggests stopping or restarting smaller instead of another wait. Stop and report if the activity log cannot show this reliably.

## Done when
- A test with a fake session (question, then two user messages, no agent activity) shows the stalled wording and no "track it again" line.
- A session that answers after a reply keeps today's behaviour.
- `docs/commands/jules.md` says what to do with a stalled run.

## Notes
- The exit code question belongs to 1111; this ticket only changes the text.
- Same day, run 112 ended "Jules was unable to complete the task" after about 75 min without asking. Both failures were larger tickets.
