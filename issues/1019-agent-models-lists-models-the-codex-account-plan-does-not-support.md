# 1019 — agent models lists models the Codex account plan does not support

**Status**: Open
**Priority**: P2 (Medium)
**Severity**: Minor
**Category**: Bug
**Related**: 943, 974, 448

---

/goal `harnez agent models` stops reporting a Codex model as "available" when the logged-in account's plan cannot use it, and a start failure for that reason is shown in full, locked by tests. Stop and report when blocked on a user decision or denied permission. Circuit breaker: on a loop, a hard blocker or no progress, clear the goal and write a failure report here.

## Problem

Reported from another session on the free OpenAI plan, where only Luna 6 and the older Terra 5.6 are usable. `harnez agent start --model codex:gpt-6.1-terra` failed at once with "The 'gpt-6.1-terra' model is not supported when using Codex with a…" (the message was cut off). The session never ran a turn, so it could not be rated. Yet `harnez agent models` listed terra as "available".

So the failure is already visible when the model runs, but the models command does not reflect it.

## Findings so far

- 943 found that codex needs a probe call to learn access, and that `codex models` needs a TTY. The list is static or probe-less, so it cannot see plan limits.
- The error text is truncated, which hides the reason (plan or account type).
- Unverified: which error string or code Codex returns for plan-limited models, and whether a cheap probe exists.

## Want

- Models that failed with a plan/account "not supported" error are marked unavailable (or "unsupported for this account") in `agent models`, at least for the cached session.
- The start error is not truncated.
- Record the error shape in the 943 study if it differs from `model_not_found`.
