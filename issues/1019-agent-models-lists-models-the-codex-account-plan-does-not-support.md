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

## Audit 2026-11-03

Jules audit (PR GH#71, report `reports/jules/1019-2.md`, checked against b0dcb501, all references held): partly done. The error shape is recorded in `docs/studies/2026-10-25-canary-model-access-discovery.md:15`. Still open: `candidateUnavailable` (`internal/subagent/driver.go:523`) has no "unsupported for this account" case, and `shortCause` (`cmd/harnez/agent_run.go:279`) truncates start errors to one line of 180 runes.

## Jules runs

| kind | session name | Jules ID | PR | state |
| --- | --- | --- | --- | --- |
| audit | harnez-jules-audit-1019 | 15569656316218008222 | none | failed 2026-11-08: stuck IN_PROGRESS finalizing report, stopped |

## Searcher findings (s1019, codex:luna:low)

- `candidateUnavailable` `internal/subagent/driver.go:562`: disabled, latest access `denied`, or exhausted quota;
  access observations read by `latestAccess` :575 (model catalog). `agent models` availability:
  `cmd/harnez/agent_cmds.go:183-184` via `CachedProviderQuotaAvailabilities`.
- Error shape (study `docs/studies/2026-10-25-canary-model-access-discovery.md:15`): HTTP 400 `model_not_found`,
  "The '<model>' model is not supported when using Codex with a ChatGPT account." No parser or fixture exists.
- `shortCause` `cmd/harnez/agent_run.go:300`: first line, 180 runes; only caller is start-error reporting.
- Tests: `internal/subagent/fallback_test.go:68`, `internal/agent/modelrows_test.go:25`,
  `cmd/harnez/agent_test.go:256`, `cmd/harnez/agent_resume_test.go:920`.

## Decision

- 2026-11-10 host (decide mode): one milestone. When a codex start/resume fails with the "not supported when
  using Codex" text (or `model_not_found`), record a model-catalog access observation with a new status
  `unsupported` (account-specific, same store as `denied`). `candidateUnavailable` treats it as unavailable;
  `agent models` shows "unsupported for this account" for that model. A later successful turn on that model
  records normal access and clears it (latest observation wins). Start errors print the full provider
  message (no 180-rune cut; keep a first-line summary only where a one-line status is required). Tests use
  a fake codex emitting the study's exact text; never the network.

## M1 delivered: unsupported access state and untruncated errors (8440796b)

Developer: `make test-q1` green, no `--- FAIL`. Host diff review plus reviewer r1019 (codex:terra:low).

## M2 Pre-Work (host + reviewer r1019)

1. P2: narrow the match. Mark `unsupported` only for the study phrase "not supported when using Codex", or
   `model_not_found` together with the model id in the same message. Test a message with `model_not_found`
   but another model id: not marked.
2. P2: bounded state. Write an observation only when the status changes from the latest one for that model
   (a success after success writes nothing). Test: two successes leave the file unchanged.
3. P2, host decision: show the first non-empty provider line in full (no rune cap), drop later raw lines
   (tracebacks, generated source). Restore the resume test's "no raw output" assertion and add a check that a
   220-char first line is not cut.
4. P2: end-to-end tests through cobra with a fake codex on PATH: a start failing with the study text, then
   `agent models` table shows "unsupported for this account" and JSON shows the new values; a later
   successful start clears it.
5. P3: isolate `internal/agent/modelrows_test.go` from the host's state (temp HOME/XDG_STATE_HOME).
6. P3: use the existing `internal/xdgpath` helper for the state dir instead of reading XDG_STATE_HOME by hand.
