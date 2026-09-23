# 508 — Reject harnez agent calls when provider quota is confirmed exhausted

**Status**: Open
**Priority**: P2 (Medium)
**Severity**: Minor
**Category**: Agentic Ergonomics
**Related**: [[507-capture-subscription-quota-snapshots-at-harnez-agent-start-resume]] (shares the pre-call quota reading), [[485-autodetect-the-default-agent-model-from-5h-and-weekly-usage-limits]] (autoselect a model from quota; this ticket only gates), [[087-generalize-flock-freshness-gate-to-codex-agy]] (shared live-fetch cache), [[112-agy-usage-poll-may-trigger-google-reauth-bot-detection]], `internal/usage`, `cmd/harnez/agent.go`

## Problem

`harnez agent start|resume` calls the provider even when its 5h or weekly
quota is already used up. The provider then fails late: after session
setup, sometimes after a partial turn, and with provider-specific error
text that orchestrators must parse and recover from.

## /goal

Before calling a model, `harnez agent start|resume` checks the provider's
quota data and rejects the call with a clear error (provider, window,
reset time) only if the quota is **confirmed** exhausted. In every other
case (quota available, data stale, unreadable or missing, or the check
timing out) it starts the model normally and lets the provider error out
if quota is in fact gone.

- If the cached reading is stale, try to refresh it through the
  collector/shared live-fetch cache. The whole check takes at most 1–2s
  and never adds fresh API pressure beyond what the cache interval allows
  (same constraint as 507).
- "Confirmed" means a fresh reading that shows the window exhausted and
  its reset time still in the future. A stale "exhausted" reading whose
  reset time has passed is not confirmation.
- The rejection is distinguishable (exit code or error kind), so
  orchestrators can switch model instead of retrying.

## Open questions (verify against live code before starting)

- The freshness threshold that counts as "confirmed".
- Whether an explicit override flag is needed. The default is fail-open,
  so it may be unnecessary.
- Share the pre-call reading with 507's start snapshot so it is fetched
  once per turn.

## Done when

- Tests cover: confirmed exhausted → rejected before any provider call;
  stale, unknown or timed-out data → the provider is called; an expired
  reset time → the provider is called; the check stays within the time bound.
